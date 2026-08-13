package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// Initialize applies the fixed SQL files that make up the current fresh-install
// baseline. These are not incremental migrations: a new deployment starts
// with an empty database, and the marker only prevents API and Worker from
// repeating initialization after a restart.
func Initialize(ctx context.Context, db *sql.DB, dir string) error {
	files := []string{"001_initial.sql", "004_seed_defaults.sql", "018_seed_anye_status_sources.sql", "019_access_security.sql", "023_remove_proxy_listen_port_setting.sql", "024_probe_result_probe_mode.sql", "025_probe_service_enabled.sql"}
	for _, name := range files {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("read database initialization file %s: %w", name, err)
		}
	}
	// API and Worker both initialize the database during startup. The advisory
	// lock makes the first-install sequence single-writer without requiring a
	// separate migration container or an upgrade framework.
	if _, err := db.ExecContext(ctx, `SELECT pg_advisory_lock(813742091)`); err != nil {
		return fmt.Errorf("lock database initialization: %w", err)
	}
	defer func() { _, _ = db.ExecContext(context.Background(), `SELECT pg_advisory_unlock(813742091)`) }()
	var err error
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
