package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/incident"
	"github.com/ouoo-code/RegistryPulse/internal/metrics"
	"github.com/ouoo-code/RegistryPulse/internal/probe"
	"github.com/ouoo-code/RegistryPulse/internal/registry"
	"github.com/redis/go-redis/v9"
)

type ProbeFunc func(context.Context, string, time.Duration) probe.Result
type SourceProvider func(context.Context) ([]domain.Source, error)
type CredentialProvider func(context.Context, domain.Source) (domain.ResolvedCredential, bool, error)

// ResultRecorder is the persistence boundary. Implementations may write the
// probe result, status transition and incident event to PostgreSQL, a queue, or
// an in-memory store without coupling the scheduler to any one backend.
type ResultRecorder interface {
	RecordProbe(context.Context, domain.Source, probe.Result, incident.Transition) error
}

var ErrLockBusy = errors.New("scheduler lock is held by another worker")

// CronInterval supports the deployment-friendly subset used by the worker:
// @every duration, */N * * * *, 0 * * * *, 0 */N * * *, and 0 0 * * *.
// The runner still applies jitter and distributed locking around each run.
func CronInterval(expression string) (time.Duration, bool) {
	expression = strings.TrimSpace(expression)
	if strings.HasPrefix(expression, "@every ") {
		d, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(expression, "@every ")))
		return d, err == nil && d > 0
	}
	parts := strings.Fields(expression)
	if len(parts) != 5 {
		return 0, false
	}
	if parts[0] == "*/1" && parts[1] == "*" {
		return time.Minute, true
	}
	if parts[0] == "0" && parts[1] == "*" {
		return time.Hour, true
	}
	if parts[0] == "0" && strings.HasPrefix(parts[1], "*/") {
		n, err := strconv.Atoi(strings.TrimPrefix(parts[1], "*/"))
		return time.Duration(n) * time.Hour, err == nil && n > 0
	}
	if parts[0] == "0" && parts[1] == "0" {
		return 24 * time.Hour, true
	}
	return 0, false
}

// Lease is a renewable-lock ownership token. Release is ownership-safe: a
// worker cannot delete a lock acquired by a different worker after expiry.
type Lease interface{ Release(context.Context) error }
type Locker interface {
	TryAcquire(context.Context, string, time.Duration) (Lease, bool, error)
}

type Config struct {
	MaxConcurrent, ResultRetries, ProbeRetries                               int
	ProbeTimeout, Interval, ResultTimeout, RunLockTTL, SourceLockTTL, Jitter time.Duration
	IntervalProvider, RetryIntervalProvider                                  func(context.Context) time.Duration
	RunLockKey, SourceLockPrefix                                             string
	Locker                                                                   Locker
	CredentialProvider                                                       CredentialProvider
}

func (c Config) withDefaults() Config {
	if c.MaxConcurrent < 1 {
		c.MaxConcurrent = 4
	}
	if c.ResultRetries < 0 {
		c.ResultRetries = 2
	}
	if c.ProbeRetries < 0 {
		c.ProbeRetries = 0
	}
	if c.ProbeTimeout <= 0 {
		c.ProbeTimeout = 15 * time.Second
	}
	if c.Interval <= 0 {
		c.Interval = 5 * time.Minute
	}
	if c.ResultTimeout <= 0 {
		c.ResultTimeout = 10 * time.Second
	}
	if c.RunLockTTL <= 0 {
		c.RunLockTTL = c.ProbeTimeout + c.ResultTimeout + 30*time.Second
	}
	if c.SourceLockTTL <= 0 {
		c.SourceLockTTL = c.ProbeTimeout + c.ResultTimeout + 30*time.Second
	}
	if c.RunLockKey == "" {
		c.RunLockKey = "registrypulse:worker:run"
	}
	if c.SourceLockPrefix == "" {
		c.SourceLockPrefix = "registrypulse:source:"
	}
	return c
}

type Runner struct {
	cfg              Config
	sources          SourceProvider
	probe            ProbeFunc
	recorder         ResultRecorder
	machine          *incident.Machine
	metrics          *metrics.Registry
	scheduleMu       sync.Mutex
	lastNormalProbes map[string]time.Time
}

func New(cfg Config, sources SourceProvider, recorder ResultRecorder, machine *incident.Machine, m *metrics.Registry) *Runner {
	cfg = cfg.withDefaults()
	if machine == nil {
		machine = incident.New(incident.Config{})
	}
	if m == nil {
		m = metrics.New()
	}
	if sources == nil {
		sources = func(context.Context) ([]domain.Source, error) { return nil, errors.New("source provider is nil") }
	}
	if recorder == nil {
		recorder = NopRecorder{}
	}
	if cfg.Locker == nil {
		cfg.Locker = NoopLocker{}
	}
	return &Runner{cfg: cfg, sources: sources, probe: probe.Run, recorder: recorder, machine: machine, metrics: m, lastNormalProbes: make(map[string]time.Time)}
}
func (r *Runner) Metrics() *metrics.Registry { return r.metrics }
func (r *Runner) Machine() *incident.Machine { return r.machine }

func (r *Runner) RunOnce(ctx context.Context) error {
	r.metrics.LockAttempt()
	runLease, ok, err := r.cfg.Locker.TryAcquire(ctx, r.cfg.RunLockKey, r.cfg.RunLockTTL)
	if err != nil {
		r.metrics.LockError()
		return fmt.Errorf("acquire run lock: %w", err)
	}
	if !ok {
		r.metrics.LockSkipped()
		return nil
	}
	r.metrics.LockAcquired()
	defer func() {
		if e := runLease.Release(context.WithoutCancel(ctx)); e != nil {
			r.metrics.LockError()
		}
	}()
	sources, err := r.sources(ctx)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, r.cfg.MaxConcurrent)
	var wg sync.WaitGroup
	var first error
	var mu sync.Mutex
	queued := 0
	r.metrics.SetQueueSize(0)
	now := time.Now().UTC()
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		due, normalProbe := r.dueKind(source, now)
		if !due {
			continue
		}
		source := source
		queued++
		r.metrics.SetQueueSize(queued)
		select {
		case <-ctx.Done():
			break
		case sem <- struct{}{}:
		}
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		go func() { defer wg.Done(); defer func() { <-sem }(); r.runSource(ctx, source, normalProbe, &mu, &first) }()
	}
	wg.Wait()
	r.metrics.SetQueueSize(0)
	if err := ctx.Err(); err != nil {
		return err
	}
	return first
}

func (r *Runner) dueKind(source domain.Source, now time.Time) (due bool, normalProbe bool) {
	normalProbe = r.isNormalProbe(source.ID, now)
	return normalProbe || r.retryDue(source, now), normalProbe
}

func (r *Runner) isNormalProbe(sourceID string, now time.Time) bool {
	interval := r.cfg.Interval
	if r.cfg.IntervalProvider != nil {
		if configured := r.cfg.IntervalProvider(context.Background()); configured > 0 {
			interval = configured
		}
	}
	r.scheduleMu.Lock()
	last, ok := r.lastNormalProbes[sourceID]
	r.scheduleMu.Unlock()
	return !ok || last.IsZero() || now.Sub(last) >= interval
}

func (r *Runner) markNormalProbe(sourceID string, now time.Time) {
	r.scheduleMu.Lock()
	r.lastNormalProbes[sourceID] = now
	r.scheduleMu.Unlock()
}

func (r *Runner) retryDue(source domain.Source, now time.Time) bool {
	if source.Status != incident.Offline && source.Status != incident.Unknown && source.Status != incident.Degraded {
		return false
	}
	interval := r.cfg.Interval
	if r.cfg.RetryIntervalProvider != nil {
		if configured := r.cfg.RetryIntervalProvider(context.Background()); configured > 0 {
			interval = configured
		}
	}
	return source.LastChecked.IsZero() || now.Sub(source.LastChecked) >= interval
}

func (r *Runner) runSource(ctx context.Context, source domain.Source, normalProbe bool, mu *sync.Mutex, first *error) {
	if r.cfg.Jitter > 0 {
		delay := time.Duration(rand.Int63n(int64(r.cfg.Jitter)))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
	lease, ok, err := r.cfg.Locker.TryAcquire(ctx, r.cfg.SourceLockPrefix+source.ID, r.cfg.SourceLockTTL)
	if err != nil {
		r.metrics.LockError()
		setFirst(mu, first, fmt.Errorf("acquire source lock %s: %w", source.ID, err))
		return
	}
	if !ok {
		r.metrics.LockSkipped()
		return
	}
	if normalProbe {
		r.markNormalProbe(source.ID, time.Now().UTC())
	}
	r.metrics.LockAcquired()
	defer func() {
		if e := lease.Release(context.WithoutCancel(ctx)); e != nil {
			r.metrics.LockError()
		}
	}()
	started := time.Now()
	r.metrics.ProbeStarted()
	result := r.safeProbe(ctx, source)
	r.metrics.ProbeFinished(result.Status == "online", time.Since(started))
	r.machine.SetMaintenance(source.ID, source.Maintenance)
	transition := r.machine.Observe(source.ID, result)
	if transition.From != transition.To || transition.Event == "incident_opened" || transition.Event == "incident_resolved" {
		r.metrics.Incident()
	}
	// Persisting a completed probe must survive cancellation of the probe loop.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.cfg.ResultTimeout)
	defer cancel()
	if !normalProbe && transition.From == transition.To && transition.Event == "" {
		return
	}
	if err := r.recordWithRetry(recordCtx, source, result, transition, mu, first); err != nil {
		setFirst(mu, first, err)
	}
}

func (r *Runner) safeProbe(ctx context.Context, source domain.Source) (result probe.Result) {
	defer func() {
		if v := recover(); v != nil {
			result = probe.Result{Status: "offline", Error: fmt.Sprintf("probe panic: %v", v), CheckedAt: time.Now().UTC()}
		}
	}()
	for attempt := 0; attempt <= r.cfg.ProbeRetries; attempt++ {
		timeout := r.cfg.ProbeTimeout
		if source.RequestTimeout > 0 {
			timeout = time.Duration(source.RequestTimeout) * time.Second
		}
		switch source.ProbeMode {
		case probe.ModeDockerPull:
			image := source.TestImageReference
			if image == "" && source.TestRepository != "" {
				image = source.TestRepository
				if source.TestTag != "" {
					image += ":" + source.TestTag
				}
			}
			if image == "" {
				image = os.Getenv("PROBE_TEST_IMAGE")
			}
			maxBytes := source.DownloadTestBytes
			if maxBytes <= 0 {
				maxBytes = envInt64("PROBE_PULL_MAX_BYTES", 64<<20)
			}
			result = probe.RunDockerPull(ctx, timeout, image, maxBytes)
		case probe.ModeHTTP:
			result = probe.RunHTTP(ctx, source.BaseURL, timeout)
		default:
			var credentials *registry.Credentials
			if r.cfg.CredentialProvider != nil {
				resolved, found, credentialErr := r.cfg.CredentialProvider(ctx, source)
				if credentialErr != nil {
					result = probe.Result{Status: "offline", ErrorStage: "authentication", Error: "credential resolution failed"}
				} else if found {
					credentials = &registry.Credentials{AuthType: resolved.AuthType, Username: resolved.Username, Secret: resolved.Secret}
				}
			}
			if result.Error == "" && strings.EqualFold(source.TestImageAuthStrategy, "required") && credentials == nil {
				result = probe.Result{Status: "offline", ErrorStage: "authentication", Error: "required credential profile not found"}
			}
			if result.Error == "" {
				result = probe.RunWithOptions(ctx, source.BaseURL, timeout, probe.Options{TestRepository: source.TestRepository, TestTag: source.TestTag, DownloadTestBytes: source.DownloadTestBytes, SkipBlob: source.ProbeMode == probe.ModeManifest, Credentials: credentials})
			}
		}
		if result.Status == "online" || attempt == r.cfg.ProbeRetries {
			return result
		}
		delay := time.Duration(1<<attempt) * 100 * time.Millisecond
		select {
		case <-ctx.Done():
			result.Error = ctx.Err().Error()
			return result
		case <-time.After(delay):
		}
	}
	return result
}

func envInt64(name string, fallback int64) int64 {
	if value, err := strconv.ParseInt(os.Getenv(name), 10, 64); err == nil && value > 0 {
		return value
	}
	return fallback
}
func (r *Runner) recordWithRetry(ctx context.Context, source domain.Source, result probe.Result, transition incident.Transition, _ *sync.Mutex, _ *error) error {
	var err error
	for attempt := 0; attempt <= r.cfg.ResultRetries; attempt++ {
		err = r.recorder.RecordProbe(ctx, source, result, transition)
		if err == nil {
			return nil
		}
		if attempt < r.cfg.ResultRetries {
			r.metrics.ResultRetry()
			delay := time.Duration(1<<attempt) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				r.metrics.ResultError()
				return fmt.Errorf("record probe: %w", err)
			case <-time.After(delay):
			}
		}
	}
	r.metrics.ResultError()
	return fmt.Errorf("record probe after %d attempts: %w", r.cfg.ResultRetries+1, err)
}
func setFirst(mu *sync.Mutex, first *error, err error) {
	mu.Lock()
	defer mu.Unlock()
	if *first == nil {
		*first = err
	}
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	for {
		interval := r.cfg.Interval
		if r.cfg.IntervalProvider != nil {
			if configured := r.cfg.IntervalProvider(ctx); configured > 0 {
				interval = configured
			}
		}
		if r.cfg.RetryIntervalProvider != nil {
			if retry := r.cfg.RetryIntervalProvider(ctx); retry > 0 && retry < interval {
				interval = retry
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
			if err := r.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
		}
	}
}

type NopRecorder struct{}

func (NopRecorder) RecordProbe(context.Context, domain.Source, probe.Result, incident.Transition) error {
	return nil
}

type NoopLocker struct{}
type noopLease struct{}

func (NoopLocker) TryAcquire(context.Context, string, time.Duration) (Lease, bool, error) {
	return noopLease{}, true, nil
}
func (noopLease) Release(context.Context) error { return nil }

type RedisLocker struct{ client *redis.Client }

func NewRedisLocker(rawURL string) (*RedisLocker, error) {
	if rawURL == "" {
		rawURL = "redis://127.0.0.1:6379/0"
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, errors.New("redis URL has no host")
	}
	db := 0
	if u.Path != "" && u.Path != "/" {
		db, err = strconv.Atoi(u.Path[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid redis database: %w", err)
		}
	}
	username, password := "", ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	return &RedisLocker{client: redis.NewClient(&redis.Options{Addr: u.Host, Username: username, Password: password, DB: db})}, nil
}
func (l *RedisLocker) Close() error { return l.client.Close() }
func (l *RedisLocker) TryAcquire(ctx context.Context, key string, ttl time.Duration) (Lease, bool, error) {
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	ok, err := l.client.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return redisLease{client: l.client, key: key, token: token}, true, nil
}

type redisLease struct {
	client     *redis.Client
	key, token string
}

func (l redisLease) Release(ctx context.Context) error {
	const script = `if redis.call('get', KEYS[1]) == ARGV[1] then return redis.call('del', KEYS[1]) else return 0 end`
	_, err := l.client.Eval(ctx, script, []string{l.key}, l.token).Result()
	return err
}
