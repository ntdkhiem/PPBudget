package service

import (
	"errors"
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
)

func day(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestBudgetMonthFor(t *testing.T) {
	posted := day(2026, time.September, 28)
	month := func(y int, m time.Month, d int) *time.Time { v := day(y, m, d); return &v }

	cases := []struct {
		name  string
		month *time.Time
		want  *time.Time
	}{
		{"nothing asked", nil, nil},
		{"next month", month(2026, time.October, 1), month(2026, time.October, 1)},
		{"any day reads as its month", month(2026, time.October, 17), month(2026, time.October, 1)},
		{"its own month is stored as none", month(2026, time.September, 1), nil},
		{"previous month", month(2026, time.August, 1), month(2026, time.August, 1)},
		{"months away", month(2026, time.December, 1), month(2026, time.December, 1)},
		{"across a year", month(2025, time.July, 1), month(2025, time.July, 1)},
		{"same month a year on is not its own", month(2027, time.September, 1), month(2027, time.September, 1)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := budgetMonthFor(posted, c.month)
			if (got == nil) != (c.want == nil) || (got != nil && !got.Equal(*c.want)) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestNextMonthIfLate(t *testing.T) {
	cases := []struct {
		posted time.Time
		days   int
		want   *time.Time
	}{
		{day(2026, time.September, 28), 5, ptr(day(2026, time.October, 1))},
		{day(2026, time.September, 26), 5, ptr(day(2026, time.October, 1))}, // 30-day month: 26th-30th
		{day(2026, time.September, 25), 5, nil},
		{day(2026, time.September, 20), 5, nil},
		{day(2026, time.December, 29), 5, ptr(day(2027, time.January, 1))}, // into the new year
		{day(2027, time.February, 25), 5, ptr(day(2027, time.March, 1))},   // 28-day month: 24th on
		{day(2027, time.February, 23), 5, nil},
	}
	for _, c := range cases {
		got := nextMonthIfLate(c.posted, c.days)
		if (got == nil) != (c.want == nil) || (got != nil && !got.Equal(*c.want)) {
			t.Errorf("%s within %d: got %v want %v", c.posted.Format("2006-01-02"), c.days, got, c.want)
		}
	}
}

// Rent posted in the last days of the month moves; the same rule leaves a
// mid-month payment alone.
func TestEvaluateRulesCountsLateRentTowardNextMonth(t *testing.T) {
	s := newEvalService()

	late := txn("RENT - OAK APTS", -250000, acctChecking)
	late.ID, late.Date = "late", day(2026, time.September, 28)
	onTime := txn("RENT - OAK APTS", -250000, acctChecking)
	onTime.ID, onTime.Date = "on-time", day(2026, time.October, 1)
	yearEnd := txn("RENT - OAK APTS", -250000, acctChecking)
	yearEnd.ID, yearEnd.Date = "year-end", day(2026, time.December, 29)

	r := rule(StrictnessAll, cond(FieldDescription, OpContains, "rent"))
	r.Actions = []domain.RuleAction{act(ActionCountTowardNextMonth, "5")}

	changes := s.evaluateRules([]domain.Rule{r}, []domain.Transaction{late, onTime, yearEnd})
	got := map[string]*time.Time{}
	for _, c := range changes {
		got[c.txn.ID] = c.budgetMonth
	}
	if len(changes) != 2 {
		t.Fatalf("expected the two late payments to change, got %d: %v", len(changes), got)
	}
	if m := got["late"]; m == nil || !m.Equal(day(2026, time.October, 1)) {
		t.Errorf("late: got %v want October", m)
	}
	if m := got["year-end"]; m == nil || !m.Equal(day(2027, time.January, 1)) {
		t.Errorf("year-end: got %v want January", m)
	}

	// Already counted toward October: nothing to change, so nothing to write.
	late.BudgetMonth = ptr(day(2026, time.October, 1))
	if changes := s.evaluateRules([]domain.Rule{r}, []domain.Transaction{late}); len(changes) != 0 {
		t.Errorf("an already-moved payment should not change again; got %d", len(changes))
	}
}

// A rule that also sets the category keeps both effects.
func TestEvaluateRulesBudgetMonthAlongsideCategory(t *testing.T) {
	s := newEvalService()
	rent := txn("RENT - OAK APTS", -250000, acctChecking)
	rent.Date = day(2026, time.September, 29)

	r := rule(StrictnessAll, cond(FieldDescription, OpContains, "rent"))
	r.Actions = []domain.RuleAction{act(ActionSetCategory, catStream), act(ActionCountTowardNextMonth, "3")}

	changes := s.evaluateRules([]domain.Rule{r}, []domain.Transaction{rent})
	if len(changes) != 1 || changes[0].categoryID == nil || changes[0].budgetMonth == nil {
		t.Fatalf("expected category and budget month; got %+v", changes)
	}
}

func TestValidateCountTowardNextMonth(t *testing.T) {
	for value, ok := range map[string]bool{"5": true, "1": true, "10": true, "0": false, "11": false, "abc": false} {
		err := validateAction(act(ActionCountTowardNextMonth, value))
		if ok && err != nil {
			t.Errorf("%q: unexpected %v", value, err)
		}
		if !ok && !errors.Is(err, apperrors.ErrInvalidInput) {
			t.Errorf("%q: want ErrInvalidInput, got %v", value, err)
		}
	}
}
