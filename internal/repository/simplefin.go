package repository

import (
	"context"
	"fmt"
	"time"
)

func (r *Repository) UpsertSimplefinAccount(ctx context.Context, sfID, name, currency string, balance int64) (string, error) {
	query := `
		INSERT INTO accounts (name, type, currency, initial_balance, simplefin_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (simplefin_id) DO UPDATE 
		SET name = EXCLUDED.name, currency = EXCLUDED.currency, updated_at = NOW()
		RETURNING id
	`
	var id string
	err := r.pool.QueryRow(ctx, query, name, "asset", currency, balance, sfID).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to upsert simplefin account: %w", err)
	}
	return id, nil
}

// LinkSimplefinAccount links an existing account to a simplefin_id
func (r *Repository) LinkSimplefinAccount(ctx context.Context, accountID string, sfID string) error {
	query := `UPDATE accounts SET simplefin_id = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, sfID, accountID)
	if err != nil {
		return fmt.Errorf("failed to link simplefin account: %w", err)
	}
	return nil
}

// TransactionExistsByDetails checks if a transaction with the same exact amount, date, and description exists
func (r *Repository) TransactionExistsByDetails(ctx context.Context, accountID string, amount int64, date time.Time, description string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM transactions 
			WHERE account_id = $1 AND amount = $2 AND date = $3 AND description = $4
		)
	`
	var exists bool
	err := r.pool.QueryRow(ctx, query, accountID, amount, date, description).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check transaction existence: %w", err)
	}
	return exists, nil
}
