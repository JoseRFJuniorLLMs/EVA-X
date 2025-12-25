package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func Connect(databaseURL string) (*DB, error) {
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is empty. Please check your .env file or environment variables")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse DATABASE_URL: %w", err)
	}

	// Configurações do pool
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Testa conexão
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return db.Pool.Ping(ctx)
}

// HealthDetailed realiza um check mais profundo (conectividade + query + stats)
func (db *DB) HealthDetailed(ctx context.Context) error {
	// 1. Testa ping
	if err := db.Health(); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// 2. Testa query simples
	var count int
	err := db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM idosos").Scan(&count)
	if err != nil {
		return fmt.Errorf("query test failed: %w", err)
	}

	// 3. Verifica se o pool está exaurido
	stats := db.Pool.Stat()
	if stats.IdleConns() == 0 && stats.AcquiredConns() >= stats.MaxConns() {
		return fmt.Errorf("connection pool exhausted (MaxConns: %d)", stats.MaxConns())
	}

	return nil
}

// AcquireLock tenta obter um lock consultivo do Postgres para evitar processamento duplicado
func (db *DB) AcquireLock(ctx context.Context, lockID int) (bool, error) {
	var acquired bool
	query := "SELECT pg_try_advisory_xact_lock($1)"
	// Nota: advisory_xact_lock dura até o fim da transação.
	// Para locks de longa duração ou manuais, usamos pg_try_advisory_lock
	query = "SELECT pg_try_advisory_lock($1)"
	err := db.Pool.QueryRow(ctx, query, lockID).Scan(&acquired)
	return acquired, err
}

// ReleaseLock libera o lock consultivo
func (db *DB) ReleaseLock(ctx context.Context, lockID int) (bool, error) {
	var released bool
	query := "SELECT pg_advisory_unlock($1)"
	err := db.Pool.QueryRow(ctx, query, lockID).Scan(&released)
	return released, err
}
