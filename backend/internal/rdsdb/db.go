package rdsdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s connect_timeout=10",
		envOr("DB_HOST", "localhost"),
		envOr("DB_PORT", "5432"),
		envOr("DB_USER", "postgres"),
		os.Getenv("DB_PASSWORD"),
		envOr("DB_NAME", "conference"),
		envOr("DB_SSLMODE", "require"),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}
	return db, nil
}

func EnsureReservationsSchema(db *sql.DB) error {
	const ddl = `
		CREATE TABLE IF NOT EXISTS reservations (
			id BIGSERIAL PRIMARY KEY,
			user_sub TEXT NOT NULL,
			hall_id TEXT NOT NULL,
			reservation_date DATE NOT NULL,
			start_time TIME NOT NULL,
			end_time TIME NOT NULL,
			attendees INTEGER NOT NULL CHECK (attendees > 0),
			purpose TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS reservations_user_sub_idx ON reservations(user_sub);
		CREATE INDEX IF NOT EXISTS reservations_hall_date_idx ON reservations(hall_id, reservation_date);
	`
	if _, err := db.Exec(ddl); err != nil {
		return fmt.Errorf("failed to ensure reservations schema: %w", err)
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
