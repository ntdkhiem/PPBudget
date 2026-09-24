package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// The action catalog.
//
// Lives in Go rather than the database for the same reason
// internal/service/rules_vocabulary.go does: a financial rule has to be
// versioned, diffable and unit-testable, and a user-editable tax rule is a
// liability. The database holds only INSTANCES -- what this catalog produced
// for one user, with their figures interpolated.
//
// Two properties are load-bearing throughout:
//
//	1. An action never fires on a value nobody supplied. A missing input
//	   produces a BLOCKED action naming the fields that would unlock it, not a
//	   confident recommendation built on a zero. Half the value of this engine
//	   is in what it declines to say.
//
//	2. An action with a due date ignores phase gating. Deadlines do not wait for
//	   phases: payroll deferrals close on 31 December with no do-over, the IRA
//	   deadline is the filing date, open enrollment is a two-week window, and a
//	   vest lands when the grant says so. A user still working through phase 1
//	   in November must still be told there are three paychecks left to capture
//	   the match.

// questContext is everything the catalog may read. Assembled once per
// generation so no rule issues its own queries and every action sees the same
// snapshot.
type questContext struct {
	Now      time.Time
	TaxYear  int
	Profile  *domain.WealthProfile
	Baseline Baseline
	Limits   domain.TaxLimits

	Accounts []domain.PlanningAccount
	// Cash is the set of account IDs that count as cash; see cashAccountIDs.
	// Baseline.LiquidAssets is their total, and any rule that moves money
	// between accounts reads this rather than the account type, which calls a
	// 401(k) an asset just as it does a checking account.
	Cash map[string]bool
	// Months is the per-month history behind Baseline, oldest first and with
	// the partial current month already dropped. Duration conditions read it
	// directly rather than inferring a streak from the action's own event log:
	// the spending data answers "were you under the cap in each of the last
	// three months" exactly, where an event log only says when we last agreed
	// that you were.
	Months     []domain.PlanningMonth
	Terms      map[string]domain.AccountTerms
	Retirement []domain.RetirementAccountTerms
	Grants     []domain.EquityGrant
	Planned    []domain.PlannedExpense
	Dependents []domain.Dependent
	Paystub    *domain.PaystubYTD

	// StaleFields marks answers past their confirmation cadence. Actions built
	// on them still render, flagged.
	StaleFields map[string]bool
	// ExistingStatus is the stored status per catalog key, so phase gating can
	// see what the user has already finished.
	ExistingStatus map[string]string
}

// questResult is one produced action.
type questResult struct {
	// KeySuffix distinguishes instances of a fan-out action, e.g. one
	// clear_high_apr_balance per card. Empty for singletons.
	KeySuffix string

	// NotApplicable removes the action entirely: a renter has no PMI to remove,
	// someone without an HDHP has no HSA to max. Distinct from Blocked, which
	// means "this may well apply, we just cannot tell yet".
	NotApplicable bool
	// Complete means the condition is already satisfied by current data.
	Complete bool

	Title  string
	Detail string
	Target *money.Money
	Due    *time.Time
	// Missing names profile fields whose answers would unblock this action.
	Missing []string
	// SuppressWhenCashFlowNegative marks actions that assume spare money. When
	// essentials exceed income, recommending them is worse than useless.
	SuppressWhenCashFlowNegative bool
}

// questDef is one catalog entry.
type questDef struct {
	Key   string
	Phase int
	// Priority orders within a phase; lower runs first.
	Priority     int
	Verification string
	// Requires lists profile fields the action cannot be evaluated without. A
	// missing one blocks the action before Evaluate runs, so no rule has to
	// re-check the same preconditions.
	Requires []string
	// AlsoMissing names further answers the action needs, for preconditions no
	// single field can express -- gross pay, for instance, comes either from the
	// profile or from an annualised paystub, so neither key belongs in Requires.
	//
	// These join the blocked card's question list so the user is asked for
	// everything at once rather than discovering a second requirement after
	// satisfying the first.
	AlsoMissing func(questContext) []string
	// Evaluate returns zero or more instances. Returning none means the action
	// does not apply to this user at all.
	Evaluate func(questContext) []questResult
}

// Phases. Phase 0 is cross-cutting: actions that belong to no stage of the
// journey because they protect it rather than advance it.
const (
	PhaseCrossCutting  = 0
	PhaseLiquidity     = 1
	PhaseEquity        = 2
	PhaseTaxAdvantaged = 3
	PhaseAllocation    = 4
	PhaseIndependence  = 5
)

// phaseDef names a phase and the actions whose completion unlocks the next one.
type phaseDef struct {
	Number int
	Name   string
	// Milestones are catalog keys. The phase is complete when every instance of
	// each is complete, skipped, or not applicable. A milestone that produced
	// no instances is satisfied: there was nothing to do.
	Milestones []string
}

var phaseDefs = []phaseDef{
	{PhaseCrossCutting, "Protection", nil},
	{PhaseLiquidity, "Liquidity and high-cost debt", []string{
		"capture_employer_match",
		"starter_emergency_fund",
		"clear_high_apr_balance",
		"full_emergency_fund",
	}},
	{PhaseEquity, "Equity and concentration risk", []string{
		"establish_sell_on_vest",
		"reserve_equity_tax_gap",
	}},
	{PhaseTaxAdvantaged, "Tax-advantaged saving", []string{
		"raise_401k_deferral",
		"max_hsa",
		"resolve_roth_route",
	}},
	{PhaseAllocation, "Investment allocation", []string{
		"reduce_employer_concentration",
		"automate_surplus_sweep",
	}},
	{PhaseIndependence, "Debt elimination and independence", []string{
		"reach_fi_number",
	}},
}

// PhaseName returns a phase's display name.
func PhaseName(n int) string {
	for _, p := range phaseDefs {
		if p.Number == n {
			return p.Name
		}
	}
	return ""
}

// catalog is every action, assembled from the per-phase files.
func catalog() []questDef {
	var defs []questDef
	defs = append(defs, crossCuttingQuests()...)
	defs = append(defs, liquidityQuests()...)
	defs = append(defs, equityQuests()...)
	defs = append(defs, taxAdvantagedQuests()...)
	defs = append(defs, allocationQuests()...)
	defs = append(defs, independenceQuests()...)
	return defs
}

// ---------------------------------------------------------------- helpers

// highAPRFloor is the behavioural floor under the relative debt threshold.
//
// The cut line is "the rate exceeds the expected after-tax portfolio return",
// not a fixed number -- but debt repayment is a CERTAIN return and market
// returns are not, so a rate somewhat below the expected return is still worth
// clearing first. Without a floor, a user with an optimistic return assumption
// would be told to carry an 8% balance indefinitely.
const highAPRFloor = 0.05

// afterTaxDrag approximates what long-term capital gains take out of a taxable
// return, so debt rates and investment returns compare like for like.
const afterTaxDrag = 0.85

// highAPRThreshold is the rate above which clearing a balance outranks
// investing. Falls back to a conventional figure when the user has no return
// assumption, rather than blocking a phase 1 action on a phase 4 input.
func highAPRThreshold(p *domain.WealthProfile) float64 {
	expected := 0.07
	if p.ExpectedReturnAPR != nil && *p.ExpectedReturnAPR > 0 {
		expected = *p.ExpectedReturnAPR
	}
	t := expected * afterTaxDrag
	if t < highAPRFloor {
		return highAPRFloor
	}
	return t
}

// usd formats whole dollars for prose. Cents are noise in a sentence like
// "clear the $3,824 balance".
func usd(m money.Money) string {
	v := int64(m)
	neg := v < 0
	if neg {
		v = -v
	}
	dollars := (v + 50) / 100 // round to nearest dollar

	s := fmt.Sprintf("%d", dollars)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)

	out := "$" + strings.Join(parts, ",")
	if neg {
		return "-" + out
	}
	return out
}

// pct formats a fraction as a percentage for prose.
func pct(f float64, digits int) string {
	return strconv.FormatFloat(f*100, 'f', digits, 64) + "%"
}

// endOfYear is the deadline for anything that runs through payroll: a missed
// 401(k) or payroll-HSA contribution has no do-over in January.
func endOfYear(now time.Time) time.Time {
	return time.Date(now.Year(), time.December, 31, 0, 0, 0, 0, time.UTC)
}

// accountByID indexes the balance snapshot the generator was handed.
func (c questContext) accountByID(id string) *domain.PlanningAccount {
	for i := range c.Accounts {
		if c.Accounts[i].ID == id {
			return &c.Accounts[i]
		}
	}
	return nil
}

// plannedWithin totals earmarked cash falling due inside a horizon.
//
// Every sweep and invest rule subtracts this first. Without it the engine sees
// idle cash and tells someone with a closing date next quarter to move their
// down payment into a broad-market index fund.
func (c questContext) plannedWithin(months int) money.Money {
	cutoff := c.Now.AddDate(0, months, 0)
	var total money.Money
	for _, e := range c.Planned {
		if e.TargetDate.Before(cutoff) {
			total += e.Amount
		}
	}
	return total
}

// grossAnnual resolves annual pay, preferring the stated figure and falling
// back to annualising a paystub.
func (c questContext) grossAnnual() (money.Money, bool) {
	if c.Profile.GrossAnnualIncome != nil && *c.Profile.GrossAnnualIncome > 0 {
		return *c.Profile.GrossAnnualIncome, true
	}
	if c.Paystub != nil && c.Paystub.Gross > 0 &&
		c.Paystub.PaychecksYTD != nil && *c.Paystub.PaychecksYTD > 0 &&
		c.Paystub.PaychecksPerYear != nil && *c.Paystub.PaychecksPerYear > 0 {
		perCheck := float64(c.Paystub.Gross) / float64(*c.Paystub.PaychecksYTD)
		return money.Money(perCheck * float64(*c.Paystub.PaychecksPerYear)), true
	}
	return 0, false
}

// paychecksRemaining is what turns "raise your deferral" into "you have three
// paychecks left to capture $4,000".
func (c questContext) paychecksRemaining() (int, bool) {
	if c.Paystub == nil || c.Paystub.PaychecksYTD == nil || c.Paystub.PaychecksPerYear == nil {
		return 0, false
	}
	left := *c.Paystub.PaychecksPerYear - *c.Paystub.PaychecksYTD
	if left < 0 {
		return 0, true
	}
	return left, true
}

// needsGrossPay reports gross pay as missing only when NEITHER the profile
// field nor a paystub can supply it.
func needsGrossPay(c questContext) []string {
	if _, ok := c.grossAnnual(); ok {
		return nil
	}
	return []string{"gross_annual_income"}
}

// householdMAGI approximates modified AGI for eligibility tests.
//
// Deliberately crude and deliberately inclusive of spouse income: the Roth
// decision it feeds is one where an under-estimate costs the user a penalty,
// so the error should lean high.
func (c questContext) householdMAGI() (money.Money, bool) {
	gross, ok := c.grossAnnual()
	if !ok {
		return 0, false
	}
	return gross, true
}
