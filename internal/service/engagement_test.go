package service

import (
	"strings"
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
)

// Every action the catalog can produce needs a timeline title, or its history
// reads as a raw key.
func TestProgressTitlesCoverTheCatalog(t *testing.T) {
	for _, def := range catalog() {
		if _, ok := progressTitles[def.Key]; !ok {
			t.Errorf("%s has no progress title", def.Key)
		}
	}
}

func TestDescribeProgress(t *testing.T) {
	at := time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)
	names := map[string]string{"card-1": "Chase Sapphire"}
	event := func(key, ev, source, note string) domain.QuestEvent {
		return domain.QuestEvent{ID: "e", CatalogKey: key, Event: ev, Source: source, Note: note, CreatedAt: at}
	}

	cases := []struct {
		name             string
		e                domain.QuestEvent
		kind, title, how string
	}{
		{"a paid-off card, seen in the balance",
			event("clear_high_apr_balance:card-1", domain.QuestEventCompleted, domain.QuestSourceAuto, ""),
			ProgressDone, "Paid off Chase Sapphire", "confirmed by your accounts and answers"},
		{"done before the plan existed",
			event("starter_emergency_fund", domain.QuestEventCompleted, domain.QuestSourceAuto, firstSightNote),
			ProgressDone, "Built a one-month cushion", "already done when your plan was first worked out"},
		{"marked by the user",
			event("fund_ira_to_limit", domain.QuestEventCompleted, domain.QuestSourceManual, "done in April"),
			ProgressDone, "Funded your IRA to the limit", "you marked it done"},
		{"slipped",
			event("clear_high_apr_balance:card-1", domain.QuestEventUncompleted, domain.QuestSourceAuto, ""),
			ProgressReopened, "Reopened: the balance on Chase Sapphire", "no longer met in your accounts and answers"},
		{"an account since deleted",
			event("sweep_idle_cash:gone", domain.QuestEventCompleted, domain.QuestSourceAuto, ""),
			ProgressDone, "Moved idle cash out of an account no longer on file", "confirmed by your accounts and answers"},
		{"skipped",
			event("max_hsa", domain.QuestEventSkipped, domain.QuestSourceManual, ""),
			ProgressSkipped, "Skipped: your HSA contributions", "you marked it not for you"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := describeProgress(c.e, names)
			if got.Kind != c.kind || got.Title != c.title || got.How != c.how {
				t.Errorf("got %q / %q / %q\nwant %q / %q / %q", got.Kind, got.Title, got.How, c.kind, c.title, c.how)
			}
			if !got.At.Equal(at) {
				t.Errorf("at: got %v", got.At)
			}
		})
	}

	// Only what the user typed is shown back; engine notes stay internal.
	if got := describeProgress(event("fund_ira_to_limit", domain.QuestEventCompleted, domain.QuestSourceManual, "done in April"), names); got.Note != "done in April" {
		t.Errorf("manual note: got %q", got.Note)
	}
	if got := describeProgress(event("starter_emergency_fund", domain.QuestEventCompleted, domain.QuestSourceAuto, firstSightNote), names); got.Note != "" {
		t.Errorf("auto note leaked: %q", got.Note)
	}
}

// Only open actions due in the coming week are worth an email, soonest first.
func TestDueReminders(t *testing.T) {
	now := time.Date(2026, time.September, 27, 18, 0, 0, 0, time.UTC)
	day := func(d int) *time.Time {
		t := time.Date(2026, time.September, 27, 0, 0, 0, 0, time.UTC).AddDate(0, 0, d)
		return &t
	}
	open := domain.QuestStatusAvailable

	quests := []domain.Quest{
		{CatalogKey: "a:later", Status: open, DueDate: day(6)},
		{CatalogKey: "b:today", Status: open, DueDate: day(0)},
		{CatalogKey: "c:edge", Status: open, DueDate: day(7)},
		{CatalogKey: "d:too-far", Status: open, DueDate: day(8)},
		{CatalogKey: "e:passed", Status: open, DueDate: day(-1)},
		{CatalogKey: "f:done", Status: domain.QuestStatusComplete, DueDate: day(2)},
		{CatalogKey: "g:skipped", Status: domain.QuestStatusSkipped, DueDate: day(2)},
		{CatalogKey: "h:undated", Status: open},
	}
	var got []string
	for _, q := range dueReminders(quests, now) {
		got = append(got, q.CatalogKey)
	}
	if want := "b:today,a:later,c:edge"; strings.Join(got, ",") != want {
		t.Errorf("got %v want %s", got, want)
	}

	if got := dueWhen(*day(0), now); got != "today" {
		t.Errorf("today: %q", got)
	}
	if got := dueWhen(*day(1), now); got != "tomorrow" {
		t.Errorf("tomorrow: %q", got)
	}
	if got := dueWhen(*day(3), now); got != "Wednesday 30 September" {
		t.Errorf("later: %q", got)
	}
}

// The email escapes what it quotes and says how to turn it off.
func TestReminderEmail(t *testing.T) {
	now := time.Date(2026, time.September, 27, 0, 0, 0, 0, time.UTC)
	due := now.AddDate(0, 0, 1)
	svc := &Service{cfg: newRulesTestService(nil).cfg}
	subject, body := svc.reminderEmail([]domain.Quest{{Title: "Sell <shares>", Detail: "A & B", DueDate: &due}}, now)
	if subject != "Due tomorrow: Sell <shares>" {
		t.Errorf("subject: %q", subject)
	}
	if !strings.Contains(body, "Sell &lt;shares&gt;") || !strings.Contains(body, "A &amp; B") {
		t.Error("the body should escape the action's text")
	}
	if !strings.Contains(body, "Turn these off under Settings") {
		t.Error("the body should say how to turn reminders off")
	}
}
