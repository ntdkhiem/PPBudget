package service

import (
	"context"

	"ntdkhiem/ppbudget-go/internal/domain"
)

// Planned expenses are one-off costs the user can see coming: an insurance
// premium, a trip, a repair. For the twelve months ahead the plan holds them
// back from the emergency fund and from any cash it would tell the user to
// move; see plannedWithin. The list itself is read in wealth_profile.go.

// CreatePlannedExpense records a planned expense and returns its id.
func (s *Service) CreatePlannedExpense(ctx context.Context, userID string, e domain.PlannedExpense) (string, error) {
	return s.repo.CreatePlannedExpense(ctx, userID, e)
}

// UpdatePlannedExpense replaces one planned expense.
func (s *Service) UpdatePlannedExpense(ctx context.Context, userID string, e domain.PlannedExpense) error {
	return s.repo.UpdatePlannedExpense(ctx, userID, e)
}

// DeletePlannedExpense removes one planned expense.
func (s *Service) DeletePlannedExpense(ctx context.Context, userID, id string) error {
	return s.repo.DeletePlannedExpense(ctx, userID, id)
}
