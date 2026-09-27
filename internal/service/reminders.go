package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/internal/repository"
)

// Deadline reminders: an email a week ahead of each dated action.
//
// The Overview's Dated band is only seen by someone who opens it, and its
// deadlines are the ones that pass whether or not they do -- payroll closes on
// 31 December, an ESPP purchase lands when the plan says. One email per action
// per due date, sent once however many times the job runs.

// reminderWindowDays is how far ahead a deadline is worth an email: close
// enough to act on, far enough to still act in time.
const reminderWindowDays = 7

// DeadlineRemindersSetting is the user setting that turns reminders off.
// Anything but "off" -- including never having set it -- leaves them on.
const DeadlineRemindersSetting = "deadline_reminders"

// dueReminders picks the open dated actions falling due within the window,
// soonest first. A deadline already past is left alone: an email saying it
// was missed helps no one.
func dueReminders(quests []domain.Quest, now time.Time) []domain.Quest {
	today := calendarDay(now)
	last := today.AddDate(0, 0, reminderWindowDays)

	var out []domain.Quest
	for _, q := range quests {
		if q.DueDate == nil {
			continue
		}
		switch q.Status {
		case domain.QuestStatusComplete, domain.QuestStatusSkipped, domain.QuestStatusNotApplicable:
			continue
		}
		due := calendarDay(*q.DueDate)
		if due.Before(today) || due.After(last) {
			continue
		}
		out = append(out, q)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DueDate.Before(*out[j].DueDate) })
	return out
}

// calendarDay drops the time of day, in UTC, which is how due dates are kept.
func calendarDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// SendDeadlineReminders emails every user with a dated action falling due
// within the week, and returns how many emails went out.
//
// Safe to call more than once a day: the in-process cron and the scheduled
// HTTP trigger both do, since a host that sleeps misses the first.
func (s *Service) SendDeadlineReminders(ctx context.Context) (int, error) {
	if !s.mailConfigured() {
		s.logger.Info("neither Resend nor SMTP configured, skipping deadline reminders")
		return 0, nil
	}
	users, err := s.repo.ListWealthUserIDs(ctx)
	if err != nil {
		return 0, err
	}
	return s.remindUsers(ctx, users, time.Now(), s.sendEmail)
}

// remindUsers works through the users one at a time. One user's failure --
// a bad address, a plan that would not build -- is logged and passed over
// rather than stopping everyone else's reminders.
func (s *Service) remindUsers(ctx context.Context, users []string, now time.Time, send emailSender) (int, error) {
	sent := 0
	var errs []error
	for _, userID := range users {
		ok, err := s.remindUser(ctx, userID, now, send)
		if err != nil {
			s.logger.Error("deadline reminder failed", "user_id", userID, "error", err)
			errs = append(errs, fmt.Errorf("user %s: %w", userID, err))
			continue
		}
		if ok {
			sent++
		}
	}
	return sent, errors.Join(errs...)
}

// remindUser sends one user a single email covering every deadline in the
// window they have not already been reminded of. Reports whether it sent one.
func (s *Service) remindUser(ctx context.Context, userID string, now time.Time, send emailSender) (bool, error) {
	if setting, err := s.repo.GetUserSetting(ctx, userID, DeadlineRemindersSetting); err != nil {
		return false, err
	} else if setting == "off" {
		return false, nil
	}

	quests, _, err := s.GenerateQuests(ctx, userID)
	if err != nil {
		return false, err
	}
	due := dueReminders(quests, now)
	if len(due) == 0 {
		return false, nil
	}

	to := s.notificationEmail(ctx, userID)
	if to == "" {
		s.logger.Info("no email address for deadline reminders, skipping", "user_id", userID)
		return false, nil
	}

	keys := make([]repository.ReminderKey, len(due))
	for i, q := range due {
		keys[i] = repository.ReminderKey{CatalogKey: q.CatalogKey, DueOn: *q.DueDate}
	}
	claim, err := s.repo.ClaimReminders(ctx, userID, keys)
	if err != nil {
		return false, err
	}
	if len(claim.Keys) == 0 {
		claim.Release(ctx)
		return false, nil // every one of them has been sent already
	}

	claimed := map[string]bool{}
	for _, k := range claim.Keys {
		claimed[k.CatalogKey] = true
	}
	var fresh []domain.Quest
	for _, q := range due {
		if claimed[q.CatalogKey] {
			fresh = append(fresh, q)
		}
	}

	subject, body := s.reminderEmail(fresh, now)
	if err := send(ctx, to, subject, body); err != nil {
		claim.Release(ctx)
		return false, err
	}
	if err := claim.Commit(ctx); err != nil {
		return false, err
	}
	s.logger.Info("deadline reminder sent", "user_id", userID, "count", len(fresh))
	return true, nil
}

// reminderEmail writes the subject and HTML body for a set of deadlines.
func (s *Service) reminderEmail(due []domain.Quest, now time.Time) (subject, body string) {
	if len(due) == 1 {
		subject = fmt.Sprintf("Due %s: %s", dueWhen(*due[0].DueDate, now), due[0].Title)
	} else {
		subject = fmt.Sprintf("%d deadlines in your plan this week", len(due))
	}

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; color: #333; max-width: 650px; margin: 0 auto; padding: 20px; }
  .item { padding: 16px 0; border-bottom: 1px solid #e9ecef; }
  .item h3 { margin: 0; font-size: 16px; color: #212529; }
  .due { display: inline-block; margin-top: 4px; font-size: 13px; font-weight: 600; color: #b45309; }
  .item p { margin: 8px 0 0 0; font-size: 14px; color: #495057; }
  .btn-primary { display: inline-block; margin-top: 24px; background-color: #18181b; color: #ffffff; padding: 10px 20px; font-size: 15px; border-radius: 8px; text-decoration: none; }
  .footer { margin-top: 32px; padding-top: 16px; border-top: 1px solid #e9ecef; text-align: center; font-size: 13px; color: #adb5bd; }
</style>
</head>
<body>
<h2>Coming up in your plan</h2>
<p>These have deadlines that pass whether or not the rest of the plan is ready.</p>
`)
	for _, q := range due {
		b.WriteString(`<div class="item">`)
		fmt.Fprintf(&b, "<h3>%s</h3>", html.EscapeString(q.Title))
		fmt.Fprintf(&b, `<span class="due">Due %s</span>`, html.EscapeString(dueWhen(*q.DueDate, now)))
		if q.Detail != "" {
			fmt.Fprintf(&b, "<p>%s</p>", html.EscapeString(q.Detail))
		}
		b.WriteString("</div>")
	}
	fmt.Fprintf(&b, `<a href="%s/wealth" class="btn-primary">Open your plan</a>`, html.EscapeString(s.cfg.FrontendURL))
	b.WriteString(`<div class="footer">Sent by PPBudget. Turn these off under Settings, Email Notifications.</div>`)
	b.WriteString("</body></html>")
	return subject, b.String()
}

// dueWhen says a due date the way a person would: "today", "tomorrow", or
// "Tuesday 30 December".
func dueWhen(due, now time.Time) string {
	days := int(calendarDay(due).Sub(calendarDay(now)).Hours() / 24)
	switch days {
	case 0:
		return "today"
	case 1:
		return "tomorrow"
	}
	return calendarDay(due).Format("Monday 2 January")
}
