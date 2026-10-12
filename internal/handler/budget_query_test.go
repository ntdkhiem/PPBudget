package handler

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseBudgetMonthQuery(t *testing.T) {
	// 01:30 UTC on 1 Nov is still 31 Oct for a viewer in California.
	now := time.Date(2026, time.November, 1, 1, 30, 0, 0, time.UTC)
	date := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

	tests := []struct {
		query     string
		wantMonth time.Time
		wantToday time.Time
		wantErr   bool
	}{
		{"month=2026-10", date(2026, 10, 1), date(2026, 11, 1), false},
		{"month=2026-10&today=2026-10-31", date(2026, 10, 1), date(2026, 10, 31), false},
		{"month=2026-10-01", date(2026, 10, 1), date(2026, 11, 1), false},
		// Too far from the server's date to be a timezone: ignored.
		{"month=2026-10&today=2030-01-01", date(2026, 10, 1), date(2026, 11, 1), false},
		{"month=2026-10&today=31-10-2026", time.Time{}, time.Time{}, true},
		{"", time.Time{}, time.Time{}, true},
		{"month=October", time.Time{}, time.Time{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/budgets/summary?"+tt.query, nil)
			month, today, err := parseBudgetMonthQuery(r, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !month.Equal(tt.wantMonth) || !today.Equal(tt.wantToday) {
				t.Errorf("got (%s, %s), want (%s, %s)", month.Format(time.DateOnly), today.Format(time.DateOnly),
					tt.wantMonth.Format(time.DateOnly), tt.wantToday.Format(time.DateOnly))
			}
		})
	}
}
