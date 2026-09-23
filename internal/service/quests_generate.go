package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Action generation.
//
// One pass over the catalog against a single snapshot of the user's data, then
// three cross-cutting passes that no individual rule should have to think
// about: the cash-flow guard, phase gating, and staleness.

// GenerateQuests rebuilds the user's action list and persists it.
//
// Regeneration is safe to run whenever anything changes: ReplaceQuests
// preserves completion and skips, refreshing only the interpolated text,
// targets and due dates -- which are exactly what go out of date as balances
// move.
func (s *Service) GenerateQuests(ctx context.Context, userID string) ([]domain.Quest, PlanSummary, error) {
	qc, err := s.buildQuestContext(ctx, userID)
	if err != nil {
		return nil, PlanSummary{}, err
	}

	quests := evaluateCatalog(qc)

	if err := s.repo.ReplaceQuests(ctx, userID, quests); err != nil {
		return nil, PlanSummary{}, err
	}
	stored, err := s.repo.ListQuests(ctx, userID)
	if err != nil {
		return nil, PlanSummary{}, err
	}

	// Record what moved. Compared against the statuses read at the start of
	// this same generation, so an action the engine closed on the user's behalf
	// leaves a trace instead of looking identical to one they ticked.
	s.recordTransitions(ctx, userID, qc.ExistingStatus, stored)
	// Summarised from the STORED list rather than the freshly generated one, so
	// completions the user has claimed are reflected in what is still owed.
	return stored, summarise(qc, stored), nil
}

// ListQuests returns the stored action list and its summary, regenerating first
// so the figures reflect current balances.
func (s *Service) ListQuests(ctx context.Context, userID string) ([]domain.Quest, PlanSummary, error) {
	return s.GenerateQuests(ctx, userID)
}

// SetQuestStatus records a completion, skip, or reversal.
func (s *Service) SetQuestStatus(ctx context.Context, userID, questID, status, source, note string) (*domain.Quest, error) {
	return s.repo.SetQuestStatus(ctx, userID, questID, status, source, note)
}

// PlanSummary is the handful of figures every wealth surface needs in common.
//
// Served alongside the action list so the client is not left recomputing the
// baseline to draw a trend strip, and so "when is the cushion finished" comes
// from the same engine that decides what the cushion is -- the two disagreeing
// was the failure mode of having a waterfall in the browser and a catalog on
// the server.
type PlanSummary struct {
	MonthlyIncome    money.Money `json:"monthly_income"`
	MonthlyOutflow   money.Money `json:"monthly_outflow"`
	EssentialMonthly money.Money `json:"essential_monthly"`
	MonthlySurplus   money.Money `json:"monthly_surplus"`
	LiquidAssets     money.Money `json:"liquid_assets"`
	NetWorth         money.Money `json:"net_worth"`
	BucketCoverage   float64     `json:"bucket_coverage"`
	MonthsOfData     int         `json:"months_of_data"`

	// TargetSavingsRate is what the chosen strategy asks for, so the client can
	// show the gap without holding its own copy of the strategy table.
	TargetSavingsRate float64 `json:"target_savings_rate"`

	// CrossoverMonths is how long until every funding action in phase 1 is
	// satisfied and the same money starts being invested instead. Null when
	// there is no surplus to fund them with -- an honest "never at this rate"
	// rather than a very large number.
	CrossoverMonths *int `json:"crossover_months"`
}

// strategyTargets mirrors STRATEGY_PROFILES in planning-math.ts.
var strategyTargets = map[string]float64{
	"aggressive": 0.30,
	"balanced":   0.20,
	"flexible":   0.15,
}

// summarise computes the shared figures from a generated action list.
func summarise(qc questContext, quests []domain.Quest) PlanSummary {
	target := strategyTargets["balanced"]
	if qc.Profile.Strategy != nil {
		if t, ok := strategyTargets[*qc.Profile.Strategy]; ok {
			target = t
		}
	}

	out := PlanSummary{
		MonthlyIncome:     qc.Baseline.MonthlyIncome,
		MonthlyOutflow:    qc.Baseline.MonthlyOutflow,
		EssentialMonthly:  qc.Baseline.EssentialMonthly,
		MonthlySurplus:    qc.Baseline.MonthlySurplus,
		LiquidAssets:      qc.Baseline.LiquidAssets,
		NetWorth:          qc.Baseline.NetWorth,
		BucketCoverage:    qc.Baseline.BucketCoverage,
		MonthsOfData:      qc.Baseline.MonthsOfData,
		TargetSavingsRate: target,
	}

	// Everything still owed across phase 1, funded sequentially out of one
	// surplus -- which is why it is a sum rather than a max.
	var owed money.Money
	for _, q := range quests {
		if q.Phase != PhaseLiquidity || q.TargetAmount == nil {
			continue
		}
		switch q.Status {
		case domain.QuestStatusAvailable, domain.QuestStatusBlocked, domain.QuestStatusLocked:
			owed += *q.TargetAmount
		}
	}
	if owed <= 0 {
		zero := 0
		out.CrossoverMonths = &zero
	} else if qc.Baseline.MonthlySurplus > 0 {
		months := int(owed / qc.Baseline.MonthlySurplus)
		if owed%qc.Baseline.MonthlySurplus != 0 {
			months++
		}
		out.CrossoverMonths = &months
	}
	return out
}

// Phase describes a phase for display: the engine and the UI must agree on
// what the stages are called and what order they come in.
type Phase struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
}

// Phases returns the phase definitions, so the client labels them from the same
// source the generator gates on rather than hardcoding a parallel list.
func (s *Service) Phases() []Phase {
	out := make([]Phase, 0, len(phaseDefs))
	for _, p := range phaseDefs {
		out = append(out, Phase{Number: p.Number, Name: p.Name})
	}
	return out
}

// ListQuestEvents returns one action's history.
func (s *Service) ListQuestEvents(ctx context.Context, userID, questID string) ([]domain.QuestEvent, error) {
	return s.repo.ListQuestEvents(ctx, userID, questID)
}

// buildQuestContext loads every input once, so no rule issues its own queries
// and all of them see a consistent snapshot.
func (s *Service) buildQuestContext(ctx context.Context, userID string) (questContext, error) {
	var qc questContext

	profile, err := s.GetWealthProfile(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("profile: %w", err)
	}
	pb, err := s.repo.GetPlanningBaseline(ctx, userID, BaselineMonths)
	if err != nil {
		return qc, fmt.Errorf("planning baseline: %w", err)
	}

	now := time.Now().UTC()
	limits, err := s.repo.GetTaxLimits(ctx, now.Year())
	if err != nil {
		// A missing tax year is an operator problem, and continuing would let
		// every contribution rule compute headroom against zero and tell the
		// user they have nothing left to contribute. Refuse instead.
		return qc, fmt.Errorf("tax limits for %d: %w", now.Year(), err)
	}

	terms, err := s.repo.ListAccountTerms(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("account terms: %w", err)
	}
	retirement, err := s.repo.ListRetirementAccountTerms(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("retirement account terms: %w", err)
	}
	grants, err := s.repo.ListEquityGrants(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("equity grants: %w", err)
	}
	planned, err := s.repo.ListPlannedExpenses(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("planned expenses: %w", err)
	}
	dependents, err := s.repo.ListDependents(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("dependents: %w", err)
	}
	paystub, err := s.repo.GetLatestPaystub(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("paystub: %w", err)
	}
	existing, err := s.repo.ListQuests(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("existing quests: %w", err)
	}

	stale := map[string]bool{}
	for _, key := range StaleFields(profile, now) {
		stale[key] = true
	}
	existingStatus := map[string]string{}
	for _, q := range existing {
		existingStatus[q.CatalogKey] = q.Status
	}

	return questContext{
		Now:            now,
		TaxYear:        now.Year(),
		Profile:        profile,
		Baseline:       ComputeBaseline(pb),
		Limits:         limits,
		Accounts:       pb.Accounts,
		Months:         completeMonths(pb.Months),
		Terms:          terms,
		Retirement:     retirement,
		Grants:         grants,
		Planned:        planned,
		Dependents:     dependents,
		Paystub:        paystub,
		StaleFields:    stale,
		ExistingStatus: existingStatus,
	}, nil
}

// evaluateCatalog runs every rule and applies the cross-cutting passes.
//
// Exported only to the package so tests can drive it from a hand-built context
// without a database.
func evaluateCatalog(qc questContext) []domain.Quest {
	var quests []domain.Quest

	// Cash-flow-negative guard. When essentials exceed income there is nothing
	// to allocate, and the only honest outputs are expense reduction and
	// income. Recommending an HSA contribution to someone running a deficit is
	// worse than saying nothing.
	cashFlowNegative := qc.Baseline.MonthsOfData > 0 &&
		qc.Baseline.MonthlySurplus <= 0 &&
		qc.Baseline.MonthlyIncome > 0

	for _, def := range catalog() {
		// Unconditional preconditions are checked once here rather than inside
		// every rule. A blocked action still renders -- with the answers that
		// would unlock it named, so the UI can price the question.
		missing := missingFields(qc, def.Requires)
		if def.AlsoMissing != nil {
			missing = append(missing, def.AlsoMissing(qc)...)
		}
		if len(missing) > 0 {
			quests = append(quests, blockedQuest(def, missing, qc))
			continue
		}

		for _, res := range def.Evaluate(qc) {
			if res.NotApplicable {
				continue
			}
			if cashFlowNegative && res.SuppressWhenCashFlowNegative {
				continue
			}
			quests = append(quests, toQuest(def, res, qc))
		}
	}

	applyPhaseGating(quests, qc)

	sort.SliceStable(quests, func(i, j int) bool {
		if quests[i].Phase != quests[j].Phase {
			return quests[i].Phase < quests[j].Phase
		}
		if quests[i].Priority != quests[j].Priority {
			return quests[i].Priority < quests[j].Priority
		}
		return quests[i].CatalogKey < quests[j].CatalogKey
	})

	return quests
}

// missingFields returns the required keys the user has not answered.
//
// Answered means a provenance row exists -- not that the value is non-zero. A
// deliberate zero (no employer match) is an answer; an absent one is not.
func missingFields(qc questContext, required []string) []string {
	var missing []string
	for _, key := range required {
		if _, ok := qc.Profile.Fields[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

func blockedQuest(def questDef, missing []string, qc questContext) domain.Quest {
	return domain.Quest{
		CatalogKey:    def.Key,
		Phase:         def.Phase,
		Priority:      def.Priority,
		Status:        domain.QuestStatusBlocked,
		Title:         blockedTitle(def.Key),
		Detail:        fmt.Sprintf("Answer %s to work this out.", describeFields(missing)),
		Verification:  def.Verification,
		MissingFields: missing,
		GeneratedAt:   qc.Now,
	}
}

func toQuest(def questDef, res questResult, qc questContext) domain.Quest {
	key := def.Key
	if res.KeySuffix != "" {
		key = def.Key + ":" + res.KeySuffix
	}

	status := domain.QuestStatusAvailable
	var completedAt *time.Time
	switch {
	case len(res.Missing) > 0:
		status = domain.QuestStatusBlocked
		// A rule that blocks from inside Evaluate (rather than on Requires)
		// has no figures to build a title from -- the figures are what is
		// missing. Fall back to the catalog's standing description so the
		// action is never nameless on screen.
		if res.Title == "" {
			res.Title = blockedTitle(def.Key)
		}
		if res.Detail == "" {
			res.Detail = fmt.Sprintf("Answer %s to work this out.", describeFields(res.Missing))
		}
	case res.Complete:
		status = domain.QuestStatusComplete
		at := qc.Now
		completedAt = &at
	}

	return domain.Quest{
		CatalogKey:    key,
		Phase:         def.Phase,
		Priority:      def.Priority,
		Status:        status,
		Title:         res.Title,
		Detail:        res.Detail,
		TargetAmount:  res.Target,
		DueDate:       res.Due,
		Verification:  def.Verification,
		Stale:         dependsOnStale(qc, def.Requires) || dependsOnStale(qc, res.Missing),
		MissingFields: res.Missing,
		CompletedAt:   completedAt,
		GeneratedAt:   qc.Now,
	}
}

func dependsOnStale(qc questContext, keys []string) bool {
	for _, k := range keys {
		if qc.StaleFields[k] {
			return true
		}
	}
	return false
}

// applyPhaseGating locks actions in phases the user has not reached.
//
// The escape hatch is the whole point: an action carrying a DUE DATE is never
// locked. Payroll deferrals close on 31 December, the IRA deadline is the
// filing date, open enrollment is a fortnight, and a vest lands when the grant
// says so. Gating those behind phase completion would produce a plan that
// silently lets hard deadlines pass while the user works on something else.
func applyPhaseGating(quests []domain.Quest, qc questContext) {
	// Status as it will stand after this generation, so a phase completed by
	// this very run unlocks the next one immediately rather than a cycle later.
	effective := map[string]string{}
	for _, q := range quests {
		effective[q.CatalogKey] = q.Status
	}
	for key, status := range qc.ExistingStatus {
		if status == domain.QuestStatusComplete || status == domain.QuestStatusSkipped {
			effective[key] = status
		}
	}

	// A phase opens once every earlier phase's milestones are met. Walking in
	// order means the first incomplete phase closes everything after it, and
	// phase 1 is always open because nothing precedes it.
	unlocked := map[int]bool{PhaseCrossCutting: true}
	open := true
	for _, phase := range phaseDefs {
		if phase.Number == PhaseCrossCutting {
			continue
		}
		unlocked[phase.Number] = open
		if !phaseComplete(phase, effective) {
			open = false
		}
	}

	for i := range quests {
		q := &quests[i]
		if q.Phase == PhaseCrossCutting || unlocked[q.Phase] {
			continue
		}
		if q.DueDate != nil {
			continue // deadlines outrank gating
		}
		if q.Status == domain.QuestStatusComplete || q.Status == domain.QuestStatusSkipped {
			continue
		}
		// Blocked outranks locked. "We need something from you" is a durable
		// fact about the action and the more actionable of the two, and the
		// phase number travels on the quest regardless -- so the UI can still
		// render "Phase 3, needs 2 answers" without the status having to carry
		// both. Overwriting it here would lose the only status that prompts
		// the user to do anything.
		if q.Status == domain.QuestStatusBlocked {
			continue
		}
		q.Status = domain.QuestStatusLocked
	}
}

func firstGatedPhase() int { return PhaseLiquidity }

// phaseComplete reports whether every milestone action is done.
//
// A milestone that produced no instances counts as satisfied: there was nothing
// to do. Fan-out milestones match on the key prefix, so every card has to be
// cleared before the debt milestone passes, not just one.
func phaseComplete(phase phaseDef, effective map[string]string) bool {
	for _, milestone := range phase.Milestones {
		for key, status := range effective {
			if key != milestone && !strings.HasPrefix(key, milestone+":") {
				continue
			}
			switch status {
			case domain.QuestStatusComplete, domain.QuestStatusSkipped, domain.QuestStatusNotApplicable:
			default:
				return false
			}
		}
	}
	return true
}

// blockedTitle names a blocked action without pretending to know its figures.
//
// A blocked action cannot state a dollar amount -- that is what is missing --
// so it says what it is about and lets MissingFields carry the ask.
func blockedTitle(key string) string {
	if t, ok := blockedTitles[key]; ok {
		return t
	}
	return "More information needed"
}

var blockedTitles = map[string]string{
	"capture_employer_match":        "Check whether you are leaving employer match behind",
	"raise_401k_deferral":           "Work out your 401(k) contribution headroom",
	"max_hsa":                       "Work out your HSA contribution headroom",
	"resolve_roth_route":            "Work out how to fund a Roth IRA",
	"mega_backdoor_roth":            "Check whether your plan allows a mega backdoor Roth",
	"reserve_equity_tax_gap":        "Work out the tax owed on your vesting equity",
	"calibrate_w4_withholding":      "Check whether your withholding covers your equity income",
	"maximize_espp_rate":            "Check whether your ESPP contribution is worth raising",
	"establish_sell_on_vest":        "Set up automatic selling on vest",
	"secure_disability_coverage":    "Check your disability coverage against your real income",
	"secure_life_coverage":          "Check your life cover against what your household needs",
	"evaluate_student_loan_payoff":  "Work out whether to pay student loans down early",
	"evaluate_mortgage_prepay":      "Work out whether to pay your mortgage down early",
	"remove_pmi":                    "Check whether you can drop mortgage insurance",
	"build_pre59_bridge":            "Work out how to reach your money before 59 and a half",
	"reduce_employer_concentration": "Work out how much of your net worth is employer stock",
	"consolidate_into_index_core":   "Work out the tax cost of consolidating your holdings",
	"reach_fi_number":               "Work out your financial independence number",

	"harvest_tax_losses":                "Check whether you have investment losses worth harvesting",
	"starter_emergency_fund":            "Work out how big a starter cushion you need",
	"full_emergency_fund":               "Work out your emergency fund target",
	"clear_high_apr_balance":            "Add interest rates to your debts",
	"clear_promo_balance_before_expiry": "Check for promotional balances about to expire",
	"sweep_idle_cash":                   "Check whether any cash is sitting idle",
	"discretionary_cap":                 "Work out your discretionary spending",
	"fund_ira_to_limit":                 "Work out your IRA contribution room",
	"auto_escalate_deferral":            "Check whether automatic increases are worth turning on",
	"eliminate_consumer_debt":           "Work out what consumer debt is left",
	"scale_savings_rate":                "Work out your savings rate",
	"enable_drip":                       "Check dividend reinvestment across your accounts",
	"automate_surplus_sweep":            "Work out how much surplus there is to sweep",
	"liquidate_vested_rsu":              "Check your upcoming equity vests",
	"sell_espp_at_purchase":             "Check your upcoming ESPP purchase",
	"elect_dependent_care_fsa":          "Check whether a dependent care FSA applies",
}

// FieldLabels exposes the human descriptions of field keys.
//
// Served to the client rather than duplicated there: the unlock prompt beside
// a blocked action and the sentence the engine writes into its detail text have
// to name the same thing, and two copies of this map would drift the first time
// a question was reworded.
func (s *Service) FieldLabels() map[string]string {
	out := make(map[string]string, len(fieldLabels))
	for k, v := range fieldLabels {
		out[k] = v
	}
	return out
}

// describeFields turns field keys into something readable in a sentence.
func describeFields(keys []string) string {
	labels := make([]string, 0, len(keys))
	for _, k := range keys {
		if l, ok := fieldLabels[k]; ok {
			labels = append(labels, l)
		} else {
			labels = append(labels, strings.ReplaceAll(k, "_", " "))
		}
	}
	switch len(labels) {
	case 0:
		return "a few more questions"
	case 1:
		return labels[0]
	case 2:
		return labels[0] + " and " + labels[1]
	default:
		return strings.Join(labels[:len(labels)-1], ", ") + " and " + labels[len(labels)-1]
	}
}

var fieldLabels = map[string]string{
	"date_of_birth":                "your date of birth",
	"filing_status":                "your tax filing status",
	"resident_state":               "the state you file in",
	"marital_status":               "whether you are married",
	"spouse_gross_annual":          "your spouse's income",
	"gross_annual_income":          "your gross pay",
	"deferral_pct":                 "your current 401(k) contribution rate",
	"match_pct":                    "your employer's match rate",
	"match_limit_pct":              "how much of your salary the match covers",
	"hdhp_enrolled":                "whether you are on a high-deductible health plan",
	"hsa_coverage_tier":            "whether your HSA covers you or your family",
	"hsa_employer_contribution":    "what your employer puts into your HSA",
	"prior_year_tax_liability":     "last year's total tax",
	"prior_year_agi":               "last year's adjusted gross income",
	"traditional_ira_balance":      "your traditional IRA balance",
	"taxable_brokerage_value":      "your taxable brokerage balance",
	"taxable_unrealized_gain":      "how much of that is unrealised gain",
	"taxable_employer_stock":       "how much employer stock you hold",
	"employer_is_public":           "whether your employer is publicly traded",
	"plan_allows_after_tax":        "whether your plan takes after-tax contributions",
	"plan_allows_in_service":       "whether your plan allows in-service withdrawals",
	"plan_accepts_rollovers":       "whether your plan accepts incoming rollovers",
	"ltd_replacement_pct":          "what share of income your disability cover replaces",
	"ltd_monthly_cap":              "your disability benefit cap",
	"life_death_benefit":           "your life insurance cover",
	"housing_tenure":               "whether you rent or own",
	"mortgage_apr":                 "your mortgage rate",
	"mortgage_balance":             "your mortgage balance",
	"pays_pmi":                     "whether you pay mortgage insurance",
	"student_loan_kind":            "whether your student loans are federal or private",
	"student_loan_idr":             "whether you are on an income-driven repayment plan",
	"student_loan_pslf":            "whether you are pursuing loan forgiveness",
	"target_independence_age":      "when you want work to become optional",
	"emergency_fund_target_months": "how many months of cover you want",
	"expected_return_apr":          "the return you expect on investments",

	// Pseudo-keys. These do not appear in the profile field registry: they
	// describe data the app is missing rather than an answer it wants, and the
	// client distinguishes them by checking against GET /wealth/fields.
	"transaction_history": "enough categorised spending to work from",
	"account_apr":         "the interest rate on each balance",
	"paystub_ytd":         "the year-to-date figures from a recent paystub",
	"espp_terms":          "your ESPP discount, contribution rate and plan maximum",
}
