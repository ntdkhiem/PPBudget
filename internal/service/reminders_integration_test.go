package service

// Deadline reminders end to end: a real plan with an ESPP purchase three days
// out, and a fake sender in place of Resend or SMTP.
//
// Gated on TEST_DATABASE_URL.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
)

type sentEmail struct{ to, subject string }

func fakeSender(out *[]sentEmail, fail error) emailSender {
	return func(_ context.Context, to, subject, _ string) error {
		if fail != nil {
			return fail
		}
		*out = append(*out, sentEmail{to, subject})
		return nil
	}
}

func TestDeadlineRemindersIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)

	// A persona user with a second ESPP buying in three days: a dated action
	// inside the week. The persona's own ESPP buys next March, outside it.
	seed := func(t *testing.T) (userID, email string) {
		userID = sfCreateTestUser(t, pool)
		seedPDFPersona(t, pool, svc, userID)
		purchase := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 3)
		if _, err := svc.UpsertEquityGrant(ctx, userID, domain.EquityGrant{
			Kind: domain.EquityKindESPP, Label: ptr("Autumn ESPP"),
			ESPPDiscountPct: ptr(0.15), ESPPContributionPct: ptr(0.05), ESPPPlanMaxPct: ptr(0.15),
			ESPPPurchaseDate: &purchase,
		}); err != nil {
			t.Fatalf("seed ESPP grant: %v", err)
		}
		if err := pool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, userID).Scan(&email); err != nil {
			t.Fatalf("user email: %v", err)
		}
		return userID, email
	}

	t.Run("sent once, however often the job runs", func(t *testing.T) {
		userID, email := seed(t)
		var sent []sentEmail

		n, err := svc.remindUsers(ctx, []string{userID}, time.Now(), fakeSender(&sent, nil))
		if err != nil || n != 1 {
			t.Fatalf("first run: sent %d, err %v", n, err)
		}
		if len(sent) != 1 || sent[0].to != email || !strings.Contains(sent[0].subject, "Sell the ESPP shares") {
			t.Fatalf("first run emails: %+v", sent)
		}

		n, err = svc.remindUsers(ctx, []string{userID}, time.Now(), fakeSender(&sent, nil))
		if err != nil || n != 0 || len(sent) != 1 {
			t.Errorf("second run: sent %d (total %d), err %v; want nothing new", n, len(sent), err)
		}
	})

	t.Run("a failed send is retried next run", func(t *testing.T) {
		userID, _ := seed(t)
		var sent []sentEmail

		n, err := svc.remindUsers(ctx, []string{userID}, time.Now(), fakeSender(&sent, errors.New("smtp down")))
		if err == nil || n != 0 {
			t.Fatalf("failing run: sent %d, err %v; want an error and nothing sent", n, err)
		}
		n, err = svc.remindUsers(ctx, []string{userID}, time.Now(), fakeSender(&sent, nil))
		if err != nil || n != 1 || len(sent) != 1 {
			t.Errorf("retry: sent %d, err %v; the released reminder should go out", n, err)
		}
	})

	t.Run("turned off in settings", func(t *testing.T) {
		userID, _ := seed(t)
		if err := svc.repo.SetUserSetting(ctx, userID, DeadlineRemindersSetting, "off"); err != nil {
			t.Fatalf("setting: %v", err)
		}
		var sent []sentEmail
		n, err := svc.remindUsers(ctx, []string{userID}, time.Now(), fakeSender(&sent, nil))
		if err != nil || n != 0 || len(sent) != 0 {
			t.Errorf("opted out: sent %d, err %v", n, err)
		}
	})

	t.Run("no mailer configured sends nothing", func(t *testing.T) {
		n, err := svc.SendDeadlineReminders(ctx) // the test config has neither Resend nor SMTP
		if err != nil || n != 0 {
			t.Errorf("got %d, %v; want 0, nil", n, err)
		}
	})
}

// The timeline reads back what the engine recorded, with names filled in.
func TestProgressTimelineIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)
	seedPDFPersona(t, pool, svc, userID)

	if _, _, err := svc.GenerateQuests(ctx, userID); err != nil {
		t.Fatalf("GenerateQuests: %v", err)
	}
	if err := svc.SetQuestStatus(ctx, userID, "fund_ira_to_limit", domain.QuestStatusComplete, ""); err != nil {
		t.Fatalf("mark: %v", err)
	}

	events, err := svc.ListProgress(ctx, userID, 0)
	if err != nil {
		t.Fatalf("ListProgress: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected events")
	}
	if first := events[0]; first.Title != "Funded your IRA to the limit" || first.How != "you marked it done" {
		t.Errorf("newest first: got %q / %q", first.Title, first.How)
	}
	for _, e := range events {
		if strings.Contains(e.Title, "{name}") || strings.Contains(e.Title, "_") {
			t.Errorf("unreadable title %q", e.Title)
		}
	}
	if capped, _ := svc.ListProgress(ctx, userID, 1); len(capped) != 1 {
		t.Errorf("limit 1: got %d", len(capped))
	}
}
