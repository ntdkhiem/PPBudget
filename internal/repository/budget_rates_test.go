package repository

import (
	"testing"
	"time"
)

func TestBudgetDayRates(t *testing.T) {
	start := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.October, 31, 0, 0, 0, 0, time.UTC)
	day := func(d int, m time.Month) time.Time { return time.Date(2026, m, d, 15, 30, 0, 0, time.UTC) }

	tests := []struct {
		name                string
		today               time.Time
		spent, left         int64
		wantSpent, wantLeft int64
	}{
		// Day 12: 12 days gone, 20 left counting today.
		{"mid month", day(12, time.October), 21_000, 29_000, 1_750, 1_450},
		// The last day can still be spent, so it is not a zero-day month end.
		{"last day", day(31, time.October), 31_000, 4_000, 1_000, 4_000},
		{"first day", day(1, time.October), 500, 9_500, 500, 306},
		// Finished: averaged over the whole month, nothing left per day.
		{"past month", day(5, time.November), 62_000, 0, 2_000, 0},
		// Not started: nothing gone yet, the full limit over every day.
		{"future month", day(20, time.September), 0, 31_000, 0, 1_000},
		{"overspent", day(12, time.October), 60_000, -10_000, 5_000, 0},
		// 20,000 / 12 = 1,666.67: rounds to the nearest cent, not down.
		{"rounds to nearest cent", day(12, time.October), 20_000, 30_000, 1_667, 1_500},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpent, gotLeft := budgetDayRates(start, end, tt.today, tt.spent, tt.left)
			if gotSpent != tt.wantSpent || gotLeft != tt.wantLeft {
				t.Errorf("budgetDayRates = (%d, %d), want (%d, %d)", gotSpent, gotLeft, tt.wantSpent, tt.wantLeft)
			}
		})
	}
}
