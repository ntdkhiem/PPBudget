package repository

import (
	"context"
	"fmt"
	"time"
)

func (r *Repository) UpsertSimplefinAccount(ctx context.Context, userID, sfID, name, currency string, balance int64) (string, error) {
	query := `
		INSERT INTO accounts (name, type, currency, initial_balance, simplefin_id, user_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (simplefin_id) DO UPDATE 
		SET name = EXCLUDED.name, currency = EXCLUDED.currency, updated_at = NOW()
		RETURNING id
	`
	var id string
	err := r.pool.QueryRow(ctx, query, name, "asset", currency, balance, sfID, userID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to upsert simplefin account: %w", err)
	}
	return id, nil
}

// LinkSimplefinAccount links an existing account to a simplefin_id
func (r *Repository) LinkSimplefinAccount(ctx context.Context, userID, accountID, sfID string) error {
	query := `UPDATE accounts SET simplefin_id = $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`
	_, err := r.pool.Exec(ctx, query, sfID, accountID, userID)
	if err != nil {
		return fmt.Errorf("failed to link simplefin account: %w", err)
	}
	return nil
}

// TransactionExistsByDetails checks if a transaction with the same exact amount, date, and description exists
func (r *Repository) TransactionExistsByDetails(ctx context.Context, userID, accountID string, amount int64, date time.Time, description string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM transactions 
			WHERE user_id = $1 AND account_id = $2 AND amount = $3 AND date = $4 AND description = $5 AND deleted_at IS NULL
		)
	`
	var exists bool
	err := r.pool.QueryRow(ctx, query, userID, accountID, amount, date, description).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check transaction existence: %w", err)
	}
	return exists, nil
}
