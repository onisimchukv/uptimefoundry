package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvec "github.com/pgvector/pgvector-go/pgx"

	"github.com/onisimchukv/uptimefoundry/internal/config"
)

// DB wraps the PostgreSQL connection pool and health checks.
type DB struct {
	Pool        *pgxpool.Pool
	Logger      *slog.Logger
	HasAGE      bool
	HasVector   bool
}

// New connects to PostgreSQL using the provided configuration and initializes type handlers.
func New(ctx context.Context, cfg *config.DatabaseConfig, logger *slog.Logger) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	if cfg.MaxOpenConnections > 0 {
		poolCfg.MaxConns = int32(cfg.MaxOpenConnections)
	}
	if cfg.MaxIdleConnections > 0 {
		poolCfg.MinConns = int32(cfg.MaxIdleConnections)
	}
	if cfg.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	}

	// Register pgvector data types for all connections in pool
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvec.RegisterTypes(ctx, conn)
	}

	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres pool: %w", err)
	}

	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	db := &DB{
		Pool:   pool,
		Logger: logger,
	}

	db.detectExtensions(ctx)

	return db, nil
}

// Close gracefully closes all database connections.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// detectExtensions checks if age and vector are loaded.
func (db *DB) detectExtensions(ctx context.Context) {
	var ageExists bool
	_ = db.Pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'age')").Scan(&ageExists)
	db.HasAGE = ageExists

	var vectorExists bool
	_ = db.Pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&vectorExists)
	db.HasVector = vectorExists

	db.Logger.Info("postgresql extensions detected",
		slog.Bool("apache_age", db.HasAGE),
		slog.Bool("pgvector", db.HasVector),
	)
}

// Ping verifies active connectivity.
func (db *DB) Ping(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}
