package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ListWealthUserIDs returns every user who has started a wealth plan -- the
// only users with dated actions to be reminded of.
func (r *Repository) ListWealthUserIDs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT user_id FROM wealth_profiles ORDER BY user_id`)
	if err != nil {
		return nil, fmt.Errorf("failed to list wealth users: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ReminderKey names one reminder: an action, on one due date.
type ReminderKey struct {
	CatalogKey string
	DueOn      time.Time
}

// ReminderClaim holds claimed reminders in an open transaction until the email
// carrying them has gone out.
//
// Claiming before sending and committing after is what makes the daily job
// safe to run twice at once. A second run's claim on the same rows waits for
// the first to finish: if the first committed, the rows exist and the second
// claims nothing; if its send failed and it released, the second sends instead.
// Recording after sending would email twice; recording before would lose the
// reminder whenever the send failed.
type ReminderClaim struct {
	tx pgx.Tx
	// Keys are the reminders this claim won, a subset of those asked for.
	Keys []ReminderKey
}

// ClaimReminders claims the reminders not yet sent to this user. The caller
// must Commit once the email is sent, or Release if it was not.
func (r *Repository) ClaimReminders(ctx context.Context, userID string, keys []ReminderKey) (*ReminderClaim, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	claim := &ReminderClaim{tx: tx}
	for _, k := range keys {
		tag, err := tx.Exec(ctx, `
			INSERT INTO wealth_reminders_sent (user_id, catalog_key, due_on)
			VALUES ($1, $2, $3)
			ON CONFLICT DO NOTHING`,
			userID, k.CatalogKey, k.DueOn.UTC().Format("2006-01-02"))
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("failed to claim reminder %q: %w", k.CatalogKey, err)
		}
		if tag.RowsAffected() == 1 {
			claim.Keys = append(claim.Keys, k)
		}
	}
	return claim, nil
}

// Commit records the claimed reminders as sent.
func (c *ReminderClaim) Commit(ctx context.Context) error {
	if err := c.tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to record reminders: %w", err)
	}
	return nil
}

// Release gives the claimed reminders back, so a later run can send them.
func (c *ReminderClaim) Release(ctx context.Context) {
	_ = c.tx.Rollback(ctx)
}
