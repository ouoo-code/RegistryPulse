package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/proxy"
	"github.com/redis/go-redis/v9"
)

func main() {
	port := envInt("PROXY_HTTP_PORT", 10800)
	if port < 1 || port > 65535 {
		slog.Error("invalid PROXY_HTTP_PORT", "port", port)
		os.Exit(64)
	}

	var redisClient *redis.Client
	if raw := strings.TrimSpace(os.Getenv("PROXY_REDIS_URL")); raw != "" {
		options, err := redis.ParseURL(raw)
		if err != nil {
			slog.Error("invalid PROXY_REDIS_URL", "error", err)
			os.Exit(64)
		}
		redisClient = redis.NewClient(options)
		defer redisClient.Close()
	}

	allowPrivate := strings.EqualFold(strings.TrimSpace(os.Getenv("PROXY_ALLOW_PRIVATE_TARGETS")), "true")
	if !allowPrivate {
		// Keep the existing probe setting as an explicit development escape hatch,
		// but never enable it implicitly from a production environment.
		allowPrivate = strings.EqualFold(strings.TrimSpace(os.Getenv("ALLOW_PRIVATE_TARGETS")), "true") && !strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	}
	staticUpstreams := splitCSV(os.Getenv("PROXY_UPSTREAMS"))
	manager, err := proxy.NewRouteManager(proxy.Config{
		CategoryID:          envString("PROXY_CATEGORY_ID", "dockerhub"),
		StaticUpstreams:     staticUpstreams,
		Redis:               redisClient,
		RouteMaxAge:         envDuration("PROXY_ROUTE_MAX_AGE", 2*time.Hour),
		RefreshInterval:     envDuration("PROXY_ROUTE_REFRESH_INTERVAL", 5*time.Second),
		FailureCooldown:     envDuration("PROXY_FAILURE_COOLDOWN", 30*time.Second),
		AllowPrivateTargets: allowPrivate,
		RedirectHosts:       splitCSV(os.Getenv("PROXY_REDIRECT_HOSTS")),
		MaxConcurrent:       envInt("PROXY_MAX_CONCURRENT", 64),
		MaxRangeBytes:       envBytes("PROXY_MAX_RANGE_BYTES", 256<<20),
		MaxManifestBytes:    envBytes("PROXY_MAX_MANIFEST_BYTES", 8<<20),
	})
	if err != nil {
		slog.Error("proxy configuration failed", "error", err)
		os.Exit(64)
	}
	runtimeConfig := proxy.DefaultRuntimeConfig(port, envString("PROXY_CATEGORY_ID", "dockerhub"), envDuration("PROXY_ROUTE_MAX_AGE", 2*time.Hour), envDuration("PROXY_FAILURE_COOLDOWN", 30*time.Second), envInt("PROXY_MAX_CONCURRENT", 64), envBytes("PROXY_MAX_RANGE_BYTES", 256<<20), envBytes("PROXY_MAX_MANIFEST_BYTES", 8<<20))
	ctx := context.Background()
	manager.Start(ctx)
	proxy.StartRuntimeConfigWatcher(ctx, redisClient, manager, runtimeConfig, envDuration("PROXY_ROUTE_REFRESH_INTERVAL", 5*time.Second))
	startedAt := time.Now().UTC()
	proxy.StartRuntimeStatusPublisher(ctx, redisClient, manager, port, startedAt, envDuration("PROXY_ROUTE_REFRESH_INTERVAL", 5*time.Second))
	handler := proxy.NewHandler(manager, splitCSV(os.Getenv("PROXY_REDIRECT_HOSTS"))).Routes()
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout is intentionally unset: layer blobs can be large and
		// must remain streamed without a small fixed response deadline.
	}
	slog.Info("registry proxy listening", "addr", server.Addr, "category", envString("PROXY_CATEGORY_ID", "dockerhub"), "image_cache", false)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("registry proxy stopped", "error", err)
		os.Exit(1)
	}
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}
	return value
}

func envDuration(name string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}
	if duration, err := time.ParseDuration(raw); err == nil {
		return duration
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}

func envBytes(name string, fallback int64) int64 {
	raw := strings.TrimSpace(strings.ToUpper(os.Getenv(name)))
	if raw == "" {
		return fallback
	}
	multiplier := int64(1)
	for suffix, value := range map[string]int64{"KB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30} {
		if strings.HasSuffix(raw, suffix) {
			raw = strings.TrimSpace(strings.TrimSuffix(raw, suffix))
			multiplier = value
			break
		}
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 || value > (1<<40)/multiplier {
		return fallback
	}
	return value * multiplier
}

func splitCSV(raw string) []string {
	var out []string
	for _, value := range strings.Split(raw, ",") {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
