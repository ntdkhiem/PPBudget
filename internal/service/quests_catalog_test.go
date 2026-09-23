package service

import (
	"strings"
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Catalog tests. Pure -- no database; evaluateCatalog is driven from a
// hand-built context.
//
// There is a named case here for each of the thirteen correctness rules the
// plan enumerates, plus the two traps that motivated the feature. Several
// assert that the engine DECLINES to say something, which is as much the point
// as what it does say: an action fired on a value nobody supplied is worse than
// no action at all, because the user has no way to know it was a guess.

// ------------------------------------------------------------- fixtures

type profileBuilder struct{ p *domain.WealthProfile }

func newProfile() *profileBuilder {
	return &profileBuilder{p: &domain.WealthProfile{
		Fields: map[string]domain.ProfileField{},
	}}
}

// set applies a field and marks it answered in one step, because the engine
// treats "answered" as the presence of a provenance row rather than a non-zero
// value -- a deliberate zero (no employer match) is an answer.
func (b *profileBuilder) set(key string, apply func(*domain.WealthProfile)) *profileBuilder {
	apply(b.p)
	b.p.Fields[key] = domain.ProfileField{
		FieldKey: key, Source: domain.FieldSourceEntered, AnsweredAt: time.Now().UTC(),
	}
	return b
}

// setDerived marks a field as computed but never confirmed by the user.
func (b *profileBuilder) setDerived(key string, apply func(*domain.WealthProfile)) *profileBuilder {
	apply(b.p)
	b.p.Fields[key] = domain.ProfileField{
		FieldKey: key, Source: domain.FieldSourceDerived, AnsweredAt: time.Now().UTC(),
	}
	return b
}

func (b *profileBuilder) build() *domain.WealthProfile { return b.p }

// baseContext is a solvent single filer with a modest surplus and no
// complications. Tests bend one thing at a time from here.
func baseContext(p *domain.WealthProfile) questContext {
	if p == nil {
		p = newProfile().build()
	}
	return questContext{
		Now:     time.Date(2026, time.September, 22, 0, 0, 0, 0, time.UTC),
		TaxYear: 2026,
		Profile: p,
		Limits:  testLimits(),
		Baseline: Baseline{
			MonthlyIncome:    680_000,
			MonthlyOutflow:   500_000,
			EssentialMonthly: 440_000,
			MonthlyWants:     60_000,
			MonthlySurplus:   180_000,
			LiquidAssets:     800_000,
			NetWorth:         5_000_000,
			BucketCoverage:   0.95,
			MonthsOfData:     6,
		},
		Terms:          map[string]domain.AccountTerms{},
		StaleFields:    map[string]bool{},
		ExistingStatus: map[string]string{},
	}
}

// findQuest returns the first action whose key matches exactly or by fan-out
// prefix.
func findQuest(quests []domain.Quest, key string) *domain.Quest {
	for i := range quests {
		if quests[i].CatalogKey == key || strings.HasPrefix(quests[i].CatalogKey, key+":") {
			return &quests[i]
		}
	}
	return nil
}

func indexOfQuest(quests []domain.Quest, key string) int {
	for i := range quests {
		if quests[i].CatalogKey == key || strings.HasPrefix(quests[i].CatalogKey, key+":") {
			return i
		}
	}
	return -1
}

func mustFind(t *testing.T, quests []domain.Quest, key string) *domain.Quest {
	t.Helper()
	q := findQuest(quests, key)
	if q == nil {
		var keys []string
		for _, x := range quests {
			keys = append(keys, x.CatalogKey)
		}
		t.Fatalf("expected an action %q; got %v", key, keys)
	}
	return q
}

// unlockAllPhases marks every milestone complete so a test can exercise one
// rule without also satisfying four phases of prerequisites.
//
// Worth keeping separate: whether a rule FIRES and whether its phase is OPEN
// are different questions, and conflating them makes a gating regression look
// like a rule regression. The gating tests below deliberately do not use this.
func unlockAllPhases(qc questContext) questContext {
	status := map[string]string{}
	for _, phase := range phaseDefs {
		for _, m := range phase.Milestones {
			status[m] = domain.QuestStatusComplete
		}
	}
	qc.ExistingStatus = status
	return qc
}

// ------------------------------------------- rule 1 & 6: phase 1 ordering

// Rules 1 and 6. The source document left the 401(k) at 2% for a year while
// paying down a card, and this plan's own first draft then put high-APR debt
// ahead of the starter buffer. Both orderings are wrong and both cost money.
func TestPhase1OrderingMatchThenBufferThenDebt(t *testing.T) {
	p := newProfile().
		set("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(14_500_000))
		}).
		set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.5) }).
		set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.06) }).
		set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.02) }).
		set("emergency_fund_target_months", func(p *domain.WealthProfile) {
			p.EmergencyFundTargetMonths = ptr(6)
		}).
		build()

	qc := baseContext(p)
	qc.Baseline.LiquidAssets = 50_000 // no cushion yet
	qc.Accounts = []domain.PlanningAccount{
		{ID: "card", Name: "Chase", Type: "liability", Balance: -500_000},
	}
	qc.Terms["card"] = domain.AccountTerms{AccountID: "card", APR: ptr(0.2249)}

	quests := evaluateCatalog(qc)

	match := indexOfQuest(quests, "capture_employer_match")
	starter := indexOfQuest(quests, "starter_emergency_fund")
	debt := indexOfQuest(quests, "clear_high_apr_balance")

	if match < 0 || starter < 0 || debt < 0 {
		t.Fatalf("expected all three actions; match=%d starter=%d debt=%d", match, starter, debt)
	}
	if match > starter || match > debt {
		t.Errorf("employer match must lead: a 50%% match is an instant 50%% return and outranks even a 22%% APR balance")
	}
	if starter > debt {
		t.Errorf("the starter buffer must come before high-APR debt: without it the paid-down card gets re-borrowed on the next surprise")
	}
	if quests[match].Priority != 0 {
		t.Errorf("capture_employer_match priority: got %d want 0", quests[match].Priority)
	}
}

// The match action must quantify what is being left behind, not just gesture.
func TestEmployerMatchQuantifiesTheShortfall(t *testing.T) {
	p := newProfile().
		set("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(20_000_000)) // $200,000
		}).
		set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.5) }).
		set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.06) }).
		set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.02) }).
		build()

	quests := evaluateCatalog(baseContext(p))
	q := mustFind(t, quests, "capture_employer_match")

	// Full match: 6% x 50% x 200,000 = 6,000. Earned at 2%: 2% x 50% x 200,000 = 2,000.
	if q.TargetAmount == nil || *q.TargetAmount != 400_000 {
		t.Errorf("forgone match: got %v want 400000 cents ($4,000)", q.TargetAmount)
	}
	if q.Status != domain.QuestStatusAvailable {
		t.Errorf("status: got %q want available", q.Status)
	}
}

// A user already at the match limit has nothing to do here.
func TestEmployerMatchCompleteWhenAtLimit(t *testing.T) {
	p := newProfile().
		set("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(20_000_000))
		}).
		set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.5) }).
		set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.06) }).
		set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.10) }).
		build()

	q := mustFind(t, evaluateCatalog(baseContext(p)), "capture_employer_match")
	if q.Status != domain.QuestStatusComplete {
		t.Errorf("status: got %q want complete", q.Status)
	}
}

// A plan with no match is an ANSWER, not a gap: the action disappears rather
// than blocking.
func TestEmployerMatchNotApplicableWithoutAMatch(t *testing.T) {
	p := newProfile().
		set("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(14_500_000))
		}).
		set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.0) }).
		set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.0) }).
		set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.02) }).
		build()

	if q := findQuest(evaluateCatalog(baseContext(p)), "capture_employer_match"); q != nil {
		t.Errorf("no match means no action; got %q", q.Title)
	}
}

// Unanswered inputs block rather than defaulting to zero, and name what would
// unlock the action so the UI can price the question.
func TestEmployerMatchBlocksWithoutInputs(t *testing.T) {
	quests := evaluateCatalog(baseContext(nil))
	q := mustFind(t, quests, "capture_employer_match")

	if q.Status != domain.QuestStatusBlocked {
		t.Fatalf("status: got %q want blocked", q.Status)
	}
	if len(q.MissingFields) == 0 {
		t.Error("a blocked action must name the fields that would unlock it")
	}
	for _, want := range []string{"match_pct", "gross_annual_income"} {
		found := false
		for _, got := range q.MissingFields {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing fields should include %q, got %v", want, q.MissingFields)
		}
	}
}

// The front-loading trap: a per-paycheck match with no true-up stops matching
// once the annual cap is hit, forfeiting the rest of the year outright.
func TestEmployerMatchWarnsAboutFrontLoadingWithoutTrueUp(t *testing.T) {
	build := func(perPaycheck, trueUp bool) *domain.Quest {
		p := newProfile().
			set("gross_annual_income", func(p *domain.WealthProfile) {
				p.GrossAnnualIncome = ptr(money.Money(20_000_000))
			}).
			set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.5) }).
			set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.06) }).
			set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.02) }).
			set("match_per_paycheck", func(p *domain.WealthProfile) { p.MatchPerPaycheck = ptr(perPaycheck) }).
			set("match_has_true_up", func(p *domain.WealthProfile) { p.MatchHasTrueUp = ptr(trueUp) }).
			build()
		return findQuest(evaluateCatalog(baseContext(p)), "capture_employer_match")
	}

	risky := build(true, false)
	if risky == nil || !strings.Contains(risky.Detail, "front-load") {
		t.Errorf("per-paycheck match with no true-up must warn against front-loading; got %q", risky.Detail)
	}
	safe := build(true, true)
	if safe != nil && strings.Contains(safe.Detail, "front-load") {
		t.Error("a plan that trues up at year end needs no front-loading warning")
	}

	// The warning keys on the true-up answer alone. Requiring match_per_paycheck
	// as well meant the question the intake actually asks could never trigger
	// the warning it exists for -- caught in the browser, where selecting "no
	// true-up" produced nothing.
	t.Run("fires on the true-up answer alone", func(t *testing.T) {
		p := newProfile().
			set("gross_annual_income", func(p *domain.WealthProfile) {
				p.GrossAnnualIncome = ptr(money.Money(20_000_000))
			}).
			set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.5) }).
			set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.06) }).
			set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.02) }).
			set("match_has_true_up", func(p *domain.WealthProfile) { p.MatchHasTrueUp = ptr(false) }).
			build()

		q := mustFind(t, evaluateCatalog(baseContext(p)), "capture_employer_match")
		if !strings.Contains(q.Detail, "front-load") {
			t.Errorf("no true-up should warn even without match_per_paycheck; got %q", q.Detail)
		}
	})

	// An unanswered true-up question stays silent rather than assuming the
	// worse case.
	t.Run("stays silent when unanswered", func(t *testing.T) {
		p := newProfile().
			set("gross_annual_income", func(p *domain.WealthProfile) {
				p.GrossAnnualIncome = ptr(money.Money(20_000_000))
			}).
			set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.5) }).
			set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.06) }).
			set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.02) }).
			build()

		q := mustFind(t, evaluateCatalog(baseContext(p)), "capture_employer_match")
		if strings.Contains(q.Detail, "front-load") {
			t.Errorf("should not assert a plan detail nobody supplied; got %q", q.Detail)
		}
	})
}

// ------------------------------------------------- rule 2: the Roth branch

// Rule 2, and the trap the whole feature exists to fix. The source document
// automated a direct Roth contribution in three chapters while the user's
// income made them ineligible -- a 6% excise tax per year until corrected.
func TestRothRouteNeverRecommendsDirectContributionWhenIneligible(t *testing.T) {
	tests := []struct {
		name         string
		filing       string
		gross        money.Money
		spouse       *money.Money
		wantDirect   bool
		wantBackdoor bool
	}{
		{
			name:   "single, comfortably under: direct is correct",
			filing: "single", gross: 12_000_000,
			wantDirect: true,
		},
		{
			name:   "single, above the phase-out: backdoor, never direct",
			filing: "single", gross: 19_000_000,
			wantBackdoor: true,
		},
		{
			// The specific trap: each income alone is under the single
			// threshold, but the household is past the joint one.
			name:   "MFJ, each under alone but household over: backdoor",
			filing: "mfj", gross: 14_500_000, spouse: ptr(money.Money(12_000_000)),
			wantBackdoor: true,
		},
		{
			name:   "MFJ, household genuinely under: direct is correct",
			filing: "mfj", gross: 11_000_000, spouse: ptr(money.Money(8_000_000)),
			wantDirect: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := newProfile().
				set("filing_status", func(p *domain.WealthProfile) { p.FilingStatus = ptr(tc.filing) }).
				set("gross_annual_income", func(p *domain.WealthProfile) { p.GrossAnnualIncome = ptr(tc.gross) }).
				set("traditional_ira_balance", func(p *domain.WealthProfile) {
					p.TraditionalIRABalance = ptr(money.Money(0))
				})
			if tc.spouse != nil {
				b.set("spouse_gross_annual", func(p *domain.WealthProfile) { p.SpouseGrossAnnual = tc.spouse })
			}

			q := mustFind(t, evaluateCatalog(baseContext(b.build())), "resolve_roth_route")
			lower := strings.ToLower(q.Title + " " + q.Detail)

			if tc.wantBackdoor {
				if !strings.Contains(lower, "backdoor") {
					t.Errorf("expected the backdoor route; got %q / %q", q.Title, q.Detail)
				}
				// The dangerous output: telling an ineligible user to contribute
				// straight to a Roth.
				if strings.Contains(lower, "straight to a roth") {
					t.Errorf("recommended a direct contribution to an INELIGIBLE user: %q", q.Detail)
				}
			}
			if tc.wantDirect && !strings.Contains(lower, "directly") && !strings.Contains(lower, "straight to a roth") {
				t.Errorf("expected a direct contribution; got %q / %q", q.Title, q.Detail)
			}
		})
	}
}

// Asking filing status without spouse income is worse than asking neither: the
// household looks eligible on one income alone.
func TestRothRouteBlocksOnMissingSpouseIncome(t *testing.T) {
	p := newProfile().
		set("filing_status", func(p *domain.WealthProfile) { p.FilingStatus = ptr("mfj") }).
		set("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(18_500_000))
		}).
		build()

	q := mustFind(t, evaluateCatalog(baseContext(p)), "resolve_roth_route")
	if q.Status != domain.QuestStatusBlocked {
		t.Fatalf("status: got %q want blocked", q.Status)
	}
	found := false
	for _, f := range q.MissingFields {
		if f == "spouse_gross_annual" {
			found = true
		}
	}
	if !found {
		t.Errorf("should ask for spouse income, got %v", q.MissingFields)
	}
}

// A DERIVED income figure is not good enough to decide this on: the penalty for
// being wrong is real, so the user has to have seen the number.
func TestRothRouteRefusesToActOnDerivedIncome(t *testing.T) {
	p := newProfile().
		set("filing_status", func(p *domain.WealthProfile) { p.FilingStatus = ptr("single") }).
		setDerived("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(19_000_000))
		}).
		build()

	q := mustFind(t, evaluateCatalog(baseContext(p)), "resolve_roth_route")
	if q.Status != domain.QuestStatusBlocked {
		t.Errorf("a high-stakes branch must not fire off a derived value; status %q", q.Status)
	}
}

// The pro-rata rule, and the fix for it.
func TestRothBackdoorNamesProRataAndItsFix(t *testing.T) {
	base := newProfile().
		set("filing_status", func(p *domain.WealthProfile) { p.FilingStatus = ptr("single") }).
		set("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(19_000_000))
		})

	t.Run("blocks until the IRA balance is known", func(t *testing.T) {
		q := mustFind(t, evaluateCatalog(baseContext(base.build())), "resolve_roth_route")
		if q.Status != domain.QuestStatusBlocked {
			t.Errorf("status: got %q want blocked", q.Status)
		}
		if !strings.Contains(strings.ToLower(q.Detail), "pro-rata") {
			t.Errorf("should explain why the balance matters; got %q", q.Detail)
		}
	})

	t.Run("names the rollover fix when the plan accepts one", func(t *testing.T) {
		p := newProfile().
			set("filing_status", func(p *domain.WealthProfile) { p.FilingStatus = ptr("single") }).
			set("gross_annual_income", func(p *domain.WealthProfile) {
				p.GrossAnnualIncome = ptr(money.Money(19_000_000))
			}).
			set("traditional_ira_balance", func(p *domain.WealthProfile) {
				p.TraditionalIRABalance = ptr(money.Money(5_000_000))
			}).
			set("plan_accepts_rollovers", func(p *domain.WealthProfile) {
				p.PlanAcceptsRollovers = ptr(true)
			}).
			build()

		q := mustFind(t, evaluateCatalog(baseContext(p)), "resolve_roth_route")
		if !strings.Contains(q.Detail, "rollover") {
			t.Errorf("should name rolling the IRA into the 401(k) as the fix; got %q", q.Detail)
		}
	})
}

// -------------------------------------- rules 3 & 7: deadlines beat gating

// Rules 3 and 7. A vest lands when the grant says so and payroll closes on 31
// December; neither waits for the user to finish an earlier phase.
func TestDatedActionsEscapePhaseGating(t *testing.T) {
	vest := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	p := newProfile().
		set("employer_is_public", func(p *domain.WealthProfile) { p.EmployerIsPublic = ptr(true) }).
		set("filing_status", func(p *domain.WealthProfile) { p.FilingStatus = ptr("single") }).
		build()

	qc := baseContext(p)
	// Phase 1 is deliberately left unfinished: no cushion and an unpaid card.
	qc.Baseline.LiquidAssets = 0
	qc.Accounts = []domain.PlanningAccount{
		{ID: "card", Name: "Chase", Type: "liability", Balance: -500_000},
	}
	qc.Terms["card"] = domain.AccountTerms{AccountID: "card", APR: ptr(0.2249)}
	qc.Grants = []domain.EquityGrant{{
		ID: "g1", Kind: domain.EquityKindRSU, Label: ptr("2026 grant"), NextVestDate: &vest,
	}}

	quests := evaluateCatalog(qc)

	vestAction := mustFind(t, quests, "liquidate_vested_rsu")
	if vestAction.Status == domain.QuestStatusLocked {
		t.Error("a dated vest must surface even while its phase is locked: the calendar does not wait for the plan")
	}
	if vestAction.DueDate == nil {
		t.Error("the vest action must carry a due date")
	}

	// An undated phase 2 action in the same run stays locked, which is what
	// makes the exemption meaningful rather than a disabled gate.
	if sell := findQuest(quests, "establish_sell_on_vest"); sell != nil {
		if sell.Status != domain.QuestStatusLocked {
			t.Errorf("an undated phase 2 action should be locked while phase 1 is unfinished; got %q", sell.Status)
		}
	}
}

// The same exemption has to cover annual deadlines, not just equity events.
func TestAnnualDeadlineActionsCarryDueDates(t *testing.T) {
	p := newProfile().
		set("hdhp_enrolled", func(p *domain.WealthProfile) { p.HDHPEnrolled = ptr(true) }).
		set("hsa_coverage_tier", func(p *domain.WealthProfile) { p.HSACoverageTier = ptr("family") }).
		set("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(14_500_000))
		}).
		set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.02) }).
		build()

	quests := evaluateCatalog(baseContext(p))

	for _, key := range []string{"max_hsa", "raise_401k_deferral"} {
		q := mustFind(t, quests, key)
		if q.DueDate == nil {
			t.Errorf("%s runs through payroll and has no do-over in January; it must carry a due date", key)
			continue
		}
		if q.DueDate.Month() != time.December || q.DueDate.Day() != 31 {
			t.Errorf("%s due date: got %v want 31 December", key, q.DueDate)
		}
		if q.Status == domain.QuestStatusLocked {
			t.Errorf("%s is dated and must not be locked", key)
		}
	}

	// The IRA is the exception that proves the rule: its deadline is the filing
	// date, not the calendar year end.
	ira := mustFind(t, quests, "fund_ira_to_limit")
	if ira.DueDate == nil || ira.DueDate.Month() != time.April {
		t.Errorf("IRA deadline should be the April filing date, got %v", ira.DueDate)
	}
}

// ------------------------------------------------- rule 4: the ESPP rate

// Rule 4. The source document collected the discount and lookback in its intake
// and then never used them; raising the contribution rate was absent entirely.
func TestMaximizeESPPRateFiresWhenBelowPlanMaximum(t *testing.T) {
	p := newProfile().
		set("employer_is_public", func(p *domain.WealthProfile) { p.EmployerIsPublic = ptr(true) }).
		set("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(14_500_000))
		}).
		build()

	qc := baseContext(p)
	qc.Grants = []domain.EquityGrant{{
		ID: "espp1", Kind: domain.EquityKindESPP,
		ESPPDiscountPct:     ptr(0.15),
		ESPPHasLookback:     ptr(true),
		ESPPContributionPct: ptr(0.05),
		ESPPPlanMaxPct:      ptr(0.15),
	}}

	q := mustFind(t, evaluateCatalog(unlockAllPhases(qc)), "maximize_espp_rate")
	if q.Status != domain.QuestStatusAvailable {
		t.Fatalf("status: got %q want available", q.Status)
	}
	if !strings.Contains(q.Detail, "lookback") {
		t.Error("a lookback provision raises the real return and should be mentioned")
	}
	if q.TargetAmount == nil || *q.TargetAmount <= 0 {
		t.Error("the action should quantify what raising the rate is worth")
	}
}

func TestMaximizeESPPRateCompleteAtPlanMaximum(t *testing.T) {
	p := newProfile().
		set("employer_is_public", func(p *domain.WealthProfile) { p.EmployerIsPublic = ptr(true) }).
		build()
	qc := baseContext(p)
	qc.Grants = []domain.EquityGrant{{
		ID: "espp1", Kind: domain.EquityKindESPP,
		ESPPDiscountPct: ptr(0.15), ESPPContributionPct: ptr(0.15), ESPPPlanMaxPct: ptr(0.15),
	}}

	q := mustFind(t, evaluateCatalog(qc), "maximize_espp_rate")
	if q.Status != domain.QuestStatusComplete {
		t.Errorf("status: got %q want complete", q.Status)
	}
}

// ---------------------------------------------- rule 5: blackout windows

// Rule 5. "Sell within 48 hours of vest" is not executable during a closed
// trading window, and the source document offered no fallback.
func TestSellOnVestOffersABlackoutFallback(t *testing.T) {
	p := newProfile().
		set("employer_is_public", func(p *domain.WealthProfile) { p.EmployerIsPublic = ptr(true) }).
		build()
	qc := baseContext(p)
	qc.Grants = []domain.EquityGrant{{
		ID: "g1", Kind: domain.EquityKindRSU,
		HasRule10b51: ptr(false), BlackoutPolicy: ptr("quarterly"),
	}}

	q := mustFind(t, evaluateCatalog(qc), "establish_sell_on_vest")
	lower := strings.ToLower(q.Detail)
	if !strings.Contains(lower, "blackout") {
		t.Errorf("should acknowledge the closed window; got %q", q.Detail)
	}
	if !strings.Contains(lower, "10b5-1") {
		t.Errorf("should name the instrument that executes inside a closed window; got %q", q.Detail)
	}
	if !strings.Contains(lower, "first open window") {
		t.Errorf("should give a fallback for a user with no 10b5-1; got %q", q.Detail)
	}
}

// The entire equity phase declines to apply to a private-company employee
// rather than issuing instructions they cannot follow.
func TestEquityPhaseDoesNotApplyToPrivateCompanyEmployees(t *testing.T) {
	p := newProfile().
		set("employer_is_public", func(p *domain.WealthProfile) { p.EmployerIsPublic = ptr(false) }).
		build()
	qc := baseContext(p)
	qc.Grants = []domain.EquityGrant{{ID: "g1", Kind: domain.EquityKindRSU}}

	quests := evaluateCatalog(qc)
	for _, key := range []string{"establish_sell_on_vest", "liquidate_vested_rsu", "maximize_espp_rate"} {
		if q := findQuest(quests, key); q != nil {
			t.Errorf("%s should not apply without a public market; got %q", key, q.Title)
		}
	}
}

// ------------------------------- rules 8, 9, 10, 12: debt and allocation

// Rule 8. The source document said to enable DRIP everywhere. In a taxable
// account that fragments basis and can create a wash sale against the very
// losses its own harvesting action tries to bank.
func TestDRIPIsRestrictedToTaxAdvantagedAccounts(t *testing.T) {
	qc := baseContext(nil)
	qc.Accounts = []domain.PlanningAccount{
		{ID: "k", Name: "401k", Type: "asset", Balance: 10_000_000},
		{ID: "b", Name: "Brokerage", Type: "asset", Balance: 5_000_000},
	}
	qc.Retirement = []domain.RetirementAccountTerms{
		{AccountID: "k", Kind: domain.RetirementKind401k},
		{AccountID: "b", Kind: domain.RetirementKindTaxable},
	}

	q := mustFind(t, evaluateCatalog(qc), "enable_drip")
	if !strings.Contains(q.Title, "tax-sheltered") {
		t.Errorf("the action should scope itself to sheltered accounts; got %q", q.Title)
	}
	lower := strings.ToLower(q.Detail)
	if !strings.Contains(lower, "off in taxable") {
		t.Errorf("should say to leave it off in taxable accounts; got %q", q.Detail)
	}
	if !strings.Contains(lower, "wash sale") {
		t.Errorf("should name the wash-sale interaction; got %q", q.Detail)
	}
}

// Rule 9. Deferred-interest promotions charge all accrued interest
// retroactively if any balance survives expiry, which inverts the usual
// lowest-rate-last ordering.
func TestPromotionalBalanceGetsItsOwnDatedAction(t *testing.T) {
	expiry := time.Date(2027, time.March, 14, 0, 0, 0, 0, time.UTC)
	qc := baseContext(nil)
	qc.Accounts = []domain.PlanningAccount{
		{ID: "promo", Name: "Store Card", Type: "liability", Balance: -300_000},
	}
	qc.Terms["promo"] = domain.AccountTerms{
		AccountID: "promo", APR: ptr(0.2699),
		PromoAPR: ptr(0.0), PromoExpiresOn: &expiry,
	}

	quests := evaluateCatalog(qc)

	promo := mustFind(t, quests, "clear_promo_balance_before_expiry")
	if promo.DueDate == nil || !promo.DueDate.Equal(expiry) {
		t.Errorf("due date: got %v want %v", promo.DueDate, expiry)
	}
	if !strings.Contains(promo.Detail, "accrued since day one") {
		t.Errorf("should explain the deferred-interest trap; got %q", promo.Detail)
	}

	// It must NOT also appear in the ordinary high-APR ordering, or the user
	// gets two contradictory instructions for one balance.
	if debt := findQuest(quests, "clear_high_apr_balance"); debt != nil {
		if strings.Contains(debt.CatalogKey, "promo") {
			t.Error("a live promotional balance must be excluded from the ordinary payoff ordering")
		}
	}
}

// Rule 10. The threshold is relative to expected return, with a behavioural
// floor because debt repayment is certain and market returns are not.
func TestHighAPRThresholdIsRelativeWithAFloor(t *testing.T) {
	tests := []struct {
		name     string
		expected *float64
		want     float64
	}{
		{"no assumption falls back to a conventional figure", nil, 0.07 * afterTaxDrag},
		{"a higher return assumption raises the bar", ptr(0.10), 0.10 * afterTaxDrag},
		{"a very low assumption is floored", ptr(0.01), highAPRFloor},
		{"zero is treated as unset", ptr(0.0), 0.07 * afterTaxDrag},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &domain.WealthProfile{ExpectedReturnAPR: tc.expected}
			got := highAPRThreshold(p)
			if diff := got - tc.want; diff > 1e-9 || diff < -1e-9 {
				t.Errorf("threshold: got %v want %v", got, tc.want)
			}
		})
	}
}

// Rule 12. Closing a paid-off card raises utilisation and shortens average
// account age, which costs the user credit score for no benefit.
func TestDebtActionsSayToKeepTheAccountOpen(t *testing.T) {
	qc := baseContext(nil)
	qc.Accounts = []domain.PlanningAccount{
		{ID: "card", Name: "Chase", Type: "liability", Balance: -500_000},
	}
	qc.Terms["card"] = domain.AccountTerms{AccountID: "card", APR: ptr(0.2249)}

	q := mustFind(t, evaluateCatalog(qc), "clear_high_apr_balance")
	if !strings.Contains(strings.ToLower(q.Detail), "keep the account open") {
		t.Errorf("should tell the user not to close the card; got %q", q.Detail)
	}
}

// A balance with no recorded rate cannot be ordered against anything. The old
// waterfall skipped these silently; naming them is how the rate gets entered.
func TestUnratedDebtsAreSurfacedRatherThanIgnored(t *testing.T) {
	qc := baseContext(nil)
	qc.Accounts = []domain.PlanningAccount{
		{ID: "card", Name: "Unknown Card", Type: "liability", Balance: -500_000},
	}

	q := mustFind(t, evaluateCatalog(qc), "clear_high_apr_balance")
	if q.Status != domain.QuestStatusBlocked {
		t.Errorf("status: got %q want blocked", q.Status)
	}
	if !strings.Contains(q.Title, "interest rate") {
		t.Errorf("should ask for the missing rate; got %q", q.Title)
	}
}

// ----------------------------------------- rule 11: cash-flow-negative guard

// Rule 11. When essentials exceed income there is nothing to allocate, and
// telling someone in that position to max an HSA is worse than saying nothing.
func TestCashFlowNegativeSuppressesSavingsActions(t *testing.T) {
	p := newProfile().
		set("hdhp_enrolled", func(p *domain.WealthProfile) { p.HDHPEnrolled = ptr(true) }).
		set("hsa_coverage_tier", func(p *domain.WealthProfile) { p.HSACoverageTier = ptr("self_only") }).
		set("emergency_fund_target_months", func(p *domain.WealthProfile) {
			p.EmergencyFundTargetMonths = ptr(6)
		}).
		build()

	solvent := baseContext(p)
	solvent.Baseline.LiquidAssets = 0 // so the cushion actions are live recommendations

	underwater := baseContext(p)
	underwater.Baseline.LiquidAssets = 0
	underwater.Baseline.MonthlyOutflow = 800_000
	underwater.Baseline.MonthlySurplus = -120_000

	before := evaluateCatalog(solvent)
	after := evaluateCatalog(underwater)

	for _, key := range []string{"max_hsa", "full_emergency_fund", "starter_emergency_fund"} {
		if findQuest(before, key) == nil {
			t.Fatalf("fixture problem: %s should be present when solvent", key)
		}
		if q := findQuest(after, key); q != nil {
			t.Errorf("%s should be suppressed while running a deficit; got %q", key, q.Title)
		}
	}
}

// ------------------------------------------- rule 13: auto-escalation

// Rule 13. The highest-return behavioural lever in the catalog, and it needs no
// new information at all.
func TestAutoEscalationFiresBelowTargetAndNotAbove(t *testing.T) {
	at := func(rate float64) *domain.Quest {
		p := newProfile().
			set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(rate) }).
			build()
		return findQuest(evaluateCatalog(baseContext(p)), "auto_escalate_deferral")
	}
	if at(0.06) == nil {
		t.Error("should fire for someone contributing 6%")
	}
	if q := at(0.18); q != nil {
		t.Errorf("should not nag someone already at 18%%; got %q", q.Title)
	}
}

// ---------------------------------------- the HSA trap and the IDR hard stop

// The second money-losing trap. "Max your HSA" is not computable from the HDHP
// flag alone, and either half wrong produces an excess contribution.
func TestHSAActionRespectsCoverageTierAndEmployerSeed(t *testing.T) {
	t.Run("blocks without a coverage tier", func(t *testing.T) {
		p := newProfile().
			set("hdhp_enrolled", func(p *domain.WealthProfile) { p.HDHPEnrolled = ptr(true) }).
			build()
		q := mustFind(t, evaluateCatalog(baseContext(p)), "max_hsa")
		if q.Status != domain.QuestStatusBlocked {
			t.Errorf("status: got %q want blocked", q.Status)
		}
	})

	t.Run("does not apply without an HDHP", func(t *testing.T) {
		p := newProfile().
			set("hdhp_enrolled", func(p *domain.WealthProfile) { p.HDHPEnrolled = ptr(false) }).
			build()
		if q := findQuest(evaluateCatalog(baseContext(p)), "max_hsa"); q != nil {
			t.Errorf("a user with no HDHP cannot hold an HSA; got %q", q.Title)
		}
	})

	t.Run("employer seed reduces the remaining room", func(t *testing.T) {
		p := newProfile().
			set("hdhp_enrolled", func(p *domain.WealthProfile) { p.HDHPEnrolled = ptr(true) }).
			set("hsa_coverage_tier", func(p *domain.WealthProfile) { p.HSACoverageTier = ptr("family") }).
			set("hsa_employer_contribution", func(p *domain.WealthProfile) {
				p.HSAEmployerContribution = ptr(money.Money(150_000))
			}).
			build()
		q := mustFind(t, evaluateCatalog(baseContext(p)), "max_hsa")
		// Family limit 875,000 less a 150,000 seed.
		if q.TargetAmount == nil || *q.TargetAmount != 725_000 {
			t.Errorf("remaining room: got %v want 725000", q.TargetAmount)
		}
	})

	t.Run("an over-contribution gets a withdrawal action, not a clamped zero", func(t *testing.T) {
		p := newProfile().
			set("hdhp_enrolled", func(p *domain.WealthProfile) { p.HDHPEnrolled = ptr(true) }).
			set("hsa_coverage_tier", func(p *domain.WealthProfile) { p.HSACoverageTier = ptr("self_only") }).
			build()
		qc := baseContext(p)
		qc.Paystub = &domain.PaystubYTD{HSAContribution: 500_000} // over the 440,000 limit

		q := mustFind(t, evaluateCatalog(qc), "max_hsa")
		if !strings.Contains(strings.ToLower(q.Title), "withdraw") {
			t.Errorf("an excess contribution needs removing, not ignoring; got %q", q.Title)
		}
		if !strings.Contains(q.Detail, "6%") {
			t.Errorf("should state the penalty that makes this urgent; got %q", q.Detail)
		}
	})
}

// The worst output the catalog could produce: extra principal on a loan headed
// for forgiveness is money destroyed, not saved.
func TestStudentLoanPayoffStopsForForgivenessPaths(t *testing.T) {
	t.Run("blocks until the forgiveness question is answered", func(t *testing.T) {
		p := newProfile().
			set("student_loan_kind", func(p *domain.WealthProfile) { p.StudentLoanKind = ptr("federal") }).
			build()
		q := mustFind(t, evaluateCatalog(baseContext(p)), "evaluate_student_loan_payoff")
		if q.Status != domain.QuestStatusBlocked {
			t.Fatalf("status: got %q want blocked", q.Status)
		}
		if !strings.Contains(q.Detail, "forgiveness") {
			t.Errorf("should explain why it will not advise yet; got %q", q.Detail)
		}
	})

	t.Run("tells a PSLF borrower NOT to prepay", func(t *testing.T) {
		p := newProfile().
			set("student_loan_kind", func(p *domain.WealthProfile) { p.StudentLoanKind = ptr("federal") }).
			set("student_loan_pslf", func(p *domain.WealthProfile) { p.StudentLoanPSLF = ptr(true) }).
			set("student_loan_idr", func(p *domain.WealthProfile) { p.StudentLoanIDR = ptr(true) }).
			build()
		q := mustFind(t, evaluateCatalog(baseContext(p)), "evaluate_student_loan_payoff")
		if !strings.Contains(strings.ToLower(q.Title), "do not pay extra") {
			t.Errorf("should actively warn against prepayment; got %q", q.Title)
		}
	})

	t.Run("evaluates normally for private loans", func(t *testing.T) {
		p := newProfile().
			set("student_loan_kind", func(p *domain.WealthProfile) { p.StudentLoanKind = ptr("private") }).
			build()
		q := mustFind(t, evaluateCatalog(unlockAllPhases(baseContext(p))), "evaluate_student_loan_payoff")
		if q.Status != domain.QuestStatusAvailable {
			t.Errorf("private loans have no forgiveness path to protect; status %q", q.Status)
		}
	})
}

// ------------------------------------------------------- phase gating

// A phase opens only when the one before it is finished, and completing a phase
// in the same run opens the next immediately rather than a cycle later.
func TestPhaseGatingOpensSequentially(t *testing.T) {
	p := newProfile().
		set("gross_annual_income", func(p *domain.WealthProfile) {
			p.GrossAnnualIncome = ptr(money.Money(20_000_000))
		}).
		set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.5) }).
		set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.06) }).
		set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.10) }).
		set("emergency_fund_target_months", func(p *domain.WealthProfile) {
			p.EmergencyFundTargetMonths = ptr(6)
		}).
		set("hdhp_enrolled", func(p *domain.WealthProfile) { p.HDHPEnrolled = ptr(true) }).
		set("hsa_coverage_tier", func(p *domain.WealthProfile) { p.HSACoverageTier = ptr("self_only") }).
		build()

	t.Run("phase 3 is locked while phase 1 is unfinished", func(t *testing.T) {
		qc := baseContext(p)
		qc.Baseline.LiquidAssets = 0 // emergency fund milestone unmet
		quests := evaluateCatalog(qc)

		// max_hsa is dated, so check an undated phase 3 action instead.
		if q := findQuest(quests, "auto_escalate_deferral"); q != nil {
			if q.Status != domain.QuestStatusLocked {
				t.Errorf("undated phase 3 action should be locked; got %q", q.Status)
			}
		}
	})

	t.Run("phase 3 opens once phase 1 and 2 milestones are met", func(t *testing.T) {
		qc := baseContext(p)
		qc.Baseline.LiquidAssets = 50_000_000 // fully funded
		qc.ExistingStatus = map[string]string{
			"establish_sell_on_vest": domain.QuestStatusComplete,
			"reserve_equity_tax_gap": domain.QuestStatusComplete,
		}
		quests := evaluateCatalog(qc)

		if q := findQuest(quests, "auto_escalate_deferral"); q != nil {
			if q.Status == domain.QuestStatusLocked {
				t.Error("phase 3 should be open once the earlier milestones are complete")
			}
		}
	})
}

// A fan-out milestone needs EVERY instance done, not just one: clearing one of
// three cards does not finish the debt milestone.
func TestFanoutMilestoneRequiresEveryInstance(t *testing.T) {
	effective := map[string]string{
		"capture_employer_match":       domain.QuestStatusComplete,
		"starter_emergency_fund":       domain.QuestStatusComplete,
		"full_emergency_fund":          domain.QuestStatusComplete,
		"clear_high_apr_balance:card1": domain.QuestStatusComplete,
		"clear_high_apr_balance:card2": domain.QuestStatusAvailable,
	}
	phase := phaseDefs[1] // liquidity
	if phaseComplete(phase, effective) {
		t.Error("one card cleared out of two should not finish the debt milestone")
	}

	effective["clear_high_apr_balance:card2"] = domain.QuestStatusComplete
	if !phaseComplete(phase, effective) {
		t.Error("all instances complete should finish the milestone")
	}
}

// A milestone that produced no instances is satisfied: there was nothing to do.
func TestMilestoneWithNoInstancesIsSatisfied(t *testing.T) {
	if !phaseComplete(phaseDefs[2], map[string]string{}) {
		t.Error("a user with no equity has nothing to complete in the equity phase")
	}
}

// ----------------------------------------------------------- integrity

// Catalog hygiene: duplicate keys would silently collide on the unique
// (user_id, catalog_key) constraint and one action would overwrite the other.
func TestCatalogKeysAreUniqueAndWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, def := range catalog() {
		if def.Key == "" {
			t.Error("every action needs a key")
		}
		if seen[def.Key] {
			t.Errorf("duplicate catalog key %q", def.Key)
		}
		seen[def.Key] = true

		if strings.Contains(def.Key, ":") {
			t.Errorf("%q: colons separate fan-out suffixes and cannot appear in a base key", def.Key)
		}
		if def.Phase < 0 || def.Phase > 5 {
			t.Errorf("%q: phase %d out of range", def.Key, def.Phase)
		}
		if def.Verification != domain.QuestVerificationAuto && def.Verification != domain.QuestVerificationManual {
			t.Errorf("%q: verification %q is neither auto nor manual", def.Key, def.Verification)
		}
		if def.Evaluate == nil {
			t.Errorf("%q: no Evaluate function", def.Key)
		}
	}
}

// Every milestone must name a real catalog entry, or a phase can never
// complete and the user is stuck behind a gate with no key.
func TestPhaseMilestonesReferenceRealActions(t *testing.T) {
	keys := map[string]bool{}
	for _, def := range catalog() {
		keys[def.Key] = true
	}
	for _, phase := range phaseDefs {
		for _, m := range phase.Milestones {
			if !keys[m] {
				t.Errorf("phase %d milestone %q does not exist in the catalog", phase.Number, m)
			}
		}
	}
}

// A blocked action must never claim a dollar figure: the figure is precisely
// what is missing.
func TestBlockedActionsDoNotAssertAmounts(t *testing.T) {
	quests := evaluateCatalog(baseContext(nil))
	blocked := 0
	for _, q := range quests {
		if q.Status != domain.QuestStatusBlocked {
			continue
		}
		blocked++
		if q.TargetAmount != nil {
			t.Errorf("%s is blocked but asserts a target of %v", q.CatalogKey, *q.TargetAmount)
		}
		if len(q.MissingFields) == 0 {
			t.Errorf("%s is blocked but names nothing that would unlock it", q.CatalogKey)
		}
	}
	if blocked == 0 {
		t.Error("an empty profile should produce blocked actions, not silence")
	}
}

// An empty profile must still produce a usable list rather than an error or a
// pile of confident nonsense.
func TestEmptyProfileProducesBlockedNotFabricatedActions(t *testing.T) {
	quests := evaluateCatalog(baseContext(nil))
	if len(quests) == 0 {
		t.Fatal("expected a list of blocked actions for a new user")
	}
	for _, q := range quests {
		switch q.Status {
		case domain.QuestStatusBlocked, domain.QuestStatusLocked,
			domain.QuestStatusAvailable, domain.QuestStatusComplete:
		default:
			t.Errorf("%s has unexpected status %q", q.CatalogKey, q.Status)
		}
		if q.Title == "" {
			t.Errorf("%s has no title", q.CatalogKey)
		}
	}
}

// Ordering is the product. Phase, then priority, then key for stability.
func TestQuestsAreOrderedByPhaseThenPriority(t *testing.T) {
	quests := evaluateCatalog(baseContext(nil))
	for i := 1; i < len(quests); i++ {
		prev, cur := quests[i-1], quests[i]
		if prev.Phase > cur.Phase {
			t.Fatalf("phase order broken at %d: %d then %d", i, prev.Phase, cur.Phase)
		}
		if prev.Phase == cur.Phase && prev.Priority > cur.Priority {
			t.Fatalf("priority order broken within phase %d: %d then %d",
				cur.Phase, prev.Priority, cur.Priority)
		}
	}
}

// Every action needs a standing title for when it blocks.
//
// A blocked action cannot build a title from figures -- the figures are exactly
// what is missing -- so it falls back to this map. A catalog entry with no
// fallback renders as the generic "More information needed", which tells the
// user nothing about what the question is for. This regressed once already
// when gofmt realigned the map and a patch silently missed its anchor.
func TestEveryCatalogEntryHasABlockedTitle(t *testing.T) {
	for _, def := range catalog() {
		if _, ok := blockedTitles[def.Key]; !ok {
			t.Errorf("%q has no entry in blockedTitles; it would render as %q",
				def.Key, blockedTitle(def.Key))
		}
	}
}

// Every field key a rule can report as missing must be describable, or the
// unlock prompt degrades to a raw snake_case identifier on screen.
func TestEveryMissingFieldKeyHasALabel(t *testing.T) {
	// Drive the catalog through several shapes of profile to surface as many
	// blocked paths as a unit test reasonably can.
	contexts := []questContext{
		baseContext(nil),
		unlockAllPhases(baseContext(nil)),
	}

	withEquity := unlockAllPhases(baseContext(newProfile().
		set("employer_is_public", func(p *domain.WealthProfile) { p.EmployerIsPublic = ptr(true) }).
		set("student_loan_kind", func(p *domain.WealthProfile) { p.StudentLoanKind = ptr("federal") }).
		set("housing_tenure", func(p *domain.WealthProfile) { p.HousingTenure = ptr("own") }).
		set("hdhp_enrolled", func(p *domain.WealthProfile) { p.HDHPEnrolled = ptr(true) }).
		set("plan_allows_after_tax", func(p *domain.WealthProfile) { p.PlanAllowsAfterTax = ptr(true) }).
		set("taxable_brokerage_value", func(p *domain.WealthProfile) {
			p.TaxableBrokerageValue = ptr(money.Money(5_000_000))
		}).
		build()))
	withEquity.Grants = []domain.EquityGrant{
		{ID: "g1", Kind: domain.EquityKindRSU},
		{ID: "e1", Kind: domain.EquityKindESPP},
	}
	contexts = append(contexts, withEquity)

	seen := map[string]bool{}
	for _, qc := range contexts {
		for _, q := range evaluateCatalog(qc) {
			for _, f := range q.MissingFields {
				seen[f] = true
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("expected some blocked actions to name missing fields")
	}
	for key := range seen {
		if _, ok := fieldLabels[key]; !ok {
			t.Errorf("missing field %q has no label; the unlock prompt would show a raw identifier", key)
		}
	}
}

// Gross pay has two sources, and a precondition written as a single field name
// cannot say so.
//
// Found in the browser: a user who had entered a full paystub was still told
// "answer your gross pay", because Requires gated on the profile field and
// blocked before Evaluate could annualise the stub. Asking for something the
// user has already supplied is the same failure as asserting a figure they
// have not — both tell them the app is not reading what they gave it.
func TestPaystubSatisfiesGrossPayPrecondition(t *testing.T) {
	baseProfile := func() *domain.WealthProfile {
		return newProfile().
			set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.5) }).
			set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.06) }).
			set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.02) }).
			build()
	}

	t.Run("blocked when neither source supplies it", func(t *testing.T) {
		q := mustFind(t, evaluateCatalog(baseContext(baseProfile())), "capture_employer_match")
		if q.Status != domain.QuestStatusBlocked {
			t.Fatalf("status: got %q want blocked", q.Status)
		}
		found := false
		for _, f := range q.MissingFields {
			if f == "gross_annual_income" {
				found = true
			}
		}
		if !found {
			t.Errorf("should still ask for gross pay, got %v", q.MissingFields)
		}
	})

	t.Run("an annualised paystub satisfies it", func(t *testing.T) {
		qc := baseContext(baseProfile())
		qc.Paystub = &domain.PaystubYTD{
			Gross:            11_000_000, // $110,000 across 18 of 26 paychecks
			PaychecksYTD:     ptr(18),
			PaychecksPerYear: ptr(26),
		}

		q := mustFind(t, evaluateCatalog(qc), "capture_employer_match")
		if q.Status == domain.QuestStatusBlocked {
			t.Fatalf("a paystub supplies gross pay; should not block. missing=%v", q.MissingFields)
		}
		// 110,000 x 26/18 = ~158,889. Full match 6% x 50% = ~4,767;
		// earned at 2% x 50% = ~1,589; shortfall ~3,178.
		if q.TargetAmount == nil {
			t.Fatal("expected a computed shortfall")
		}
		if *q.TargetAmount < 300_000 || *q.TargetAmount > 340_000 {
			t.Errorf("shortfall from annualised pay: got %s, expected around $3,178", usd(*q.TargetAmount))
		}
	})

	t.Run("the profile field still works on its own", func(t *testing.T) {
		p := newProfile().
			set("match_pct", func(p *domain.WealthProfile) { p.MatchPct = ptr(0.5) }).
			set("match_limit_pct", func(p *domain.WealthProfile) { p.MatchLimitPct = ptr(0.06) }).
			set("deferral_pct", func(p *domain.WealthProfile) { p.DeferralPct = ptr(0.02) }).
			set("gross_annual_income", func(p *domain.WealthProfile) {
				p.GrossAnnualIncome = ptr(money.Money(20_000_000))
			}).
			build()

		q := mustFind(t, evaluateCatalog(baseContext(p)), "capture_employer_match")
		if q.Status == domain.QuestStatusBlocked {
			t.Fatalf("should not block; missing=%v", q.MissingFields)
		}
		if q.TargetAmount == nil || *q.TargetAmount != 400_000 {
			t.Errorf("shortfall: got %v want 400000", q.TargetAmount)
		}
	})
}

// ------------------------------------------------- duration conditions

// A streak is consecutive from the most recent month backwards.
//
// Counting good months anywhere in the window would let a broken run pass, and
// telling someone they held a limit they broke in March is worse than saying
// nothing about it.
func TestMonthsUnderCapCountsConsecutiveFromTheEnd(t *testing.T) {
	month := func(wants int64) domain.PlanningMonth {
		return domain.PlanningMonth{Wants: money.Money(wants)}
	}
	const cap = money.Money(50_000)

	tests := []struct {
		name  string
		wants []int64 // oldest first
		want  int
	}{
		{"no history", nil, 0},
		{"all clean", []int64{40_000, 45_000, 30_000}, 3},
		{"latest over the cap breaks it immediately", []int64{40_000, 45_000, 60_000}, 0},
		{"a bad month mid-run stops the count there", []int64{40_000, 90_000, 45_000, 30_000}, 2},
		{"exactly at the cap counts as under", []int64{50_000, 50_000}, 2},
		{"one penny over does not", []int64{50_001, 50_000}, 1},
		{
			// The case an average hides: five good months carry one bad one.
			name:  "an average would pass this; a streak does not",
			wants: []int64{10_000, 10_000, 200_000, 10_000, 10_000},
			want:  2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			months := make([]domain.PlanningMonth, 0, len(tc.wants))
			for _, w := range tc.wants {
				months = append(months, month(w))
			}
			if got := monthsUnderCap(months, cap); got != tc.want {
				t.Errorf("streak: got %d want %d", got, tc.want)
			}
		})
	}
}

// The discretionary cap is the only action whose condition is about a period
// rather than a present balance, so it is the only one that can be wrong in
// this particular way.
func TestDiscretionaryCapRequiresAStreakNotAnAverage(t *testing.T) {
	// Income 680,000/mo, so the cap is 68,000.
	withMonths := func(wants ...int64) questContext {
		qc := baseContext(nil)
		months := make([]domain.PlanningMonth, 0, len(wants))
		for _, w := range wants {
			months = append(months, domain.PlanningMonth{
				Income: 680_000, Wants: money.Money(w),
			})
		}
		qc.Months = months
		qc.Baseline.MonthsOfData = len(months)
		var total int64
		for _, w := range wants {
			total += w
		}
		if len(wants) > 0 {
			qc.Baseline.MonthlyWants = money.Money(total / int64(len(wants)))
		}
		return qc
	}

	t.Run("three clean months completes it", func(t *testing.T) {
		q := mustFind(t, evaluateCatalog(withMonths(50_000, 40_000, 60_000)), "discretionary_cap")
		if q.Status != domain.QuestStatusComplete {
			t.Errorf("status: got %q want complete", q.Status)
		}
	})

	t.Run("a broken streak does not, even when the average passes", func(t *testing.T) {
		// Average is 62,000 — under the 68,000 cap — but March blew it.
		qc := withMonths(10_000, 10_000, 200_000, 10_000, 80_000)
		q := mustFind(t, evaluateCatalog(qc), "discretionary_cap")
		if q.Status == domain.QuestStatusComplete {
			t.Errorf("an average under the cap should not pass a streak test; detail was %q", q.Detail)
		}
	})

	t.Run("too little history says so rather than guessing", func(t *testing.T) {
		q := mustFind(t, evaluateCatalog(withMonths(40_000)), "discretionary_cap")
		if q.Status == domain.QuestStatusComplete {
			t.Error("one month cannot establish a three-month streak")
		}
		if !strings.Contains(q.Detail, "not yet enough history") {
			t.Errorf("should say why it cannot tell; got %q", q.Detail)
		}
	})

	t.Run("partial progress is reported", func(t *testing.T) {
		// Two clean months at the end of a four-month window.
		qc := withMonths(200_000, 200_000, 50_000, 40_000)
		q := mustFind(t, evaluateCatalog(qc), "discretionary_cap")
		if q.Status == domain.QuestStatusComplete {
			t.Fatal("two months is short of the three-month bar")
		}
		if !strings.Contains(q.Detail, "2 months in") {
			t.Errorf("should credit the streak so far; got %q", q.Detail)
		}
	})
}
