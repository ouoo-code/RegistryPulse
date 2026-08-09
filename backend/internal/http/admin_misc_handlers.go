package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/auth"
	"github.com/ouoo-code/RegistryPulse/internal/database"
	"github.com/ouoo-code/RegistryPulse/internal/domain"
	"github.com/ouoo-code/RegistryPulse/internal/notification"
)

func (s *Server) adminTOTP(w http.ResponseWriter, r *http.Request) {
	permission := "settings.read"
	if r.Method != http.MethodGet {
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
	if r.Method == http.MethodGet {
		var enabled bool
		var secret string
		if err := db.QueryRowContext(r.Context(), `SELECT COALESCE(totp_enabled,false),COALESCE(totp_secret,'') FROM users WHERE id=$1`, user.ID).Scan(&enabled, &secret); err != nil {
			writeError(w, 404, "USER_NOT_FOUND", "user not found")
			return
		}
		// A TOTP secret is equivalent to a password. It is only returned once
		// by the generate action and must never be exposed by a status query.
		writeData(w, map[string]any{"enabled": enabled, "configured": secret != ""}, nil)
		return
	}
	var input struct {
		Action string `json:"action"`
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeError(w, 400, "INVALID_JSON", "invalid request body")
		return
	}
	if input.Action == "generate" {
		bytes := make([]byte, 20)
		if _, err := rand.Read(bytes); err != nil {
			writeError(w, 500, "TOTP_GENERATE_FAILED", "could not generate TOTP secret")
			return
		}
		secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
		uri := "otpauth://totp/Container%20Registry%20Monitor:" + url.QueryEscape(user.Username) + "?secret=" + secret + "&issuer=Container%20Registry%20Monitor"
		writeData(w, map[string]string{"secret": secret, "otpauth_uri": uri}, nil)
		return
	}
	if input.Action == "enable" {
		secret := strings.TrimRight(strings.ToUpper(strings.TrimSpace(input.Secret)), "=")
		if secret == "" || !auth.VerifyTOTP(secret, input.Code, time.Now().UTC()) {
			writeError(w, 400, "TOTP_INVALID", "the TOTP code is invalid")
			return
		}
		if _, err := db.ExecContext(r.Context(), `UPDATE users SET totp_secret=$1,totp_enabled=true WHERE id=$2`, secret, user.ID); err != nil {
			writeError(w, 500, "TOTP_UPDATE_FAILED", "could not enable TOTP")
			return
		}
		s.audit(r, user, "auth.totp.enable", "user/"+user.ID, nil)
		writeData(w, map[string]bool{"enabled": true}, nil)
		return
	}
	if input.Action == "disable" {
		var secret string
		if err := db.QueryRowContext(r.Context(), `SELECT COALESCE(totp_secret,'') FROM users WHERE id=$1`, user.ID).Scan(&secret); err != nil || !auth.VerifyTOTP(secret, input.Code, time.Now().UTC()) {
			writeError(w, 400, "TOTP_INVALID", "the TOTP code is invalid")
			return
		}
		if _, err := db.ExecContext(r.Context(), `UPDATE users SET totp_enabled=false WHERE id=$1`, user.ID); err != nil {
			writeError(w, 500, "TOTP_UPDATE_FAILED", "could not disable TOTP")
			return
		}
		s.audit(r, user, "auth.totp.disable", "user/"+user.ID, nil)
		writeData(w, map[string]bool{"enabled": false}, nil)
		return
	}
	writeError(w, 400, "INVALID_TOTP_ACTION", "unsupported TOTP action")
}

type dbHandle interface{ DBHandle() *sql.DB }

func (s *Server) db(w http.ResponseWriter) *sql.DB {
	provider, ok := s.store.(dbHandle)
	if !ok || provider.DBHandle() == nil {
		writeError(w, 503, "DATABASE_REQUIRED", "this administration operation requires PostgreSQL")
		return nil
	}
	return provider.DBHandle()
}

func (s *Server) adminCategories(w http.ResponseWriter, r *http.Request) {
	permission := "source.read"
	if r.Method != http.MethodGet {
		permission = "source.write"
	}
	user, ok := s.requirePermission(w, r, permission)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		writeData(w, s.store.Categories(), map[string]any{"count": len(s.store.Categories())})
		return
	}
	db := s.db(w)
	if db == nil {
		return
	}
	if r.Method == http.MethodPost {
		var in domain.Category
		if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.Slug) == "" || strings.TrimSpace(in.Name) == "" {
			writeError(w, 400, "INVALID_CATEGORY", "id, slug and name are required")
			return
		}
		if err := validateCategoryDefaultTestImage(r.Context(), db, in.ID, in.DefaultTestImageID, in.DefaultProbeMode); err != nil {
			writeError(w, 400, "INVALID_TEST_IMAGE_SCOPE", err.Error())
			return
		}
		_, err := db.ExecContext(r.Context(), `INSERT INTO registry_categories(id,slug,name,description,icon,official_url,default_test_repository,default_test_tag,default_test_image_id,default_probe_mode,default_timeout_seconds,default_manifest_path,auth_type,enabled,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')::uuid,$10,$11,$12,$13,$14,$15)`, in.ID, in.Slug, in.Name, in.Description, in.Icon, in.OfficialURL, in.DefaultTestRepository, in.DefaultTestTag, in.DefaultTestImageID, in.DefaultProbeMode, in.DefaultTimeoutSeconds, in.DefaultManifestPath, in.AuthType, in.Enabled, in.SortOrder)
		if err != nil {
			writeError(w, 500, "CATEGORY_CREATE_FAILED", "could not create category")
			return
		}
		s.audit(r, user, "category.create", in.ID, in)
		writeJSON(w, 201, in, nil)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/categories/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, 400, "INVALID_CATEGORY", "category id is required")
		return
	}
	id := parts[0]
	if r.Method == http.MethodDelete {
		var sourceCount int
		if err := db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM registry_sources WHERE category_id=$1`, id).Scan(&sourceCount); err != nil {
			writeError(w, 500, "CATEGORY_DELETE_CHECK_FAILED", "could not check category references")
			return
		}
		if sourceCount > 0 {
			writeError(w, 409, "CATEGORY_IN_USE", "category is referenced by one or more sources")
			return
		}
		if _, err := db.ExecContext(r.Context(), `DELETE FROM registry_categories WHERE id=$1`, id); err != nil {
			writeError(w, 500, "CATEGORY_DELETE_FAILED", "could not delete category")
			return
		}
		s.audit(r, user, "category.delete", id, nil)
		writeData(w, map[string]bool{"deleted": true}, nil)
		return
	}
	if r.Method == http.MethodPut {
		var in domain.Category
		if json.NewDecoder(r.Body).Decode(&in) != nil {
			writeError(w, 400, "INVALID_JSON", "invalid request body")
			return
		}
		if err := validateCategoryDefaultTestImage(r.Context(), db, id, in.DefaultTestImageID, in.DefaultProbeMode); err != nil {
			writeError(w, 400, "INVALID_TEST_IMAGE_SCOPE", err.Error())
			return
		}
		res, err := db.ExecContext(r.Context(), `UPDATE registry_categories SET slug=$1,name=$2,description=$3,icon=$4,official_url=$5,default_test_repository=$6,default_test_tag=$7,default_test_image_id=NULLIF($8,'')::uuid,default_probe_mode=$9,default_timeout_seconds=$10,default_manifest_path=$11,auth_type=$12,enabled=$13,sort_order=$14 WHERE id=$15`, in.Slug, in.Name, in.Description, in.Icon, in.OfficialURL, in.DefaultTestRepository, in.DefaultTestTag, in.DefaultTestImageID, in.DefaultProbeMode, in.DefaultTimeoutSeconds, in.DefaultManifestPath, in.AuthType, in.Enabled, in.SortOrder, id)
		if err != nil {
			writeError(w, 500, "CATEGORY_UPDATE_FAILED", "could not update category")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, 404, "CATEGORY_NOT_FOUND", "category not found")
			return
		}
		s.audit(r, user, "category.update", id, in)
		writeData(w, in, nil)
		return
	}
	writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
}

func validateCategoryDefaultTestImage(ctx context.Context, db *sql.DB, categoryID, imageID, mode string) error {
	if strings.TrimSpace(imageID) == "" {
		return nil
	}
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "registry"
	}
	if !supportedProbeMode(mode) {
		return errors.New("unsupported default_probe_mode")
	}
	if err := database.TestImageApplicable(ctx, db, imageID, categoryID, mode); err != nil {
		if errors.Is(err, database.ErrTestImageNotApplicable) {
			return errors.New("default_test_image_id is not applicable to category_id and default_probe_mode")
		}
		if errors.Is(err, domain.ErrNotFound) {
			return errors.New("default_test_image_id was not found")
		}
		return err
	}
	return nil
}

func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
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
	if r.Method == http.MethodGet {
		rows, err := db.QueryContext(r.Context(), `SELECT key,value FROM system_settings ORDER BY key`)
		if err != nil {
			writeError(w, 500, "SETTINGS_QUERY_FAILED", "could not query settings")
			return
		}
		defer rows.Close()
		out := map[string]json.RawMessage{}
		for rows.Next() {
			var key string
			var value []byte
			if rows.Scan(&key, &value) == nil {
				out[key] = json.RawMessage(value)
			}
		}
		writeData(w, out, nil)
		return
	}
	if r.Method != http.MethodPut {
		writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	var input map[string]any
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeError(w, 400, "INVALID_JSON", "invalid request body")
		return
	}
	for key, value := range input {
		payload, err := json.Marshal(value)
		if err != nil {
			writeError(w, 400, "INVALID_SETTING", "invalid setting")
			return
		}
		if _, err = db.ExecContext(r.Context(), `INSERT INTO system_settings(key,value,updated_at) VALUES($1,$2::jsonb,now()) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=now()`, key, string(payload)); err != nil {
			writeError(w, 500, "SETTINGS_UPDATE_FAILED", "could not update settings")
			return
		}
	}
	s.audit(r, user, "settings.update", "system", input)
	writeData(w, input, nil)
}

func (s *Server) adminNotifications(w http.ResponseWriter, r *http.Request) {
	permission := "settings.read"
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
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
	if strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/test") {
		if r.Method != http.MethodPost {
			writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		id := strings.TrimSuffix(strings.TrimRight(r.URL.Path, "/"), "/test")
		id = strings.Trim(strings.TrimPrefix(id, "/api/v1/admin/notifications/"), "/")
		var channel notification.Channel
		var raw []byte
		if err := db.QueryRowContext(r.Context(), `SELECT type,name,enabled,config FROM notification_channels WHERE id=$1`, id).Scan(&channel.Type, &channel.Name, &channel.Enabled, &raw); err != nil {
			writeError(w, 404, "NOTIFICATION_NOT_FOUND", "notification not found")
			return
		}
		if err := json.Unmarshal(raw, &channel.Config); err != nil {
			writeError(w, 500, "NOTIFICATION_INVALID_CONFIG", "notification configuration is invalid")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		event := notification.Event{Title: "Container Registry Monitor test", Message: "Notification test message", Status: "test"}
		if err := notification.Send(ctx, channel, event); err != nil {
			_, _ = db.ExecContext(r.Context(), `INSERT INTO notification_logs(channel_id,event_type,status,attempts,error) VALUES($1,$2,'failed',1,$3)`, id, "test", err.Error())
			writeError(w, 502, "NOTIFICATION_SEND_FAILED", err.Error())
			return
		}
		_, _ = db.ExecContext(r.Context(), `INSERT INTO notification_logs(channel_id,event_type,status,attempts) VALUES($1,$2,'sent',1)`, id, "test")
		s.audit(r, user, "notification.test", id, nil)
		writeData(w, map[string]any{"id": id, "queued": true}, nil)
		return
	}
	if r.Method == http.MethodGet {
		rows, err := db.QueryContext(r.Context(), `SELECT id::text,type,name,enabled,config,created_at,updated_at FROM notification_channels ORDER BY name`)
		if err != nil {
			writeError(w, 500, "NOTIFICATION_QUERY_FAILED", "could not query notifications")
			return
		}
		defer rows.Close()
		type item struct {
			ID        string          `json:"id"`
			Type      string          `json:"type"`
			Name      string          `json:"name"`
			Enabled   bool            `json:"enabled"`
			Config    json.RawMessage `json:"config"`
			CreatedAt any             `json:"created_at"`
			UpdatedAt any             `json:"updated_at"`
		}
		out := []item{}
		for rows.Next() {
			var v item
			var raw []byte
			if rows.Scan(&v.ID, &v.Type, &v.Name, &v.Enabled, &raw, &v.CreatedAt, &v.UpdatedAt) == nil {
				v.Config = json.RawMessage(redactNotificationConfig(raw))
				out = append(out, v)
			}
		}
		writeData(w, out, map[string]any{"count": len(out)})
		return
	}
	if r.Method == http.MethodDelete {
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/notifications/"), "/")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, 400, "INVALID_NOTIFICATION", "notification id is required")
			return
		}
		res, err := db.ExecContext(r.Context(), `DELETE FROM notification_channels WHERE id=$1`, id)
		if err != nil {
			writeError(w, 500, "NOTIFICATION_DELETE_FAILED", "could not delete notification")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, 404, "NOTIFICATION_NOT_FOUND", "notification not found")
			return
		}
		s.audit(r, user, "notification.delete", id, nil)
		writeData(w, map[string]bool{"deleted": true}, nil)
		return
	}
	var in struct {
		ID      string         `json:"id"`
		Type    string         `json:"type"`
		Name    string         `json:"name"`
		Enabled bool           `json:"enabled"`
		Config  map[string]any `json:"config"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.Type == "" || in.Name == "" {
		writeError(w, 400, "INVALID_NOTIFICATION", "type and name are required")
		return
	}
	raw, _ := json.Marshal(in.Config)
	if r.Method == http.MethodPost {
		err := db.QueryRowContext(r.Context(), `INSERT INTO notification_channels(type,name,enabled,config) VALUES($1,$2,$3,$4::jsonb) RETURNING id::text`, in.Type, in.Name, in.Enabled, string(raw)).Scan(&in.ID)
		if err != nil {
			writeError(w, 500, "NOTIFICATION_CREATE_FAILED", "could not create notification")
			return
		}
		in.Config = redactNotificationMap(in.Config)
		s.audit(r, user, "notification.create", in.ID, in)
		writeJSON(w, 201, in, nil)
		return
	}
	if r.Method == http.MethodPut {
		if in.ID == "" {
			in.ID = strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/notifications/"), "/")
		}
		var previousRaw []byte
		if err := db.QueryRowContext(r.Context(), `SELECT config FROM notification_channels WHERE id=$1`, in.ID).Scan(&previousRaw); err != nil {
			writeError(w, 404, "NOTIFICATION_NOT_FOUND", "notification not found")
			return
		}
		var previous map[string]any
		_ = json.Unmarshal(previousRaw, &previous)
		for key, value := range in.Config {
			if value == "***" && previous[key] != nil {
				in.Config[key] = previous[key]
			}
		}
		raw, _ = json.Marshal(in.Config)
		res, err := db.ExecContext(r.Context(), `UPDATE notification_channels SET type=$1,name=$2,enabled=$3,config=$4::jsonb,updated_at=now() WHERE id=$5`, in.Type, in.Name, in.Enabled, string(raw), in.ID)
		if err != nil {
			writeError(w, 500, "NOTIFICATION_UPDATE_FAILED", "could not update notification")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, 404, "NOTIFICATION_NOT_FOUND", "notification not found")
			return
		}
		in.Config = redactNotificationMap(in.Config)
		s.audit(r, user, "notification.update", in.ID, in)
		writeData(w, in, nil)
		return
	}
	writeError(w, 405, "METHOD_NOT_ALLOWED", "method not allowed")
}

func redactNotificationConfig(raw []byte) []byte {
	var config map[string]any
	if json.Unmarshal(raw, &config) != nil {
		return []byte(`{}`)
	}
	return mustJSON(redactNotificationMap(config))
}

func redactNotificationMap(config map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range config {
		lower := strings.ToLower(key)
		if lower == "token" || lower == "secret" || lower == "password" || strings.HasSuffix(lower, "_token") || strings.HasSuffix(lower, "_secret") || strings.HasSuffix(lower, "_password") {
			out[key] = "***"
		} else {
			out[key] = value
		}
	}
	return out
}

func mustJSON(value any) []byte { raw, _ := json.Marshal(value); return raw }
