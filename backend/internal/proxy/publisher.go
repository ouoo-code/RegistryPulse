package proxy

import (
	"context"
	"log/slog"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/redis/go-redis/v9"
)

type SourceProvider interface {
	Sources() []domain.Source
}

// StartSnapshotPublisher publishes only routing/health metadata. It is safe
// to call with a nil Redis client; the API can still serve normally.
func StartSnapshotPublisher(ctx context.Context, client redis.UniversalClient, provider SourceProvider, interval time.Duration) {
	if client == nil || provider == nil {
		return
	}
	if interval < time.Second {
		interval = 5 * time.Second
	}
	go func() {
		publish := func() {
			publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := PublishSnapshot(publishCtx, client, BuildSnapshot(provider.Sources()))
			cancel()
			if err != nil {
				slog.Warn("proxy route snapshot publish failed", "error", err)
			}
		}
		publish()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				publish()
			}
		}
	}()
}

// StartRuntimeConfigWatcher applies administrator changes without restarting
// the proxy process. The listen port remains a Compose concern and is exposed
// in status as a restart-required setting.
func StartRuntimeConfigWatcher(ctx context.Context, client redis.UniversalClient, manager *RouteManager, fallback RuntimeConfig, interval time.Duration) {
	if manager == nil {
		return
	}
	if err := manager.ApplyRuntimeConfig(fallback); err != nil {
		slog.Warn("proxy fallback configuration rejected", "error", err)
	}
	if client == nil {
		return
	}
	if interval < time.Second {
		interval = 5 * time.Second
	}
	go func() {
		apply := func() {
			readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			config, err := LoadRuntimeConfig(readCtx, client)
			cancel()
			if err != nil {
				return
			}
			if err := manager.ApplyRuntimeConfig(config); err != nil {
				slog.Warn("proxy runtime configuration rejected", "error", err)
			}
		}
		apply()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				apply()
			}
		}
	}()
}

func StartRuntimeStatusPublisher(ctx context.Context, client redis.UniversalClient, manager *RouteManager, actualPort int, startedAt time.Time, interval time.Duration) {
	if client == nil || manager == nil {
		return
	}
	if interval < time.Second {
		interval = 5 * time.Second
	}
	go func() {
		publish := func() {
			state := manager.RuntimeState()
			status := RuntimeStatus{
				Running:        true,
				Enabled:        state.Enabled,
				TransportMode:  state.TransportMode,
				Ready:          state.Ready,
				ActualPort:     actualPort,
				ConfiguredPort: state.ConfiguredPort,
				CategoryID:     state.CategoryID,
				CandidateCount: state.CandidateCount,
				LastError:      state.LastError,
				StartedAt:      startedAt.UTC().Format(time.RFC3339),
				LastSeenAt:     time.Now().UTC().Format(time.RFC3339),
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := PublishRuntimeStatus(writeCtx, client, status); err != nil {
				slog.Warn("proxy status publish failed", "error", err)
			}
			cancel()
		}
		publish()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				publish()
			}
		}
	}()
}
