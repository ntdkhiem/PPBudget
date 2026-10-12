package service

import (
	"time"
)

// The month a transaction counts toward, when it is not the month it posted.
//
// Rent paid on the 28th for next month posts this month; the budget month
// moves it where it belongs for budgets, reports and the plan, while the
// posted date stays what the bank says.

// budgetMonthFor normalises a requested budget month against the posted date:
// the first of that month, or nil when it is the posted month itself -- one
// way to say "its own month", so nothing downstream has to compare the two.
// Any month may be chosen: an annual bill can count toward the month it
// covers, however far that is from when it posted.
func budgetMonthFor(posted time.Time, month *time.Time) *time.Time {
	if month == nil {
		return nil
	}
	m := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	if monthsApart(posted, m) == 0 {
		return nil
	}
	return &m
}

// monthsApart counts calendar months from a's month to b's.
func monthsApart(a, b time.Time) int {
	return (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
}

// nextMonthIfLate is the rule action's test: the first of the next month when
// posted falls within the last `days` days of its month, else nil. "Within 5
// days" means the 26th onward in a 30-day month.
func nextMonthIfLate(posted time.Time, days int) *time.Time {
	first := time.Date(posted.Year(), posted.Month(), 1, 0, 0, 0, 0, time.UTC)
	next := first.AddDate(0, 1, 0)
	daysInMonth := next.AddDate(0, 0, -1).Day()
	if posted.Day() <= daysInMonth-days {
		return nil
	}
	return &next
}
