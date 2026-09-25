package repository

// Integration tests for Wealth Strategy persistence. Gated on TEST_DATABASE_URL;
// see balances_integration_test.go for the helpers and the throwaway-database warning.

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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
	// And the provenance row goes with it. Everything downstream reads
	// "answered" as "a provenance row exists", so a row surviving a cleared
	// value would report the question as answered while the answer is gone --
	// the generator would stop asking for it and evaluate against nothing.
	if f, ok := got.Fields["match_pct"]; ok {
		t.Errorf("match_pct was cleared but still claims provenance %+v", f)
	}
	// Its neighbours are untouched: clearing one answer is not a reset.
	if _, ok := got.Fields["match_limit_pct"]; !ok {
		t.Error("clearing match_pct removed the provenance for match_limit_pct")
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

func TestPlannedExpensesAndTermsIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "wealth-terms")

	cardID, err := repo.CreateAccount(ctx, userID, "Wealth Test Card", "liability", "USD", 0, nil)
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

// Roles decide what the plan counts as cash, debt and investment, so the
// write path has to keep them consistent with the balance-sheet side and
// leave them alone when an edit does not mention them.
func TestAccountRolesIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "account-roles")

	roleOf := func(id string) *string {
		t.Helper()
		a, err := repo.GetAccount(ctx, userID, id)
		if err != nil {
			t.Fatalf("GetAccount: %v", err)
		}
		return a.Role
	}

	id, err := repo.CreateAccount(ctx, userID, "Ally HYSA", "asset", "USD", 623_614, ptr(domain.RoleSavings))
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if r := roleOf(id); r == nil || *r != domain.RoleSavings {
		t.Fatalf("role on create: got %v want savings", r)
	}

	// An edit that does not mention the role leaves it alone.
	if err := repo.UpdateAccount(ctx, userID, id, domain.AccountUpdate{Name: "Ally Savings", Type: "asset"}); err != nil {
		t.Fatalf("UpdateAccount (rename): %v", err)
	}
	if r := roleOf(id); r == nil || *r != domain.RoleSavings {
		t.Errorf("a rename cleared the role: got %v", r)
	}

	// A role that disagrees with the type is refused as bad input, not a 500.
	err = repo.UpdateAccount(ctx, userID, id, domain.AccountUpdate{
		Name: "Ally Savings", Type: "liability", RoleSet: true, Role: ptr(domain.RoleSavings),
	})
	if !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("savings on a liability: got %v, want ErrInvalidInput", err)
	}

	// Clearing is explicit.
	if err := repo.UpdateAccount(ctx, userID, id, domain.AccountUpdate{
		Name: "Ally Savings", Type: "asset", RoleSet: true,
	}); err != nil {
		t.Fatalf("UpdateAccount (clear): %v", err)
	}
	if r := roleOf(id); r != nil {
		t.Errorf("role should be cleared, got %q", *r)
	}

	// Linking a tax treatment makes the account an investment in the same write.
	if err := repo.UpsertRetirementAccountTerms(ctx, userID, domain.RetirementAccountTerms{
		AccountID: id, Kind: domain.RetirementKindRothIRA, MonthlyContribution: 43_333,
	}); err != nil {
		t.Fatalf("UpsertRetirementAccountTerms: %v", err)
	}
	if r := roleOf(id); r == nil || *r != domain.RoleInvestment {
		t.Errorf("a linked account should be an investment, got %v", r)
	}

	// A debt cannot hold retirement savings.
	card, err := repo.CreateAccount(ctx, userID, "Chase Sapphire", "liability", "USD", -382_435, ptr(domain.RoleCreditCard))
	if err != nil {
		t.Fatalf("CreateAccount (card): %v", err)
	}
	err = repo.UpsertRetirementAccountTerms(ctx, userID, domain.RetirementAccountTerms{
		AccountID: card, Kind: domain.RetirementKind401k,
	})
	if !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("linking a card: got %v, want ErrInvalidInput", err)
	}
	if terms, _ := repo.ListRetirementAccountTerms(ctx, userID); len(terms) != 1 {
		t.Errorf("the refused link should not have written terms; got %+v", terms)
	}
}

// A reading of the action list writes only what changed. The same status twice
// is no write at all, and the first achievement survives a later dip.
func TestQuestTransitionsRecordOnlyChangesIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "quest-transitions")
	const key = "starter_emergency_fund"

	record := func(status, event string) bool {
		t.Helper()
		changed, err := repo.RecordQuestTransition(ctx, userID, key, status, event, domain.QuestSourceAuto, "")
		if err != nil {
			t.Fatalf("RecordQuestTransition(%s): %v", status, err)
		}
		return changed
	}
	events := func() []domain.QuestEvent {
		t.Helper()
		evs, err := repo.ListQuestEvents(ctx, userID, key)
		if err != nil {
			t.Fatalf("ListQuestEvents: %v", err)
		}
		return evs
	}

	if !record(domain.QuestStatusAvailable, domain.QuestEventGenerated) {
		t.Fatal("first sight should write")
	}
	if record(domain.QuestStatusAvailable, domain.QuestEventGenerated) {
		t.Error("the same status again should write nothing")
	}
	if n := len(events()); n != 1 {
		t.Errorf("events after an unchanged reading: got %d want 1", n)
	}

	if !record(domain.QuestStatusComplete, domain.QuestEventCompleted) {
		t.Fatal("completion should write")
	}
	// A change with no event worth recording still moves the status.
	if !record(domain.QuestStatusAvailable, "") {
		t.Fatal("reopening should write")
	}
	state, err := repo.ListQuestState(ctx, userID)
	if err != nil {
		t.Fatalf("ListQuestState: %v", err)
	}
	if s := state[key]; s.Status != domain.QuestStatusAvailable || s.AchievedAt == nil {
		t.Errorf("after a dip: status %q, achieved %v -- the first achievement must be kept", s.Status, s.AchievedAt)
	}
	if n := len(events()); n != 2 {
		t.Errorf("events: got %d want 2 (generated, completed)", n)
	}
}

// Two tabs reading the list at once both see the same change. The transition
// must be written, and its event appended, exactly once.
func TestQuestTransitionIsRecordedOnceUnderConcurrencyIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "quest-concurrency")
	const key = "full_emergency_fund"

	if _, err := repo.RecordQuestTransition(ctx, userID, key, domain.QuestStatusAvailable,
		domain.QuestEventGenerated, domain.QuestSourceAuto, ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const readers = 8
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			changed, err := repo.RecordQuestTransition(ctx, userID, key, domain.QuestStatusComplete,
				domain.QuestEventCompleted, domain.QuestSourceAuto, "verified from your accounts")
			if err != nil {
				t.Errorf("RecordQuestTransition: %v", err)
			}
			if changed {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()

	if w := wins.Load(); w != 1 {
		t.Errorf("exactly one reader should record the change; %d did", w)
	}
	evs, _ := repo.ListQuestEvents(ctx, userID, key)
	var completed int
	for _, e := range evs {
		if e.Event == domain.QuestEventCompleted {
			completed++
		}
	}
	if completed != 1 {
		t.Errorf("completed events: got %d want 1", completed)
	}
}

// Marks are the user's word. Setting one moves the stored status with it, so
// the next reading has nothing new to record; taking one back reopens the
// action and keeps the history of both.
func TestQuestMarksIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)
	userID := createTestUser(t, pool, "quest-marks")
	const done, notForMe = "establish_sell_on_vest", "harvest_tax_losses"

	if err := repo.SetQuestMark(ctx, userID, done, domain.QuestStatusComplete, "set up the 10b5-1"); err != nil {
		t.Fatalf("SetQuestMark(complete): %v", err)
	}
	if err := repo.SetQuestMark(ctx, userID, notForMe, domain.QuestStatusSkipped, ""); err != nil {
		t.Fatalf("SetQuestMark(skipped): %v", err)
	}
	if err := repo.SetQuestMark(ctx, userID, done, "available", ""); !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("a mark other than done or skipped: got %v, want ErrInvalidInput", err)
	}

	marks, err := repo.ListQuestMarks(ctx, userID)
	if err != nil {
		t.Fatalf("ListQuestMarks: %v", err)
	}
	if m := marks[done]; m.Mark != domain.QuestStatusComplete || m.Note != "set up the 10b5-1" {
		t.Errorf("done mark: got %+v", m)
	}
	if m := marks[notForMe]; m.Mark != domain.QuestStatusSkipped {
		t.Errorf("skip mark: got %+v", m)
	}
	state, _ := repo.ListQuestState(ctx, userID)
	if s := state[done]; s.Status != domain.QuestStatusComplete || s.AchievedAt == nil {
		t.Errorf("the stored status should move with the mark; got %+v", s)
	}
	// The next reading sees the same status and records nothing more.
	if changed, _ := repo.RecordQuestTransition(ctx, userID, done, domain.QuestStatusComplete,
		domain.QuestEventCompleted, domain.QuestSourceAuto, ""); changed {
		t.Error("a reading that agrees with the mark should not write")
	}

	if err := repo.ClearQuestMark(ctx, userID, done); err != nil {
		t.Fatalf("ClearQuestMark: %v", err)
	}
	if err := repo.ClearQuestMark(ctx, userID, done); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("clearing twice: got %v, want ErrNotFound", err)
	}
	marks, _ = repo.ListQuestMarks(ctx, userID)
	if _, still := marks[done]; still {
		t.Error("the mark should be gone")
	}

	evs, err := repo.ListQuestEvents(ctx, userID, done)
	if err != nil {
		t.Fatalf("ListQuestEvents: %v", err)
	}
	if len(evs) != 2 || evs[0].Event != domain.QuestEventUncompleted || evs[1].Event != domain.QuestEventCompleted {
		t.Fatalf("history newest first: got %+v", evs)
	}
	if evs[1].Source != domain.QuestSourceManual || evs[1].Note != "set up the 10b5-1" {
		t.Errorf("the completion should be the user's, with their note; got %+v", evs[1])
	}
}
