package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Action generation.
//
// One pass over the catalog against a single snapshot of the user's data, then
// the cross-cutting passes no individual rule should have to think about: the
// cash-flow guard, phase gating, the user's own marks, and staleness.

// GenerateQuests works out the user's action list from their current data.
//
// The list itself is stored nowhere, so its figures are never stale and a page
// load is not a database write. The only writes are for actions whose status
// changed since it was last seen -- on an ordinary load, none.
func (s *Service) GenerateQuests(ctx context.Context, userID string) ([]domain.Quest, PlanSummary, error) {
	quests, _, summary, err := s.ListQuests(ctx, userID)
	return quests, summary, err
}

// ListQuests returns the action list, the phases with what unlocks each, and
// the summary, all from one reading.
func (s *Service) ListQuests(ctx context.Context, userID string) ([]domain.Quest, []Phase, PlanSummary, error) {
	qc, err := s.buildQuestContext(ctx, userID)
	if err != nil {
		return nil, nil, PlanSummary{}, err
	}

	quests := evaluateCatalog(qc)
	// Record what moved since the last reading, so an action the engine closed
	// on the user's behalf leaves a trace instead of looking identical to one
	// they ticked.
	s.recordTransitions(ctx, userID, qc.State, quests)
	summary := summarise(qc, quests)
	datedFunding(quests, summary.Funding)
	return quests, phaseProgress(quests, qc), summary, nil
}

// SetQuestStatus records the user marking an action done or not for them, or
// taking that back with "available".
//
// Only keys the catalog can produce are accepted. A fan-out key names one
// instance -- one card, one grant -- and is accepted by its base.
func (s *Service) SetQuestStatus(ctx context.Context, userID, key, status, note string) error {
	base, _, _ := strings.Cut(key, ":")
	if !isCatalogKey(base) {
		return apperrors.ErrNotFound
	}
	switch status {
	case domain.QuestStatusComplete, domain.QuestStatusSkipped:
		return s.repo.SetQuestMark(ctx, userID, key, status, note)
	case domain.QuestStatusAvailable:
		// Reopening something never marked is already the case, not an error.
		if err := s.repo.ClearQuestMark(ctx, userID, key); err != nil && !errors.Is(err, apperrors.ErrNotFound) {
			return err
		}
		return nil
	}
	return fmt.Errorf("%w: cannot set an action to %q", apperrors.ErrInvalidInput, status)
}

// isCatalogKey reports whether the catalog defines an action with this key.
func isCatalogKey(key string) bool {
	for _, def := range catalog() {
		if def.Key == key {
			return true
		}
	}
	return false
}

// PlanSummary is the handful of figures every wealth surface needs in common.
//
// Served alongside the action list so the client is not left recomputing the
// baseline to draw a trend strip, and so "when is the cushion finished" comes
// from the same engine that decides what the cushion is -- the two disagreeing
// was the failure mode of having a waterfall in the browser and a catalog on
// the server.
type PlanSummary struct {
	MonthlyIncome     money.Money `json:"monthly_income"`
	MonthlyOutflow    money.Money `json:"monthly_outflow"`
	EssentialMonthly  money.Money `json:"essential_monthly"`
	MonthlyWants      money.Money `json:"monthly_wants"`
	MonthlyUnbucketed money.Money `json:"monthly_unbucketed"`
	MonthlySavings    money.Money `json:"monthly_savings"`
	MonthlySurplus    money.Money `json:"monthly_surplus"`
	LiquidAssets      money.Money `json:"liquid_assets"`
	TotalLiabilities  money.Money `json:"total_liabilities"`
	NetWorth          money.Money `json:"net_worth"`
	BucketCoverage    float64     `json:"bucket_coverage"`
	MonthsOfData      int         `json:"months_of_data"`

	// TargetSavingsRate is what the chosen strategy asks for, so the client can
	// show the gap without holding its own copy of the strategy table.
	TargetSavingsRate float64 `json:"target_savings_rate"`

	// CrossoverMonths is how long until every funding action in phase 1 is
	// satisfied and the same money starts being invested instead. Null when
	// the surplus never gets there -- there is none, or a card's interest
	// outruns it -- an honest "never at this rate" rather than a very large
	// number.
	CrossoverMonths *int `json:"crossover_months"`
	// CrossoverOn is the month that happens, as a date on its first day.
	CrossoverOn *time.Time `json:"crossover_on"`

	// Funding is the schedule CrossoverMonths is the end of, served so the
	// Cash page draws the same stages instead of re-deriving them.
	Funding []FundingStage `json:"funding"`

	// The independence number, the spending it is built on, and what counts
	// toward it -- the figures reach_fi_number decides on, served so the
	// Retirement tab shows the same ones. Zero when there is no spending yet.
	FITarget       money.Money `json:"fi_target"`
	FIAnnualSpend  money.Money `json:"fi_annual_spend"`
	InvestedAssets money.Money `json:"invested_assets"`

	// Workplace is what goes in through payroll; nil while the contribution
	// rate or pay is unknown.
	Workplace *WorkplaceSaving   `json:"workplace"`
	Limits    ContributionLimits `json:"limits"`
}

// FundingStage is one phase 1 action the monthly surplus pays for.
type FundingStage struct {
	QuestID string `json:"quest_id"`
	// Remaining is what the surplus still has to put in: zero once the action
	// is done, skipped, or has no figure yet.
	Remaining money.Money `json:"remaining"`
	// StartsInMonths and MonthsToComplete are null when the surplus never
	// finishes the stage, which is "never at this rate". A stage with nothing
	// remaining takes zero months and has no start.
	StartsInMonths   *int `json:"starts_in_months"`
	MonthsToComplete *int `json:"months_to_complete"`
	// CompletesOn is the month it finishes, as a date on its first day.
	CompletesOn *time.Time `json:"completes_on"`
}

// fundedFromSurplus names the phase 1 actions paid for out of the monthly
// surplus.
//
// The rest of the phase carries targets of a different kind -- a yearly match
// claimed through payroll, a monthly spending cap, cash moved between accounts
// the user already has. Adding those to the sum mixed yearly figures with
// monthly ones and pushed the crossover out by months that no one was saving
// toward.
var fundedFromSurplus = map[string]bool{
	"starter_emergency_fund":            true,
	"clear_promo_balance_before_expiry": true,
	"clear_high_apr_balance":            true,
	"full_emergency_fund":               true,
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
		MonthlyWants:      qc.Baseline.MonthlyWants,
		MonthlyUnbucketed: qc.Baseline.MonthlyUnbucketed,
		MonthlySavings:    qc.Baseline.MonthlySavings,
		MonthlySurplus:    qc.Baseline.MonthlySurplus,
		LiquidAssets:      qc.Baseline.LiquidAssets,
		TotalLiabilities:  qc.Baseline.TotalLiabilities,
		NetWorth:          qc.Baseline.NetWorth,
		BucketCoverage:    qc.Baseline.BucketCoverage,
		MonthsOfData:      qc.Baseline.MonthsOfData,
		TargetSavingsRate: target,
		InvestedAssets:    qc.investedAssets(),
		Limits:            qc.contributionLimits(),
	}
	out.Funding, out.CrossoverMonths = fundingSchedule(quests, qc.Baseline.MonthlySurplus, qc.debtMonthlyRate, qc.Now)
	if out.CrossoverMonths != nil && *out.CrossoverMonths > 0 {
		on := monthStart(qc.Now, *out.CrossoverMonths)
		out.CrossoverOn = &on
	}
	out.FITarget, out.FIAnnualSpend, _ = qc.fiTarget()
	if saving, ok := qc.workplaceSaving(); ok {
		out.Workplace = &saving
	}
	return out
}

// maxScheduleMonths bounds the simulation. Past fifty years "never at this
// rate" is the honest answer, and a card whose interest outruns the surplus
// would otherwise never finish.
const maxScheduleMonths = 600

// fundingSchedule pays phase 1's funding actions out of the surplus month by
// month, in the engine's order, and dates each one.
//
// Each action takes the whole surplus until it is satisfied. A debt keeps
// accruing interest at monthlyRate until it is cleared -- including while it
// waits behind the cushion -- so a card is dearer by the time its turn comes;
// dividing today's balance by the surplus ignored that. With no interest the
// result is the plain running-total schedule, and the last stage always ends
// on the crossover. A nil monthlyRate charges no interest.
func fundingSchedule(
	quests []domain.Quest, surplus money.Money, monthlyRate func(key string) float64, now time.Time,
) ([]FundingStage, *int) {
	type slot struct {
		stage      FundingStage
		balance    float64 // cents, fractional while interest accrues
		rate       float64
		start, end int // 1-based months from now; 0 = not yet
	}

	var slots []*slot
	// The starter cushion and the full fund are measured against the same
	// cash, so the full fund's gap already contains the starter's. Summing the
	// two asked the surplus for the first month of essentials twice.
	var starterQueued money.Money
	for _, q := range quests {
		base, _, _ := strings.Cut(q.CatalogKey, ":")
		if q.Phase != PhaseLiquidity || !fundedFromSurplus[base] {
			continue
		}

		var remaining money.Money
		switch q.Status {
		case domain.QuestStatusAvailable, domain.QuestStatusBlocked, domain.QuestStatusLocked:
			if q.TargetAmount != nil && *q.TargetAmount > 0 {
				remaining = *q.TargetAmount
			}
		}
		switch base {
		case "starter_emergency_fund":
			starterQueued = remaining
		case "full_emergency_fund":
			remaining = max(remaining-starterQueued, 0)
		}

		s := &slot{stage: FundingStage{QuestID: q.ID, Remaining: remaining}, balance: float64(remaining)}
		if monthlyRate != nil {
			s.rate = monthlyRate(q.CatalogKey)
		}
		slots = append(slots, s)
	}

	open := 0
	for _, s := range slots {
		if s.balance > 0 {
			open++
		}
	}
	crossover := 0
	for m := 1; surplus > 0 && open > 0 && m <= maxScheduleMonths; m++ {
		// Interest first: a balance waiting its turn still grows.
		for _, s := range slots {
			if s.balance > 0 && s.rate > 0 {
				s.balance += s.balance * s.rate
			}
		}
		budget := float64(surplus)
		for _, s := range slots {
			if s.balance <= 0 || budget <= 0 {
				continue
			}
			if s.start == 0 {
				s.start = m
			}
			pay := min(budget, s.balance)
			s.balance -= pay
			budget -= pay
			if s.balance < 0.5 { // under half a cent is paid
				s.balance, s.end = 0, m
				open--
			}
		}
		if open == 0 {
			crossover = m
		}
	}

	stages := make([]FundingStage, 0, len(slots))
	for _, s := range slots {
		st := s.stage
		switch {
		case st.Remaining == 0:
			zero := 0
			st.MonthsToComplete = &zero
		case s.end > 0:
			start, months, done := s.start-1, s.end-s.start+1, monthStart(now, s.end)
			st.StartsInMonths, st.MonthsToComplete, st.CompletesOn = &start, &months, &done
		}
		stages = append(stages, st)
	}

	switch {
	case open > 0:
		return stages, nil // never at this rate
	case crossover == 0:
		zero := 0 // nothing was owed
		return stages, &zero
	}
	return stages, &crossover
}

// monthStart is the first day of the month n months on from now's. Month n of
// a schedule is paid from the surplus n months out, so a stage that ends in
// month 1 finishes next month, not in what is left of this one.
func monthStart(now time.Time, n int) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, n, 0)
}

// debtMonthlyRate is the monthly interest on a funding action's balance: a
// card's APR over twelve. The cushions earn nothing here, and a promotional
// balance is interest-free until its own dated action says otherwise.
func (c questContext) debtMonthlyRate(key string) float64 {
	base, account, ok := strings.Cut(key, ":")
	if !ok || base != "clear_high_apr_balance" {
		return 0
	}
	if t, ok := c.Terms[account]; ok && t.APR != nil {
		return *t.APR / 12
	}
	return 0
}

// datedFunding writes each open funding action's finishing month into its
// detail, so the action itself says when. The emergency fund used to say how
// many months its own gap would take, as if nothing were queued ahead of it.
func datedFunding(quests []domain.Quest, stages []FundingStage) {
	for _, st := range stages {
		if st.CompletesOn == nil || st.Remaining <= 0 {
			continue
		}
		for i := range quests {
			if quests[i].ID == st.QuestID && quests[i].Status == domain.QuestStatusAvailable {
				quests[i].Detail = strings.TrimSpace(quests[i].Detail + fmt.Sprintf(
					" At your current surplus this is done in %s.", st.CompletesOn.Format("January 2006")))
			}
		}
	}
}

// Phase describes a phase for display, with what finishes it: the engine and
// the UI must agree on what the stages are called, what order they come in and
// what stands between the user and the next one.
type Phase struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
	// Unlocked is whether every earlier phase is finished.
	Unlocked bool `json:"unlocked"`
	// Milestones are what finishes this phase and so unlocks the next.
	Milestones []Milestone `json:"milestones"`
}

// Milestone is one condition for finishing a phase.
type Milestone struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Done  bool   `json:"done"`
	// Applies is false when the user has nothing to do for it at all -- no
	// card to clear, no equity to sell -- which counts as done for gating
	// but is not worth showing as an achievement.
	Applies bool `json:"applies"`
}

// milestoneLabels state each milestone as the condition it checks, so the
// list reads as what is left rather than as more actions.
var milestoneLabels = map[string]string{
	"capture_employer_match":        "No employer match left unclaimed",
	"starter_emergency_fund":        "A month of essentials in cash",
	"clear_high_apr_balance":        "No high-interest balances",
	"full_emergency_fund":           "A full emergency fund",
	"establish_sell_on_vest":        "Equity sold as it vests",
	"reserve_equity_tax_gap":        "Tax on equity set aside",
	"raise_401k_deferral":           "Your 401(k) allowance in use",
	"max_hsa":                       "Your HSA funded for the year",
	"resolve_roth_route":            "A way into a Roth IRA settled",
	"reduce_employer_concentration": "Employer stock under 5% of net worth",
	"automate_surplus_sweep":        "Surplus invested automatically",
	"reach_fi_number":               "Your independence number reached",
}

// phaseProgress reports each phase with its milestones, from the same
// statuses the gating used, so what the page says unlocks a phase is what
// actually does.
func phaseProgress(quests []domain.Quest, qc questContext) []Phase {
	effective := effectiveStatuses(quests, qc)
	unlocked := unlockedPhases(effective)

	out := make([]Phase, 0, len(phaseDefs))
	for _, p := range phaseDefs {
		phase := Phase{Number: p.Number, Name: p.Name, Unlocked: unlocked[p.Number], Milestones: []Milestone{}}
		for _, key := range p.Milestones {
			done, applies := milestoneDone(key, effective)
			phase.Milestones = append(phase.Milestones, Milestone{
				Key: key, Label: milestoneLabels[key], Done: done, Applies: applies,
			})
		}
		out = append(out, phase)
	}
	return out
}

// ListQuestEvents returns one action's history.
func (s *Service) ListQuestEvents(ctx context.Context, userID, key string) ([]domain.QuestEvent, error) {
	return s.repo.ListQuestEvents(ctx, userID, key)
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
	state, err := s.repo.ListQuestState(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("quest state: %w", err)
	}
	marks, err := s.repo.ListQuestMarks(ctx, userID)
	if err != nil {
		return qc, fmt.Errorf("quest marks: %w", err)
	}

	stale := map[string]bool{}
	for _, key := range StaleFields(profile, now) {
		stale[key] = true
	}
	cash := cashAccountIDs(pb.Accounts)

	// A tax treatment only describes invested money. Terms left on an account
	// since reclassified as something else are ignored rather than deleted, so
	// switching the role back restores them.
	invested := make(map[string]bool)
	for _, a := range pb.Accounts {
		if a.HasRole(domain.RoleInvestment) {
			invested[a.ID] = true
		}
	}
	retirement = slices.DeleteFunc(retirement, func(t domain.RetirementAccountTerms) bool {
		return !invested[t.AccountID]
	})

	return questContext{
		Now:         now,
		TaxYear:     now.Year(),
		Profile:     profile,
		Baseline:    applyProfile(ComputeBaseline(pb, now), profile, pb.Accounts, cash),
		Limits:      limits,
		Accounts:    pb.Accounts,
		Cash:        cash,
		Months:      completeMonths(pb.Months, now),
		Terms:       terms,
		Retirement:  retirement,
		Grants:      grants,
		Planned:     planned,
		Dependents:  dependents,
		Paystub:     paystub,
		StaleFields: stale,
		State:       state,
		Marks:       marks,
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
	applyMarks(quests, qc)

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
		ID:            def.Key,
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
	var completedSource string
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
		// When it became complete, not when this reading happened.
		at := qc.Now
		if s, ok := qc.State[key]; ok && s.Status == domain.QuestStatusComplete {
			at = s.ChangedAt
		}
		completedAt = &at
		completedSource = domain.QuestSourceAuto
	}

	return domain.Quest{
		ID:              key,
		CatalogKey:      key,
		Phase:           def.Phase,
		Priority:        def.Priority,
		Status:          status,
		Title:           res.Title,
		Detail:          res.Detail,
		Variant:         res.Variant,
		TargetAmount:    res.Target,
		DueDate:         res.Due,
		Verification:    def.Verification,
		Stale:           dependsOnStale(qc, def.Requires) || dependsOnStale(qc, res.Missing),
		MissingFields:   res.Missing,
		CompletedAt:     completedAt,
		CompletedSource: completedSource,
		GeneratedAt:     qc.Now,
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
	unlocked := unlockedPhases(effectiveStatuses(quests, qc))

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

// markStands reports whether the user's mark decides an action's status.
//
// A skip always does. A completion does where the app cannot check it -- an
// action verified by hand -- or could not work it out this time: "we do not
// know" is no grounds to contradict someone. For an action it can check and
// just did, the data wins, because a claim the plan can disprove is worse than
// no claim.
func markStands(q domain.Quest, m domain.QuestMark) bool {
	if m.Mark == domain.QuestStatusSkipped {
		return true
	}
	return q.Verification == domain.QuestVerificationManual ||
		q.Status == domain.QuestStatusBlocked || q.Status == domain.QuestStatusLocked
}

// applyMarks lays the user's marks over the computed statuses, and says so on
// an action the data shows was done once and has slipped since.
func applyMarks(quests []domain.Quest, qc questContext) {
	for i := range quests {
		q := &quests[i]
		if m, ok := qc.Marks[q.CatalogKey]; ok && markStands(*q, m) {
			q.Status = m.Mark
			q.CompletedAt, q.CompletedSource = nil, ""
			if m.Mark == domain.QuestStatusComplete {
				at := m.CreatedAt
				q.CompletedAt, q.CompletedSource = &at, domain.QuestSourceManual
			}
			continue
		}
		if q.Status == domain.QuestStatusAvailable && q.Verification == domain.QuestVerificationAuto &&
			qc.achieved(q.CatalogKey) {
			q.Detail = strings.TrimSpace(q.Detail + " You had this done before, and it has slipped since.")
		}
	}
}

// effectiveStatuses is the status each action counts as for gating: what this
// reading says, the user's marks wherever they stand, and every past
// achievement. Reading the marks here means a phase finished by this very
// reading opens the next one straight away.
//
// An achievement stays achieved. A phase once finished does not lock the ones
// after it again because a balance dipped: the dipped action reopens in place,
// which is the right amount of alarm.
func effectiveStatuses(quests []domain.Quest, qc questContext) map[string]string {
	effective := map[string]string{}
	for _, q := range quests {
		effective[q.CatalogKey] = q.Status
		if m, ok := qc.Marks[q.CatalogKey]; ok && markStands(q, m) {
			effective[q.CatalogKey] = m.Mark
		}
	}
	for key, s := range qc.State {
		if s.AchievedAt != nil {
			effective[key] = domain.QuestStatusComplete
		}
	}
	return effective
}

// unlockedPhases reports which phases are open. A phase opens once every
// earlier phase's milestones are met: walking in order, the first unfinished
// phase closes everything after it, and phase 1 is always open because nothing
// precedes it.
func unlockedPhases(effective map[string]string) map[int]bool {
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
	return unlocked
}

// phaseComplete reports whether every milestone action is done.
func phaseComplete(phase phaseDef, effective map[string]string) bool {
	for _, milestone := range phase.Milestones {
		if done, _ := milestoneDone(milestone, effective); !done {
			return false
		}
	}
	return true
}

// milestoneDone reports whether every instance of a milestone is done, and
// whether it produced any instance at all.
//
// A milestone that produced no instances counts as done: there was nothing to
// do. Fan-out milestones match on the key prefix, so every card has to be
// cleared before the debt milestone passes, not just one.
func milestoneDone(milestone string, effective map[string]string) (done, applies bool) {
	done = true
	for key, status := range effective {
		if key != milestone && !strings.HasPrefix(key, milestone+":") {
			continue
		}
		switch status {
		case domain.QuestStatusNotApplicable:
		case domain.QuestStatusComplete, domain.QuestStatusSkipped:
			applies = true
		default:
			applies = true
			done = false
		}
	}
	return done, applies
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
	"classify_accounts":             "Say what each of your accounts is",
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

// describeFields turns field keys into something readable in a sentence, with
// the same labels the client shows beside a blocked action (wealth_fields.go).
func describeFields(keys []string) string {
	labels := make([]string, 0, len(keys))
	for _, k := range keys {
		if l := fieldLabel(k); l != "" {
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
