package repository

// Gated on TEST_DATABASE_URL. Rollover runs against the database's
// CURRENT_DATE, so every date here is built from today (UTC, the container's
// timezone).

import (
	"context"
	"testing"
	"time"
)

func TestRolloverSubscriptionsIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "sub-rollover")

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	first := func(years, months int) time.Time {
		return time.Date(today.Year()+years, today.Month()+time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	}

	cases := []struct {
		name, cycle  string
		stored, want time.Time
	}{
		// Last visited a month and more ago: the old age()-based step added
		// two months here and skipped this month's bill entirely.
		{"monthly, a month behind", "monthly", first(0, -1), first(0, 0)},
		{"monthly, three months behind", "monthly", first(0, -3), first(0, 0)},
		{"monthly, this month already", "monthly", first(0, 0), first(0, 0)},
		{"yearly, a year behind", "yearly", first(-1, 0), first(0, 0)},
		// Earlier in the year than this month: next year's is the first one
		// on or after the 1st.
		{"yearly, earlier month", "yearly", first(-1, -1), first(0, -1).AddDate(1, 0, 0)},
		// Weekly used to never move: now it sits on this week's occurrence.
		{"weekly, weeks behind", "weekly", today.AddDate(0, 0, -20), today.AddDate(0, 0, -6)},
		{"weekly, due today", "weekly", today.AddDate(0, 0, -14), today},
		{"weekly, this week", "weekly", today.AddDate(0, 0, -6), today.AddDate(0, 0, -6)},
	}

	ids := make([]string, len(cases))
	for i, c := range cases {
		if err := pool.QueryRow(ctx,
			`INSERT INTO subscriptions (user_id, name, amount, billing_cycle, next_billing_date)
			 VALUES ($1, $2, 1000, $3, $4) RETURNING id`,
			userID, c.name, c.cycle, c.stored,
		).Scan(&ids[i]); err != nil {
			t.Fatalf("insert %s: %v", c.name, err)
		}
	}

	subs, err := repo.ListSubscriptions(ctx, userID)
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	got := map[string]time.Time{}
	for _, s := range subs {
		got[s.ID] = s.NextBillingDate
	}
	for i, c := range cases {
		if d := got[ids[i]]; !d.Equal(c.want) {
			t.Errorf("%s: next_billing_date = %s, want %s", c.name, d.Format(time.DateOnly), c.want.Format(time.DateOnly))
		}
	}
}

func TestListSubscriptionsLastPaidDateIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "sub-last-paid")

	acct, err := repo.CreateAccount(ctx, userID, "Checking", "asset", "USD", 0, nil)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	future := time.Now().UTC().AddDate(0, 0, 10)
	var paid, unpaid string
	for _, s := range []struct {
		name string
		id   *string
	}{{"Paid", &paid}, {"Unpaid", &unpaid}} {
		if err := pool.QueryRow(ctx,
			`INSERT INTO subscriptions (user_id, name, amount, billing_cycle, next_billing_date)
			 VALUES ($1, $2, 1000, 'monthly', $3) RETURNING id`, userID, s.name, future,
		).Scan(s.id); err != nil {
			t.Fatalf("insert %s: %v", s.name, err)
		}
	}

	link := func(amount int64, date string) string {
		id := insertTxn(t, pool, userID, acct, amount, date, "SUB")
		if _, err := pool.Exec(ctx, `UPDATE transactions SET subscription_id = $1 WHERE id = $2`, paid, id); err != nil {
			t.Fatalf("link: %v", err)
		}
		return id
	}
	link(-1000, "2026-08-05")
	link(-1000, "2026-09-05")
	link(1000, "2026-09-20") // a refund is not a payment
	softDeleteTxn(t, pool, link(-1000, "2026-09-25"))

	subs, err := repo.ListSubscriptions(ctx, userID)
	if err != nil {
		t.Fatalf("ListSubscriptions: %v", err)
	}
	for _, s := range subs {
		switch s.ID {
		case paid:
			if s.LastPaidDate == nil || s.LastPaidDate.Format(time.DateOnly) != "2026-09-05" {
				t.Errorf("Paid last_paid_date = %v, want 2026-09-05", s.LastPaidDate)
			}
		case unpaid:
			if s.LastPaidDate != nil {
				t.Errorf("Unpaid last_paid_date = %v, want nil", s.LastPaidDate)
			}
		}
	}
}
