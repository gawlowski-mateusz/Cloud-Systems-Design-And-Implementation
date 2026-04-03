package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_NAME", "cloud-systems-design-and-implementation"),
		getEnv("DB_SSLMODE", "disable"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return db, nil
}

func EnsureSchema(db *sql.DB) error {
	const usersTable = `
		CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			full_name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if _, err := db.Exec(usersTable); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	const reservationsTable = `
		CREATE TABLE IF NOT EXISTS reservations (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			hall_id TEXT NOT NULL,
			reservation_date DATE NOT NULL,
			start_time TIME NOT NULL,
			end_time TIME NOT NULL,
			attendees INTEGER NOT NULL CHECK (attendees > 0),
			purpose TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if _, err := db.Exec(reservationsTable); err != nil {
		return fmt.Errorf("failed to create reservations table: %w", err)
	}

	const mediaFilesTable = `
		CREATE TABLE IF NOT EXISTS media_files (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			original_name TEXT NOT NULL,
			content_type TEXT NOT NULL,
			size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
			object_key TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`

	if _, err := db.Exec(mediaFilesTable); err != nil {
		return fmt.Errorf("failed to create media_files table: %w", err)
	}

	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
