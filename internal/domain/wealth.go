package domain

import (
	"time"

	"ntdkhiem/ppbudget-go/pkg/money"
)

// Wealth Strategy domain types.
//
// Kept out of domain.go because that file holds the transaction-ledger core;
// these are the planning layer that reads it.
//
// POINTERS EVERYWHERE, DELIBERATELY. Almost every field here is a pointer
// because nil means "the user has not answered this yet", and intake is
// progressive -- an eight-screen core produces a real action list, and the rest
// is asked later against the action each answer unlocks. A zero value cannot
// carry that meaning: a nil EmployerMatchPct means "we do not know whether
// there is a match" while 0 means "there is no match", and the generator owes
// the user a different action in each case. Collapsing the two would let rules
// fire on numbers nobody supplied.

// ProfileFieldSource records where a profile answer came from.
//
// The distinction is load-bearing rather than cosmetic: high-stakes branches
// require an answer the user actually saw. The backdoor Roth decision must not
// fire off a MAGI the app inferred on its own.
const (
	// FieldSourceDerived: computed from the user's own data, never shown or
	// confirmed. Usable for display and for low-stakes rules.
	FieldSourceDerived = "derived"
	// FieldSourceConfirmed: derived, then shown to the user, who accepted it.
	FieldSourceConfirmed = "confirmed"
	// FieldSourceEntered: typed by the user.
	FieldSourceEntered = "entered"
	// FieldSourceDefault: a statutory or conventional default the app applied
	// (the 22% supplemental rate, a 6-month emergency fund target).
	FieldSourceDefault = "default"
)

// ProfileField is the provenance and age of a single profile answer.
// The absence of a row for a field key means it has never been answered, which
// is how progressive intake knows what to ask next.
type ProfileField struct {
	FieldKey   string    `json:"field_key"`
	Source     string    `json:"source"`
	AnsweredAt time.Time `json:"answered_at"`
}

// WealthProfile is the typed replacement for the 'financial_plan' and
// 'retirement_plan' JSON blobs in user_settings.
type WealthProfile struct {
	UserID string `json:"user_id"`

	// Household and horizon.
	DateOfBirth            *time.Time   `json:"date_of_birth,omitempty"`
	MaritalStatus          *string      `json:"marital_status,omitempty"`
	FilingStatus           *string      `json:"filing_status,omitempty"`
	ResidentState          *string      `json:"resident_state,omitempty"`
	SpouseGrossAnnual      *money.Money `json:"spouse_gross_annual,omitempty"`
	SpouseHasWorkplacePlan *bool        `json:"spouse_has_workplace_plan,omitempty"`
	TargetIndependenceAge  *int         `json:"target_independence_age,omitempty"`

	// Income. The paystub table is richer, but the employer-match rule is a
	// share of salary and needs one annual figure.
	GrossAnnualIncome *money.Money `json:"gross_annual_income,omitempty"`

	// Employer plan mechanics.
	DeferralPct          *float64 `json:"deferral_pct,omitempty"`
	DeferralRothShare    *float64 `json:"deferral_roth_share,omitempty"`
	MatchPct             *float64 `json:"match_pct,omitempty"`
	MatchLimitPct        *float64 `json:"match_limit_pct,omitempty"`
	MatchPerPaycheck     *bool    `json:"match_per_paycheck,omitempty"`
	MatchHasTrueUp       *bool    `json:"match_has_true_up,omitempty"`
	PlanAllowsAfterTax   *bool    `json:"plan_allows_after_tax,omitempty"`
	PlanAllowsInService  *bool    `json:"plan_allows_in_service,omitempty"`
	PlanAcceptsRollovers *bool    `json:"plan_accepts_rollovers,omitempty"`

	// HSA.
	HDHPEnrolled            *bool        `json:"hdhp_enrolled,omitempty"`
	HSACoverageTier         *string      `json:"hsa_coverage_tier,omitempty"`
	HSAEmployerContribution *money.Money `json:"hsa_employer_contribution,omitempty"`

	// Tax.
	PriorYearTaxLiability *money.Money `json:"prior_year_tax_liability,omitempty"`
	PriorYearAGI          *money.Money `json:"prior_year_agi,omitempty"`
	AnnualBonus           *money.Money `json:"annual_bonus,omitempty"`
	BonusMonth            *int         `json:"bonus_month,omitempty"`

	// Targets. Months, not dollars -- the dollar figure is an output.
	EmergencyFundTargetMonths *int `json:"emergency_fund_target_months,omitempty"`

	// Strategy is how hard the user wants to save, which sets the target rate
	// every "saving X against a target of Y" comparison measures against.
	Strategy *string `json:"strategy,omitempty"`

	// Overrides: the user telling us our arithmetic is wrong about them --
	// lumpy income, or "needs" spending that is really discretionary. Held
	// apart from the derived figure rather than replacing it, so the engine can
	// still say which numbers the user has corrected. nil means no override,
	// which is not an override of zero. Which accounts count as cash is not an
	// override: it is each account's role.
	OverrideMonthlyIncome     *money.Money `json:"override_monthly_income,omitempty"`
	OverrideEssentialExpenses *money.Money `json:"override_essential_expenses,omitempty"`

	// Holdings.
	TraditionalIRABalance *money.Money `json:"traditional_ira_balance,omitempty"`
	TaxableBrokerageValue *money.Money `json:"taxable_brokerage_value,omitempty"`
	TaxableUnrealizedGain *money.Money `json:"taxable_unrealized_gain,omitempty"`
	TaxableEmployerStock  *money.Money `json:"taxable_employer_stock,omitempty"`

	// Employer equity context. EmployerIsPublic gates the whole equity phase.
	EmployerIsPublic *bool   `json:"employer_is_public,omitempty"`
	EmployerTicker   *string `json:"employer_ticker,omitempty"`
	HasStockOptions  *bool   `json:"has_stock_options,omitempty"`

	// Risk transfer.
	LTDReplacementPct *float64     `json:"ltd_replacement_pct,omitempty"`
	LTDMonthlyCap     *money.Money `json:"ltd_monthly_cap,omitempty"`
	LTDPremiumPretax  *bool        `json:"ltd_premium_pretax,omitempty"`
	LifeDeathBenefit  *money.Money `json:"life_death_benefit,omitempty"`

	// Housing and debt.
	HousingTenure   *string      `json:"housing_tenure,omitempty"`
	MortgageAPR     *float64     `json:"mortgage_apr,omitempty"`
	MortgageBalance *money.Money `json:"mortgage_balance,omitempty"`
	PaysPMI         *bool        `json:"pays_pmi,omitempty"`
	StudentLoanKind *string      `json:"student_loan_kind,omitempty"`
	StudentLoanIDR  *bool        `json:"student_loan_idr,omitempty"`
	StudentLoanPSLF *bool        `json:"student_loan_pslf,omitempty"`

	// Projection assumptions, migrated from the retirement_plan blob.
	ExpectedReturnAPR *float64     `json:"expected_return_apr,omitempty"`
	InflationAPR      *float64     `json:"inflation_apr,omitempty"`
	WithdrawalRate    *float64     `json:"withdrawal_rate,omitempty"`
	TargetAnnualSpend *money.Money `json:"target_annual_spend,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Fields carries provenance for every answered field, keyed by field key.
	// Populated on read; ignored on write (the repository derives it from what
	// the caller actually sets).
	Fields map[string]ProfileField `json:"fields,omitempty"`
}

// AccountTerms holds the per-account facts an aggregator does not return.
type AccountTerms struct {
	AccountID string   `json:"account_id"`
	APY       *float64 `json:"apy,omitempty"`
	APR       *float64 `json:"apr,omitempty"`
	// A 0% promotional balance is excluded from the ordinary payoff ordering
	// and handled by a dated action instead: deferred-interest cards charge all
	// accrued interest retroactively if any balance survives the expiry.
	PromoAPR         *float64     `json:"promo_apr,omitempty"`
	PromoExpiresOn   *time.Time   `json:"promo_expires_on,omitempty"`
	StatementBalance *money.Money `json:"statement_balance,omitempty"`
	MinPayment       *money.Money `json:"min_payment,omitempty"`
	DueDay           *int         `json:"due_day,omitempty"`
}

// Retirement account kinds. Mirrors AccountKind in retirement-math.ts.
const (
	RetirementKind401k     = "401k"
	RetirementKindRoth401k = "roth_401k"
	RetirementKindIRA      = "ira"
	RetirementKindRothIRA  = "roth_ira"
	RetirementKindHSA      = "hsa"
	RetirementKindTaxable  = "taxable"
)

// RetirementAccountTerms maps a real PPBudget account to a tax treatment, so
// balances are never hand-typed.
//
// Carries no annual limit: those resolve from TaxLimits by year. A
// user-maintained limit is wrong every January.
type RetirementAccountTerms struct {
	AccountID           string      `json:"account_id"`
	Kind                string      `json:"kind"`
	MonthlyContribution money.Money `json:"monthly_contribution"`
}

// Dependent is stored by birth year rather than age so the row does not rot.
type Dependent struct {
	ID        string `json:"id"`
	BirthYear int    `json:"birth_year"`
	Label     string `json:"label,omitempty"`
}

// PlannedExpense is near-term earmarked cash, carved out of every sweep and
// invest rule. Without it the engine sees idle cash and tells someone with a
// closing date next quarter to put their down payment in an index fund.
type PlannedExpense struct {
	ID         string      `json:"id"`
	Label      string      `json:"label"`
	Amount     money.Money `json:"amount"`
	TargetDate time.Time   `json:"target_date"`
}

// Goal is a user-authored savings target.
//
// The only content in this feature the app cannot re-derive: a name, an amount
// and a date that came from the person, not from their transactions. Goals are
// funded sequentially rather than in parallel, so each one has an honest start
// date instead of every one having an optimistic finish date.
type Goal struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	TargetAmount    money.Money `json:"target_amount"`
	TargetDate      *time.Time  `json:"target_date,omitempty"`
	LinkedAccountID *string     `json:"linked_account_id,omitempty"`
	// CurrentAmount is used only when no account is linked; otherwise progress
	// comes from the real balance rather than a number kept by hand.
	CurrentAmount money.Money `json:"current_amount"`
	Priority      int         `json:"priority"`
}

// PaystubYTD is year-to-date payroll as printed on a stub.
//
// Replaces three questions that asked the user to recall per-paycheck figures.
// Rows are dated rather than overwritten so a mid-year deferral change shows up
// as a delta instead of silently mis-pacing the rest of the year.
//
// FICA is absent on purpose: it is mechanical, so it is computed from Gross and
// the wage base rather than asked for.
type PaystubYTD struct {
	ID               string      `json:"id"`
	AsOfDate         time.Time   `json:"as_of_date"`
	Gross            money.Money `json:"gross"`
	FederalWithheld  money.Money `json:"federal_withheld"`
	StateWithheld    money.Money `json:"state_withheld"`
	Pretax401k       money.Money `json:"pretax_401k"`
	Roth401k         money.Money `json:"roth_401k"`
	HSAContribution  money.Money `json:"hsa_contribution"`
	ESPPContribution money.Money `json:"espp_contribution"`
	PretaxBenefits   money.Money `json:"pretax_benefits"`
	// Tracked separately because federal supplemental withholding steps from
	// 22% to 37% once aggregate supplemental wages cross the threshold. That
	// crossover is a dated event for an equity-compensated earner.
	SupplementalWages money.Money `json:"supplemental_wages"`
	PaychecksYTD      *int        `json:"paychecks_ytd,omitempty"`
	PaychecksPerYear  *int        `json:"paychecks_per_year,omitempty"`
}

// Equity instrument kinds.
const (
	EquityKindRSU  = "rsu"
	EquityKindESPP = "espp"
	EquityKindISO  = "iso"
	EquityKindNSO  = "nso"
)

// EquityGrant is one grant of employer equity.
//
// GrantDate, TotalShares, SharesVested and NextVestDate together are the
// ANCHOR. A vesting frequency alone cannot produce a date: it says how often
// shares vest, not where in the schedule the user currently stands.
type EquityGrant struct {
	ID    string  `json:"id"`
	Kind  string  `json:"kind"`
	Label *string `json:"label,omitempty"`

	GrantDate     *time.Time `json:"grant_date,omitempty"`
	TotalShares   *float64   `json:"total_shares,omitempty"`
	SharesVested  *float64   `json:"shares_vested,omitempty"`
	NextVestDate  *time.Time `json:"next_vest_date,omitempty"`
	VestFrequency *string    `json:"vest_frequency,omitempty"`
	VestShare     *float64   `json:"vest_share,omitempty"`

	ESPPDiscountPct     *float64   `json:"espp_discount_pct,omitempty"`
	ESPPHasLookback     *bool      `json:"espp_has_lookback,omitempty"`
	ESPPContributionPct *float64   `json:"espp_contribution_pct,omitempty"`
	ESPPPlanMaxPct      *float64   `json:"espp_plan_max_pct,omitempty"`
	ESPPOfferingStart   *time.Time `json:"espp_offering_start,omitempty"`
	ESPPPurchaseDate    *time.Time `json:"espp_purchase_date,omitempty"`

	StrikePrice    *money.Money `json:"strike_price,omitempty"`
	ExpirationDate *time.Time   `json:"expiration_date,omitempty"`

	// "Sell within 48 hours of vest" is not executable during a closed window.
	HasRule10b51   *bool   `json:"has_10b5_1,omitempty"`
	BlackoutPolicy *string `json:"blackout_policy,omitempty"`

	// nil means "use the statutory default", not zero.
	SupplementalWithholdingPct *float64 `json:"supplemental_withholding_pct,omitempty"`
}

// Quest statuses.
const (
	QuestStatusLocked        = "locked"
	QuestStatusBlocked       = "blocked"
	QuestStatusAvailable     = "available"
	QuestStatusComplete      = "complete"
	QuestStatusSkipped       = "skipped"
	QuestStatusNotApplicable = "not_applicable"
)

// Quest variants.
const (
	// QuestVariantOverLimit: contributions already exceed the year's ceiling,
	// and the action is to withdraw the excess.
	QuestVariantOverLimit = "over_limit"
)

// Quest verification methods: whether the app can check an action against the
// user's own data (auto), or only take their word for it (manual).
const (
	QuestVerificationAuto   = "auto"
	QuestVerificationManual = "manual"
)

// Quest event kinds and sources.
const (
	QuestEventGenerated   = "generated"
	QuestEventActivated   = "activated"
	QuestEventCompleted   = "completed"
	QuestEventUncompleted = "uncompleted"
	QuestEventSkipped     = "skipped"
	QuestEventUnskipped   = "unskipped"

	QuestSourceAuto   = "auto"
	QuestSourceManual = "manual"
	QuestSourceSystem = "system"
)

// Quest is one generated action for one user.
//
// The catalog that produces these lives in Go, not the database -- a financial
// rule has to be versioned, diffable and unit-testable, and a user-editable tax
// rule is a liability. Actions are worked out on every read and not stored;
// only what the user said about them (QuestMark) and what the engine last saw
// (QuestState) are.
type Quest struct {
	// ID is the catalog key: an action is identified by what it is, not by a
	// row, since there is no row.
	ID         string `json:"id"`
	CatalogKey string `json:"catalog_key"`
	Phase      int    `json:"phase"`
	Priority   int    `json:"priority"`
	Status     string `json:"status"`

	// Interpolated at generation, so rendering never re-runs the engine.
	Title  string `json:"title"`
	Detail string `json:"detail,omitempty"`
	// Variant names which of an action's outcomes this is, where one action
	// can say materially different things -- an HSA that still has room, or one
	// that is over the limit. Pages branch on this, never on the prose.
	Variant string `json:"variant,omitempty"`

	TargetAmount *money.Money `json:"target_amount,omitempty"`
	// An action carrying a due date surfaces even while its phase is locked.
	// Deadlines do not wait for phases: payroll deferrals close on 31 December,
	// open enrollment lasts two weeks, and a vest lands when the grant says so.
	DueDate *time.Time `json:"due_date,omitempty"`

	Verification string `json:"verification"`
	// Set when a computed input is past its confirmation cadence. The action
	// still renders, flagged -- a plan built on stale figures should be
	// visible, not silently absent.
	Stale bool `json:"stale"`
	// Field keys whose answers would move this action out of 'blocked'. Lets
	// the UI say "answer 2 questions to unlock" beside the action's value.
	MissingFields []string `json:"missing_fields"`

	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// CompletedSource says whether the user claimed this or the engine
	// concluded it. A manual claim stands where the app cannot check it; a
	// computed completion has to track the data it came from.
	CompletedSource string    `json:"completed_source,omitempty"`
	GeneratedAt     time.Time `json:"generated_at"`
}

// QuestMark is what the user said about an action: done, or not for them.
// Kept by catalog key, so it outlives any one reading of the list.
type QuestMark struct {
	Mark      string // QuestStatusComplete or QuestStatusSkipped
	Note      string
	CreatedAt time.Time
}

// QuestState is the last status the engine worked out for an action. It is
// written only when that status changes, which is what lets a read of the
// list write nothing on an ordinary page load.
type QuestState struct {
	Status string
	// AchievedAt is the first time the action was complete, and is never
	// cleared. Phase gating reads it: a finished phase stays finished.
	AchievedAt *time.Time
	ChangedAt  time.Time
}

// QuestEvent is one entry in the append-only history behind an action.
//
// Separate from Quest.Status because duration conditions are queries over
// history rather than facts about the present. Keyed by catalog key rather
// than by a row, so an action the catalog stops producing keeps its history.
type QuestEvent struct {
	ID         string    `json:"id"`
	CatalogKey string    `json:"catalog_key"`
	Event      string    `json:"event"`
	Source     string    `json:"source"`
	Note       string    `json:"note,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// TaxLimits is one tax year's statutory figures, resolved from the tax_limits
// table rather than hardcoded. Amounts are cents; rates are fractions.
//
// Every rule reads limits through this. The companion discipline is that the
// app stores the user's year-to-date contributions and computes headroom at
// evaluation time -- it never stores "their limit", which goes stale on 1
// January.
type TaxLimits struct {
	TaxYear int                    `json:"tax_year"`
	Amounts map[string]money.Money `json:"amounts"`
	Rates   map[string]float64     `json:"rates"`
}

// Amount returns a cents figure and whether the year defines it.
func (t TaxLimits) Amount(key string) (money.Money, bool) {
	v, ok := t.Amounts[key]
	return v, ok
}

// Rate returns a fractional rate and whether the year defines it.
func (t TaxLimits) Rate(key string) (float64, bool) {
	v, ok := t.Rates[key]
	return v, ok
}

// Limit keys. Referenced by the catalog so a typo is a compile error rather
// than a silently missing limit at evaluation time.
const (
	LimitElectiveDeferral402g   = "elective_deferral_402g"
	LimitCatchup50Plus          = "catchup_50_plus"
	LimitCatchup60To63          = "catchup_60_to_63"
	LimitTotalAdditions415c     = "total_additions_415c"
	LimitIRAContribution        = "ira_contribution"
	LimitIRACatchup50Plus       = "ira_catchup_50_plus"
	LimitHSASelfOnly            = "hsa_self_only"
	LimitHSAFamily              = "hsa_family"
	LimitHSACatchup55Plus       = "hsa_catchup_55_plus"
	LimitSocialSecurityWageBase = "social_security_wage_base"
	LimitRothMAGISingleStart    = "roth_magi_phaseout_single_start"
	LimitRothMAGISingleEnd      = "roth_magi_phaseout_single_end"
	LimitRothMAGIMFJStart       = "roth_magi_phaseout_mfj_start"
	LimitRothMAGIMFJEnd         = "roth_magi_phaseout_mfj_end"
	LimitSafeHarborAGIThreshold = "safe_harbor_agi_threshold"
	LimitESPP423AnnualCap       = "espp_423_annual_cap"
	LimitSupplementalThreshold  = "supplemental_wage_threshold"
	LimitDependentCareFSACap    = "dependent_care_fsa_cap"
	LimitAddlMedicareSingle     = "additional_medicare_threshold_single"
	LimitAddlMedicareMFJ        = "additional_medicare_threshold_mfj"
	LimitSecure2RothCatchupWage = "secure2_roth_catchup_wage_threshold"

	RateSupplementalLow    = "supplemental_rate_low"
	RateSupplementalHigh   = "supplemental_rate_high"
	RateFICASocialSecurity = "fica_social_security_rate"
	RateFICAMedicare       = "fica_medicare_rate"
	RateFICAAddlMedicare   = "fica_additional_medicare_rate"
	RateSafeHarborStandard = "safe_harbor_rate_standard"
	RateSafeHarborHighAGI  = "safe_harbor_rate_high_agi"
)
