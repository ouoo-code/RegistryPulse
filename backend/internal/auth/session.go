package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type User struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	Role               string `json:"role,omitempty"`
	TOTPEnabled        bool   `json:"totp_enabled,omitempty"`
	MustChangePassword bool   `json:"must_change_password,omitempty"`
}
type SessionStore struct {
	DB  *sql.DB
	TTL time.Duration
}

var ErrTOTPRequired = fmt.Errorf("totp verification failed")

// HasPermission evaluates the user's effective role permissions. Admin is
// intentionally an explicit superuser role, while all other roles must be
// granted a named permission through role_permissions.
func (s *SessionStore) HasPermission(ctx context.Context, userID, permission string) (bool, error) {
	if s == nil || s.DB == nil {
		return false, fmt.Errorf("authentication store is unavailable")
	}
	var allowed bool
	err := s.DB.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_roles ur JOIN roles r ON r.id=ur.role_id
			WHERE ur.user_id=$1 AND (r.name='admin' OR EXISTS(
				SELECT 1 FROM role_permissions rp JOIN permissions p ON p.id=rp.permission_id
				WHERE rp.role_id=r.id AND p.name=$2)))`, userID, permission).Scan(&allowed)
	return allowed, err
}

func (s *SessionStore) Audit(ctx context.Context, userID, action, resource string, details any) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("authentication store is unavailable")
	}
	payload, err := json.Marshal(details)
	if err != nil {
		return err
	}
	var actor any
	if userID != "" {
		actor = userID
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO audit_logs(user_id,action,resource,details) VALUES($1,$2,$3,$4::jsonb)`, actor, action, resource, string(payload))
	return err
}

func (s *SessionStore) Authenticate(ctx context.Context, username, password, totpCode string) (User, string, error) {
	if s == nil || s.DB == nil {
		return User{}, "", fmt.Errorf("authentication store is unavailable")
	}
	var user User
	var hash, totpSecret string
	err := s.DB.QueryRowContext(ctx, `SELECT u.id::text,u.username,u.password_hash,COALESCE((SELECT r.name FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=u.id ORDER BY CASE WHEN r.name='admin' THEN 0 ELSE 1 END,r.name LIMIT 1),'user'),COALESCE(u.totp_secret,''),COALESCE(u.totp_enabled,false),COALESCE(u.must_change_password,false) FROM users u WHERE u.username=$1 AND u.is_active`, username).Scan(&user.ID, &user.Username, &hash, &user.Role, &totpSecret, &user.TOTPEnabled, &user.MustChangePassword)
	if err == sql.ErrNoRows || !CheckPassword(hash, password) {
		return User{}, "", ErrInvalidCredentials
	}
	if err != nil {
		return User{}, "", fmt.Errorf("authenticate: %w", err)
	}
	if user.TOTPEnabled && !VerifyTOTP(totpSecret, totpCode, time.Now().UTC()) {
		return User{}, "", ErrTOTPRequired
	}
	token, err := newToken()
	if err != nil {
		return User{}, "", err
	}
	ttl := s.TTL
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at) VALUES($1,$2,$3)`, tokenHash(token), user.ID, time.Now().UTC().Add(ttl))
	if err != nil {
		return User{}, "", fmt.Errorf("create session: %w", err)
	}
	return user, token, nil
}

func (s *SessionStore) Resolve(ctx context.Context, token string) (User, error) {
	if s == nil || s.DB == nil || token == "" {
		return User{}, ErrInvalidCredentials
	}
	var u User
	err := s.DB.QueryRowContext(ctx, `SELECT u.id::text,u.username,COALESCE((SELECT r.name FROM user_roles ur JOIN roles r ON r.id=ur.role_id WHERE ur.user_id=u.id ORDER BY CASE WHEN r.name='admin' THEN 0 ELSE 1 END,r.name LIMIT 1),'user'),COALESCE(u.totp_enabled,false),COALESCE(u.must_change_password,false) FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND u.is_active`, tokenHash(token)).Scan(&u.ID, &u.Username, &u.Role, &u.TOTPEnabled, &u.MustChangePassword)
	if err == sql.ErrNoRows {
		return User{}, ErrInvalidCredentials
	}
	return u, err
}
func (s *SessionStore) Revoke(ctx context.Context, token string) error {
	if s == nil || s.DB == nil || token == "" {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=$1`, tokenHash(token))
	return err
}

// RevokeAll rotates a user's session credentials after a sensitive account
// change, preventing previously issued bearer/session tokens from remaining
// valid indefinitely.
func (s *SessionStore) RevokeAll(ctx context.Context, userID string) error {
	if s == nil || s.DB == nil || userID == "" {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID)
	return err
}
func (s *SessionStore) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if s == nil || s.DB == nil {
		return fmt.Errorf("authentication store is unavailable")
	}
	var hash string
	if err := s.DB.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id=$1 AND is_active`, userID).Scan(&hash); err == sql.ErrNoRows {
		return ErrInvalidCredentials
	} else if err != nil {
		return err
	}
	if !CheckPassword(hash, currentPassword) {
		return ErrInvalidCredentials
	}
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE users SET password_hash=$1,updated_at=now() WHERE id=$2`, newHash, userID)
	if err == nil {
		err = s.RevokeAll(ctx, userID)
	}
	return err
}
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
