package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/ouoo-code/RegistryPulse/internal/auth"
)

func (s *Server) hasPermission(r *http.Request, permission string) (auth.User, bool) {
	if s.adminToken != "" && bearerToken(r) == s.adminToken {
		return auth.User{Role: "admin"}, true
	}
	user, ok := s.currentUser(r)
	if !ok {
		return auth.User{}, false
	}
	if user.Role == "admin" {
		return user, true
	}
	allowed, err := s.sessions.HasPermission(r.Context(), user.ID, permission)
	return user, err == nil && allowed
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) >= 7 && (h[:7] == "Bearer " || h[:7] == "bearer ") {
		return h[7:]
	}
	return ""
}

func (s *Server) requirePermission(w http.ResponseWriter, r *http.Request, permission string) (auth.User, bool) {
	user, ok := s.hasPermission(r, permission)
	if !ok {
		writeError(w, http.StatusUnauthorized, "FORBIDDEN", "required permission: "+permission)
		return auth.User{}, false
	}
	return user, true
}

// requireAdmin protects account and role administration. These operations
// change who can access the console, so a normal settings.write permission is
// intentionally not sufficient.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	user, ok := s.hasPermission(r, "")
	if !ok || user.Role != "admin" {
		writeError(w, http.StatusForbidden, "ADMIN_REQUIRED", "administrator role is required")
		return auth.User{}, false
	}
	return user, true
}

func (s *Server) audit(ctxRequest *http.Request, user auth.User, action, resource string, details any) {
	if s.sessions == nil {
		return
	}
	_ = s.sessions.Audit(ctxRequest.Context(), user.ID, user.Username, action, resource, details)
}

func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	if _, ok := s.requirePermission(w, r, "audit.read"); !ok {
		return
	}
	if s.sessions == nil || s.sessions.DB == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication is not configured")
		return
	}
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 && value <= 500 {
			limit = value
		}
	}
	rows, err := s.sessions.DB.QueryContext(r.Context(), `SELECT id,user_id::text,actor_username,action,resource,details,created_at FROM audit_logs ORDER BY created_at DESC,id DESC LIMIT $1`, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "AUDIT_QUERY_FAILED", "could not query audit logs")
		return
	}
	defer rows.Close()
	type entry struct {
		ID        int64           `json:"id"`
		UserID    string          `json:"user_id,omitempty"`
		Actor     string          `json:"actor_username,omitempty"`
		Action    string          `json:"action"`
		Resource  string          `json:"resource"`
		Details   json.RawMessage `json:"details"`
		CreatedAt any             `json:"created_at"`
	}
	out := []entry{}
	for rows.Next() {
		var value entry
		var userID sql.NullString
		var details []byte
		if err := rows.Scan(&value.ID, &userID, &value.Actor, &value.Action, &value.Resource, &details, &value.CreatedAt); err != nil {
			writeError(w, 500, "AUDIT_QUERY_FAILED", "could not read audit logs")
			return
		}
		if userID.Valid {
			value.UserID = userID.String
		}
		value.Details = json.RawMessage(details)
		out = append(out, value)
	}
	if err := rows.Err(); err != nil {
		writeError(w, 500, "AUDIT_QUERY_FAILED", "could not read audit logs")
		return
	}
	writeData(w, out, map[string]any{"count": len(out)})
}
