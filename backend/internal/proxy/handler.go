package proxy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/probe"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	CategoryID          string
	StaticUpstreams     []string
	Redis               redis.UniversalClient
	RouteMaxAge         time.Duration
	RefreshInterval     time.Duration
	FailureCooldown     time.Duration
	AllowPrivateTargets bool
	RedirectHosts       []string
	MaxConcurrent       int
	MaxRangeBytes       int64
	MaxManifestBytes    int64
}

type RouteManager struct {
	mu             sync.RWMutex
	categoryID     string
	transportMode  string
	redis          redis.UniversalClient
	maxAge         time.Duration
	refresh        time.Duration
	cooldown       time.Duration
	allowPrivate   bool
	maxRange       atomic.Int64
	maxManifest    atomic.Int64
	maxConcurrent  atomic.Int64
	active         atomic.Int64
	enabled        atomic.Bool
	configuredPort int
	static         []Candidate
	snapshot       RouteSnapshot
	failures       map[string]time.Time
	lastError      error
}

func NewRouteManager(cfg Config) (*RouteManager, error) {
	category := strings.TrimSpace(cfg.CategoryID)
	if category == "" {
		category = "dockerhub"
	}
	if cfg.RouteMaxAge <= 0 {
		cfg.RouteMaxAge = 2 * time.Hour
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 5 * time.Second
	}
	if cfg.FailureCooldown <= 0 {
		cfg.FailureCooldown = 30 * time.Second
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 64
	}
	if cfg.MaxRangeBytes <= 0 {
		cfg.MaxRangeBytes = 256 << 20
	}
	if cfg.MaxManifestBytes <= 0 {
		cfg.MaxManifestBytes = 8 << 20
	}
	m := &RouteManager{
		categoryID:    category,
		transportMode: TransportModeForward,
		redis:         cfg.Redis,
		maxAge:        cfg.RouteMaxAge,
		refresh:       cfg.RefreshInterval,
		cooldown:      cfg.FailureCooldown,
		allowPrivate:  cfg.AllowPrivateTargets,
		failures:      make(map[string]time.Time),
	}
	m.maxRange.Store(cfg.MaxRangeBytes)
	m.maxManifest.Store(cfg.MaxManifestBytes)
	m.maxConcurrent.Store(int64(cfg.MaxConcurrent))
	m.enabled.Store(true)
	for _, raw := range cfg.StaticUpstreams {
		candidate, err := staticCandidate(raw, category, cfg.AllowPrivateTargets)
		if err != nil {
			return nil, err
		}
		m.static = append(m.static, candidate)
	}
	sort.SliceStable(m.static, func(i, j int) bool { return candidateLess(m.static[i], m.static[j]) })
	return m, nil
}

func (m *RouteManager) Start(ctx context.Context) {
	if m.redis == nil {
		return
	}
	go func() {
		m.refreshSnapshot(ctx)
		ticker := time.NewTicker(m.refresh)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.refreshSnapshot(ctx)
			}
		}
	}()
}

func (m *RouteManager) refreshSnapshot(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	snapshot, err := LoadSnapshot(ctx, m.redis)
	cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.lastError = err
		return
	}
	m.snapshot = snapshot
	m.lastError = nil
}

func (m *RouteManager) Candidates() []Candidate {
	now := time.Now().UTC()
	m.mu.RLock()
	snapshot := m.snapshot
	static := append([]Candidate(nil), m.static...)
	failures := make(map[string]time.Time, len(m.failures))
	for key, until := range m.failures {
		failures[key] = until
	}
	maxAge := m.maxAge
	category := m.categoryID
	m.mu.RUnlock()

	items := make([]Candidate, 0, len(snapshot.Candidates)+len(static))
	seen := make(map[string]struct{})
	if !snapshot.GeneratedAt.IsZero() && now.Sub(snapshot.GeneratedAt) <= maxAge {
		for _, candidate := range snapshot.Candidates {
			if !candidateAllowed(candidate, category, now, maxAge, m.allowPrivate) {
				continue
			}
			key := candidateKey(candidate)
			if until := failures[key]; !until.IsZero() && now.Before(until) {
				continue
			}
			seen[key] = struct{}{}
			items = append(items, candidate)
		}
	}
	// Static upstreams are an explicit operator-configured fallback. They are
	// never blended ahead of a healthy database route, but keep a fresh install
	// usable before the first worker result exists.
	if len(items) == 0 {
		for _, candidate := range static {
			key := candidateKey(candidate)
			if _, exists := seen[key]; exists {
				continue
			}
			if until := failures[key]; !until.IsZero() && now.Before(until) {
				continue
			}
			items = append(items, candidate)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return candidateLess(items[i], items[j]) })
	return items
}

func (m *RouteManager) MarkFailure(candidate Candidate) {
	m.mu.Lock()
	m.failures[candidateKey(candidate)] = time.Now().Add(m.cooldown)
	m.mu.Unlock()
}

func (m *RouteManager) MarkSuccess(candidate Candidate) {
	m.mu.Lock()
	delete(m.failures, candidateKey(candidate))
	m.mu.Unlock()
}

func (m *RouteManager) Enabled() bool { return m.enabled.Load() }

func (m *RouteManager) TransportMode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.transportMode
}

func (m *RouteManager) Ready() bool { return m.Enabled() && len(m.Candidates()) > 0 }

func (m *RouteManager) MaxRangeBytes() int64    { return m.maxRange.Load() }
func (m *RouteManager) MaxManifestBytes() int64 { return m.maxManifest.Load() }

func (m *RouteManager) TryAcquire() bool {
	limit := m.maxConcurrent.Load()
	if limit < 1 {
		return false
	}
	for {
		current := m.active.Load()
		if current >= limit {
			return false
		}
		if m.active.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func (m *RouteManager) Release() { m.active.Add(-1) }

func (m *RouteManager) ApplyRuntimeConfig(config RuntimeConfig) error {
	normalized, err := config.Normalize()
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.categoryID = normalized.CategoryID
	m.transportMode = normalized.TransportMode
	m.maxAge = normalized.RouteMaxAge()
	m.cooldown = normalized.FailureCooldown()
	m.configuredPort = normalized.ListenPort
	m.mu.Unlock()
	m.maxRange.Store(normalized.MaxRangeBytes())
	m.maxManifest.Store(normalized.MaxManifestBytes())
	m.maxConcurrent.Store(int64(normalized.MaxConcurrent))
	m.enabled.Store(normalized.Enabled)
	return nil
}

func (m *RouteManager) RuntimeState() RuntimeState {
	m.mu.RLock()
	category := m.categoryID
	transportMode := m.transportMode
	port := m.configuredPort
	lastError := ""
	if m.lastError != nil {
		lastError = m.lastError.Error()
	}
	m.mu.RUnlock()
	return RuntimeState{Enabled: m.Enabled(), TransportMode: transportMode, CategoryID: category, CandidateCount: len(m.Candidates()), Ready: m.Ready(), LastError: lastError, ConfiguredPort: port}
}

func (m *RouteManager) LastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastError
}

func staticCandidate(raw, category string, allowPrivate bool) (Candidate, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if err := validateUpstream(raw, allowPrivate); err != nil {
		return Candidate{}, fmt.Errorf("invalid proxy upstream %q: %w", raw, err)
	}
	u, _ := url.Parse(raw)
	return Candidate{CategoryID: category, Name: u.Host, BaseURL: raw, RegistryHost: u.Host, Status: "online", Enabled: true, IsOfficial: true}, nil
}

func candidateAllowed(candidate Candidate, category string, now time.Time, maxAge time.Duration, allowPrivate bool) bool {
	if category != "" && !strings.EqualFold(candidate.CategoryID, category) {
		return false
	}
	if !candidate.Enabled || candidate.Maintenance || (candidate.Status != "online" && candidate.Status != "degraded") {
		return false
	}
	if candidate.SourceID != "" && candidate.LastChecked.IsZero() {
		return false
	}
	if !candidate.LastChecked.IsZero() && now.Sub(candidate.LastChecked) > maxAge {
		return false
	}
	return validateUpstream(candidate.BaseURL, allowPrivate) == nil
}

func validateUpstream(raw string, allowPrivate bool) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.User != nil || u.Hostname() == "" || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && strings.Contains(u.Path, "..") {
		return errors.New("upstream must be an absolute URL without credentials or path traversal")
	}
	if err := probe.ValidateTarget(raw, allowPrivate); err != nil {
		return err
	}
	return nil
}

func candidateKey(candidate Candidate) string {
	if strings.TrimSpace(candidate.SourceID) != "" {
		return candidate.SourceID
	}
	return strings.TrimRight(strings.ToLower(candidate.BaseURL), "/")
}

type Handler struct {
	manager       *RouteManager
	client        *http.Client
	redirectHosts map[string]struct{}
	auth          authBindings
	stats         proxyStats
}

type requestIDContextKey struct{}

func NewHandler(manager *RouteManager, redirectHosts []string) *Handler {
	allowed := make(map[string]struct{}, len(redirectHosts))
	for _, host := range redirectHosts {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			allowed[host] = struct{}{}
		}
	}
	h := &Handler{manager: manager, redirectHosts: allowed, auth: newAuthBindings()}
	h.client = &http.Client{
		Transport: safeTransport(manager.allowPrivate),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many upstream redirects")
			}
			if err := probe.ValidateTarget(req.URL.String(), manager.allowPrivate); err != nil {
				return fmt.Errorf("redirect target rejected: %w", err)
			}
			if _, err := probe.ResolveHost(req.Context(), req.URL.Hostname(), manager.allowPrivate); err != nil {
				return fmt.Errorf("redirect target rejected: %w", err)
			}
			if policy, ok := req.Context().Value(redirectPolicyKey{}).(map[string]struct{}); ok {
				if _, allowed := policy[strings.ToLower(req.URL.Hostname())]; !allowed {
					return fmt.Errorf("redirect host %q is not allowlisted", req.URL.Hostname())
				}
			}
			return nil
		},
	}
	return h
}

type redirectPolicyKey struct{}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", h.live)
	mux.HandleFunc("/health/ready", h.ready)
	mux.HandleFunc("/metrics", h.metrics)
	mux.HandleFunc("/v2", h.forward)
	mux.HandleFunc("/v2/", h.forward)
	return h.requestIdentity(requestSecurity(h.manager, h.logging(mux)))
}

func (h *Handler) requestIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2" || strings.HasPrefix(r.URL.Path, "/v2/") {
			requestID := newRequestID()
			w.Header().Set("X-RegistryPulse-Request-ID", requestID)
			r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, requestID))
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		h.stats.requests.Add(1)
		h.stats.durationNanos.Add(time.Since(started).Nanoseconds())
	})
}

func requestSecurity(manager *RouteManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if strings.HasPrefix(r.URL.Path, "/v2") {
				w.Header().Set("Allow", "GET, HEAD")
				writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "push and mutation requests are disabled")
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/v2") && !manager.Enabled() {
			writeError(w, http.StatusServiceUnavailable, "PROXY_DISABLED", "registry proxy is disabled")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v2") && manager.TransportMode() == TransportModeForward {
			if err := validateRegistryRequest(r, manager.MaxRangeBytes()); err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_REGISTRY_REQUEST", err.Error())
				return
			}
			if !manager.TryAcquire() {
				writeError(w, http.StatusTooManyRequests, "PROXY_BUSY", "proxy concurrency limit reached")
				return
			}
			defer manager.Release()
		}
		if strings.HasPrefix(r.URL.Path, "/v2") && !manager.Ready() {
			writeError(w, http.StatusServiceUnavailable, "NO_UPSTREAM", "no healthy registry upstream is available")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) live(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ready(w http.ResponseWriter, _ *http.Request) {
	if !h.manager.Enabled() {
		writeError(w, http.StatusServiceUnavailable, "PROXY_DISABLED", "registry proxy is disabled")
		return
	}
	if !h.manager.Ready() {
		writeError(w, http.StatusServiceUnavailable, "NO_UPSTREAM", "no healthy registry upstream is available")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) metrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = io.WriteString(w, h.stats.prometheus())
}

func (h *Handler) forward(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v2" && !strings.HasPrefix(r.URL.Path, "/v2/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "registry API path not found")
		return
	}
	requestID, _ := r.Context().Value(requestIDContextKey{}).(string)
	if requestID == "" {
		requestID = newRequestID()
	}
	started := time.Now()
	requestKind := registryRequestKind(r.URL.Path)
	transportMode := h.manager.TransportMode()
	w.Header().Set("X-RegistryPulse-Request-ID", requestID)
	h.stats.active.Add(1)
	defer h.stats.active.Add(-1)
	h.stats.requests.Add(1)
	defer func() {
		h.stats.durationNanos.Add(time.Since(started).Nanoseconds())
		slog.Debug("registry proxy request completed", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "kind", requestKind, "transport_mode", transportMode, "duration", time.Since(started))
	}()
	finish := func(status int, bytes int64) {
		h.stats.observeResponse(status, bytes)
	}
	candidates := h.manager.Candidates()
	if len(candidates) == 0 {
		finish(http.StatusServiceUnavailable, 0)
		writeError(w, http.StatusServiceUnavailable, "NO_UPSTREAM", "no healthy registry upstream is available")
		return
	}
	bound, hasBound := h.auth.binding(r)
	if hasBound {
		candidates = prioritizeCandidate(candidates, bound)
	}
	attempts := 0
	for i, candidate := range candidates {
		attempts++
		upstream, err := url.Parse(candidate.BaseURL)
		if err != nil {
			h.manager.MarkFailure(candidate)
			continue
		}
		if h.manager.TransportMode() == TransportModeRedirect {
			h.manager.MarkSuccess(candidate)
			h.stats.successes.Add(1)
			h.stats.redirects.Add(1)
			finish(http.StatusTemporaryRedirect, 0)
			slog.Debug("registry proxy request redirected", "request_id", requestID, "candidate", candidate.Name, "attempts", attempts)
			h.redirect(w, r, upstream)
			return
		}
		requestContext := r.Context()
		policy := make(map[string]struct{}, len(h.redirectHosts)+1)
		policy[strings.ToLower(upstream.Hostname())] = struct{}{}
		for host := range h.redirectHosts {
			policy[host] = struct{}{}
		}
		requestContext = context.WithValue(requestContext, redirectPolicyKey{}, policy)
		// A client bearer token is forwarded only to the source that issued the
		// latest challenge for this client/token. It is never sent to a failover
		// source, and only its hash is retained in process memory.
		forwardAuth := hasBound && candidateKey(bound) == candidateKey(candidate)
		upstreamRequest, err := buildUpstreamRequest(requestContext, r, upstream, forwardAuth)
		if err != nil {
			h.manager.MarkFailure(candidate)
			continue
		}
		response, err := h.client.Do(upstreamRequest)
		if err != nil {
			h.manager.MarkFailure(candidate)
			h.stats.upstreamFailures.Add(1)
			slog.Debug("registry proxy upstream request failed", "request_id", requestID, "candidate", candidate.Name, "error", err)
			continue
		}
		if response.StatusCode == http.StatusUnauthorized {
			h.auth.rememberChallenge(r, candidate)
			if hasBound {
				h.auth.forgetToken(r)
			}
		}
		if forwardAuth {
			h.auth.rememberToken(r, candidate)
		}
		if retryableStatus(response.StatusCode) && i < len(candidates)-1 {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
			_ = response.Body.Close()
			h.manager.MarkFailure(candidate)
			h.stats.upstreamFailures.Add(1)
			h.stats.retries.Add(1)
			slog.Debug("registry proxy failing over", "request_id", requestID, "candidate", candidate.Name, "status", response.StatusCode)
			continue
		}
		h.manager.MarkSuccess(candidate)
		if isManifestPath(r.URL.Path) && r.Method != http.MethodHead {
			body, tooLarge, readErr := readBoundedBody(response.Body, h.manager.MaxManifestBytes())
			_ = response.Body.Close()
			if readErr != nil {
				finish(http.StatusBadGateway, 0)
				writeError(w, http.StatusBadGateway, "UPSTREAM_READ_FAILED", "could not read the registry manifest")
				return
			}
			if tooLarge {
				finish(http.StatusRequestEntityTooLarge, 0)
				writeError(w, http.StatusRequestEntityTooLarge, "MANIFEST_TOO_LARGE", "manifest exceeds the configured proxy limit")
				return
			}
			copyResponseHeaders(w.Header(), response.Header)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			w.WriteHeader(response.StatusCode)
			written, writeErr := w.Write(body)
			finish(response.StatusCode, int64(written))
			if writeErr != nil {
				slog.Debug("registry proxy manifest response write failed", "request_id", requestID, "error", writeErr)
			}
			if response.StatusCode >= 200 && response.StatusCode < 400 {
				h.stats.successes.Add(1)
			}
			return
		}
		copyResponseHeaders(w.Header(), response.Header)
		w.WriteHeader(response.StatusCode)
		var forwarded int64
		if r.Method != http.MethodHead {
			forwarded, err = io.Copy(w, response.Body)
		}
		_ = response.Body.Close()
		finish(response.StatusCode, forwarded)
		if err != nil {
			slog.Debug("registry proxy response stream ended with error", "request_id", requestID, "candidate", candidate.Name, "error", err, "bytes", forwarded)
		}
		if response.StatusCode >= 200 && response.StatusCode < 400 {
			h.stats.successes.Add(1)
		}
		slog.Debug("registry proxy request served", "request_id", requestID, "candidate", candidate.Name, "status", response.StatusCode, "attempts", attempts, "bytes", forwarded)
		return
	}
	finish(http.StatusBadGateway, 0)
	writeError(w, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "all registry upstreams failed")
}

func newRequestID() string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func registryRequestKind(path string) string {
	normalized := strings.TrimSuffix(path, "/")
	switch {
	case normalized == "/v2":
		return "api"
	case strings.Contains(normalized, "/manifests/"):
		return "manifest"
	case strings.Contains(normalized, "/blobs/"):
		return "blob"
	default:
		return "other"
	}
}

func readBoundedBody(body io.Reader, limit int64) ([]byte, bool, error) {
	if limit < 1 {
		return nil, false, errors.New("body limit must be positive")
	}
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, false, err
	}
	return data, int64(len(data)) > limit, nil
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, upstream *url.URL) {
	escapedPath := r.URL.EscapedPath()
	if escapedPath == "" {
		escapedPath = "/v2/"
	}
	target := strings.TrimRight(upstream.String(), "/") + escapedPath
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	// 307 preserves the original method and headers while allowing the
	// registry client to fetch the body directly from the selected upstream.
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func buildUpstreamRequest(ctx context.Context, incoming *http.Request, upstream *url.URL, forwardAuth bool) (*http.Request, error) {
	escapedPath := incoming.URL.EscapedPath()
	if escapedPath == "" {
		escapedPath = "/v2/"
	}
	rawURL := strings.TrimRight(upstream.String(), "/") + escapedPath
	if incoming.URL.RawQuery != "" {
		rawURL += "?" + incoming.URL.RawQuery
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, incoming.Method, target.String(), nil)
	if err != nil {
		return nil, err
	}
	copyRequestHeaders(request.Header, incoming.Header, forwardAuth)
	request.Host = target.Host
	return request, nil
}

func copyRequestHeaders(dst, src http.Header, forwardAuth bool) {
	allowed := map[string]bool{"Accept": true, "Range": true, "If-Range": true, "If-None-Match": true, "If-Modified-Since": true, "User-Agent": true, "Docker-Distribution-API-Version": true, "Authorization": true}
	for key, values := range src {
		canonical := http.CanonicalHeaderKey(key)
		if !allowed[canonical] || (!forwardAuth && strings.EqualFold(key, "Authorization")) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func prioritizeCandidate(candidates []Candidate, preferred Candidate) []Candidate {
	preferredKey := candidateKey(preferred)
	for index, candidate := range candidates {
		if candidateKey(candidate) != preferredKey {
			continue
		}
		if index == 0 {
			return candidates
		}
		ordered := make([]Candidate, 0, len(candidates))
		ordered = append(ordered, candidate)
		ordered = append(ordered, candidates[:index]...)
		ordered = append(ordered, candidates[index+1:]...)
		return ordered
	}
	return candidates
}

type authBinding struct {
	candidate Candidate
	expires   time.Time
}

type authBindings struct {
	mu      sync.Mutex
	clients map[string]authBinding
	tokens  map[string]authBinding
}

func newAuthBindings() authBindings {
	return authBindings{clients: make(map[string]authBinding), tokens: make(map[string]authBinding)}
}

func (a *authBindings) binding(r *http.Request) (Candidate, bool) {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	if key := bearerFingerprint(r.Header.Get("Authorization")); key != "" {
		if binding, ok := a.tokens[key]; ok && now.Before(binding.expires) {
			return binding.candidate, true
		}
		delete(a.tokens, key)
	}
	client := authClientKey(r)
	if binding, ok := a.clients[client]; ok && now.Before(binding.expires) {
		return binding.candidate, true
	}
	delete(a.clients, client)
	return Candidate{}, false
}

func (a *authBindings) rememberChallenge(r *http.Request, candidate Candidate) {
	a.mu.Lock()
	a.clients[authClientKey(r)] = authBinding{candidate: candidate, expires: time.Now().Add(5 * time.Minute)}
	a.mu.Unlock()
}

func (a *authBindings) rememberToken(r *http.Request, candidate Candidate) {
	if key := bearerFingerprint(r.Header.Get("Authorization")); key != "" {
		a.mu.Lock()
		a.tokens[key] = authBinding{candidate: candidate, expires: time.Now().Add(5 * time.Minute)}
		a.mu.Unlock()
	}
}

func (a *authBindings) forgetToken(r *http.Request) {
	if key := bearerFingerprint(r.Header.Get("Authorization")); key != "" {
		a.mu.Lock()
		delete(a.tokens, key)
		a.mu.Unlock()
	}
}

func bearerFingerprint(raw string) string {
	parts := strings.Fields(raw)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(parts[1]))
	return string(sum[:])
}

func authClientKey(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func copyResponseHeaders(dst, src http.Header) {
	hopByHop := map[string]bool{"Connection": true, "Proxy-Connection": true, "Keep-Alive": true, "Proxy-Authenticate": true, "Proxy-Authorization": true, "TE": true, "Trailer": true, "Transfer-Encoding": true, "Upgrade": true, "Via": true, "Forwarded": true, "Cookie": true, "Set-Cookie": true, "Location": true}
	for key, values := range src {
		if hopByHop[http.CanonicalHeaderKey(key)] {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func validateRegistryRequest(r *http.Request, maxRange int64) error {
	if r.ContentLength != 0 {
		return errors.New("request bodies are not supported")
	}
	if raw := r.Header.Get("Range"); raw != "" {
		if len(raw) > 256 || strings.Contains(raw, ",") || !strings.HasPrefix(strings.ToLower(raw), "bytes=") {
			return errors.New("only one bytes range is supported")
		}
		rangeValue := strings.TrimSpace(raw[len("bytes="):])
		bounds := strings.Split(rangeValue, "-")
		if len(bounds) != 2 {
			return errors.New("invalid bytes range")
		}
		if maxRange > 0 {
			if bounds[0] == "" {
				suffix, err := strconv.ParseInt(strings.TrimSpace(bounds[1]), 10, 64)
				if err != nil || suffix < 0 || suffix > maxRange {
					return errors.New("bytes range exceeds the proxy limit")
				}
			} else if start, err := strconv.ParseInt(strings.TrimSpace(bounds[0]), 10, 64); err != nil || start < 0 {
				return errors.New("invalid bytes range")
			} else if strings.TrimSpace(bounds[1]) != "" {
				end, endErr := strconv.ParseInt(strings.TrimSpace(bounds[1]), 10, 64)
				if endErr != nil || end < start || end-start+1 > maxRange {
					return errors.New("bytes range exceeds the proxy limit")
				}
			}
		}
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	if path == "/v2" {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(path, "/v2/"), "/")
	if len(parts) < 3 {
		return errors.New("registry path is not allowed")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return errors.New("invalid registry path")
		}
	}
	for i := 1; i < len(parts); i++ {
		if parts[i] == "manifests" || parts[i] == "blobs" {
			if i == len(parts)-1 {
				return errors.New("registry reference is missing")
			}
			return nil
		}
	}
	return errors.New("registry endpoint is not allowed")
}

func isManifestPath(path string) bool {
	return strings.Contains(strings.TrimSuffix(path, "/"), "/manifests/")
}

func retryableStatus(status int) bool {
	return status == 500 || status == 502 || status == 503 || status == 504
}

func safeTransport(allowPrivate bool) *http.Transport {
	return &http.Transport{
		// Do not honor HTTP_PROXY/HTTPS_PROXY here. An ambient egress proxy can
		// resolve the destination outside our DNS/IP policy and turn this into
		// an SSRF bypass.
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := probe.ResolveHost(ctx, host, allowPrivate)
			if err != nil {
				return nil, err
			}
			dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
			var lastErr error
			for _, ip := range ips {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			return nil, lastErr
		},
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}

type proxyStats struct {
	requests         atomic.Int64
	successes        atomic.Int64
	upstreamFailures atomic.Int64
	retries          atomic.Int64
	redirects        atomic.Int64
	active           atomic.Int64
	bytesForwarded   atomic.Int64
	durationNanos    atomic.Int64
	status1xx        atomic.Int64
	status2xx        atomic.Int64
	status3xx        atomic.Int64
	status4xx        atomic.Int64
	status5xx        atomic.Int64
}

func (s *proxyStats) observeResponse(status int, bytes int64) {
	if bytes > 0 {
		s.bytesForwarded.Add(bytes)
	}
	switch {
	case status >= 100 && status < 200:
		s.status1xx.Add(1)
	case status >= 200 && status < 300:
		s.status2xx.Add(1)
	case status >= 300 && status < 400:
		s.status3xx.Add(1)
	case status >= 400 && status < 500:
		s.status4xx.Add(1)
	case status >= 500:
		s.status5xx.Add(1)
	}
}

func (s *proxyStats) prometheus() string {
	requests := s.requests.Load()
	average := float64(0)
	if requests > 0 {
		average = float64(s.durationNanos.Load()) / float64(requests) / float64(time.Second)
	}
	return fmt.Sprintf("# HELP registrypulse_proxy_requests_total Total proxy requests.\n# TYPE registrypulse_proxy_requests_total counter\nregistrypulse_proxy_requests_total %d\n# HELP registrypulse_proxy_success_total Successful proxy responses.\n# TYPE registrypulse_proxy_success_total counter\nregistrypulse_proxy_success_total %d\n# HELP registrypulse_proxy_upstream_failure_total Upstream failures and failovers.\n# TYPE registrypulse_proxy_upstream_failure_total counter\nregistrypulse_proxy_upstream_failure_total %d\n# HELP registrypulse_proxy_retries_total Upstream failover attempts.\n# TYPE registrypulse_proxy_retries_total counter\nregistrypulse_proxy_retries_total %d\n# HELP registrypulse_proxy_redirects_total Redirect responses returned to clients.\n# TYPE registrypulse_proxy_redirects_total counter\nregistrypulse_proxy_redirects_total %d\n# HELP registrypulse_proxy_active_requests Current active proxy requests.\n# TYPE registrypulse_proxy_active_requests gauge\nregistrypulse_proxy_active_requests %d\n# HELP registrypulse_proxy_bytes_forwarded_total Response bytes forwarded to clients.\n# TYPE registrypulse_proxy_bytes_forwarded_total counter\nregistrypulse_proxy_bytes_forwarded_total %d\n# HELP registrypulse_proxy_responses_total Responses by status class.\n# TYPE registrypulse_proxy_responses_total counter\nregistrypulse_proxy_responses_total{class=\"1xx\"} %d\nregistrypulse_proxy_responses_total{class=\"2xx\"} %d\nregistrypulse_proxy_responses_total{class=\"3xx\"} %d\nregistrypulse_proxy_responses_total{class=\"4xx\"} %d\nregistrypulse_proxy_responses_total{class=\"5xx\"} %d\n# HELP registrypulse_proxy_request_duration_seconds Average proxy request duration.\nregistrypulse_proxy_request_duration_seconds %.6f\n", requests, s.successes.Load(), s.upstreamFailures.Load(), s.retries.Load(), s.redirects.Load(), s.active.Load(), s.bytesForwarded.Load(), s.status1xx.Load(), s.status2xx.Load(), s.status3xx.Load(), s.status4xx.Load(), s.status5xx.Load(), average)
}
