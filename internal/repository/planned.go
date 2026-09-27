package repository

import (
	"context"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
)

// UpdatePlannedExpense replaces one planned expense's label, amount and date.
//
// Listing, creating and deleting live with the other wealth tables in
// wealth.go. This is the edit the Cash page's "Coming up" list needs, so a
// wrong amount can be corrected without deleting the row and adding it again.
func (r *Repository) UpdatePlannedExpense(ctx context.Context, userID string, e domain.PlannedExpense) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE planned_expenses SET label = $3, amount = $4, target_date = $5
		WHERE id = $2 AND user_id = $1`,
		userID, e.ID, e.Label, e.Amount.ToInt64(), e.TargetDate)
	if err != nil {
		return constraintError(err, "planned expense")
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
