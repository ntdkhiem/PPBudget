package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// GetAppSetting fetches a setting by key
func (r *Repository) GetAppSetting(ctx context.Context, key string) (string, error) {
	var value string
	query := `SELECT value FROM app_settings WHERE key = $1`
	err := r.pool.QueryRow(ctx, query, key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil // Return empty string if not found
		}
		return "", err
	}
	return value, nil
}

// SetAppSetting upserts a setting by key
func (r *Repository) SetAppSetting(ctx context.Context, key, value string) error {
	query := `
		INSERT INTO app_settings (key, value, updated_at) 
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query, key, value)
	return err
}
