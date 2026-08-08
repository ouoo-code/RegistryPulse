package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/database"
	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/incident"
	"github.com/ouoo-code/RegistryPulse/internal/probe"
	"github.com/ouoo-code/RegistryPulse/internal/scheduler"
)

type resultRecorder struct {
	db     *sql.DB
	logger *slog.Logger
}

func (r resultRecorder) RecordProbe(ctx context.Context, source domain.Source, result probe.Result, t incident.Transition) error {
	r.logger.Info("probe recorded", "source", source.ID, "status", result.Status, "state", t.To, "event", t.Event, "error", result.Error)
	if r.db == nil {
		return nil
	}
	resolvedIPs, _ := json.Marshal(result.ResolvedIPs)
	_, err := r.db.ExecContext(ctx, `INSERT INTO probe_results(source_id,status,dns_duration_ms,tcp_duration_ms,tls_duration_ms,registry_duration_ms,manifest_duration_ms,blob_duration_ms,blob_bytes,error,error_stage,dns_success,resolved_ips,tcp_success,remote_ip,remote_port,tls_success,tls_version,tls_cipher,certificate_subject,certificate_issuer,certificate_days_remaining,registry_api_success,registry_api_status,manifest_success,manifest_status,manifest_content_type,manifest_digest,blob_success,blob_status,blob_ttfb_ms,blob_speed_bps,checked_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32,$33)`, source.ID, t.To, result.DNSMS, result.TCPMS, result.TLSMS, result.RegistryMS, result.ManifestMS, result.BlobMS, result.BlobBytes, result.Error, result.ErrorStage, result.DNSSuccess, string(resolvedIPs), result.TCPSuccess, result.RemoteIP, result.RemotePort, result.TLSSuccess, result.TLSVersion, result.TLSCipher, result.CertificateSubject, result.CertificateIssuer, result.CertificateDaysRemaining, result.RegistrySuccess, result.RegistryStatus, result.ManifestSuccess, result.ManifestStatus, result.ManifestContentType, result.ManifestDigest, result.BlobSuccess, result.BlobStatus, result.BlobTTFBMS, result.BlobSpeedBPS, result.CheckedAt)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE registry_sources SET status=$1,response_ms=$2,last_checked=$3,error=$4,updated_at=now() WHERE id=$5`, t.To, result.RegistryMS+result.ManifestMS+result.BlobMS, result.CheckedAt, result.Error, source.ID)
	return err
}

type notifyingRecorder struct {
	store  *database.Store
	logger *slog.Logger
}

func (r notifyingRecorder) RecordProbe(ctx context.Context, source domain.Source, result probe.Result, t incident.Transition) error {
	if err := r.store.RecordProbe(ctx, source, result, t); err != nil {
		return err
	}
	r.store.NotifyTransition(ctx, source, result, t)
	r.logger.Info("probe recorded", "source", source.ID, "status", result.Status, "state", t.To, "event", t.Event)
	return nil
}

func envInt(name string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return fallback
}
func dbSettingInt(db *sql.DB, key string, fallback int) int {
	var raw string
	if err := db.QueryRow(`SELECT COALESCE(value->>'value',value#>>'{}') FROM system_settings WHERE key=$1`, key).Scan(&raw); err == nil {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			return value
		}
	}
	return fallback
}
func dbSettingDuration(db *sql.DB, key string, fallback time.Duration) time.Duration {
	minutes := dbSettingInt(db, key, 0)
	if minutes > 0 {
		return time.Duration(minutes) * time.Minute
	}
	return fallback
}
func retentionPolicyFromEnv() database.RetentionPolicy {
	days := func(name string, fallback int) time.Duration {
		value := envInt(name, fallback)
		return time.Duration(value) * 24 * time.Hour
	}
	return database.RetentionPolicy{
		ProbeResults:      days("DATA_RETENTION_RAW_DAYS", 30),
		ResolvedIncidents: days("DATA_RETENTION_INCIDENT_DAYS", envInt("DATA_RETENTION_AUDIT_DAYS", 180)),
		IncidentEvents:    days("DATA_RETENTION_AUDIT_DAYS", 180),
	}
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	var store domain.Store = domain.NewMemoryStore()
	var recorder scheduler.ResultRecorder = resultRecorder{logger: slog.Default()}
	var closeDB func() = func() {}
	failureThreshold := envInt("FAILURE_THRESHOLD", 3)
	recoveryThreshold := envInt("RECOVERY_THRESHOLD", 2)
	slowThreshold := envInt("SLOW_THRESHOLD", 3)
	if os.Getenv("DATABASE_URL") != "" {
		db, err := database.Open(ctx)
		if err != nil {
			slog.Error("database unavailable", "error", err)
			os.Exit(1)
		}
		dir := os.Getenv("MIGRATIONS_DIR")
		if dir == "" {
			dir = "/migrations"
		}
		if err = database.Migrate(ctx, db, dir); err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		dbStore := database.NewStore(db)
		store = dbStore
		recorder = notifyingRecorder{store: dbStore, logger: slog.Default()}
		failureThreshold = dbSettingInt(db, "failure_threshold", failureThreshold)
		recoveryThreshold = dbSettingInt(db, "recovery_threshold", recoveryThreshold)
		slowThreshold = dbSettingInt(db, "slow_threshold", slowThreshold)
		if err := dbStore.RollupProbeStats(ctx); err != nil {
			slog.Warn("initial stats rollup failed", "error", err)
		}
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := dbStore.RollupProbeStats(ctx); err != nil {
						slog.Warn("stats rollup failed", "error", err)
					}
				}
			}
		}()
		go func() {
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if report, err := dbStore.CleanupRetention(ctx, time.Now().UTC(), retentionPolicyFromEnv()); err != nil {
						slog.Warn("retention cleanup failed", "error", err)
					} else {
						slog.Info("retention cleanup completed", "probe_results_deleted", report.ProbeResultsDeleted, "incident_events_deleted", report.IncidentEventsDeleted, "incidents_deleted", report.IncidentsDeleted)
					}
				}
			}
		}()
		closeDB = func() { _ = db.Close() }
	}
	defer closeDB()
	interval := 30 * time.Minute
	if os.Getenv("APP_ENV") == "development" {
		interval = time.Minute
	}
	if raw := os.Getenv("PROBE_INTERVAL"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			interval = parsed
		}
	}
	retryInterval := 3 * time.Minute
	if raw := os.Getenv("PROBE_RETRY_INTERVAL"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			retryInterval = parsed
		}
	}
	if raw := os.Getenv("PROBE_CRON"); raw != "" {
		if parsed, ok := scheduler.CronInterval(raw); ok {
			interval = parsed
		} else {
			slog.Warn("unsupported PROBE_CRON expression", "expression", raw)
		}
	}
	jitter := time.Duration(envInt("SCHEDULE_JITTER_SECONDS", 0)) * time.Second
	var locker scheduler.Locker = scheduler.NoopLocker{}
	var closeRedis func() = func() {}
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		l, err := scheduler.NewRedisLocker(redisURL)
		if err != nil {
			slog.Error("invalid redis URL", "error", err)
			os.Exit(1)
		}
		locker = l
		closeRedis = func() { _ = l.Close() }
	}
	defer closeRedis()
	maxConcurrent := envInt("MAX_CONCURRENT", envInt("PROBE_MAX_CONCURRENCY", 4))
	intervalProvider := func(ctx context.Context) time.Duration { return interval }
	if db, ok := store.(interface{ DBHandle() *sql.DB }); ok && db.DBHandle() != nil {
		dbHandle := db.DBHandle()
		intervalProvider = func(_ context.Context) time.Duration {
			return dbSettingDuration(dbHandle, "probe_interval_minutes", interval)
		}
	}
	retryIntervalProvider := func(_ context.Context) time.Duration { return retryInterval }
	if db, ok := store.(interface{ DBHandle() *sql.DB }); ok && db.DBHandle() != nil {
		dbHandle := db.DBHandle()
		retryIntervalProvider = func(_ context.Context) time.Duration {
			return dbSettingDuration(dbHandle, "probe_retry_interval_minutes", retryInterval)
		}
	}
	machine := incident.New(incident.Config{FailureThreshold: failureThreshold, RecoveryThreshold: recoveryThreshold, SlowThreshold: slowThreshold})
	for _, source := range store.Sources() {
		machine.Seed(source.ID, source.Status, source.Maintenance)
	}
	runner := scheduler.New(scheduler.Config{Interval: interval, IntervalProvider: intervalProvider, RetryIntervalProvider: retryIntervalProvider, Jitter: jitter, MaxConcurrent: maxConcurrent, ProbeTimeout: 15 * time.Second, ResultTimeout: time.Duration(envInt("RESULT_TIMEOUT_SECONDS", 10)) * time.Second, ResultRetries: envInt("RESULT_RETRIES", 2), ProbeRetries: envInt("PROBE_RETRIES", 2), Locker: locker}, func(ctx context.Context) ([]domain.Source, error) { return store.Sources(), nil }, recorder, machine, nil)
	slog.Info("worker started", "normal_interval", interval, "retry_interval", retryInterval, "max_concurrent", maxConcurrent)
	if err := runner.Run(ctx); err != nil && ctx.Err() == nil {
		slog.Error("worker stopped with error", "error", err)
		os.Exit(1)
	}
	slog.Info("worker stopped")
}
