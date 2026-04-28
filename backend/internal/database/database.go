package database

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*DB, error) {
	poolcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	var lastErr error
	var pool *pgxpool.Pool
	for i := 0; i < 3; i++ {
		pool, lastErr = pgxpool.NewWithConfig(ctx, poolcfg)
		if lastErr != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		if lastErr = pool.Ping(ctx); lastErr != nil {
			pool.Close()
			pool = nil
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	if pool == nil {
		if lastErr == nil {
			lastErr = fmt.Errorf("failed to connect after retries")
		}
		return nil, fmt.Errorf("connect to db after retries: %w", lastErr)
	}

	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

func (db *DB) RunMigrations(ctx context.Context, migrationsFS fs.FS) error {
	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		var found int
		err := db.Pool.QueryRow(ctx,
			`SELECT 1 FROM schema_migrations WHERE version = $1`, entry.Name(),
		).Scan(&found)
		if err == nil {
			log.Printf("migration %s already applied, skipping", entry.Name())
			continue
		}

		content, err := fs.ReadFile(migrationsFS, entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		tx, err := db.Pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration tx: %w", err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("exec migration %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations (version) VALUES ($1)`, entry.Name(),
		); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", entry.Name(), err)
		}
		log.Printf("applied migration %s", entry.Name())
	}

	return nil
}

func (db *DB) Health(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}