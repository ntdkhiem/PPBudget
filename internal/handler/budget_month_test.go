package handler

import (
	"encoding/json"
	"testing"
	"time"
)

// Absent must differ from null: an edit from a screen that does not know the
// field may not clear it.
func TestParseBudgetMonth(t *testing.T) {
	cases := []struct {
		raw     string
		set     bool
		month   string // yyyy-mm, "" for none
		invalid bool
	}{
		{raw: "", set: false},
		{raw: "null", set: true},
		{raw: `"2026-10"`, set: true, month: "2026-10"},
		{raw: `"2026-10-01"`, set: true, month: "2026-10"},
		{raw: `"October"`, invalid: true},
		{raw: `42`, invalid: true},
	}
	for _, c := range cases {
		got, err := parseBudgetMonth(json.RawMessage(c.raw))
		if c.invalid {
			if err == nil {
				t.Errorf("%s: want an error", c.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.raw, err)
			continue
		}
		if got.Set != c.set {
			t.Errorf("%s: set = %v want %v", c.raw, got.Set, c.set)
		}
		switch {
		case c.month == "" && got.Month != nil:
			t.Errorf("%s: month = %v want none", c.raw, got.Month)
		case c.month != "" && (got.Month == nil || got.Month.Format("2006-01") != c.month || got.Month.Day() != 1 || got.Month.Location() != time.UTC):
			t.Errorf("%s: month = %v want %s-01 UTC", c.raw, got.Month, c.month)
		}
	}
}
