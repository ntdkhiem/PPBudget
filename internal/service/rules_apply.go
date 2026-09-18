package service

import (
	"context"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
)

// ruleChange is the net effect of every matching rule on one transaction,
// accumulated across the whole run before anything is written.
type ruleChange struct {
	txn            domain.Transaction
	categoryID     *string
	subscriptionID *string
	accountID      string
}

// RulePreviewSample is one before/after row shown in the preview dialog.
type RulePreviewSample struct {
	ID                string    `json:"id"`
	Date              time.Time `json:"date"`
	Description       string    `json:"description"`
	Amount            int64     `json:"amount"`
	CurrentCategoryID *string   `json:"current_category_id"`
	NewCategoryID     *string   `json:"new_category_id"`
	CurrentAccountID  string    `json:"current_account_id"`
	NewAccountID      string    `json:"new_account_id"`
	CurrentSubID      *string   `json:"current_subscription_id"`
	NewSubID          *string   `json:"new_subscription_id"`
}

// RulePreview is the result of a dry run.
type RulePreview struct {
	TotalScanned     int                 `json:"total_scanned"`
	MatchCount       int                 `json:"match_count"`
	WouldUpdateCount int                 `json:"would_update_count"`
	Samples          []RulePreviewSample `json:"samples"`
}

// evaluateRules walks txns once and returns the pending change for every
// transaction at least one rule wants to modify.
//
// Rules are applied in the order given -- callers pass them in priority order,
// which ListRulesDetailed already returns (priority DESC, created_at DESC). A
// later rule overwrites an earlier one's field, so the *last* rule in the slice
// wins ties; passing highest-priority-first therefore means highest priority is
// applied first and lowest wins. We reverse-iterate to make priority actually
// authoritative: the highest-priority rule writes last.
func (s *Service) evaluateRules(rules []domain.Rule, txns []domain.Transaction) []ruleChange {
	res := NewRuleResolver()
	changes := make([]ruleChange, 0)

	for _, t := range txns {
		pending := ruleChange{
			txn:            t,
			categoryID:     t.CategoryID,
			subscriptionID: t.SubscriptionID,
			accountID:      t.AccountID,
		}
		matched := false

		// Lowest priority first, so the highest-priority rule writes last and
		// its value is the one that survives.
		for i := len(rules) - 1; i >= 0; i-- {
			rule := rules[i]
			if !MatchesRule(rule, t, res) {
				continue
			}
			matched = true
			for _, act := range rule.Actions {
				if act.Value == "" {
					continue
				}
				switch act.ActionType {
				case ActionSetCategory:
					v := act.Value
					pending.categoryID = &v
				case ActionSetAccount:
					pending.accountID = act.Value
				case ActionLinkToSubscription:
					v := act.Value
					pending.subscriptionID = &v
				}
			}
		}

		if matched && changedFrom(t, pending) {
			changes = append(changes, pending)
		}
	}

	if bad := res.BadPatterns(); len(bad) > 0 {
		s.logger.Warn("rules engine skipped conditions with invalid regex patterns", "patterns", bad)
	}

	return changes
}

func changedFrom(t domain.Transaction, c ruleChange) bool {
	return !samePtr(t.CategoryID, c.categoryID) ||
		!samePtr(t.SubscriptionID, c.subscriptionID) ||
		t.AccountID != c.accountID
}

func samePtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// loadScopedTransactions fetches the transaction set an apply or preview runs
// against. It is deliberately called once per run, not once per rule.
func (s *Service) loadScopedTransactions(ctx context.Context, userID string, runAll bool, startDate, endDate *time.Time) ([]domain.Transaction, error) {
	var sDate, eDate *time.Time
	if !runAll {
		sDate = startDate
		eDate = endDate
	}
	return s.repo.GetTransactionsByDateRange(ctx, userID, sDate, eDate)
}

// ApplyRules evaluates every rule against a single load of the transaction set
// and writes the resulting changes. It returns the number of transactions
// actually updated.
//
// The previous implementation loaded the full transaction set once per rule --
// each load running four correlated subqueries and two json_agg aggregations
// per row -- and then rewrote every transaction_links row of each match just to
// change a category.
func (s *Service) ApplyRules(ctx context.Context, userID string, rules []domain.Rule, runAll bool, startDate, endDate *time.Time) (int, error) {
	if len(rules) == 0 {
		return 0, nil
	}

	txns, err := s.loadScopedTransactions(ctx, userID, runAll, startDate, endDate)
	if err != nil {
		return 0, err
	}

	changes := s.evaluateRules(rules, txns)
	if len(changes) == 0 {
		return 0, nil
	}

	updates := make([]domain.TransactionRuleUpdate, 0, len(changes))
	for _, c := range changes {
		updates = append(updates, domain.TransactionRuleUpdate{
			ID:             c.txn.ID,
			CategoryID:     c.categoryID,
			SubscriptionID: c.subscriptionID,
			AccountID:      c.accountID,
		})
	}

	updated, err := s.repo.ApplyRuleUpdates(ctx, userID, updates)
	if err != nil {
		return updated, err
	}

	// A short write means rows were filtered by ownership or soft-deletion
	// between the read and the write. Surface it rather than silently
	// under-reporting, which is what the old per-row `continue` did.
	if updated != len(updates) {
		s.logger.Warn("rules apply updated fewer transactions than matched",
			"user_id", userID, "matched", len(updates), "updated", updated)
	}

	return updated, nil
}

// ApplyRule applies a single saved rule. It is a thin wrapper over ApplyRules
// so the HTTP path and the sync path share one implementation.
func (s *Service) ApplyRule(ctx context.Context, userID, ruleID string, runAll bool, startDate, endDate *time.Time) (int, error) {
	rule, err := s.repo.GetRule(ctx, userID, ruleID)
	if err != nil {
		return 0, err
	}
	return s.ApplyRules(ctx, userID, []domain.Rule{*rule}, runAll, startDate, endDate)
}

// ApplyActiveRules applies every active rule for a user in one pass. This is
// the sync path (internal/service/simplefin.go).
func (s *Service) ApplyActiveRules(ctx context.Context, userID string, runAll bool, startDate, endDate *time.Time) (int, error) {
	all, err := s.repo.ListRulesDetailed(ctx, userID)
	if err != nil {
		return 0, err
	}

	active := make([]domain.Rule, 0, len(all))
	for _, r := range all {
		if r.IsActive {
			active = append(active, r)
		}
	}

	return s.ApplyRules(ctx, userID, active, runAll, startDate, endDate)
}

// PreviewRule runs a rule without writing anything and reports what would
// change. It shares evaluateRules with the apply path, so a preview cannot
// disagree with the apply that follows it.
//
// The rule need not be saved -- the dialog previews rules that do not exist
// yet -- so the rule is validated here rather than relying on the write path.
func (s *Service) PreviewRule(ctx context.Context, userID string, rule *domain.Rule, runAll bool, startDate, endDate *time.Time, limit int) (*RulePreview, error) {
	NormalizeRule(rule)
	if err := ValidateRule(rule); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}

	txns, err := s.loadScopedTransactions(ctx, userID, runAll, startDate, endDate)
	if err != nil {
		return nil, err
	}

	res := NewRuleResolver()
	matchCount := 0
	for _, t := range txns {
		if MatchesRule(*rule, t, res) {
			matchCount++
		}
	}

	changes := s.evaluateRules([]domain.Rule{*rule}, txns)

	preview := &RulePreview{
		TotalScanned:     len(txns),
		MatchCount:       matchCount,
		WouldUpdateCount: len(changes),
		Samples:          make([]RulePreviewSample, 0, min(limit, len(changes))),
	}

	for i, c := range changes {
		if i >= limit {
			break
		}
		preview.Samples = append(preview.Samples, RulePreviewSample{
			ID:                c.txn.ID,
			Date:              c.txn.Date,
			Description:       c.txn.Description,
			Amount:            c.txn.Amount.ToInt64(),
			CurrentCategoryID: c.txn.CategoryID,
			NewCategoryID:     c.categoryID,
			CurrentAccountID:  c.txn.AccountID,
			NewAccountID:      c.accountID,
			CurrentSubID:      c.txn.SubscriptionID,
			NewSubID:          c.subscriptionID,
		})
	}

	return preview, nil
}
