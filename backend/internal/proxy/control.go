package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ConfigKey = "registrypulse:proxy:config:v1"
	StatusKey = "registrypulse:proxy:status:v1"
)

// RuntimeConfig is the small, non-secret control snapshot shared by the API
// and the registry proxy. ListenPort is intentionally configuration metadata:
// the host-side Compose port mapping still requires a container restart.
type RuntimeConfig struct {
	Enabled                bool   `json:"enabled"`
	TransportMode          string `json:"transport_mode"`
	ListenPort             int    `json:"listen_port"`
	CategoryID             string `json:"category_id"`
	RouteMaxAgeMinutes     int    `json:"route_max_age_minutes"`
	FailureCooldownSeconds int    `json:"failure_cooldown_seconds"`
	MaxConcurrent          int    `json:"max_concurrent"`
	MaxRangeMB             int    `json:"max_range_mb"`
	MaxManifestMB          int    `json:"max_manifest_mb"`
	UpdatedAt              string `json:"updated_at,omitempty"`
}

func DefaultRuntimeConfig(port int, category string, routeMaxAge, failureCooldown time.Duration, maxConcurrent int, maxRange, maxManifest int64) RuntimeConfig {
	if port <= 0 {
		port = 10800
	}
	if strings.TrimSpace(category) == "" {
		category = "dockerhub"
	}
	if routeMaxAge <= 0 {
		routeMaxAge = 2 * time.Hour
	}
	if failureCooldown <= 0 {
		failureCooldown = 30 * time.Second
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 64
	}
	if maxRange <= 0 {
		maxRange = 256 << 20
	}
	if maxManifest <= 0 {
		maxManifest = 8 << 20
	}
	return RuntimeConfig{
		Enabled:                true,
		TransportMode:          TransportModeForward,
		ListenPort:             port,
		CategoryID:             strings.TrimSpace(category),
		RouteMaxAgeMinutes:     max(1, int(routeMaxAge.Round(time.Minute)/time.Minute)),
		FailureCooldownSeconds: max(1, int(failureCooldown.Round(time.Second)/time.Second)),
		MaxConcurrent:          maxConcurrent,
		MaxRangeMB:             max(1, int(maxRange/(1<<20))),
		MaxManifestMB:          max(1, int(maxManifest/(1<<20))),
	}
}

func (c RuntimeConfig) Normalize() (RuntimeConfig, error) {
	c.TransportMode = strings.ToLower(strings.TrimSpace(c.TransportMode))
	if c.TransportMode == "" {
		c.TransportMode = TransportModeForward
	}
	if c.TransportMode != TransportModeForward && c.TransportMode != TransportModeRedirect {
		return RuntimeConfig{}, errors.New("transport mode must be forward or redirect")
	}
	c.CategoryID = strings.TrimSpace(c.CategoryID)
	if c.ListenPort < 1024 || c.ListenPort > 65535 {
		return RuntimeConfig{}, errors.New("listen port must be between 1024 and 65535")
	}
	if c.CategoryID == "" || len(c.CategoryID) > 100 || strings.ContainsAny(c.CategoryID, " \t\r\n") {
		return RuntimeConfig{}, errors.New("category id is invalid")
	}
	if c.RouteMaxAgeMinutes < 1 || c.RouteMaxAgeMinutes > 10080 {
		return RuntimeConfig{}, errors.New("route max age must be between 1 and 10080 minutes")
	}
	if c.FailureCooldownSeconds < 1 || c.FailureCooldownSeconds > 3600 {
		return RuntimeConfig{}, errors.New("failure cooldown must be between 1 and 3600 seconds")
	}
	if c.MaxConcurrent < 1 || c.MaxConcurrent > 1024 {
		return RuntimeConfig{}, errors.New("max concurrent must be between 1 and 1024")
	}
	if c.MaxRangeMB < 1 || c.MaxRangeMB > 4096 {
		return RuntimeConfig{}, errors.New("max range must be between 1 and 4096 MB")
	}
	if c.MaxManifestMB < 1 || c.MaxManifestMB > 64 {
		return RuntimeConfig{}, errors.New("max manifest must be between 1 and 64 MB")
	}
	return c, nil
}

func (c RuntimeConfig) RouteMaxAge() time.Duration {
	return time.Duration(c.RouteMaxAgeMinutes) * time.Minute
}
func (c RuntimeConfig) FailureCooldown() time.Duration {
	return time.Duration(c.FailureCooldownSeconds) * time.Second
}
func (c RuntimeConfig) MaxRangeBytes() int64    { return int64(c.MaxRangeMB) << 20 }
func (c RuntimeConfig) MaxManifestBytes() int64 { return int64(c.MaxManifestMB) << 20 }

func PublishRuntimeConfig(ctx context.Context, client redis.UniversalClient, config RuntimeConfig) error {
	if client == nil {
		return nil
	}
	normalized, err := config.Normalize()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	return client.Set(ctx, ConfigKey, payload, 0).Err()
}

func LoadRuntimeConfig(ctx context.Context, client redis.UniversalClient) (RuntimeConfig, error) {
	if client == nil {
		return RuntimeConfig{}, nil
	}
	payload, err := client.Get(ctx, ConfigKey).Bytes()
	if err != nil {
		return RuntimeConfig{}, err
	}
	var config RuntimeConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return RuntimeConfig{}, err
	}
	return config.Normalize()
}

type RuntimeStatus struct {
	Running        bool   `json:"running"`
	Enabled        bool   `json:"enabled"`
	TransportMode  string `json:"transport_mode"`
	Ready          bool   `json:"ready"`
	ActualPort     int    `json:"actual_port"`
	ConfiguredPort int    `json:"configured_port"`
	CategoryID     string `json:"category_id"`
	CandidateCount int    `json:"candidate_count"`
	LastError      string `json:"last_error,omitempty"`
	StartedAt      string `json:"started_at"`
	LastSeenAt     string `json:"last_seen_at"`
}

func PublishRuntimeStatus(ctx context.Context, client redis.UniversalClient, status RuntimeStatus) error {
	if client == nil {
		return nil
	}
	payload, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return client.Set(ctx, StatusKey, payload, 30*time.Second).Err()
}

func LoadRuntimeStatus(ctx context.Context, client redis.UniversalClient) (RuntimeStatus, error) {
	if client == nil {
		return RuntimeStatus{}, nil
	}
	payload, err := client.Get(ctx, StatusKey).Bytes()
	if err != nil {
		return RuntimeStatus{}, err
	}
	var status RuntimeStatus
	if err := json.Unmarshal(payload, &status); err != nil {
		return RuntimeStatus{}, fmt.Errorf("decode proxy status: %w", err)
	}
	return status, nil
}

type RuntimeState struct {
	Enabled        bool
	TransportMode  string
	CategoryID     string
	CandidateCount int
	Ready          bool
	LastError      string
	ConfiguredPort int
}

const (
	TransportModeForward  = "forward"
	TransportModeRedirect = "redirect"
)
