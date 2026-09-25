package repository

import (
	"context"
	"fmt"
	"time"
)

// UpsertSimplefinAccount creates the account for a SimpleFin feed, or renames
// the one already linked to it. Type and role apply only on creation: after
// that they are the user's, and a sync must not undo a correction.
func (r *Repository) UpsertSimplefinAccount(ctx context.Context, userID, sfID, name, currency, accType string, role *string) (string, error) {
	// Conflict target is (user_id, simplefin_id): a SimpleFIN id is unique only
	// within one connection, so scoping by user keeps two people who link the
	// same bank from upserting into each other's accounts (migration 0023).
	query := `
		INSERT INTO accounts (name, type, role, currency, simplefin_id, user_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, simplefin_id) DO UPDATE
		SET name = EXCLUDED.name, currency = EXCLUDED.currency, updated_at = NOW()
		WHERE accounts.user_id = EXCLUDED.user_id
		RETURNING id
	`
	var id string
	err := r.pool.QueryRow(ctx, query, name, accType, role, currency, sfID, userID).Scan(&id)
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
