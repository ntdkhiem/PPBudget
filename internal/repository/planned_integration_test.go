package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
)

// A planned expense can be listed, edited in place and removed, and no one
// else's row can be touched by id.
func TestPlannedExpensesCRUDIntegration(t *testing.T) {
	pool := setupTestDB(t)
	repo := New(pool)
	ctx := context.Background()
	userID := createTestUser(t, pool, "planned-crud")
	otherID := createTestUser(t, pool, "planned-other")

	day := func(m time.Month, d int) time.Time { return time.Date(2027, m, d, 0, 0, 0, 0, time.UTC) }
	id, err := repo.CreatePlannedExpense(ctx, userID, domain.PlannedExpense{
		Label: "Car insurance", Amount: 120_000, TargetDate: day(time.March, 15),
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.CreatePlannedExpense(ctx, userID, domain.PlannedExpense{
		Label: "Trip", Amount: 250_000, TargetDate: day(time.January, 10),
	}); err != nil {
		t.Fatalf("create second: %v", err)
	}

	edit := domain.PlannedExpense{ID: id, Label: "Car insurance (6 mo)", Amount: 135_000, TargetDate: day(time.April, 1)}
	if err := repo.UpdatePlannedExpense(ctx, userID, edit); err != nil {
		t.Fatalf("update: %v", err)
	}

	list, err := repo.ListPlannedExpenses(ctx, userID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].Label != "Trip" {
		t.Fatalf("want two rows, soonest first; got %+v", list)
	}
	got := list[1]
	if got.ID != id || got.Label != edit.Label || got.Amount != edit.Amount || !got.TargetDate.Equal(edit.TargetDate) {
		t.Errorf("update did not stick: got %+v", got)
	}

	// Another user's id is not found, whether editing or deleting.
	if err := repo.UpdatePlannedExpense(ctx, otherID, edit); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("update as another user: got %v want ErrNotFound", err)
	}
	if err := repo.DeletePlannedExpense(ctx, otherID, id); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("delete as another user: got %v want ErrNotFound", err)
	}

	// The table refuses a non-positive amount even if a caller forgets to.
	bad := edit
	bad.Amount = 0
	if err := repo.UpdatePlannedExpense(ctx, userID, bad); err == nil {
		t.Error("a zero amount should be refused")
	}

	if err := repo.DeletePlannedExpense(ctx, userID, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := repo.DeletePlannedExpense(ctx, userID, id); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("second delete: got %v want ErrNotFound", err)
	}
}
