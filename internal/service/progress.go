package service

import (
	"context"
	"strings"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
)

// The progress timeline: what the user has done, read back out of the action
// history.
//
// Every status change across the completion line is already recorded (see
// recordTransitions); this turns those rows into sentences. Titles are fixed
// per action and written in the past tense, not taken from the live list,
// whose titles carry the figures of the moment -- "Clear the $3,824 balance on
// Chase Sapphire" reads wrong the day after it is cleared.

// Progress kinds. Pages branch on these, never on the prose.
const (
	ProgressDone     = "done"
	ProgressReopened = "reopened"
	ProgressSkipped  = "skipped"
	ProgressRestored = "restored"
)

// ProgressEvent is one line of the timeline.
type ProgressEvent struct {
	ID         string `json:"id"`
	CatalogKey string `json:"catalog_key"`
	Kind       string `json:"kind"`
	// Source is auto when the engine saw it in the user's data, manual when the
	// user said so.
	Source string `json:"source"`
	Title  string `json:"title"`
	// How says what settled it: the user's data, or the user.
	How string `json:"how"`
	// Note is what the user wrote when marking an action, if anything.
	Note string    `json:"note,omitempty"`
	At   time.Time `json:"at"`
}

const (
	defaultProgressLimit = 20
	maxProgressLimit     = 100
)

// firstSightNote is the note recordTransitions leaves on an action that was
// already complete when the plan first produced it. Matched here so the
// timeline can say "already done" rather than claim the user just did it.
const firstSightNote = "met when this action was first worked out"

// progressTitle is how one action reads in the timeline. Both forms may carry
// {name}, the account or grant a fan-out instance is about.
type progressTitle struct {
	// Done is the past-tense line for a completion.
	Done string
	// Subject names the action for everything else: "Reopened: <subject>".
	Subject string
}

// progressTitles covers every catalog key; a test holds it to that.
var progressTitles = map[string]progressTitle{
	// Protection
	"secure_disability_coverage": {"Checked your disability coverage", "your disability coverage"},
	"secure_life_coverage":       {"Checked your life cover", "your life cover"},

	// Phase 1
	"classify_accounts":                 {"Said what each of your accounts is", "your account roles"},
	"capture_employer_match":            {"Claimed your full employer match", "your employer match"},
	"starter_emergency_fund":            {"Built a one-month cushion", "your starter cushion"},
	"clear_promo_balance_before_expiry": {"Cleared the promotional balance on {name}", "the promotional balance on {name}"},
	"clear_high_apr_balance":            {"Paid off {name}", "the balance on {name}"},
	"sweep_idle_cash":                   {"Moved idle cash out of {name}", "idle cash in {name}"},
	"discretionary_cap":                 {"Held discretionary spending under your cap", "your discretionary spending cap"},
	"full_emergency_fund":               {"Built your full emergency fund", "your emergency fund"},

	// Phase 2
	"establish_sell_on_vest": {"Set up selling on vest", "selling on vest"},
	"liquidate_vested_rsu":   {"Sold the vested shares from {name}", "the vested shares from {name}"},
	"sell_espp_at_purchase":  {"Sold the purchased shares from {name}", "the purchased shares from {name}"},
	"reserve_equity_tax_gap": {"Set aside the tax on your equity", "the tax on your equity"},
	"calibrate_w4_withholding": {
		"Brought your withholding into line with your equity income", "your W-4 withholding",
	},
	"maximize_espp_rate": {"Raised your contribution to {name}", "your contribution to {name}"},

	// Phase 3
	"raise_401k_deferral":      {"Raised your 401(k) contribution", "your 401(k) contribution"},
	"auto_escalate_deferral":   {"Turned on automatic 401(k) increases", "automatic 401(k) increases"},
	"max_hsa":                  {"Filled your HSA for the year", "your HSA contributions"},
	"resolve_roth_route":       {"Settled how you fund a Roth IRA", "your Roth IRA route"},
	"mega_backdoor_roth":       {"Set up a mega backdoor Roth", "a mega backdoor Roth"},
	"fund_ira_to_limit":        {"Funded your IRA to the limit", "your IRA contribution"},
	"elect_dependent_care_fsa": {"Elected a dependent care FSA", "a dependent care FSA"},

	// Phase 4
	"reduce_employer_concentration": {"Brought employer stock under 5% of your net worth", "your employer stock concentration"},
	"consolidate_into_index_core":   {"Consolidated into index funds", "consolidating into index funds"},
	"harvest_tax_losses":            {"Harvested your investment losses", "harvesting investment losses"},
	"enable_drip":                   {"Turned on dividend reinvestment", "dividend reinvestment"},
	"automate_surplus_sweep":        {"Automated investing your surplus", "investing your surplus automatically"},

	// Phase 5
	"eliminate_consumer_debt":      {"Cleared your consumer debt", "your consumer debt"},
	"evaluate_student_loan_payoff": {"Settled your student loan plan", "paying student loans down early"},
	"remove_pmi":                   {"Dropped your mortgage insurance", "your mortgage insurance"},
	"evaluate_mortgage_prepay":     {"Settled your mortgage plan", "paying your mortgage down early"},
	"build_pre59_bridge":           {"Planned your money before 59 and a half", "your money before 59 and a half"},
	"scale_savings_rate":           {"Reached your target savings rate", "your savings rate"},
	"reach_fi_number":              {"Reached your independence number", "your independence number"},
}

// grantFanOut names the actions whose key suffix is an equity grant rather
// than an account.
var grantFanOut = map[string]bool{
	"liquidate_vested_rsu":  true,
	"sell_espp_at_purchase": true,
	"maximize_espp_rate":    true,
}

// ListProgress returns the user's timeline, newest first.
func (s *Service) ListProgress(ctx context.Context, userID string, limit int) ([]ProgressEvent, error) {
	if limit <= 0 {
		limit = defaultProgressLimit
	}
	limit = min(limit, maxProgressLimit)

	events, err := s.repo.ListProgressEvents(ctx, userID, limit)
	if err != nil {
		return nil, err
	}

	var names map[string]string
	for _, e := range events {
		if strings.Contains(e.CatalogKey, ":") {
			if names, err = s.progressNames(ctx, userID); err != nil {
				return nil, err
			}
			break
		}
	}

	out := make([]ProgressEvent, 0, len(events))
	for _, e := range events {
		out = append(out, describeProgress(e, names))
	}
	return out, nil
}

// progressNames maps account and grant IDs to what the user calls them.
func (s *Service) progressNames(ctx context.Context, userID string) (map[string]string, error) {
	names := map[string]string{}
	accounts, err := s.repo.ListAccounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, a := range accounts {
		names[a.ID] = a.Name
	}
	grants, err := s.repo.ListEquityGrants(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, g := range grants {
		names[g.ID] = grantName(g)
	}
	return names, nil
}

func grantName(g domain.EquityGrant) string {
	if g.Label != nil && strings.TrimSpace(*g.Label) != "" {
		return strings.TrimSpace(*g.Label)
	}
	switch g.Kind {
	case domain.EquityKindESPP:
		return "your ESPP"
	case domain.EquityKindRSU:
		return "your RSU grant"
	}
	return "your stock grant"
}

// describeProgress turns one history row into a timeline line.
func describeProgress(e domain.QuestEvent, names map[string]string) ProgressEvent {
	base, suffix, fanOut := strings.Cut(e.CatalogKey, ":")

	t, ok := progressTitles[base]
	if !ok {
		// A retired action keeps its history; its key is the best name left.
		subject := strings.ReplaceAll(base, "_", " ")
		t = progressTitle{Done: "Finished: " + subject, Subject: subject}
	}
	name := names[suffix]
	if name == "" {
		name = "an account no longer on file"
		if grantFanOut[base] {
			name = "a grant no longer on file"
		}
	}
	fill := func(s string) string {
		if !fanOut {
			return s
		}
		return strings.ReplaceAll(s, "{name}", name)
	}

	out := ProgressEvent{
		ID:         e.ID,
		CatalogKey: e.CatalogKey,
		Source:     e.Source,
		At:         e.CreatedAt,
	}
	manual := e.Source == domain.QuestSourceManual
	if manual {
		out.Note = e.Note
	}

	switch e.Event {
	case domain.QuestEventCompleted:
		out.Kind, out.Title = ProgressDone, fill(t.Done)
		switch {
		case manual:
			out.How = "you marked it done"
		case e.Note == firstSightNote:
			out.How = "already done when your plan was first worked out"
		default:
			out.How = "confirmed by your accounts and answers"
		}
	case domain.QuestEventUncompleted:
		out.Kind, out.Title = ProgressReopened, "Reopened: "+fill(t.Subject)
		if manual {
			out.How = "you took back your mark"
		} else {
			out.How = "no longer met in your accounts and answers"
		}
	case domain.QuestEventSkipped:
		out.Kind, out.Title = ProgressSkipped, "Skipped: "+fill(t.Subject)
		out.How = "you marked it not for you"
	case domain.QuestEventUnskipped:
		out.Kind, out.Title = ProgressRestored, "Back on your plan: "+fill(t.Subject)
		out.How = "you put it back"
	}
	return out
}
