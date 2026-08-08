package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}
type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	if s.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication is not configured")
		return
	}
	var in loginRequest
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Username) == "" || in.Password == "" {
		writeError(w, http.StatusBadRequest, "INVALID_CREDENTIALS", "username and password are required")
		return
	}
	key := r.RemoteAddr + ":" + strings.ToLower(strings.TrimSpace(in.Username))
	s.loginMu.Lock()
	failure := s.loginFailures[key]
	blocked := failure.Count >= 5 && time.Since(failure.Since) < 15*time.Minute
	s.loginMu.Unlock()
	if blocked {
		writeError(w, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "too many failed login attempts")
		return
	}
	user, token, err := s.sessions.Authenticate(r.Context(), strings.TrimSpace(in.Username), in.Password, in.TOTPCode)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		s.loginMu.Lock()
		failure = s.loginFailures[key]
		if failure.Count == 0 || time.Since(failure.Since) >= 15*time.Minute {
			failure = loginFailure{Since: time.Now()}
		}
		failure.Count++
		s.loginFailures[key] = failure
		s.loginMu.Unlock()
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid username or password")
		return
	}
	s.loginMu.Lock()
	delete(s.loginFailures, key)
	s.loginMu.Unlock()
	if err != nil {
		if errors.Is(err, auth.ErrTOTPRequired) {
			writeError(w, http.StatusUnauthorized, "TOTP_REQUIRED", "a valid TOTP code is required")
			return
		}
		writeError(w, http.StatusInternalServerError, "AUTHENTICATION_FAILED", "could not authenticate")
		return
	}
	s.setSessionCookie(w, token)
	s.setCSRFTokenCookie(w)
	s.audit(r, user, "auth.login", "session", map[string]any{"username": user.Username})
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "token_type": "Bearer", "expires_in": int64(s.sessionTTL().Seconds()), "user": user}, nil)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	user, _ := s.currentUser(r)
	if s.sessions != nil {
		if err := s.sessions.Revoke(r.Context(), sessionToken(r)); err != nil {
			writeError(w, http.StatusInternalServerError, "LOGOUT_FAILED", "could not revoke session")
			return
		}
	}
	s.audit(r, user, "auth.logout", "session", nil)
	s.clearSessionCookie(w)
	writeData(w, map[string]bool{"logged_out": true}, nil)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	user, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	writeData(w, user, nil)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
		return
	}
	if s.sessions == nil {
		writeError(w, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "authentication is not configured")
		return
	}
	user, ok := s.currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	var in changePasswordRequest
	if json.NewDecoder(r.Body).Decode(&in) != nil || in.CurrentPassword == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "current_password and new_password are required")
		return
	}
	if err := s.sessions.ChangePassword(r.Context(), user.ID, in.CurrentPassword, in.NewPassword); errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "current password is incorrect")
		return
	} else if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PASSWORD", err.Error())
		return
	}
	s.audit(r, user, "auth.change_password", "user/"+user.ID, nil)
	writeData(w, map[string]bool{"changed": true}, nil)
}

func (s *Server) currentUser(r *http.Request) (auth.User, bool) {
	if s.sessions == nil {
		return auth.User{}, false
	}
	token := sessionToken(r)
	if token == "" {
		return auth.User{}, false
	}
	u, err := s.sessions.Resolve(r.Context(), token)
	return u, err == nil
}
func sessionToken(r *http.Request) string {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	if c, err := r.Cookie("session"); err == nil {
		return c.Value
	}
	return ""
}
func (s *Server) sessionTTL() time.Duration {
	if s.sessions != nil && s.sessions.TTL > 0 {
		return s.sessions.TTL
	}
	return 24 * time.Hour
}
func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, Path: "/", MaxAge: int(s.sessionTTL().Seconds()), HttpOnly: true, Secure: strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production"), SameSite: http.SameSiteLaxMode})
}
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.SetCookie(w, &http.Cookie{Name: "csrf_token", Value: "", Path: "/", MaxAge: -1, SameSite: http.SameSiteLaxMode})
}

func (s *Server) setCSRFTokenCookie(w http.ResponseWriter) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "csrf_token", Value: hex.EncodeToString(b), Path: "/", MaxAge: int(s.sessionTTL().Seconds()), Secure: strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production"), SameSite: http.SameSiteLaxMode})
}
