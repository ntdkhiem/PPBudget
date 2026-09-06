package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

const DefaultUserID = "00000000-0000-0000-0000-000000000001"

// GetUserSetting fetches a user setting by userID and key
func (r *Repository) GetUserSetting(ctx context.Context, userID, key string) (string, error) {
	var value string
	query := `SELECT value FROM user_settings WHERE user_id = $1 AND key = $2`
	err := r.pool.QueryRow(ctx, query, userID, key).Scan(&value)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil // Return empty string if not found
		}
		return "", err
	}
	return value, nil
}

// SetUserSetting upserts a user setting by userID and key
func (r *Repository) SetUserSetting(ctx context.Context, userID, key, value string) error {
	query := `
		INSERT INTO user_settings (user_id, key, value, updated_at) 
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query, userID, key, value)
	return err
}


// GetUserSettings fetches a user setting by userID and key
func (r *Repository) GetUserSettings(ctx context.Context, userID, key string) (string, error) {
	return r.GetUserSetting(ctx, userID, key)
}

// SetUserSettings upserts a user setting by userID and key
func (r *Repository) SetUserSettings(ctx context.Context, userID, key, value string) error {
	return r.SetUserSetting(ctx, userID, key, value)
}

type SimpleFinUserConfig struct {
	UserID     string
	ConfigJSON string
}

// GetAllUsersWithSimpleFin retrieves all users who have a simplefin_config configured
func (r *Repository) GetAllUsersWithSimpleFin(ctx context.Context) ([]SimpleFinUserConfig, error) {
	query := `SELECT user_id, value FROM user_settings WHERE key = 'simplefin_config'`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users with simplefin config: %w", err)
	}
	defer rows.Close()

	var configs []SimpleFinUserConfig
	for rows.Next() {
		var cfg SimpleFinUserConfig
		if err := rows.Scan(&cfg.UserID, &cfg.ConfigJSON); err == nil {
			configs = append(configs, cfg)
		}
	}
	return configs, nil
}

type ExportTransactionRow struct {
	Date        time.Time
	Description string
	Amount      int64
	Account     string
	Category    string
}

func (r *Repository) ExportTransactions(ctx context.Context, userID string) ([]ExportTransactionRow, error) {
	query := `
		SELECT t.date, t.description, t.amount, a.name as account, COALESCE(c.name, 'Uncategorized') as category
		FROM transactions t
		JOIN accounts a ON t.account_id = a.id
		LEFT JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = $1 AND t.deleted_at IS NULL
		ORDER BY t.date DESC, t.created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []ExportTransactionRow
	for rows.Next() {
		var row ExportTransactionRow
		if err := rows.Scan(&row.Date, &row.Description, &row.Amount, &row.Account, &row.Category); err != nil {
			return nil, err
		}
		txns = append(txns, row)
	}
	return txns, nil
}
