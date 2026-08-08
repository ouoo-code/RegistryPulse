package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ouoo-code/RegistryPulse/internal/auth"
	"github.com/ouoo-code/RegistryPulse/internal/database"
	"github.com/ouoo-code/RegistryPulse/internal/domain"
	httpapi "github.com/ouoo-code/RegistryPulse/internal/http"
	"github.com/redis/go-redis/v9"
)

func main() {
	port := os.Getenv("API_HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	if _, err := strconv.Atoi(port); err != nil {
		slog.Error("invalid API_HTTP_PORT", "error", err)
		os.Exit(1)
	}
	store := domain.Store(domain.NewMemoryStore())
	var dbClose func() = func() {}
	var sessions *auth.SessionStore
	var agents domain.AgentRegistry
	var db *sql.DB
	var redisClient *redis.Client
	var err error
	if os.Getenv("DATABASE_URL") != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		db, err = database.Open(ctx)
		if err != nil {
			slog.Error("database unavailable", "error", err)
			os.Exit(1)
		}
		dir := os.Getenv("MIGRATIONS_DIR")
		if dir == "" {
			dir = "migrations"
		}
		if _, err = os.Stat(dir); err != nil {
			dir = "/migrations"
		}
		if err = database.Migrate(ctx, db, dir); err != nil {
			slog.Error("database migration failed", "error", err)
			os.Exit(1)
		}
		if adminUser, adminPassword := os.Getenv("ADMIN_USERNAME"), os.Getenv("ADMIN_PASSWORD"); adminUser != "" && adminPassword != "" {
			if err = auth.EnsureAdmin(ctx, db, adminUser, adminPassword); err != nil {
				slog.Error("admin bootstrap failed", "error", err)
				os.Exit(1)
			}
		}
		store = database.NewStore(db)
		sessions = &auth.SessionStore{DB: db, TTL: 24 * time.Hour}
		agents = database.NewAgentRegistry(db)
		dbClose = func() { _ = db.Close() }
		defer dbClose()
		slog.Info("postgres store enabled", "migrations", dir)
	}
	var handler http.Handler
	var apiServer *httpapi.Server
	if agents != nil {
		apiServer = httpapi.NewWithAgentRegistry(store, sessions, agents)
	} else {
		apiServer = httpapi.New(store, sessions)
	}
	if db != nil {
		if raw := os.Getenv("REDIS_URL"); raw != "" {
			if options, parseErr := redis.ParseURL(raw); parseErr == nil {
				redisClient = redis.NewClient(options)
				defer redisClient.Close()
			}
		}
		apiServer.SetReadinessChecker(func(checkCtx context.Context) error {
			if err := database.Ping(checkCtx, db); err != nil {
				return err
			}
			if redisClient == nil {
				return nil
			}
			return redisClient.Ping(checkCtx).Err()
		})
	}
	handler = apiServer.Routes()
	srv := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	slog.Info("api listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
