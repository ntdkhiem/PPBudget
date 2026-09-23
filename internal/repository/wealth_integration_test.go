package repository

// Integration tests for Wealth Strategy persistence. Gated on TEST_DATABASE_URL;
// see balances_integration_test.go for the helpers and the throwaway-database warning.

import (
	"context"
	"errors"
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

func ptr[T any](v T) *T { return &v }

// The write path takes the fields to set explicitly rather than inferring them
// from which pointers are non-nil, because those are different questions: nil
// means "not mentioned in this request", while a key in setKeys with a nil
// value means "the user cleared this". These tests pin that distinction, since
// collapsing it would make an answer impossible to unset and would silently
// rewrite every field on every partial save.
func TestWealthProfileRoundTripIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "wealth-profile")

	// A user who has never started intake gets an empty profile, not an error.
	// The generator reads that as "every field unanswered" and locks the
	// actions that need them, which is correct for a new account.
	empty, err := repo.GetWealthProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetWealthProfile on a fresh user: %v", err)
	}
	if empty.DateOfBirth != nil || empty.MatchPct != nil {
		t.Fatalf("fresh profile should be entirely unanswered, got %+v", empty)
	}
	if len(empty.Fields) != 0 {
		t.Fatalf("fresh profile should have no provenance, got %d entries", len(empty.Fields))
	}

	dob := time.Date(1990, 4, 12, 0, 0, 0, 0, time.UTC)
	p := &domain.WealthProfile{
		DateOfBirth:   &dob,
		FilingStatus:  ptr("mfj"),
		ResidentState: ptr("CA"),
		MatchPct:      ptr(0.5),
		MatchLimitPct: ptr(0.06),
	}
	keys := []string{"date_of_birth", "filing_status", "resident_state", "match_pct", "match_limit_pct"}
	if err := repo.UpsertWealthProfile(ctx, userID, p, keys, domain.FieldSourceEntered); err != nil {
		t.Fatalf("UpsertWealthProfile: %v", err)
	}

	got, err := repo.GetWealthProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetWealthProfile: %v", err)
	}
	if got.DateOfBirth == nil || !got.DateOfBirth.Equal(dob) {
		t.Errorf("date_of_birth: got %v want %v", got.DateOfBirth, dob)
	}
	if got.FilingStatus == nil || *got.FilingStatus != "mfj" {
		t.Errorf("filing_status: got %v want mfj", got.FilingStatus)
	}
	if got.MatchPct == nil || *got.MatchPct != 0.5 {
		t.Errorf("match_pct: got %v want 0.5", got.MatchPct)
	}
	// Untouched fields stay unanswered rather than defaulting to zero.
	if got.DeferralPct != nil {
		t.Errorf("deferral_pct was never set; expected nil, got %v", *got.DeferralPct)
	}
	for _, k := range keys {
		f, ok := got.Fields[k]
		if !ok {
			t.Errorf("no provenance recorded for %q", k)
			continue
		}
		if f.Source != domain.FieldSourceEntered {
			t.Errorf("%q source: got %q want %q", k, f.Source, domain.FieldSourceEntered)
		}
	}

	// A partial save touches only its own fields.
	if err := repo.UpsertWealthProfile(ctx, userID,
		&domain.WealthProfile{DeferralPct: ptr(0.06)},
		[]string{"deferral_pct"}, domain.FieldSourceEntered); err != nil {
		t.Fatalf("partial upsert: %v", err)
	}
	got, err = repo.GetWealthProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetWealthProfile after partial: %v", err)
	}
	if got.DeferralPct == nil || *got.DeferralPct != 0.06 {
		t.Errorf("deferral_pct: got %v want 0.06", got.DeferralPct)
	}
	if got.FilingStatus == nil || *got.FilingStatus != "mfj" {
		t.Errorf("partial save clobbered filing_status: got %v", got.FilingStatus)
	}

	// Naming a key with a nil value clears it. This is the case that
	// non-nil-inference could never express.
	if err := repo.UpsertWealthProfile(ctx, userID,
		&domain.WealthProfile{MatchPct: nil},
		[]string{"match_pct"}, domain.FieldSourceEntered); err != nil {
		t.Fatalf("clearing upsert: %v", err)
	}
	got, err = repo.GetWealthProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetWealthProfile after clear: %v", err)
	}
	if got.MatchPct != nil {
		t.Errorf("match_pct should have been cleared, got %v", *got.MatchPct)
	}
}

// Provenance is load-bearing: the generator refuses to run high-stakes branches
// off a value the user never saw. A derived pre-fill that is later confirmed
// must actually change source, or the backdoor Roth decision would stay blocked
// after the user answered it.
func TestWealthProfileProvenanceTransitionIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "wealth-provenance")

	p := &domain.WealthProfile{TargetAnnualSpend: ptr(money.Money(5_322_400))}
	if err := repo.UpsertWealthProfile(ctx, userID, p,
		[]string{"target_annual_spend"}, domain.FieldSourceDerived); err != nil {
		t.Fatalf("derived upsert: %v", err)
	}

	got, _ := repo.GetWealthProfile(ctx, userID)
	first := got.Fields["target_annual_spend"]
	if first.Source != domain.FieldSourceDerived {
		t.Fatalf("expected derived, got %q", first.Source)
	}

	time.Sleep(10 * time.Millisecond) // so answered_at can move measurably

	if err := repo.UpsertWealthProfile(ctx, userID, p,
		[]string{"target_annual_spend"}, domain.FieldSourceConfirmed); err != nil {
		t.Fatalf("confirming upsert: %v", err)
	}
	got, _ = repo.GetWealthProfile(ctx, userID)
	second := got.Fields["target_annual_spend"]
	if second.Source != domain.FieldSourceConfirmed {
		t.Errorf("source: got %q want %q", second.Source, domain.FieldSourceConfirmed)
	}
	if !second.AnsweredAt.After(first.AnsweredAt) {
		t.Errorf("answered_at should advance on re-affirmation: %v then %v",
			first.AnsweredAt, second.AnsweredAt)
	}
}

func TestWealthProfileRejectsUnknownFieldIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "wealth-unknown")

	err := repo.UpsertWealthProfile(ctx, userID, &domain.WealthProfile{},
		[]string{"not_a_real_field"}, domain.FieldSourceEntered)
	if !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("unknown field key should be rejected as invalid input, got %v", err)
	}

	err = repo.UpsertWealthProfile(ctx, userID, &domain.WealthProfile{},
		[]string{"filing_status"}, "made_up_source")
	if !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("unknown source should be rejected as invalid input, got %v", err)
	}
}

// A missing year must be an error rather than an empty set: an empty set would
// let every contribution rule compute headroom against a zero limit and tell
// the user they have nothing left to contribute.
func TestTaxLimitsIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	for _, year := range []int{2025, 2026} {
		limits, err := repo.GetTaxLimits(ctx, year)
		if err != nil {
			t.Fatalf("GetTaxLimits(%d): %v", year, err)
		}
		deferral, ok := limits.Amount(domain.LimitElectiveDeferral402g)
		if !ok || deferral <= 0 {
			t.Errorf("%d: 402(g) limit missing or non-positive: %v", year, deferral)
		}
		selfOnly, ok := limits.Amount(domain.LimitHSASelfOnly)
		if !ok || selfOnly <= 0 {
			t.Errorf("%d: HSA self-only limit missing", year)
		}
		family, ok := limits.Amount(domain.LimitHSAFamily)
		if !ok || family <= selfOnly {
			t.Errorf("%d: HSA family limit should exceed self-only, got %v vs %v", year, family, selfOnly)
		}
		rate, ok := limits.Rate(domain.RateSupplementalLow)
		if !ok || rate <= 0 || rate >= 1 {
			t.Errorf("%d: supplemental rate out of range: %v", year, rate)
		}
	}

	if _, err := repo.GetTaxLimits(ctx, 1999); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("unseeded year should return ErrNotFound, got %v", err)
	}
}

// Regeneration runs whenever the profile changes, so it must never undo work.
func TestQuestReplacePreservesCompletionIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "wealth-quests")

	initial := []domain.Quest{
		{CatalogKey: "capture_employer_match", Phase: 1, Priority: 0,
			Status: domain.QuestStatusAvailable, Title: "Capture $4,000 of employer match",
			Verification: domain.QuestVerificationManual},
		{CatalogKey: "starter_emergency_fund", Phase: 1, Priority: 10,
			Status: domain.QuestStatusAvailable, Title: "Build a one-month buffer",
			Verification: domain.QuestVerificationManual},
		{CatalogKey: "max_hsa", Phase: 3, Priority: 20,
			Status: domain.QuestStatusLocked, Title: "Max your HSA",
			Verification: domain.QuestVerificationManual},
	}
	if err := repo.ReplaceQuests(ctx, userID, initial); err != nil {
		t.Fatalf("ReplaceQuests: %v", err)
	}

	quests, err := repo.ListQuests(ctx, userID)
	if err != nil {
		t.Fatalf("ListQuests: %v", err)
	}
	if len(quests) != 3 {
		t.Fatalf("expected 3 quests, got %d", len(quests))
	}
	// Ordering is phase then priority: the match must lead.
	if quests[0].CatalogKey != "capture_employer_match" {
		t.Errorf("employer match should sort first, got %q", quests[0].CatalogKey)
	}

	if _, err := repo.SetQuestStatus(ctx, userID, quests[0].ID,
		domain.QuestStatusComplete, domain.QuestSourceManual, "done in payroll"); err != nil {
		t.Fatalf("SetQuestStatus: %v", err)
	}

	// Regenerate with fresh text and a dropped action.
	regenerated := []domain.Quest{
		{CatalogKey: "capture_employer_match", Phase: 1, Priority: 0,
			Status: domain.QuestStatusAvailable, Title: "REGENERATED TITLE",
			Verification: domain.QuestVerificationManual},
		{CatalogKey: "starter_emergency_fund", Phase: 1, Priority: 10,
			Status: domain.QuestStatusAvailable, Title: "Build a one-month buffer",
			Verification: domain.QuestVerificationManual},
	}
	if err := repo.ReplaceQuests(ctx, userID, regenerated); err != nil {
		t.Fatalf("ReplaceQuests (regenerate): %v", err)
	}

	quests, _ = repo.ListQuests(ctx, userID)
	byKey := map[string]domain.Quest{}
	for _, q := range quests {
		byKey[q.CatalogKey] = q
	}

	match := byKey["capture_employer_match"]
	if match.Status != domain.QuestStatusComplete {
		t.Errorf("regeneration undid a completion: status %q", match.Status)
	}
	if match.CompletedAt == nil {
		t.Error("completed_at lost on regeneration")
	}
	// Text still refreshes on a completed action, so the record reads correctly.
	if match.Title != "REGENERATED TITLE" {
		t.Errorf("title should refresh even when complete, got %q", match.Title)
	}
	// An action the catalog stopped producing is pruned, unless it was done.
	if _, still := byKey["max_hsa"]; still {
		t.Error("max_hsa was dropped by the catalog and not complete; expected it pruned")
	}

	events, err := repo.ListQuestEvents(ctx, userID, match.ID)
	if err != nil {
		t.Fatalf("ListQuestEvents: %v", err)
	}
	if len(events) != 1 || events[0].Event != domain.QuestEventCompleted {
		t.Errorf("expected one 'completed' event, got %+v", events)
	}
	if events[0].Source != domain.QuestSourceManual {
		t.Errorf("event source: got %q want manual", events[0].Source)
	}
}

func TestPlannedExpensesAndTermsIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "wealth-terms")

	cardID, err := repo.CreateAccount(ctx, userID, "Wealth Test Card", "liability", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// APR finally lives somewhere typed rather than inside a JSON blob.
	if err := repo.UpsertAccountTerms(ctx, userID, domain.AccountTerms{
		AccountID:  cardID,
		APR:        ptr(0.2249),
		MinPayment: ptr(money.Money(3500)),
		DueDay:     ptr(14),
	}); err != nil {
		t.Fatalf("UpsertAccountTerms: %v", err)
	}
	terms, err := repo.ListAccountTerms(ctx, userID)
	if err != nil {
		t.Fatalf("ListAccountTerms: %v", err)
	}
	if got := terms[cardID].APR; got == nil || *got != 0.2249 {
		t.Errorf("apr: got %v want 0.2249", got)
	}

	// Terms on someone else's account must be refused, not silently written.
	otherUser := createTestUser(t, pool, "wealth-terms-other")
	err = repo.UpsertAccountTerms(ctx, otherUser, domain.AccountTerms{AccountID: cardID, APR: ptr(0.01)})
	if !errors.Is(err, apperrors.ErrForbidden) {
		t.Errorf("cross-user account terms write should be forbidden, got %v", err)
	}

	// Planned expenses are what stop the engine from investing a down payment.
	target := time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC)
	id, err := repo.CreatePlannedExpense(ctx, userID, domain.PlannedExpense{
		Label: "House down payment", Amount: money.Money(6_000_000), TargetDate: target,
	})
	if err != nil {
		t.Fatalf("CreatePlannedExpense: %v", err)
	}
	expenses, err := repo.ListPlannedExpenses(ctx, userID)
	if err != nil {
		t.Fatalf("ListPlannedExpenses: %v", err)
	}
	if len(expenses) != 1 || expenses[0].Amount != money.Money(6_000_000) {
		t.Fatalf("expected one planned expense of 6,000,000 cents, got %+v", expenses)
	}
	if err := repo.DeletePlannedExpense(ctx, userID, id); err != nil {
		t.Fatalf("DeletePlannedExpense: %v", err)
	}
	if err := repo.DeletePlannedExpense(ctx, userID, id); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("second delete should report not found, got %v", err)
	}
}

func TestEquityGrantsIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "wealth-equity")

	// A schedule without an anchor is not a projection: grant date, shares
	// granted and shares vested are what turn "quarterly" into a real date.
	vest := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	grant := domain.EquityGrant{
		Kind:           domain.EquityKindRSU,
		Label:          ptr("2026 refresh"),
		GrantDate:      ptr(time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)),
		TotalShares:    ptr(1000.0),
		SharesVested:   ptr(125.0),
		NextVestDate:   &vest,
		VestFrequency:  ptr("quarterly"),
		VestShare:      ptr(0.0625),
		HasRule10b51:   ptr(false),
		BlackoutPolicy: ptr("quarterly"),
	}
	id, err := repo.UpsertEquityGrant(ctx, userID, grant)
	if err != nil {
		t.Fatalf("UpsertEquityGrant (create): %v", err)
	}

	grants, err := repo.ListEquityGrants(ctx, userID)
	if err != nil {
		t.Fatalf("ListEquityGrants: %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("expected 1 grant, got %d", len(grants))
	}
	if grants[0].NextVestDate == nil || !grants[0].NextVestDate.Equal(vest) {
		t.Errorf("next_vest_date: got %v want %v", grants[0].NextVestDate, vest)
	}
	if grants[0].SharesVested == nil || *grants[0].SharesVested != 125.0 {
		t.Errorf("shares_vested: got %v want 125", grants[0].SharesVested)
	}

	grant.ID = id
	grant.SharesVested = ptr(187.5)
	if _, err := repo.UpsertEquityGrant(ctx, userID, grant); err != nil {
		t.Fatalf("UpsertEquityGrant (update): %v", err)
	}
	grants, _ = repo.ListEquityGrants(ctx, userID)
	if grants[0].SharesVested == nil || *grants[0].SharesVested != 187.5 {
		t.Errorf("shares_vested after update: got %v want 187.5", grants[0].SharesVested)
	}

	if err := repo.DeleteEquityGrant(ctx, userID, id); err != nil {
		t.Fatalf("DeleteEquityGrant: %v", err)
	}
	if err := repo.DeleteEquityGrant(ctx, userID, id); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("second delete should report not found, got %v", err)
	}
}

// Regeneration must be free to revise what the ENGINE concluded while leaving
// what the USER claimed alone.
//
// A computed completion is only a reading of the data -- "your cushion covers a
// month" -- and has to move when the data does. A manual completion is a claim
// about something the app cannot observe -- "I set up the 10b5-1" -- and the
// engine has no standing to contradict it. Collapsing the two either way is a
// real failure: sticky auto-completions let a plan report success built on a
// balance that has since fallen, and revisable manual ones silently undo work.
func TestQuestCompletionStickinessDependsOnSourceIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "wealth-sticky")

	// Two actions the generator itself closed.
	completedAt := time.Now().UTC()
	if err := repo.ReplaceQuests(ctx, userID, []domain.Quest{
		{CatalogKey: "starter_emergency_fund", Phase: 1, Status: domain.QuestStatusComplete,
			Title: "You have a one-month cushion", Verification: domain.QuestVerificationAuto,
			CompletedAt: &completedAt},
		{CatalogKey: "establish_sell_on_vest", Phase: 2, Status: domain.QuestStatusComplete,
			Title: "Selling on vest is automatic", Verification: domain.QuestVerificationManual,
			CompletedAt: &completedAt},
	}); err != nil {
		t.Fatalf("ReplaceQuests: %v", err)
	}

	quests, _ := repo.ListQuests(ctx, userID)
	for _, q := range quests {
		if q.CompletedSource != domain.QuestSourceAuto {
			t.Errorf("%s: generator-closed actions must be stamped auto, got %q",
				q.CatalogKey, q.CompletedSource)
		}
	}

	// The user then claims the second one themselves.
	var sellID string
	for _, q := range quests {
		if q.CatalogKey == "establish_sell_on_vest" {
			sellID = q.ID
		}
	}
	if _, err := repo.SetQuestStatus(ctx, userID, sellID,
		domain.QuestStatusComplete, domain.QuestSourceManual, "set up the plan"); err != nil {
		t.Fatalf("SetQuestStatus: %v", err)
	}

	// Regenerate with both now computing as NOT complete -- the cushion was
	// spent, and the engine cannot see the 10b5-1 at all.
	if err := repo.ReplaceQuests(ctx, userID, []domain.Quest{
		{CatalogKey: "starter_emergency_fund", Phase: 1, Status: domain.QuestStatusAvailable,
			Title: "Put $4,435 aside as a starter cushion", Verification: domain.QuestVerificationAuto},
		{CatalogKey: "establish_sell_on_vest", Phase: 2, Status: domain.QuestStatusAvailable,
			Title: "Set up automatic selling when equity vests", Verification: domain.QuestVerificationManual},
	}); err != nil {
		t.Fatalf("ReplaceQuests (regenerate): %v", err)
	}

	byKey := map[string]domain.Quest{}
	after, _ := repo.ListQuests(ctx, userID)
	for _, q := range after {
		byKey[q.CatalogKey] = q
	}

	if got := byKey["starter_emergency_fund"].Status; got != domain.QuestStatusAvailable {
		t.Errorf("an auto-completion must be revisable when the data changes; status %q", got)
	}
	if byKey["starter_emergency_fund"].CompletedAt != nil {
		t.Error("a reopened auto-completion should clear its timestamp")
	}
	if got := byKey["establish_sell_on_vest"].Status; got != domain.QuestStatusComplete {
		t.Errorf("a manual claim must survive regeneration; status %q", got)
	}
	if got := byKey["establish_sell_on_vest"].CompletedSource; got != domain.QuestSourceManual {
		t.Errorf("manual source should persist; got %q", got)
	}
}

// `verification` decides whether the engine may overturn a manual claim.
//
// The rule is about what the app can SEE. For an action it can observe -- a
// balance, a contribution rate -- a stale claim that something was done is
// worse than no claim, because the user is now looking at a plan that reports
// success it can itself disprove. For one it cannot observe -- setting up a
// 10b5-1 -- their word is the only evidence in existence and the engine has no
// standing to contradict it.
func TestManualClaimSurvivesOnlyWhereTheAppCannotSeeIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "wealth-verification")

	completedAt := time.Now().UTC()
	seed := []domain.Quest{
		{CatalogKey: "starter_emergency_fund", Phase: 1, Status: domain.QuestStatusAvailable,
			Title: "Put aside a starter cushion", Verification: domain.QuestVerificationAuto},
		{CatalogKey: "establish_sell_on_vest", Phase: 2, Status: domain.QuestStatusAvailable,
			Title: "Set up automatic selling", Verification: domain.QuestVerificationManual},
		{CatalogKey: "max_hsa", Phase: 3, Status: domain.QuestStatusAvailable,
			Title: "Max your HSA", Verification: domain.QuestVerificationAuto},
	}
	if err := repo.ReplaceQuests(ctx, userID, seed); err != nil {
		t.Fatalf("ReplaceQuests: %v", err)
	}

	stored, _ := repo.ListQuests(ctx, userID)
	byKey := map[string]domain.Quest{}
	for _, q := range stored {
		byKey[q.CatalogKey] = q
	}

	// The user claims all three by hand.
	for _, key := range []string{"starter_emergency_fund", "establish_sell_on_vest", "max_hsa"} {
		if _, err := repo.SetQuestStatus(ctx, userID, byKey[key].ID,
			domain.QuestStatusComplete, domain.QuestSourceManual, ""); err != nil {
			t.Fatalf("SetQuestStatus(%s): %v", key, err)
		}
	}
	_ = completedAt

	// Regeneration disagrees about the first (it can see the balance), cannot
	// work out the third (an answer went missing), and says nothing about the
	// second that it is entitled to say.
	regenerated := []domain.Quest{
		{CatalogKey: "starter_emergency_fund", Phase: 1, Status: domain.QuestStatusAvailable,
			Title: "Put aside a starter cushion", Verification: domain.QuestVerificationAuto},
		{CatalogKey: "establish_sell_on_vest", Phase: 2, Status: domain.QuestStatusAvailable,
			Title: "Set up automatic selling", Verification: domain.QuestVerificationManual},
		{CatalogKey: "max_hsa", Phase: 3, Status: domain.QuestStatusBlocked,
			Title: "Work out your HSA contribution headroom", Verification: domain.QuestVerificationAuto,
			MissingFields: []string{"hsa_coverage_tier"}},
	}
	if err := repo.ReplaceQuests(ctx, userID, regenerated); err != nil {
		t.Fatalf("ReplaceQuests (regenerate): %v", err)
	}

	after, _ := repo.ListQuests(ctx, userID)
	got := map[string]domain.Quest{}
	for _, q := range after {
		got[q.CatalogKey] = q
	}

	if s := got["starter_emergency_fund"].Status; s != domain.QuestStatusAvailable {
		t.Errorf("an auto-verifiable action the data contradicts should reopen; status %q", s)
	}
	if s := got["establish_sell_on_vest"].Status; s != domain.QuestStatusComplete {
		t.Errorf("the app cannot see a 10b5-1; the user's claim stands. status %q", s)
	}
	// "We could not work it out" is not the same as "it is not done", and only
	// the second justifies overturning someone.
	if s := got["max_hsa"].Status; s != domain.QuestStatusComplete {
		t.Errorf("a blocked recomputation must not overturn a manual claim; status %q", s)
	}
}
