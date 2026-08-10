package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/proxy"
	"github.com/redis/go-redis/v9"
)

func (s *Server) adminProxy(w http.ResponseWriter, r *http.Request) {
	permission := "settings.read"
	if r.Method == http.MethodPut {
		permission = "settings.write"
	}
	user, ok := s.requirePermission(w, r, permission)
	if !ok {
		return
	}
	db := s.db(w)
	if db == nil {
		return
	}

	config, err := loadProxyRuntimeConfig(r.Context(), db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PROXY_CONFIG_QUERY_FAILED", "could not load proxy configuration")
		return
	}
	if r.Method == http.MethodGet {
		writeData(w, proxyAdminData(r.Context(), s.redis, config), nil)
		return
	}
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}

	var input proxy.RuntimeConfig
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "invalid proxy configuration")
		return
	}
	// The proxy bind port is owned by Docker Compose, not by the runtime
	// control plane. Keep the container port from the deployment environment
	// even when older clients or stored payloads omit it.
	input.ListenPort = config.ListenPort
	input.UpdatedAt = ""
	input, err = input.Normalize()
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PROXY_CONFIG", err.Error())
		return
	}
	if err := saveProxyRuntimeConfig(r.Context(), db, input); err != nil {
		writeError(w, http.StatusInternalServerError, "PROXY_CONFIG_UPDATE_FAILED", "could not save proxy configuration")
		return
	}
	publishErr := proxy.PublishRuntimeConfig(r.Context(), s.redis, input)
	s.audit(r, user, "proxy.config.update", "registry-proxy", input)
	data := proxyAdminData(r.Context(), s.redis, input)
	if publishErr != nil {
		data["control_snapshot_published"] = false
		writeJSON(w, http.StatusServiceUnavailable, data, map[string]any{"warning": "configuration saved, but Redis control snapshot is unavailable"})
		return
	}
	data["control_snapshot_published"] = true
	writeData(w, data, nil)
}

func proxyAdminData(ctx context.Context, client redis.UniversalClient, config proxy.RuntimeConfig) map[string]any {
	status, err := proxy.LoadRuntimeStatus(ctx, client)
	statusAvailable := err == nil && status.LastSeenAt != ""
	if statusAvailable {
		if parsed, parseErr := time.Parse(time.RFC3339, status.LastSeenAt); parseErr == nil && time.Since(parsed) > 30*time.Second {
			statusAvailable = false
		}
	}
	return map[string]any{
		"config":                     config,
		"status":                     status,
		"status_available":           statusAvailable,
		"control_snapshot_published": client != nil && err == nil,
	}
}

func loadProxyRuntimeConfig(ctx context.Context, db *sql.DB) (proxy.RuntimeConfig, error) {
	config := proxy.DefaultRuntimeConfig(configuredProxyPort(), envOrDefault("PROXY_CATEGORY_ID", "dockerhub"), 0, 0, 64, 256<<20, 8<<20)
	rows, err := db.QueryContext(ctx, `SELECT key,value FROM system_settings WHERE key LIKE 'proxy_%'`)
	if err != nil {
		return config, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var raw []byte
		if err := rows.Scan(&key, &raw); err != nil {
			return config, err
		}
		value := unwrapSetting(raw)
		switch key {
		case "proxy_enabled":
			_ = json.Unmarshal(value, &config.Enabled)
		case "proxy_transport_mode":
			_ = json.Unmarshal(value, &config.TransportMode)
		case "proxy_category_id":
			_ = json.Unmarshal(value, &config.CategoryID)
		case "proxy_route_max_age_minutes":
			_ = json.Unmarshal(value, &config.RouteMaxAgeMinutes)
		case "proxy_failure_cooldown_seconds":
			_ = json.Unmarshal(value, &config.FailureCooldownSeconds)
		case "proxy_max_concurrent":
			_ = json.Unmarshal(value, &config.MaxConcurrent)
		case "proxy_max_range_mb":
			_ = json.Unmarshal(value, &config.MaxRangeMB)
		case "proxy_max_manifest_mb":
			_ = json.Unmarshal(value, &config.MaxManifestMB)
		}
	}
	if err := rows.Err(); err != nil {
		return config, err
	}
	return config.Normalize()
}

func saveProxyRuntimeConfig(ctx context.Context, db *sql.DB, config proxy.RuntimeConfig) error {
	values := map[string]any{
		"proxy_enabled":                  config.Enabled,
		"proxy_transport_mode":           config.TransportMode,
		"proxy_category_id":              config.CategoryID,
		"proxy_route_max_age_minutes":    config.RouteMaxAgeMinutes,
		"proxy_failure_cooldown_seconds": config.FailureCooldownSeconds,
		"proxy_max_concurrent":           config.MaxConcurrent,
		"proxy_max_range_mb":             config.MaxRangeMB,
		"proxy_max_manifest_mb":          config.MaxManifestMB,
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for key, value := range values {
		payload, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO system_settings(key,value,updated_at) VALUES($1,$2::jsonb,now()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=now()`, key, string(payload)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func unwrapSetting(raw []byte) json.RawMessage {
	var value any
	if json.Unmarshal(raw, &value) == nil {
		if object, ok := value.(map[string]any); ok {
			if nested, exists := object["value"]; exists {
				if encoded, err := json.Marshal(nested); err == nil {
					return encoded
				}
			}
		}
	}
	return raw
}

func configuredProxyPort() int {
	if value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("PROXY_HTTP_PORT"))); err == nil && value >= 1024 && value <= 65535 {
		return value
	}
	return 10800
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
