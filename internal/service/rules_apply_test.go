package service

import (
	"io"
	"log/slog"
	"testing"

	"ntdkhiem/ppbudget-go/internal/domain"
)

const (
	subNetflix = "33333333-3333-3333-3333-333333333333"
	subSpotify = "44444444-4444-4444-4444-444444444444"
	catStream  = "55555555-5555-5555-5555-555555555555"
)

// evaluateRules needs only the logger; the repository is untouched.
func newEvalService() *Service {
	return &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func act(actionType, value string) domain.RuleAction {
	return domain.RuleAction{ActionType: actionType, Value: value}
}

func strPtr(s string) *string { return &s }

// The link_to_subscription action is what the Recurring Payments page reads to
// decide whether a subscription was paid this period, and it had no coverage.
func TestEvaluateRulesLinksSubscription(t *testing.T) {
	s := newEvalService()

	netflix := txn("NETFLIX.COM", -1599, acctChecking)
	netflix.ID = "netflix-txn"
	groceries := txn("SAFEWAY #221", -8200, acctChecking)
	groceries.ID = "groceries-txn"

	r := rule(StrictnessAll, cond(FieldDescription, OpContains, "netflix"))
	r.Actions = []domain.RuleAction{act(ActionLinkToSubscription, subNetflix)}

	changes := s.evaluateRules([]domain.Rule{r}, []domain.Transaction{netflix, groceries})

	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].txn.ID != netflix.ID {
		t.Errorf("changed the wrong transaction: got %q, want %q", changes[0].txn.ID, netflix.ID)
	}
	if changes[0].subscriptionID == nil || *changes[0].subscriptionID != subNetflix {
		t.Errorf("subscription_id = %v, want %q", changes[0].subscriptionID, subNetflix)
	}
	// A subscription-only rule must leave the other two writable columns alone.
	if changes[0].categoryID != nil {
		t.Errorf("category_id = %v, want nil", changes[0].categoryID)
	}
	if changes[0].accountID != netflix.AccountID {
		t.Errorf("account_id = %q, want %q", changes[0].accountID, netflix.AccountID)
	}
}

func TestEvaluateRulesLinksSubscriptionAlongsideCategory(t *testing.T) {
	s := newEvalService()

	netflix := txn("NETFLIX.COM", -1599, acctChecking)
	r := rule(StrictnessAll, cond(FieldDescription, OpContains, "netflix"))
	r.Actions = []domain.RuleAction{
		act(ActionSetCategory, catStream),
		act(ActionLinkToSubscription, subNetflix),
	}

	changes := s.evaluateRules([]domain.Rule{r}, []domain.Transaction{netflix})
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].categoryID == nil || *changes[0].categoryID != catStream {
		t.Errorf("category_id = %v, want %q", changes[0].categoryID, catStream)
	}
	if changes[0].subscriptionID == nil || *changes[0].subscriptionID != subNetflix {
		t.Errorf("subscription_id = %v, want %q", changes[0].subscriptionID, subNetflix)
	}
}

// An already-linked transaction is not rewritten, so a repeat apply is a no-op
// rather than a batch of pointless UPDATEs.
func TestEvaluateRulesSkipsAlreadyLinkedSubscription(t *testing.T) {
	s := newEvalService()

	netflix := txn("NETFLIX.COM", -1599, acctChecking)
	netflix.SubscriptionID = strPtr(subNetflix)

	r := rule(StrictnessAll, cond(FieldDescription, OpContains, "netflix"))
	r.Actions = []domain.RuleAction{act(ActionLinkToSubscription, subNetflix)}

	if changes := s.evaluateRules([]domain.Rule{r}, []domain.Transaction{netflix}); len(changes) != 0 {
		t.Fatalf("expected no changes for an already-linked transaction, got %d", len(changes))
	}
}

// Rules arrive highest-priority-first, and evaluateRules reverse-iterates so the
// highest-priority rule writes last. Two rules pointing one transaction at
// different subscriptions must resolve to the higher-priority one.
func TestEvaluateRulesSubscriptionPriorityWins(t *testing.T) {
	s := newEvalService()

	netflix := txn("NETFLIX.COM", -1599, acctChecking)

	high := rule(StrictnessAll, cond(FieldDescription, OpContains, "netflix"))
	high.Priority = 10
	high.Actions = []domain.RuleAction{act(ActionLinkToSubscription, subNetflix)}

	low := rule(StrictnessAll, cond(FieldDescription, OpContains, "netflix"))
	low.Priority = 1
	low.Actions = []domain.RuleAction{act(ActionLinkToSubscription, subSpotify)}

	changes := s.evaluateRules([]domain.Rule{high, low}, []domain.Transaction{netflix})
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	if changes[0].subscriptionID == nil || *changes[0].subscriptionID != subNetflix {
		t.Errorf("subscription_id = %v, want the higher-priority %q", changes[0].subscriptionID, subNetflix)
	}
}
