package service

// The headline verification for the action engine: feed it the persona from
// "Wealth Strategy.pdf" and assert that what comes back is the CORRECTED plan
// rather than a reproduction of the document.
//
// The source document is a five-chapter plan for a single filer in California
// on a $145,000 base with an $80,000 RSU grant, an ESPP, a 401(k) left at 2%,
// and a Chase card carrying over 20% APR. Its numbers largely reconcile; its
// sequencing and several of its tax conclusions do not.
//
// Gated on TEST_DATABASE_URL.

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Figures taken from the document, in cents.
const (
	personaGrossAnnual   = 16_500_000 // $145,000 base plus ~$20,000/yr of vesting RSUs
	personaMonthlyIncome = 682_360    // $6,823.60 net
	personaEssentials    = 443_538    // $4,435.38 "Needs"
	personaWants         = 50_000     // the $500 discretionary cap
	personaChaseBalance  = 382_435    // $3,824.35 statement balance
	personaBofASavings   = 583_622    // $5,836.22 sitting at ~0%
	personaAllyHYSA      = 623_614    // $6,236.14
)

// setRole classifies an account the way the Accounts page would. Unclassified
// accounts count as neither cash nor investment, so fixtures must say.
func setRole(t *testing.T, pool *pgxpool.Pool, accountID, role string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE accounts SET role = $2 WHERE id = $1`, accountID, role); err != nil {
		t.Fatalf("set role %s on %s: %v", role, accountID, err)
	}
}

func seedPDFPersona(t *testing.T, pool *pgxpool.Pool, svc *Service, userID string) {
	t.Helper()
	ctx := context.Background()

	checking := mkAccount(t, pool, userID, "BofA Checking")
	savings := mkAccount(t, pool, userID, "BofA Savings")
	hysa := mkAccount(t, pool, userID, "Ally HYSA")
	setRole(t, pool, checking, domain.RoleChecking)
	setRole(t, pool, savings, domain.RoleSavings)
	setRole(t, pool, hysa, domain.RoleSavings)

	var chase string
	if err := pool.QueryRow(ctx,
		`INSERT INTO accounts (user_id, name, type, role, currency) VALUES ($1, 'Chase Sapphire', 'liability', 'credit_card', 'USD') RETURNING id`,
		userID).Scan(&chase); err != nil {
		t.Fatalf("create card: %v", err)
	}

	// Balances via opening snapshots, which is how the app anchors them.
	setBalance := func(id string, cents int64) {
		if _, err := pool.Exec(ctx,
			`INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
			 VALUES ($1, $2, '-infinity'::date, $3, 'opening')
			 ON CONFLICT (account_id, as_of_date) DO UPDATE SET balance = EXCLUDED.balance`,
			userID, id, cents); err != nil {
			t.Fatalf("seed balance: %v", err)
		}
	}
	setBalance(checking, 250_000)
	setBalance(savings, personaBofASavings)
	setBalance(hysa, personaAllyHYSA)
	setBalance(chase, -personaChaseBalance)

	// Three complete months of income and essentials so the baseline is real.
	rent := mkCategory(t, pool, userID, "Housing")
	if _, err := pool.Exec(ctx,
		`INSERT INTO budgets (user_id, name, category_id, amount, period_type, start_date, end_date, bucket)
		 VALUES ($1, 'Housing', $2, $3, 'monthly',
		         date_trunc('month', CURRENT_DATE - INTERVAL '6 months')::date,
		         (date_trunc('month', CURRENT_DATE - INTERVAL '6 months') + INTERVAL '1 month - 1 day')::date,
		         'needs')`,
		userID, rent, personaEssentials); err != nil {
		t.Fatalf("seed budget: %v", err)
	}
	for i := 1; i <= 3; i++ {
		date := time.Now().UTC().AddDate(0, -i, 0).Format("2006-01-02")
		mkTxnCat(t, pool, userID, checking, rent, -personaEssentials, date, "Rent and essentials")
		mkTxn(t, pool, userID, checking, personaMonthlyIncome, date, "Paycheck")
	}

	// Terms the aggregator never returns: the card's rate, and the fact that
	// the BofA savings is dead money next to the Ally account.
	mustUpsertTerms := func(terms domain.AccountTerms) {
		if err := svc.UpsertAccountTerms(ctx, userID, terms); err != nil {
			t.Fatalf("upsert terms: %v", err)
		}
	}
	mustUpsertTerms(domain.AccountTerms{AccountID: chase, APR: ptr(0.2249)})
	mustUpsertTerms(domain.AccountTerms{AccountID: savings, APY: ptr(0.0001)})
	mustUpsertTerms(domain.AccountTerms{AccountID: hysa, APY: ptr(0.042)})

	// The profile. Match terms are supplied here because the document never
	// asked for them -- which is precisely the omission that left the 401(k) at
	// 2% for a year.
	profile := &domain.WealthProfile{
		FilingStatus:              ptr("single"),
		ResidentState:             ptr("CA"),
		MaritalStatus:             ptr("single"),
		GrossAnnualIncome:         ptr(money.Money(personaGrossAnnual)),
		DeferralPct:               ptr(0.02),
		MatchPct:                  ptr(0.5),
		MatchLimitPct:             ptr(0.06),
		EmployerIsPublic:          ptr(true),
		EmployerTicker:            ptr("EXMP"),
		EmergencyFundTargetMonths: ptr(6),
		DateOfBirth:               ptr(time.Date(1990, 6, 15, 0, 0, 0, 0, time.UTC)),
	}
	keys := []string{
		"filing_status", "resident_state", "marital_status", "gross_annual_income",
		"deferral_pct", "match_pct", "match_limit_pct", "employer_is_public",
		"employer_ticker", "emergency_fund_target_months", "date_of_birth",
	}
	if _, err := svc.SaveWealthProfile(ctx, userID, profile, keys, domain.FieldSourceEntered); err != nil {
		t.Fatalf("save profile: %v", err)
	}

	// The $80,000 grant vesting $5,000 quarterly, first tranche ~41 days out,
	// and the ESPP purchasing on 1 March.
	vest := time.Now().UTC().AddDate(0, 0, 41)
	if _, err := svc.UpsertEquityGrant(ctx, userID, domain.EquityGrant{
		Kind: domain.EquityKindRSU, Label: ptr("New hire grant"),
		GrantDate:    ptr(time.Now().UTC().AddDate(0, -2, 0)),
		NextVestDate: &vest, VestFrequency: ptr("quarterly"), VestShare: ptr(0.0625),
		HasRule10b51: ptr(false), BlackoutPolicy: ptr("quarterly"),
	}); err != nil {
		t.Fatalf("seed RSU grant: %v", err)
	}
	purchase := time.Date(time.Now().UTC().Year()+1, time.March, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.UpsertEquityGrant(ctx, userID, domain.EquityGrant{
		Kind: domain.EquityKindESPP, Label: ptr("ESPP"),
		ESPPDiscountPct: ptr(0.15), ESPPHasLookback: ptr(true),
		ESPPContributionPct: ptr(0.05), ESPPPlanMaxPct: ptr(0.15),
		ESPPPurchaseDate: &purchase,
	}); err != nil {
		t.Fatalf("seed ESPP grant: %v", err)
	}
}

func TestPDFPersonaProducesCorrectedPlanIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	seedPDFPersona(t, pool, svc, userID)

	quests, _, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests: %v", err)
	}
	if len(quests) == 0 {
		t.Fatal("expected an action list")
	}

	find := func(key string) *domain.Quest { return findQuest(quests, key) }
	index := func(key string) int { return indexOfQuest(quests, key) }

	// --- Correction 1: the match leads, rather than waiting a year. ---------
	t.Run("employer match comes first, not in year three", func(t *testing.T) {
		match := find("capture_employer_match")
		if match == nil {
			t.Fatal("expected an employer match action")
		}
		if match.Phase != PhaseLiquidity || match.Priority != 0 {
			t.Errorf("match should be the first action of phase 1; got phase %d priority %d",
				match.Phase, match.Priority)
		}
		// 6% x 50% x $165,000 = $4,950 available; 2% x 50% x $165,000 = $1,650 earned.
		if match.TargetAmount == nil || *match.TargetAmount != 330_000 {
			t.Errorf("forgone match: got %v want 330000 cents ($3,300)", match.TargetAmount)
		}
		if debt := index("clear_high_apr_balance"); debt >= 0 && index("capture_employer_match") > debt {
			t.Error("the document paid down the card first; the match outranks it")
		}
	})

	// --- Correction 6: buffer before debt. ---------------------------------
	t.Run("starter buffer precedes the card payoff", func(t *testing.T) {
		starter, debt := index("starter_emergency_fund"), index("clear_high_apr_balance")
		if starter < 0 || debt < 0 {
			t.Skip("fixture produced no cushion or debt action")
		}
		if starter > debt {
			t.Error("a cushion has to exist before the card is cleared, or the balance returns")
		}
	})

	// --- Correction 2: the Roth contribution the document automated. -------
	t.Run("does not automate a direct Roth contribution", func(t *testing.T) {
		roth := find("resolve_roth_route")
		if roth == nil {
			t.Fatal("expected a Roth route action")
		}
		lower := strings.ToLower(roth.Title + " " + roth.Detail)
		// At this income the answer is either the phase-out or the backdoor.
		// What it must never be is the document's unqualified direct transfer.
		if strings.Contains(lower, "straight to a roth ira without any workaround") {
			t.Errorf("recommended an unqualified direct contribution at $165k income: %q", roth.Detail)
		}
		if !strings.Contains(lower, "phase-out") && !strings.Contains(lower, "backdoor") {
			t.Errorf("should engage with the income limit the document ignored; got %q", roth.Detail)
		}
	})

	// --- Correction 3 and 7: dated events outrank the phase order. ----------
	t.Run("the 41-day vest surfaces despite phase 1 being unfinished", func(t *testing.T) {
		vest := find("liquidate_vested_rsu")
		if vest == nil {
			t.Fatal("expected a dated action for the upcoming vest")
		}
		if vest.Status == domain.QuestStatusLocked {
			t.Error("the document filed this under months 6-12; it lands in 41 days")
		}
		if vest.DueDate == nil {
			t.Error("the vest action must carry its real date")
		}
		espp := find("sell_espp_at_purchase")
		if espp == nil || espp.DueDate == nil {
			t.Fatal("expected a dated ESPP purchase action")
		}
		if espp.DueDate.Month() != time.March || espp.DueDate.Day() != 1 {
			t.Errorf("ESPP purchase date: got %v want 1 March", espp.DueDate)
		}
	})

	// --- Correction 4: the ESPP rate the document never mentioned. ---------
	t.Run("raises the ESPP contribution rate", func(t *testing.T) {
		espp := find("maximize_espp_rate")
		if espp == nil {
			t.Fatal("the document collected the discount and lookback and never used them")
		}
		if !strings.Contains(espp.Detail, "lookback") {
			t.Errorf("should credit the lookback; got %q", espp.Detail)
		}
	})

	// --- Correction 5: blackout windows. -----------------------------------
	t.Run("offers a fallback for closed trading windows", func(t *testing.T) {
		sell := find("establish_sell_on_vest")
		if sell == nil {
			t.Fatal("expected a sell-on-vest action")
		}
		if !strings.Contains(strings.ToLower(sell.Detail), "blackout") {
			t.Errorf("the document said sell within 48 hours with no fallback; got %q", sell.Detail)
		}
	})

	// --- The HSA branch the document assumed away. -------------------------
	t.Run("does not assume HDHP enrolment", func(t *testing.T) {
		hsa := find("max_hsa")
		if hsa == nil {
			t.Fatal("expected an HSA action, blocked rather than absent")
		}
		if hsa.Status != domain.QuestStatusBlocked {
			t.Errorf("the document told this user to turn the HSA back on without checking eligibility; got %q", hsa.Status)
		}
	})

	// --- The gap the document had nothing on at all. -----------------------
	t.Run("protects the income that funds the whole plan", func(t *testing.T) {
		if find("secure_disability_coverage") == nil {
			t.Error("no disability action: the document optimised the portfolio and ignored the paycheck")
		}
	})

	// --- The number the document implied but never stated. -----------------
	t.Run("states the independence number explicitly", func(t *testing.T) {
		fi := find("reach_fi_number")
		if fi == nil {
			t.Fatal("expected a financial independence action")
		}
		// 25x essential annual spend: $4,435.38 x 12 / 0.04.
		want := money.Money(personaEssentials * 12 * 25)
		if fi.TargetAmount == nil {
			t.Fatal("the independence number must be a number")
		}
		if diff := *fi.TargetAmount - want; diff > 100 || diff < -100 {
			t.Errorf("independence number: got %s want about %s", usd(*fi.TargetAmount), usd(want))
		}
	})

	// --- Dead cash, netted against anything earmarked. ---------------------
	t.Run("sweeps the zero-yield savings into the high-yield account", func(t *testing.T) {
		if find("sweep_idle_cash") == nil {
			t.Error("expected an action moving the 0% BofA savings into the Ally account")
		}
	})
}

// A user who has answered nothing still gets a usable list: blocked actions
// naming what they need, never invented figures.
func TestPDFPersonaWithoutAnswersBlocksRatherThanGuessesIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	quests, _, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests: %v", err)
	}
	if len(quests) == 0 {
		t.Fatal("a new user should still get a list of what to answer")
	}

	var blocked int
	for _, q := range quests {
		if q.Status == domain.QuestStatusBlocked {
			blocked++
			if q.TargetAmount != nil {
				t.Errorf("%s is blocked yet asserts %s", q.CatalogKey, usd(*q.TargetAmount))
			}
		}
		if q.Title == "" {
			t.Errorf("%s has no title", q.CatalogKey)
		}
	}
	if blocked == 0 {
		t.Error("expected blocked actions naming the missing answers")
	}

	// Specifically: the two decisions that cost money if guessed at.
	for _, key := range []string{"resolve_roth_route", "max_hsa"} {
		q := findQuest(quests, key)
		if q == nil {
			t.Errorf("%s should be present and blocked", key)
			continue
		}
		if q.Status != domain.QuestStatusBlocked {
			t.Errorf("%s must not fire on an empty profile; got %q", key, q.Status)
		}
	}
}

// The summary served alongside the action list.
//
// It exists so the client stops recomputing a baseline of its own. When the
// browser held a waterfall and the server held a catalog, "when is the cushion
// finished" had two answers that could differ -- and a user shown two dates for
// the same milestone has no reason to believe either.
func TestPlanSummaryMatchesTheActionListIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	seedPDFPersona(t, pool, svc, userID)

	quests, summary, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests: %v", err)
	}

	if summary.EssentialMonthly != personaEssentials {
		t.Errorf("essentials: got %d want %d", summary.EssentialMonthly, personaEssentials)
	}
	if summary.MonthsOfData == 0 {
		t.Error("expected the seeded months to be counted")
	}

	// Crossover is when the surplus has paid for phase 1, worked out here from
	// the persona's own figures rather than by re-adding the action list: the
	// Chase balance and the interest it charges while it is paid down, plus
	// whatever a six-month fund still lacks. The forgone match is claimed
	// through payroll and the idle savings are moved rather than saved, so
	// neither is owed -- the old sum counted both, and put the crossover twice
	// as far out.
	surplus := money.Money(personaMonthlyIncome - personaEssentials)
	if summary.MonthlySurplus != surplus {
		t.Fatalf("surplus: got %d want %d", summary.MonthlySurplus, surplus)
	}
	// The savings already cover the starter cushion, so the card is first in
	// line: paid from the first month, charging 22.49% only while it is.
	owed := float64(personaChaseBalance)
	for card := owed; card > 0; {
		interest := card * 0.2249 / 12
		owed += interest
		card += interest - float64(surplus)
	}
	if gap := money.Money(6*personaEssentials) - summary.LiquidAssets; gap > 0 {
		owed += float64(gap)
	}
	want := int(math.Ceil(owed / float64(surplus)))
	if summary.CrossoverMonths == nil || *summary.CrossoverMonths != want {
		t.Errorf("crossover: got %v want %d", summary.CrossoverMonths, want)
	}
	if summary.CrossoverOn == nil || !summary.CrossoverOn.Equal(monthStart(time.Now().UTC(), want)) {
		t.Errorf("crossover date: got %v want the first of the month %d months out", summary.CrossoverOn, want)
	}

	// The schedule the Cash page draws holds only what the surplus pays for,
	// and its last stage ends where the crossover says.
	byID := map[string]domain.Quest{}
	for _, q := range quests {
		byID[q.ID] = q
	}
	var scheduled []string
	for _, f := range summary.Funding {
		key := byID[f.QuestID].CatalogKey
		scheduled = append(scheduled, key)
		if base, _, _ := strings.Cut(key, ":"); !fundedFromSurplus[base] {
			t.Errorf("%s is not paid for out of the surplus, but was scheduled", key)
		}
	}
	if len(summary.Funding) == 0 {
		t.Fatal("expected a funding schedule for the card and the emergency fund")
	}
	last := summary.Funding[len(summary.Funding)-1]
	if last.StartsInMonths == nil || last.MonthsToComplete == nil || summary.CrossoverMonths == nil ||
		*last.StartsInMonths+*last.MonthsToComplete != *summary.CrossoverMonths {
		t.Errorf("the last stage should end at the crossover; schedule %v, stages %+v", scheduled, summary.Funding)
	}

	// The breakdown the Cash page's movable-money card needs.
	if summary.MonthlyWants != 0 || summary.MonthlyOutflow != personaEssentials {
		t.Errorf("breakdown: wants %d, outflow %d", summary.MonthlyWants, summary.MonthlyOutflow)
	}

	// Strategy drives the target rate; the persona never set one, so the
	// balanced default applies rather than a zero that would read as "save
	// nothing".
	if summary.TargetSavingsRate <= 0 {
		t.Errorf("target savings rate should fall back to a real default, got %v", summary.TargetSavingsRate)
	}
}

// Auto-verification, end to end: fund the cushion and the action closes itself.
//
// This is the whole point of build phase 5. Before it, an action the engine
// concluded looked identical in the history to one the user ticked, so there
// was no way to tell a verified fact from a claim.
func TestFundingAnAccountClosesTheActionIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	checking := mkAccount(t, pool, userID, "Checking")
	savings := mkAccount(t, pool, userID, "Savings")
	setRole(t, pool, checking, domain.RoleChecking)
	setRole(t, pool, savings, domain.RoleSavings)

	// Essentials of $2,000/month, established over three complete months.
	const essentials = 200_000
	rent := mkCategory(t, pool, userID, "Rent")
	if _, err := pool.Exec(ctx,
		`INSERT INTO budgets (user_id, name, category_id, amount, period_type, start_date, end_date, bucket)
		 VALUES ($1, 'Rent', $2, $3, 'monthly',
		         date_trunc('month', CURRENT_DATE - INTERVAL '6 months')::date,
		         (date_trunc('month', CURRENT_DATE - INTERVAL '6 months') + INTERVAL '1 month - 1 day')::date,
		         'needs')`,
		userID, rent, essentials); err != nil {
		t.Fatalf("seed budget: %v", err)
	}
	for i := 1; i <= 3; i++ {
		date := time.Now().UTC().AddDate(0, -i, 0).Format("2006-01-02")
		mkTxnCat(t, pool, userID, checking, rent, -essentials, date, "Rent")
		mkTxn(t, pool, userID, checking, 500_000, date, "Paycheck")
	}

	setBalance := func(account string, cents int64) {
		if _, err := pool.Exec(ctx,
			`INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
			 VALUES ($1, $2, '-infinity'::date, $3, 'opening')
			 ON CONFLICT (account_id, as_of_date) DO UPDATE SET balance = EXCLUDED.balance`,
			userID, account, cents); err != nil {
			t.Fatalf("set balance: %v", err)
		}
	}
	// Checking accrues its own transactions -- three months of $5,000 in against
	// $2,000 out is $9,000 of liquid assets on its own -- so the opening balance
	// offsets that. Without it the cushion is already funded and the test would
	// be asserting against a condition that was never false.
	setBalance(checking, -850_000)
	// Leaves $500 in savings: under a month of essentials to begin with.
	setBalance(savings, 50_000)

	if _, err := svc.SaveWealthProfile(ctx, userID,
		&domain.WealthProfile{EmergencyFundTargetMonths: ptr(6)},
		[]string{"emergency_fund_target_months"}, domain.FieldSourceEntered); err != nil {
		t.Fatalf("save profile: %v", err)
	}

	before, _, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests: %v", err)
	}
	starter := findQuest(before, "starter_emergency_fund")
	if starter == nil {
		t.Fatal("expected a starter cushion action")
	}
	if starter.Status != domain.QuestStatusAvailable {
		t.Fatalf("with $500 against $2,000 of essentials this should be outstanding; got %q", starter.Status)
	}

	// Fund it. Nothing else changes, and nobody clicks anything.
	setBalance(savings, 300_000)

	after, _, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests (after funding): %v", err)
	}
	starterAfter := findQuest(after, "starter_emergency_fund")
	if starterAfter == nil {
		t.Fatal("the action disappeared instead of completing")
	}
	if starterAfter.Status != domain.QuestStatusComplete {
		t.Errorf("funding the account should close the action without a click; got %q",
			starterAfter.Status)
	}
	// Recorded as the engine's conclusion, which is what keeps it revisable if
	// the balance later falls.
	if starterAfter.CompletedSource != domain.QuestSourceAuto {
		t.Errorf("completed_source: got %q want auto", starterAfter.CompletedSource)
	}

	// And it left a trace saying so.
	events, err := svc.ListQuestEvents(ctx, userID, starterAfter.ID)
	if err != nil {
		t.Fatalf("ListQuestEvents: %v", err)
	}
	var completed *domain.QuestEvent
	for i := range events {
		if events[i].Event == domain.QuestEventCompleted {
			completed = &events[i]
		}
	}
	if completed == nil {
		t.Fatalf("expected a completion event; got %+v", events)
	}
	if completed.Source != domain.QuestSourceAuto {
		t.Errorf("event source: got %q want auto", completed.Source)
	}
	if completed.Note == "" {
		t.Error("an auto-completion should say what verified it")
	}

	// Spend it back down: a computed completion tracks its data rather than
	// outliving it.
	setBalance(savings, 10_000)
	reopened, _, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests (after spending): %v", err)
	}
	if q := findQuest(reopened, "starter_emergency_fund"); q == nil ||
		q.Status == domain.QuestStatusComplete {
		t.Error("draining the account should reopen an auto-verified action")
	}
}

// Paying a card off completes its action, end to end through regeneration.
//
// It used to vanish: the rule produced nothing for a zero balance, so the
// prune in ReplaceQuests deleted the row and cascaded away its history.
func TestPayingOffACardCompletesItsActionIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)

	var card string
	if err := pool.QueryRow(ctx,
		`INSERT INTO accounts (user_id, name, type, role, currency) VALUES ($1, 'Chase Sapphire', 'liability', 'credit_card', 'USD') RETURNING id`,
		userID).Scan(&card); err != nil {
		t.Fatalf("create card: %v", err)
	}
	setBalance := func(cents int64) {
		if _, err := pool.Exec(ctx,
			`INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
			 VALUES ($1, $2, '-infinity'::date, $3, 'opening')
			 ON CONFLICT (account_id, as_of_date) DO UPDATE SET balance = EXCLUDED.balance`,
			userID, card, cents); err != nil {
			t.Fatalf("set balance: %v", err)
		}
	}
	setBalance(-382_435)
	if err := svc.UpsertAccountTerms(ctx, userID, domain.AccountTerms{AccountID: card, APR: ptr(0.2249)}); err != nil {
		t.Fatalf("upsert terms: %v", err)
	}

	key := "clear_high_apr_balance:" + card
	before, _, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests: %v", err)
	}
	open := findQuest(before, key)
	if open == nil || open.Status != domain.QuestStatusAvailable {
		t.Fatalf("expected an open action for the card; got %+v", open)
	}

	setBalance(0)

	after, _, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests (after paying off): %v", err)
	}
	done := findQuest(after, key)
	if done == nil {
		t.Fatal("paying off the card deleted its action instead of completing it")
	}
	if done.ID != open.ID {
		t.Errorf("the action should complete in place; id %s became %s", open.ID, done.ID)
	}
	if done.Status != domain.QuestStatusComplete || done.CompletedSource != domain.QuestSourceAuto {
		t.Errorf("status %q from %q, want complete from auto", done.Status, done.CompletedSource)
	}

	events, err := svc.ListQuestEvents(ctx, userID, done.ID)
	if err != nil {
		t.Fatalf("ListQuestEvents: %v", err)
	}
	var generated, completed bool
	for _, e := range events {
		generated = generated || e.Event == domain.QuestEventGenerated
		completed = completed || e.Event == domain.QuestEventCompleted
	}
	if !generated || !completed {
		t.Errorf("history should run from generated to completed; got %+v", events)
	}
}

// Reading the action list is a read. The first reading records what it finds;
// a second one, with nothing changed, writes nothing at all.
func TestReadingTheActionListWritesNothingIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	svc := newRulesTestService(pool)
	userID := sfCreateTestUser(t, pool)
	seedPDFPersona(t, pool, svc, userID)

	footprint := func() (states, events int, lastChange time.Time) {
		t.Helper()
		if err := pool.QueryRow(ctx, `
			SELECT (SELECT count(*) FROM wealth_quest_state WHERE user_id = $1),
			       (SELECT count(*) FROM wealth_quest_events WHERE user_id = $1),
			       (SELECT COALESCE(max(changed_at), 'epoch') FROM wealth_quest_state WHERE user_id = $1)`,
			userID).Scan(&states, &events, &lastChange); err != nil {
			t.Fatalf("footprint: %v", err)
		}
		return
	}

	first, _, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests: %v", err)
	}
	states, events, changed := footprint()
	if states != len(first) {
		t.Errorf("the first reading should record every action once: %d states for %d actions", states, len(first))
	}

	if _, _, err := svc.GenerateQuests(ctx, userID); err != nil {
		t.Fatalf("GenerateQuests (again): %v", err)
	}
	s2, e2, c2 := footprint()
	if s2 != states || e2 != events || !c2.Equal(changed) {
		t.Errorf("an unchanged reading wrote: states %d->%d, events %d->%d, last change %v->%v",
			states, s2, events, e2, changed, c2)
	}

	// Marking something done is the user's event; the reading after it must
	// not add an engine event restating it.
	if err := svc.SetQuestStatus(ctx, userID, "establish_sell_on_vest", domain.QuestStatusComplete, ""); err != nil {
		t.Fatalf("SetQuestStatus: %v", err)
	}
	after, _, err := svc.GenerateQuests(ctx, userID)
	if err != nil {
		t.Fatalf("GenerateQuests (after mark): %v", err)
	}
	if q := findQuest(after, "establish_sell_on_vest"); q == nil || q.Status != domain.QuestStatusComplete {
		t.Fatalf("the marked action should read as complete; got %+v", q)
	}
	evs, _ := svc.ListQuestEvents(ctx, userID, "establish_sell_on_vest")
	var completions int
	for _, e := range evs {
		if e.Event == domain.QuestEventCompleted {
			completions++
			if e.Source != domain.QuestSourceManual {
				t.Errorf("the completion should be the user's, got source %q", e.Source)
			}
		}
	}
	if completions != 1 {
		t.Errorf("completion events: got %d want exactly the user's one", completions)
	}

	// Keys the catalog cannot produce are refused; fan-out keys are accepted by base.
	if err := svc.SetQuestStatus(ctx, userID, "no_such_action", domain.QuestStatusComplete, ""); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("unknown key: got %v, want ErrNotFound", err)
	}
	if err := svc.SetQuestStatus(ctx, userID, "clear_high_apr_balance:any-card", domain.QuestStatusSkipped, ""); err != nil {
		t.Errorf("a fan-out key should be accepted: %v", err)
	}
}
