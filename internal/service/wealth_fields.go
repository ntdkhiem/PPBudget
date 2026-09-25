package service

import (
	"sort"
	"time"
)

// Profile question specifications.
//
// Everything a screen needs to ask a question -- its label, what kind of input
// it takes, the choices it offers, how long an answer stays trustworthy -- in
// one table, served to the client by GET /wealth/fields. The client used to
// infer a field's type from the shape of its key, which turned a list of
// account ids into a dollar box, and kept its own copies of the choices; now it
// asks. The repository's registry in wealth_fields.go maps the same keys to
// columns, and a test holds the two to the same key set.

// Field kinds. Money is integer cents; percent is a fraction (0.06 for 6%).
const (
	FieldKindMoney   = "money"
	FieldKindPercent = "percent"
	FieldKindInteger = "integer"
	FieldKindBoolean = "boolean"
	FieldKindDate    = "date"
	FieldKindText    = "text"
	FieldKindChoice  = "choice"
)

// FieldChoice is one allowed value of a choice field. The values are the ones
// the schema's CHECK constraints accept.
type FieldChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ProfileFieldInfo is one question as the client receives it.
type ProfileFieldInfo struct {
	Key     string        `json:"key"`
	Label   string        `json:"label"`
	Kind    string        `json:"kind"`
	Choices []FieldChoice `json:"choices,omitempty"`
	// Correction marks an override of a derived figure rather than a question:
	// having none is normal, so it is never listed as unanswered.
	Correction bool `json:"correction,omitempty"`
}

type fieldSpec struct {
	Label      string
	Kind       string
	Choices    []FieldChoice
	Correction bool
	// StaleAfter follows how fast the fact moves: pay and benefits on an
	// annual cycle, balances within months. Zero never goes stale -- a date of
	// birth does not change.
	StaleAfter time.Duration
}

const (
	yearly    = 365 * 24 * time.Hour
	quarterly = 90 * 24 * time.Hour
)

var profileFields = map[string]fieldSpec{
	// Household and horizon.
	"date_of_birth": {Label: "your date of birth", Kind: FieldKindDate},
	"marital_status": {Label: "whether you are married", Kind: FieldKindChoice, Choices: []FieldChoice{
		{"single", "Single"}, {"married", "Married"}, {"domestic_partner", "Domestic partner"},
	}},
	"filing_status": {Label: "your tax filing status", Kind: FieldKindChoice, StaleAfter: yearly, Choices: []FieldChoice{
		{"single", "Single"}, {"mfj", "Married, jointly"}, {"mfs", "Married, separately"},
		{"hoh", "Head of household"}, {"qss", "Surviving spouse"},
	}},
	"resident_state":            {Label: "the state you file in", Kind: FieldKindText},
	"spouse_gross_annual":       {Label: "your spouse's income", Kind: FieldKindMoney, StaleAfter: yearly},
	"spouse_has_workplace_plan": {Label: "whether your spouse has a workplace retirement plan", Kind: FieldKindBoolean},
	"target_independence_age":   {Label: "when you want work to become optional", Kind: FieldKindInteger},

	// Pay and the workplace plan.
	"gross_annual_income":    {Label: "your gross pay", Kind: FieldKindMoney, StaleAfter: yearly},
	"deferral_pct":           {Label: "your current 401(k) contribution rate", Kind: FieldKindPercent, StaleAfter: yearly},
	"deferral_roth_share":    {Label: "how much of your 401(k) contribution goes to Roth", Kind: FieldKindPercent, StaleAfter: yearly},
	"match_pct":              {Label: "your employer's match rate", Kind: FieldKindPercent, StaleAfter: yearly},
	"match_limit_pct":        {Label: "how much of your salary the match covers", Kind: FieldKindPercent, StaleAfter: yearly},
	"match_per_paycheck":     {Label: "whether your employer matches each paycheck", Kind: FieldKindBoolean},
	"match_has_true_up":      {Label: "whether your plan trues up the match at year end", Kind: FieldKindBoolean},
	"plan_allows_after_tax":  {Label: "whether your plan takes after-tax contributions", Kind: FieldKindBoolean},
	"plan_allows_in_service": {Label: "whether your plan allows in-service withdrawals", Kind: FieldKindBoolean},
	"plan_accepts_rollovers": {Label: "whether your plan accepts incoming rollovers", Kind: FieldKindBoolean},

	// HSA.
	"hdhp_enrolled": {Label: "whether you are on a high-deductible health plan", Kind: FieldKindBoolean, StaleAfter: yearly},
	"hsa_coverage_tier": {Label: "whether your HSA covers you or your family", Kind: FieldKindChoice, StaleAfter: yearly,
		Choices: []FieldChoice{{"self_only", "Just me"}, {"family", "Family"}}},
	"hsa_employer_contribution": {Label: "what your employer puts into your HSA", Kind: FieldKindMoney, StaleAfter: yearly},

	// Tax.
	"prior_year_tax_liability": {Label: "last year's total tax", Kind: FieldKindMoney, StaleAfter: yearly},
	"prior_year_agi":           {Label: "last year's adjusted gross income", Kind: FieldKindMoney, StaleAfter: yearly},
	"annual_bonus":             {Label: "your yearly bonus", Kind: FieldKindMoney, StaleAfter: yearly},
	"bonus_month":              {Label: "the month your bonus is paid", Kind: FieldKindInteger},

	// Targets and corrections.
	"emergency_fund_target_months": {Label: "how many months of cover you want", Kind: FieldKindInteger},
	"strategy": {Label: "how hard you want to save", Kind: FieldKindChoice, Choices: []FieldChoice{
		{"aggressive", "Aggressive"}, {"balanced", "Balanced"}, {"flexible", "Flexible"},
	}},
	"override_monthly_income":     {Label: "your corrected monthly income", Kind: FieldKindMoney, Correction: true},
	"override_essential_expenses": {Label: "your corrected monthly essentials", Kind: FieldKindMoney, Correction: true},

	// Holdings.
	"traditional_ira_balance": {Label: "your traditional IRA balance", Kind: FieldKindMoney, StaleAfter: quarterly},
	"taxable_brokerage_value": {Label: "your taxable brokerage balance", Kind: FieldKindMoney, StaleAfter: quarterly},
	"taxable_unrealized_gain": {Label: "how much of that is unrealised gain", Kind: FieldKindMoney, StaleAfter: quarterly},
	"taxable_employer_stock":  {Label: "how much employer stock you hold", Kind: FieldKindMoney, StaleAfter: quarterly},

	// Employer equity.
	"employer_is_public": {Label: "whether your employer is publicly traded", Kind: FieldKindBoolean},
	"employer_ticker":    {Label: "your employer's ticker", Kind: FieldKindText},
	"has_stock_options":  {Label: "whether you hold stock options", Kind: FieldKindBoolean},

	// Risk transfer.
	"ltd_replacement_pct": {Label: "what share of income your disability cover replaces", Kind: FieldKindPercent, StaleAfter: yearly},
	"ltd_monthly_cap":     {Label: "your disability benefit cap", Kind: FieldKindMoney, StaleAfter: yearly},
	"ltd_premium_pretax":  {Label: "whether your disability premium is paid pre-tax", Kind: FieldKindBoolean},
	"life_death_benefit":  {Label: "your life insurance cover", Kind: FieldKindMoney, StaleAfter: yearly},

	// Housing and debt.
	"housing_tenure": {Label: "whether you rent or own", Kind: FieldKindChoice,
		Choices: []FieldChoice{{"rent", "Rent"}, {"own", "Own"}}},
	"mortgage_apr":     {Label: "your mortgage rate", Kind: FieldKindPercent},
	"mortgage_balance": {Label: "your mortgage balance", Kind: FieldKindMoney, StaleAfter: quarterly},
	"pays_pmi":         {Label: "whether you pay mortgage insurance", Kind: FieldKindBoolean},
	"student_loan_kind": {Label: "whether your student loans are federal or private", Kind: FieldKindChoice, Choices: []FieldChoice{
		{"none", "None"}, {"federal", "Federal"}, {"private", "Private"}, {"both", "Both"},
	}},
	"student_loan_idr":  {Label: "whether you are on an income-driven repayment plan", Kind: FieldKindBoolean},
	"student_loan_pslf": {Label: "whether you are pursuing loan forgiveness", Kind: FieldKindBoolean},

	// Projection assumptions.
	"expected_return_apr": {Label: "the return you expect on investments", Kind: FieldKindPercent},
	"inflation_apr":       {Label: "the inflation you expect", Kind: FieldKindPercent},
	"withdrawal_rate":     {Label: "the share of savings you plan to draw each year", Kind: FieldKindPercent},
	"target_annual_spend": {Label: "what you plan to spend a year in retirement", Kind: FieldKindMoney},
}

// pseudoFieldLabels name data the app is missing rather than a question it can
// ask. They appear in an action's missing fields but are not profile fields,
// which is how the client tells the two apart.
var pseudoFieldLabels = map[string]string{
	"transaction_history": "enough categorised spending to work from",
	"account_apr":         "the interest rate on each balance",
	"account_roles":       "what each of your accounts is",
	"paystub_ytd":         "the year-to-date figures from a recent paystub",
	"espp_terms":          "your ESPP discount, contribution rate and plan maximum",
}

// fieldLabel names a field or pseudo-field for a sentence, or "" if unknown.
func fieldLabel(key string) string {
	if spec, ok := profileFields[key]; ok {
		return spec.Label
	}
	return pseudoFieldLabels[key]
}

// ProfileFields lists every question, in a stable order, for the client.
func (s *Service) ProfileFields() []ProfileFieldInfo {
	out := make([]ProfileFieldInfo, 0, len(profileFields))
	for key, spec := range profileFields {
		out = append(out, ProfileFieldInfo{
			Key: key, Label: spec.Label, Kind: spec.Kind, Choices: spec.Choices, Correction: spec.Correction,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// FieldLabels maps every field and pseudo-field to its label, so the unlock
// prompt beside a blocked action names things the same way the engine's own
// detail text does.
func (s *Service) FieldLabels() map[string]string {
	out := make(map[string]string, len(profileFields)+len(pseudoFieldLabels))
	for key, spec := range profileFields {
		out[key] = spec.Label
	}
	for key, label := range pseudoFieldLabels {
		out[key] = label
	}
	return out
}
