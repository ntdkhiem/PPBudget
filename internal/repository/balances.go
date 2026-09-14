package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// execQuerier is the subset of pgx.Tx / *pgxpool.Pool used by the balance writes.
type execQuerier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// querier returns tx when non-nil, otherwise the pool.
func (r *Repository) querier(tx pgx.Tx) execQuerier {
	if tx != nil {
		return tx
	}
	return r.pool
}

// SetAccountBalanceOnly sets the account's balance_only flag. Enabling it soft-deletes the
// account's transactions and removes their links, returning how many were deleted.
// Disabling only clears the flag; nothing is restored.
func (r *Repository) SetAccountBalanceOnly(ctx context.Context, userID, accountID string, enabled bool) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := r.checkOwnership(ctx, tx, "accounts", accountID, userID); err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance_only = $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`,
		enabled, accountID, userID,
	); err != nil {
		return 0, fmt.Errorf("failed to update balance_only: %w", err)
	}

	var deleted int64
	if enabled {
		// Remove links touching this account's live transactions so the other side's
		// effective_amount isn't distorted by a link to a deleted transaction.
		if _, err := tx.Exec(ctx, `
			DELETE FROM transaction_links l
			USING transactions t
			WHERE t.account_id = $1 AND t.user_id = $2 AND t.deleted_at IS NULL
			  AND (l.source_transaction_id = t.id OR l.target_transaction_id = t.id)
		`, accountID, userID); err != nil {
			return 0, fmt.Errorf("failed to delete transaction links: %w", err)
		}

		tag, err := tx.Exec(ctx,
			`UPDATE transactions SET deleted_at = NOW() WHERE account_id = $1 AND user_id = $2 AND deleted_at IS NULL`,
			accountID, userID,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to soft-delete transactions: %w", err)
		}
		deleted = tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("failed to commit balance_only update: %w", err)
	}
	return deleted, nil
}

// BalanceOnlyAccountIDs returns the set of the user's account ids flagged balance_only.
func (r *Repository) BalanceOnlyAccountIDs(ctx context.Context, userID string) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM accounts WHERE user_id = $1 AND balance_only`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list balance-only accounts: %w", err)
	}
	defer rows.Close()

	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan balance-only account: %w", err)
		}
		ids[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list balance-only accounts: %w", err)
	}
	return ids, nil
}

// UpsertBalanceSnapshot writes a snapshot keyed on (account_id, as_of_date); last write wins.
// A nil tx uses the pool.
func (r *Repository) UpsertBalanceSnapshot(ctx context.Context, tx pgx.Tx, userID, accountID string, asOf time.Time, balance money.Money, available *money.Money, source string, reportedAt *time.Time) error {
	if source != domain.BalanceSourceSimplefin && source != domain.BalanceSourceManual {
		return fmt.Errorf("invalid snapshot source %q (use SetOpeningBalance for opening): %w", source, apperrors.ErrInvalidInput)
	}
	if asOf.IsZero() {
		return fmt.Errorf("snapshot as_of date is required: %w", apperrors.ErrInvalidInput)
	}
	if err := r.checkOwnership(ctx, tx, "accounts", accountID, userID); err != nil {
		return err
	}

	u := asOf.UTC()
	asOfDate := time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)

	var availableCents *int64
	if available != nil {
		v := available.ToInt64()
		availableCents = &v
	}

	query := `
		INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, available_balance, source, reported_at)
		VALUES ($1, $2, $3::date, $4, $5, $6, $7)
		ON CONFLICT (account_id, as_of_date) DO UPDATE
		SET balance = EXCLUDED.balance,
		    available_balance = EXCLUDED.available_balance,
		    source = EXCLUDED.source,
		    reported_at = EXCLUDED.reported_at
	`
	if _, err := r.querier(tx).Exec(ctx, query, userID, accountID, asOfDate, balance.ToInt64(), availableCents, source, reportedAt); err != nil {
		return fmt.Errorf("failed to upsert balance snapshot: %w", err)
	}
	return nil
}

// SetOpeningBalance creates or replaces the account's opening ('-infinity') snapshot. A nil tx uses the pool.
func (r *Repository) SetOpeningBalance(ctx context.Context, tx pgx.Tx, userID, accountID string, balance money.Money) error {
	if err := r.checkOwnership(ctx, tx, "accounts", accountID, userID); err != nil {
		return err
	}
	query := `
		INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
		VALUES ($1, $2, '-infinity'::date, $3, 'opening')
		ON CONFLICT (account_id, as_of_date) DO UPDATE
		SET balance = EXCLUDED.balance
	`
	if _, err := r.querier(tx).Exec(ctx, query, userID, accountID, balance.ToInt64()); err != nil {
		return fmt.Errorf("failed to set opening balance: %w", err)
	}
	return nil
}

// ListBalanceSnapshots returns snapshots newest first, with the opening snapshot last.
func (r *Repository) ListBalanceSnapshots(ctx context.Context, userID, accountID string) ([]domain.BalanceSnapshot, error) {
	if err := r.checkOwnership(ctx, nil, "accounts", accountID, userID); err != nil {
		return nil, err
	}
	query := `
		SELECT id, account_id,
		       CASE WHEN as_of_date = '-infinity'::date THEN NULL ELSE as_of_date END AS as_of_date,
		       balance, available_balance, source, reported_at, created_at
		FROM account_balance_snapshots
		WHERE account_id = $1 AND user_id = $2
		ORDER BY account_balance_snapshots.as_of_date DESC
	`
	rows, err := r.pool.Query(ctx, query, accountID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list balance snapshots: %w", err)
	}
	defer rows.Close()

	snapshots := []domain.BalanceSnapshot{}
	for rows.Next() {
		var s domain.BalanceSnapshot
		var balance int64
		var available *int64
		if err := rows.Scan(&s.ID, &s.AccountID, &s.AsOfDate, &balance, &available, &s.Source, &s.ReportedAt, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan balance snapshot: %w", err)
		}
		s.Balance = money.Money(balance)
		if available != nil {
			m := money.Money(*available)
			s.AvailableBalance = &m
		}
		snapshots = append(snapshots, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to list balance snapshots: %w", err)
	}
	return snapshots, nil
}

// DeleteBalanceSnapshot removes a non-opening snapshot.
func (r *Repository) DeleteBalanceSnapshot(ctx context.Context, userID, accountID, snapshotID string) error {
	if err := r.checkOwnership(ctx, nil, "accounts", accountID, userID); err != nil {
		return err
	}

	var source string
	err := r.pool.QueryRow(ctx,
		`SELECT source FROM account_balance_snapshots WHERE id = $1 AND account_id = $2 AND user_id = $3`,
		snapshotID, accountID, userID,
	).Scan(&source)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.ErrNotFound
		}
		return fmt.Errorf("failed to get balance snapshot: %w", err)
	}
	if source == domain.BalanceSourceOpening {
		return fmt.Errorf("opening balance snapshot cannot be deleted: %w", apperrors.ErrInvalidInput)
	}

	tag, err := r.pool.Exec(ctx,
		`DELETE FROM account_balance_snapshots WHERE id = $1 AND account_id = $2 AND user_id = $3 AND source <> 'opening'`,
		snapshotID, accountID, userID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete balance snapshot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
