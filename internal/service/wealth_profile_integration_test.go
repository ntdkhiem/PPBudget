package service

// Integration tests for the wealth profile backfill and derivation. Gated on
// TEST_DATABASE_URL; reuses sfSetupTestDB / sfCreateTestUser from
// simplefin_integration_test.go.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ntdkhiem/ppbudget-go/internal/domain"
)

// mkTxnCat inserts a categorized transaction. The rules tests' mkTxn leaves
// category_id null, which would land the spend in the unbucketed pile and
// never reach essentials.
func mkTxnCat(t *testing.T, pool *pgxpool.Pool, userID, accountID, categoryID string, cents int64, date, description string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO transactions (account_id, user_id, category_id, amount, date, description)
		 VALUES ($1, $2, $3, $4, $5::date, $6)`,
		accountID, userID, categoryID, cents, date, description,
	); err != nil {
		t.Fatalf("failed to insert categorized transaction: %v", err)
	}
}

// The backfill runs once, on first read, rather than as a migration step: it
// needs no downtime and costs nothing for users who never open the page.
//
// Everything it carries over is marked 'derived' rather than 'entered'. The
// user never saw these values in this context, and one of them -- date of birth,
// reconstructed from a stored age -- is genuinely approximate. Marking them
// derived is what makes the intake ask for confirmation instead of quietly
// building a plan on an inference.
func TestWealthProfileBackfillFromLegacyBlobsIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	cardID := mkAccount(t, pool, userID, "Legacy Card")
	k401ID := mkAccount(t, pool, userID, "Legacy 401k")

	financialPlan := `{
		"version": 1,
		"strategy": "balanced",
		"emergency_fund": {"target_months": 9},
		"debts": [{"account_id": "` + cardID + `", "apr": 0.2249, "min_payment_cents": 3500}],
		"assumptions": {"invest_return_apr": 0.07}
	}`
	retirementPlan := `{
		"version": 1,
		"current_age": 36,
		"target_retirement_age": 55,
		"gross_annual_income_cents": 14500000,
		"expected_return_apr": 0.07,
		"inflation_apr": 0.03,
		"withdrawal_rate": 0.04,
		"accounts": [{
			"account_id": "` + k401ID + `",
			"kind": "401k",
			"monthly_contribution_cents": 24166,
			"employer_match_pct": 0.5,
			"employer_match_limit_pct": 0.06
		}]
	}`
	if err := svc.SetUserSetting(ctx, userID, "financial_plan", financialPlan); err != nil {
		t.Fatalf("seed financial_plan: %v", err)
	}
	if err := svc.SetUserSetting(ctx, userID, "retirement_plan", retirementPlan); err != nil {
		t.Fatalf("seed retirement_plan: %v", err)
	}

	profile, err := svc.GetWealthProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetWealthProfile: %v", err)
	}

	if profile.EmergencyFundTargetMonths == nil || *profile.EmergencyFundTargetMonths != 9 {
		t.Errorf("emergency fund months: got %v want 9", profile.EmergencyFundTargetMonths)
	}
	if profile.TargetIndependenceAge == nil || *profile.TargetIndependenceAge != 55 {
		t.Errorf("target independence age: got %v want 55", profile.TargetIndependenceAge)
	}
	if profile.GrossAnnualIncome == nil || *profile.GrossAnnualIncome != 14_500_000 {
		t.Errorf("gross annual income: got %v want 14500000", profile.GrossAnnualIncome)
	}
	// Match terms were per-account in the blob but describe one workplace plan,
	// so they move up to the profile where the match rule reads them.
	if profile.MatchPct == nil || *profile.MatchPct != 0.5 {
		t.Errorf("match_pct: got %v want 0.5", profile.MatchPct)
	}
	if profile.MatchLimitPct == nil || *profile.MatchLimitPct != 0.06 {
		t.Errorf("match_limit_pct: got %v want 0.06", profile.MatchLimitPct)
	}

	// Age reconstructs only to the year, and the exact date decides catch-up
	// eligibility, so this must arrive as an approximation to confirm.
	if profile.DateOfBirth == nil {
		t.Fatal("date_of_birth should be approximated from the stored age")
	}
	wantYear := time.Now().UTC().Year() - 36
	if profile.DateOfBirth.Year() != wantYear {
		t.Errorf("approximated birth year: got %d want %d", profile.DateOfBirth.Year(), wantYear)
	}

	// The provenance is the point. A backfilled value must not look like one
	// the user supplied.
	for _, key := range []string{
		"emergency_fund_target_months", "target_independence_age",
		"gross_annual_income", "match_pct", "date_of_birth",
	} {
		f, ok := profile.Fields[key]
		if !ok {
			t.Errorf("no provenance recorded for backfilled field %q", key)
			continue
		}
		if f.Source != domain.FieldSourceDerived {
			t.Errorf("%q: backfilled values must be %q, got %q",
				key, domain.FieldSourceDerived, f.Source)
		}
	}

	// APR leaves the blob for a typed column, which is what clears the cash
	// plan's "you have liabilities but no interest rates entered" state.
	terms, err := svc.ListAccountTerms(ctx, userID)
	if err != nil {
		t.Fatalf("ListAccountTerms: %v", err)
	}
	if got := terms[cardID].APR; got == nil || *got != 0.2249 {
		t.Errorf("carried-over APR: got %v want 0.2249", got)
	}
	if got := terms[cardID].MinPayment; got == nil || *got != 3500 {
		t.Errorf("carried-over minimum payment: got %v want 3500", got)
	}

	// Idempotent: a second read must not re-run the backfill and stamp fresh
	// provenance over an answer the user has since confirmed.
	if _, err := svc.SaveWealthProfile(ctx, userID,
		&domain.WealthProfile{GrossAnnualIncome: profile.GrossAnnualIncome},
		[]string{"gross_annual_income"}, domain.FieldSourceConfirmed); err != nil {
		t.Fatalf("confirming a backfilled value: %v", err)
	}
	again, err := svc.GetWealthProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetWealthProfile (second): %v", err)
	}
	if got := again.Fields["gross_annual_income"].Source; got != domain.FieldSourceConfirmed {
		t.Errorf("a re-run backfill overwrote a confirmed answer: source is %q", got)
	}
}

// A user with no legacy blobs must not acquire a phantom profile.
func TestWealthProfileNoBackfillWhenNothingToCarryIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	profile, err := svc.GetWealthProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetWealthProfile: %v", err)
	}
	if len(profile.Fields) != 0 {
		t.Errorf("expected no answered fields, got %v", profile.Fields)
	}
	if profile.GrossAnnualIncome != nil || profile.DateOfBirth != nil {
		t.Error("a fresh user should have an entirely unanswered profile")
	}
}

// Derived figures must agree with what Cash Plan already shows, or the user
// sees one number on one screen and a plan built on a different one elsewhere.
func TestDeriveProfileValuesMatchesBaselineIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	acct := mkAccount(t, pool, userID, "Derive Checking")
	rent := mkCategory(t, pool, userID, "Rent")

	// Bucket the category as a need so the spend lands in essentials rather
	// than the unbucketed pile. The planning query carries the most recent
	// budget row at or before each month forward, so one row dated before the
	// fixtures covers all of them.
	if _, err := pool.Exec(ctx,
		`INSERT INTO budgets (user_id, name, category_id, amount, period_type, start_date, end_date, bucket)
		 VALUES ($1, 'Rent', $2, 300000, 'monthly',
		         date_trunc('month', CURRENT_DATE - INTERVAL '6 months')::date,
		         (date_trunc('month', CURRENT_DATE - INTERVAL '6 months') + INTERVAL '1 month - 1 day')::date,
		         'needs')`,
		userID, rent); err != nil {
		t.Fatalf("seed budget: %v", err)
	}

	// Three complete prior months of identical rent, so the average is exact
	// and any divergence is a real disagreement rather than rounding.
	for i := 1; i <= 3; i++ {
		date := time.Now().UTC().AddDate(0, -i, 0).Format("2006-01-02")
		mkTxnCat(t, pool, userID, acct, rent, -250000, date, "Rent")
		mkTxn(t, pool, userID, acct, 600000, date, "Paycheck")
	}

	pb, err := svc.GetPlanningBaseline(ctx, userID, BaselineMonths)
	if err != nil {
		t.Fatalf("GetPlanningBaseline: %v", err)
	}
	baseline := ComputeBaseline(pb, time.Now())
	// Nothing has posted this month, so all three prior months are complete.
	// Dropping the last month with activity instead of the current one
	// counted two.
	if baseline.MonthsOfData != 3 {
		t.Errorf("months of data: got %d want 3", baseline.MonthsOfData)
	}

	derived, err := svc.DeriveProfileValues(ctx, userID)
	if err != nil {
		t.Fatalf("DeriveProfileValues: %v", err)
	}
	if derived.MonthsOfData != baseline.MonthsOfData {
		t.Errorf("months of data: derived %d, baseline %d", derived.MonthsOfData, baseline.MonthsOfData)
	}

	var spend *DerivedValue
	for i := range derived.Values {
		if derived.Values[i].FieldKey == "target_annual_spend" {
			spend = &derived.Values[i]
		}
	}
	if spend == nil {
		t.Fatalf("no target_annual_spend derived; values were %+v", derived.Values)
	}
	if want := baseline.EssentialMonthly * 12; *spend.Amount != want {
		t.Errorf("target annual spend: derived %d, baseline implies %d", *spend.Amount, want)
	}
	if spend.AlreadyAnswered {
		t.Error("nothing was answered yet; AlreadyAnswered should be false")
	}

	// Questions are listed as unanswered; corrections to derived figures are
	// not, since having none is the normal state.
	var asksBirthDate bool
	for _, key := range derived.Unanswered {
		asksBirthDate = asksBirthDate || key == "date_of_birth"
		if strings.HasPrefix(key, "override_") {
			t.Errorf("%s is a correction, not a question, and should not be listed as unanswered", key)
		}
	}
	if !asksBirthDate {
		t.Errorf("an unanswered question should still be listed; got %v", derived.Unanswered)
	}

	// Deriving writes nothing. Confirming is a separate, explicit act -- which
	// is what separates a figure the user saw from one the app inferred.
	after, err := svc.GetWealthProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetWealthProfile: %v", err)
	}
	if _, ok := after.Fields["target_annual_spend"]; ok {
		t.Error("DeriveProfileValues must not persist anything")
	}
}

// Staleness flags an answer past its cadence rather than hiding the action
// built on it: a plan quietly using last year's salary is the failure, and
// dropping the action silently hides it just as well as trusting it.
func TestStaleFieldsHonoursPerFieldCadence(t *testing.T) {
	now := time.Now().UTC()
	p := &domain.WealthProfile{Fields: map[string]domain.ProfileField{
		// Annual cadence, answered 400 days ago: stale.
		"gross_annual_income": {FieldKey: "gross_annual_income", Source: domain.FieldSourceEntered,
			AnsweredAt: now.AddDate(0, 0, -400)},
		// Annual cadence, answered last week: fresh.
		"match_pct": {FieldKey: "match_pct", Source: domain.FieldSourceEntered,
			AnsweredAt: now.AddDate(0, 0, -7)},
		// Quarterly cadence, answered 100 days ago: stale.
		"taxable_brokerage_value": {FieldKey: "taxable_brokerage_value", Source: domain.FieldSourceEntered,
			AnsweredAt: now.AddDate(0, 0, -100)},
		// No cadence: a birth date does not go out of date.
		"date_of_birth": {FieldKey: "date_of_birth", Source: domain.FieldSourceEntered,
			AnsweredAt: now.AddDate(-10, 0, 0)},
	}}

	stale := map[string]bool{}
	for _, k := range StaleFields(p, now) {
		stale[k] = true
	}

	if !stale["gross_annual_income"] {
		t.Error("salary answered 400 days ago should be stale")
	}
	if stale["match_pct"] {
		t.Error("match answered a week ago should be fresh")
	}
	if !stale["taxable_brokerage_value"] {
		t.Error("a brokerage balance 100 days old should be stale on a quarterly cadence")
	}
	if stale["date_of_birth"] {
		t.Error("date of birth has no cadence and must never be stale")
	}
	// An unanswered field is unasked, not stale.
	if stale["prior_year_agi"] {
		t.Error("an unanswered field must not report as stale")
	}
}

// The last four things the financial_plan blob held.
//
// Goals matter most of the three: strategy and the overrides can be re-entered
// from memory, but a goal is a name, an amount and a date that came from the
// person. Nothing re-derives those, so losing one in the migration loses it
// outright.
func TestBackfillCarriesGoalsStrategyAndOverridesIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	savings := mkAccount(t, pool, userID, "Ally HYSA")

	financialPlan := `{
		"version": 1,
		"strategy": "aggressive",
		"overrides": {
			"monthly_income_cents": 682360,
			"essential_expenses_cents": 443538,
			"liquid_account_ids": ["` + savings + `"]
		},
		"emergency_fund": {"target_months": 6},
		"goals": [
			{"id":"a","name":"House down payment","target_cents":6000000,"target_date":"2027-03-01","priority":0},
			{"id":"b","name":"New car","target_cents":2500000,"linked_account_id":"` + savings + `","current_cents":500000,"priority":1}
		],
		"assumptions": {"invest_return_apr": 0.07}
	}`
	if err := svc.SetUserSetting(ctx, userID, "financial_plan", financialPlan); err != nil {
		t.Fatalf("seed financial_plan: %v", err)
	}

	profile, err := svc.GetWealthProfile(ctx, userID)
	if err != nil {
		t.Fatalf("GetWealthProfile: %v", err)
	}

	if profile.Strategy == nil || *profile.Strategy != "aggressive" {
		t.Errorf("strategy: got %v want aggressive", profile.Strategy)
	}
	if profile.OverrideMonthlyIncome == nil || *profile.OverrideMonthlyIncome != 682_360 {
		t.Errorf("income override: got %v want 682360", profile.OverrideMonthlyIncome)
	}
	if profile.OverrideEssentialExpenses == nil || *profile.OverrideEssentialExpenses != 443_538 {
		t.Errorf("essentials override: got %v want 443538", profile.OverrideEssentialExpenses)
	}
	// The hand-picked cash list arrives as a role on the account itself.
	accounts, err := svc.ListAccounts(ctx, userID)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	for _, a := range accounts {
		if a.ID == savings && (a.Role == nil || *a.Role != domain.RoleSavings) {
			t.Errorf("the picked cash account should now be savings; role is %v", a.Role)
		}
	}

	goals, err := svc.ListGoals(ctx, userID)
	if err != nil {
		t.Fatalf("ListGoals: %v", err)
	}
	if len(goals) != 2 {
		t.Fatalf("expected both goals carried over, got %d", len(goals))
	}
	// Ordered by the user's own priority, not by insertion or name.
	if goals[0].Name != "House down payment" {
		t.Errorf("first goal: got %q want the priority-0 one", goals[0].Name)
	}
	if goals[0].TargetAmount != 6_000_000 {
		t.Errorf("target: got %v want 6000000", goals[0].TargetAmount)
	}
	if goals[0].TargetDate == nil || goals[0].TargetDate.Format("2006-01-02") != "2027-03-01" {
		t.Errorf("target date: got %v want 2027-03-01", goals[0].TargetDate)
	}
	if goals[1].LinkedAccountID == nil || *goals[1].LinkedAccountID != savings {
		t.Errorf("linked account: got %v want %s", goals[1].LinkedAccountID, savings)
	}
	if goals[1].CurrentAmount != 500_000 {
		t.Errorf("current amount: got %v want 500000", goals[1].CurrentAmount)
	}
}

func TestGoalsCRUDIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	target := time.Date(2028, 6, 1, 0, 0, 0, 0, time.UTC)
	id, err := svc.UpsertGoal(ctx, userID, domain.Goal{
		Name: "Sabbatical", TargetAmount: 3_000_000, TargetDate: &target, Priority: 0,
	})
	if err != nil {
		t.Fatalf("UpsertGoal: %v", err)
	}

	goals, _ := svc.ListGoals(ctx, userID)
	if len(goals) != 1 || goals[0].Name != "Sabbatical" {
		t.Fatalf("expected one goal, got %+v", goals)
	}

	if _, err := svc.UpsertGoal(ctx, userID, domain.Goal{
		ID: id, Name: "Sabbatical", TargetAmount: 4_000_000, Priority: 0,
	}); err != nil {
		t.Fatalf("UpsertGoal (update): %v", err)
	}
	goals, _ = svc.ListGoals(ctx, userID)
	if goals[0].TargetAmount != 4_000_000 {
		t.Errorf("updated target: got %v want 4000000", goals[0].TargetAmount)
	}

	// A goal linked to someone else's account must be refused rather than
	// silently stored against a row the user cannot see.
	other := sfCreateTestUser(t, pool)
	otherAccount := mkAccount(t, pool, other, "Their Savings")
	if _, err := svc.UpsertGoal(ctx, userID, domain.Goal{
		Name: "Sneaky", TargetAmount: 100, LinkedAccountID: &otherAccount,
	}); err == nil {
		t.Error("linking a goal to another user's account should be refused")
	}

	if err := svc.DeleteGoal(ctx, userID, id); err != nil {
		t.Fatalf("DeleteGoal: %v", err)
	}
	if err := svc.DeleteGoal(ctx, userID, id); err == nil {
		t.Error("second delete should report not found")
	}
}
