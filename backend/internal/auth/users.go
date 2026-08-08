package auth

import (
	"context"
	"database/sql"
	"fmt"
)

func CreateUser(ctx context.Context, db *sql.DB, username, password string) (string, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return "", err
	}
	var id string
	err = db.QueryRowContext(ctx, `INSERT INTO users(username,password_hash) VALUES($1,$2) RETURNING id::text`, username, hash).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}
	return id, nil
}

// EnsureAdmin creates the configured bootstrap administrator when it does not
// exist, and makes sure that account has the admin role. It never replaces an
// existing password, so changing ADMIN_PASSWORD cannot silently reset access.
func EnsureAdmin(ctx context.Context, db *sql.DB, username, password string) error {
	if db == nil || username == "" {
		return fmt.Errorf("admin database or username is missing")
	}
	var userID string
	err := db.QueryRowContext(ctx, `SELECT id::text FROM users WHERE username=$1`, username).Scan(&userID)
	if err == sql.ErrNoRows {
		if password == "" {
			return fmt.Errorf("admin user does not exist and ADMIN_PASSWORD is empty")
		}
		userID, err = CreateUser(ctx, db, username, password)
	}
	if err != nil {
		return err
	}
	var roleID string
	if err = db.QueryRowContext(ctx, `INSERT INTO roles(name) VALUES('admin') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id::text`).Scan(&roleID); err != nil {
		return fmt.Errorf("ensure admin role: %w", err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, userID, roleID)
	if err != nil {
		return fmt.Errorf("assign admin role: %w", err)
	}
	return nil
}
