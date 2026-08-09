package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open opens PostgreSQL using DATABASE_URL (or DATABASE_DSN) and verifies it
// is reachable. The caller owns the returned *sql.DB and must close it.
func Open(ctx context.Context) (*sql.DB, error) {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	}
	if dsn == "" {
		return nil, errors.New("DATABASE_URL or DATABASE_DSN is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := Ping(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func Ping(ctx context.Context, db *sql.DB) error {
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres ping: %w", err)
	}
	return nil
}

// Initialize applies the initial SQL files in lexical order. Each file is one
// transaction; schema_migrations is only a restart marker for the current
// installation baseline, not an upgrade path for historical schemas.
func Initialize(ctx context.Context, db *sql.DB, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read database initialization files: %w", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	if _, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create initialization marker: %w", err)
	}
	for _, name := range files {
		var applied bool
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, name).Scan(&applied); err != nil {
			return fmt.Errorf("check initialization file %s: %w", name, err)
		}
		if applied {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read initialization file %s: %w", name, err)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin initialization file %s: %w", name, err)
		}
		if _, err = tx.ExecContext(ctx, string(contents)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply initialization file %s: %w", name, err)
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record initialization file %s: %w", name, err)
		}
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("commit initialization file %s: %w", name, err)
		}
	}
	return nil
}
