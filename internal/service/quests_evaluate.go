package service

import (
	"context"
	"fmt"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Auto-verification.
//
// Most of the work already happens inside the catalog: a rule that can see its
// own condition returns Complete when it holds, so "your cushion covers a
// month" resolves itself from the balance every time the list is built. What
// was missing is everything around that:
//
//   - the transitions left no trace, so an action the engine closed looked in
//     the history exactly like one the user ticked;
//   - `verification` was recorded and never consulted, so a manual claim on an
//     action the app CAN observe survived data that contradicted it;
//   - conditions about a period of time were evaluated against an average,
//     which is not the same question. "Under the cap on average over six
//     months" and "under the cap in each of the last three" differ precisely
//     in the case that matters: one bad month hidden by five good ones.
//
// The evaluator is deliberately not a scheduled job. Generation already runs on
// every read, so the data is never staler than the page; a cron would add a
// second path to the same conclusion and a new way for the two to disagree.

// recordTransitions writes an event for every action whose status changed.
//
// Called after the new list is persisted, comparing against the statuses read
// at the start of the same generation. Failures are logged rather than
// returned: an action list the user can see beats a 500 over an audit row, and
// the row is recoverable on the next pass while the page is not.
func (s *Service) recordTransitions(
	ctx context.Context, userID string, before map[string]string, after []domain.Quest,
) {
	for _, q := range after {
		prior, existed := before[q.CatalogKey]
		if existed && prior == q.Status {
			continue
		}

		event, note := transitionEvent(prior, existed, q)
		if event == "" {
			continue
		}
		if err := s.repo.AppendQuestEvent(
			ctx, userID, q.ID, event, domain.QuestSourceAuto, note,
		); err != nil {
			s.logger.Warn("could not record quest transition",
				"user_id", userID, "catalog_key", q.CatalogKey, "error", err)
		}
	}
}

// transitionEvent names what happened, or returns empty for changes not worth
// a row.
//
// Only movements across the completion line are recorded. An action shifting
// between blocked and available as answers arrive is ordinary churn, and
// logging it would bury the two events anyone actually wants to find.
func transitionEvent(prior string, existed bool, q domain.Quest) (event, note string) {
	switch {
	case !existed:
		// First sight. Worth a row only when it arrives already satisfied,
		// which is the engine having verified something on the user's behalf
		// before they ever saw it.
		if q.Status == domain.QuestStatusComplete {
			return domain.QuestEventCompleted, "met when this action was first worked out"
		}
		return domain.QuestEventGenerated, ""

	case q.Status == domain.QuestStatusComplete:
		return domain.QuestEventCompleted, "verified from your accounts"

	case prior == domain.QuestStatusComplete:
		// The engine has reopened something. This is the transition most worth
		// recording: either the data moved, or a manual claim was overturned by
		// data that contradicted it.
		if q.Verification == domain.QuestVerificationAuto {
			return domain.QuestEventUncompleted, "no longer met in your accounts"
		}
		return domain.QuestEventUncompleted, ""
	}
	return "", ""
}

// ---------------------------------------------------- duration conditions

// monthsUnderCap counts the trailing complete months whose discretionary spend
// stayed at or below a ceiling, stopping at the first that did not.
//
// Consecutive from the most recent backwards, not a count of good months
// anywhere in the window: a streak that a bad month broke is not a streak, and
// treating it as one would tell someone they had held a limit they had not.
func monthsUnderCap(months []domain.PlanningMonth, cap money.Money) int {
	streak := 0
	for i := len(months) - 1; i >= 0; i-- {
		if months[i].Wants > cap {
			break
		}
		streak++
	}
	return streak
}

// EvaluateQuestConditions is the public entry point for a caller that wants a
// verification pass without rendering: generation performs one on every read,
// so this is simply that, named for what it does.
func (s *Service) EvaluateQuestConditions(ctx context.Context, userID string) ([]domain.Quest, error) {
	quests, _, err := s.GenerateQuests(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("evaluate: %w", err)
	}
	return quests, nil
}
