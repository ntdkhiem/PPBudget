package repository

import (
	"ntdkhiem/ppbudget-go/internal/domain"
)

// The wealth profile field registry.
//
// One table drives three things that would otherwise drift apart:
//
//	1. the SELECT column list and its Scan destinations,
//	2. the dynamic UPDATE built from whichever fields a caller actually set,
//	3. the canonical set of field keys, which progressive intake uses to decide
//	   what to ask next and which the generator uses to name the answers that
//	   would unlock a blocked action.
//
// Adding a question is one entry here plus one column in a migration. Keeping
// the read and write paths in one list is the point: a SELECT/Scan mismatch is
// the classic way a wide nullable table starts silently returning the wrong
// column, and with 45 of them that is a matter of when, not whether.
//
// Field keys are part of the API. They appear in Quest.MissingFields and in
// wealth_profile_fields.field_key, so renaming one is a migration, not a
// refactor.

type profileFieldDef struct {
	// Key is the stable public identifier, matching the JSON tag on
	// domain.WealthProfile.
	Key string
	// Column is the wealth_profiles column.
	Column string
	// Dest returns a pointer to scan this column into.
	Dest func(*domain.WealthProfile) any
	// Value returns the value to write, or nil when the field is unset.
	// Returning an untyped nil writes SQL NULL.
	Value func(*domain.WealthProfile) any
}

// ptrValue collapses a typed nil pointer to an untyped nil so pgx writes NULL
// rather than a typed nil it would reject.
func ptrValue[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

var profileFieldDefs = []profileFieldDef{
	// Household and horizon.
	{"date_of_birth", "date_of_birth",
		func(p *domain.WealthProfile) any { return &p.DateOfBirth },
		func(p *domain.WealthProfile) any { return ptrValue(p.DateOfBirth) }},
	{"marital_status", "marital_status",
		func(p *domain.WealthProfile) any { return &p.MaritalStatus },
		func(p *domain.WealthProfile) any { return ptrValue(p.MaritalStatus) }},
	{"filing_status", "filing_status",
		func(p *domain.WealthProfile) any { return &p.FilingStatus },
		func(p *domain.WealthProfile) any { return ptrValue(p.FilingStatus) }},
	{"resident_state", "resident_state",
		func(p *domain.WealthProfile) any { return &p.ResidentState },
		func(p *domain.WealthProfile) any { return ptrValue(p.ResidentState) }},
	{"spouse_gross_annual", "spouse_gross_annual",
		func(p *domain.WealthProfile) any { return &p.SpouseGrossAnnual },
		func(p *domain.WealthProfile) any { return ptrValue(p.SpouseGrossAnnual) }},
	{"spouse_has_workplace_plan", "spouse_has_workplace_plan",
		func(p *domain.WealthProfile) any { return &p.SpouseHasWorkplacePlan },
		func(p *domain.WealthProfile) any { return ptrValue(p.SpouseHasWorkplacePlan) }},
	{"target_independence_age", "target_independence_age",
		func(p *domain.WealthProfile) any { return &p.TargetIndependenceAge },
		func(p *domain.WealthProfile) any { return ptrValue(p.TargetIndependenceAge) }},

	// Income.
	{"gross_annual_income", "gross_annual_income",
		func(p *domain.WealthProfile) any { return &p.GrossAnnualIncome },
		func(p *domain.WealthProfile) any { return ptrValue(p.GrossAnnualIncome) }},

	// Employer plan mechanics.
	{"deferral_pct", "deferral_pct",
		func(p *domain.WealthProfile) any { return &p.DeferralPct },
		func(p *domain.WealthProfile) any { return ptrValue(p.DeferralPct) }},
	{"deferral_roth_share", "deferral_roth_share",
		func(p *domain.WealthProfile) any { return &p.DeferralRothShare },
		func(p *domain.WealthProfile) any { return ptrValue(p.DeferralRothShare) }},
	{"match_pct", "match_pct",
		func(p *domain.WealthProfile) any { return &p.MatchPct },
		func(p *domain.WealthProfile) any { return ptrValue(p.MatchPct) }},
	{"match_limit_pct", "match_limit_pct",
		func(p *domain.WealthProfile) any { return &p.MatchLimitPct },
		func(p *domain.WealthProfile) any { return ptrValue(p.MatchLimitPct) }},
	{"match_per_paycheck", "match_per_paycheck",
		func(p *domain.WealthProfile) any { return &p.MatchPerPaycheck },
		func(p *domain.WealthProfile) any { return ptrValue(p.MatchPerPaycheck) }},
	{"match_has_true_up", "match_has_true_up",
		func(p *domain.WealthProfile) any { return &p.MatchHasTrueUp },
		func(p *domain.WealthProfile) any { return ptrValue(p.MatchHasTrueUp) }},
	{"plan_allows_after_tax", "plan_allows_after_tax",
		func(p *domain.WealthProfile) any { return &p.PlanAllowsAfterTax },
		func(p *domain.WealthProfile) any { return ptrValue(p.PlanAllowsAfterTax) }},
	{"plan_allows_in_service", "plan_allows_in_service",
		func(p *domain.WealthProfile) any { return &p.PlanAllowsInService },
		func(p *domain.WealthProfile) any { return ptrValue(p.PlanAllowsInService) }},
	{"plan_accepts_rollovers", "plan_accepts_rollovers",
		func(p *domain.WealthProfile) any { return &p.PlanAcceptsRollovers },
		func(p *domain.WealthProfile) any { return ptrValue(p.PlanAcceptsRollovers) }},

	// HSA.
	{"hdhp_enrolled", "hdhp_enrolled",
		func(p *domain.WealthProfile) any { return &p.HDHPEnrolled },
		func(p *domain.WealthProfile) any { return ptrValue(p.HDHPEnrolled) }},
	{"hsa_coverage_tier", "hsa_coverage_tier",
		func(p *domain.WealthProfile) any { return &p.HSACoverageTier },
		func(p *domain.WealthProfile) any { return ptrValue(p.HSACoverageTier) }},
	{"hsa_employer_contribution", "hsa_employer_contribution",
		func(p *domain.WealthProfile) any { return &p.HSAEmployerContribution },
		func(p *domain.WealthProfile) any { return ptrValue(p.HSAEmployerContribution) }},

	// Tax.
	{"prior_year_tax_liability", "prior_year_tax_liability",
		func(p *domain.WealthProfile) any { return &p.PriorYearTaxLiability },
		func(p *domain.WealthProfile) any { return ptrValue(p.PriorYearTaxLiability) }},
	{"prior_year_agi", "prior_year_agi",
		func(p *domain.WealthProfile) any { return &p.PriorYearAGI },
		func(p *domain.WealthProfile) any { return ptrValue(p.PriorYearAGI) }},
	{"annual_bonus", "annual_bonus",
		func(p *domain.WealthProfile) any { return &p.AnnualBonus },
		func(p *domain.WealthProfile) any { return ptrValue(p.AnnualBonus) }},
	{"bonus_month", "bonus_month",
		func(p *domain.WealthProfile) any { return &p.BonusMonth },
		func(p *domain.WealthProfile) any { return ptrValue(p.BonusMonth) }},

	// Targets.
	{"emergency_fund_target_months", "emergency_fund_target_months",
		func(p *domain.WealthProfile) any { return &p.EmergencyFundTargetMonths },
		func(p *domain.WealthProfile) any { return ptrValue(p.EmergencyFundTargetMonths) }},

	// Strategy and overrides.
	{"strategy", "strategy",
		func(p *domain.WealthProfile) any { return &p.Strategy },
		func(p *domain.WealthProfile) any { return ptrValue(p.Strategy) }},
	{"override_monthly_income", "override_monthly_income",
		func(p *domain.WealthProfile) any { return &p.OverrideMonthlyIncome },
		func(p *domain.WealthProfile) any { return ptrValue(p.OverrideMonthlyIncome) }},
	{"override_essential_expenses", "override_essential_expenses",
		func(p *domain.WealthProfile) any { return &p.OverrideEssentialExpenses },
		func(p *domain.WealthProfile) any { return ptrValue(p.OverrideEssentialExpenses) }},

	// Holdings.
	{"traditional_ira_balance", "traditional_ira_balance",
		func(p *domain.WealthProfile) any { return &p.TraditionalIRABalance },
		func(p *domain.WealthProfile) any { return ptrValue(p.TraditionalIRABalance) }},
	{"taxable_brokerage_value", "taxable_brokerage_value",
		func(p *domain.WealthProfile) any { return &p.TaxableBrokerageValue },
		func(p *domain.WealthProfile) any { return ptrValue(p.TaxableBrokerageValue) }},
	{"taxable_unrealized_gain", "taxable_unrealized_gain",
		func(p *domain.WealthProfile) any { return &p.TaxableUnrealizedGain },
		func(p *domain.WealthProfile) any { return ptrValue(p.TaxableUnrealizedGain) }},
	{"taxable_employer_stock", "taxable_employer_stock",
		func(p *domain.WealthProfile) any { return &p.TaxableEmployerStock },
		func(p *domain.WealthProfile) any { return ptrValue(p.TaxableEmployerStock) }},

	// Employer equity context.
	{"employer_is_public", "employer_is_public",
		func(p *domain.WealthProfile) any { return &p.EmployerIsPublic },
		func(p *domain.WealthProfile) any { return ptrValue(p.EmployerIsPublic) }},
	{"employer_ticker", "employer_ticker",
		func(p *domain.WealthProfile) any { return &p.EmployerTicker },
		func(p *domain.WealthProfile) any { return ptrValue(p.EmployerTicker) }},
	{"has_stock_options", "has_stock_options",
		func(p *domain.WealthProfile) any { return &p.HasStockOptions },
		func(p *domain.WealthProfile) any { return ptrValue(p.HasStockOptions) }},

	// Risk transfer.
	{"ltd_replacement_pct", "ltd_replacement_pct",
		func(p *domain.WealthProfile) any { return &p.LTDReplacementPct },
		func(p *domain.WealthProfile) any { return ptrValue(p.LTDReplacementPct) }},
	{"ltd_monthly_cap", "ltd_monthly_cap",
		func(p *domain.WealthProfile) any { return &p.LTDMonthlyCap },
		func(p *domain.WealthProfile) any { return ptrValue(p.LTDMonthlyCap) }},
	{"ltd_premium_pretax", "ltd_premium_pretax",
		func(p *domain.WealthProfile) any { return &p.LTDPremiumPretax },
		func(p *domain.WealthProfile) any { return ptrValue(p.LTDPremiumPretax) }},
	{"life_death_benefit", "life_death_benefit",
		func(p *domain.WealthProfile) any { return &p.LifeDeathBenefit },
		func(p *domain.WealthProfile) any { return ptrValue(p.LifeDeathBenefit) }},

	// Housing and debt.
	{"housing_tenure", "housing_tenure",
		func(p *domain.WealthProfile) any { return &p.HousingTenure },
		func(p *domain.WealthProfile) any { return ptrValue(p.HousingTenure) }},
	{"mortgage_apr", "mortgage_apr",
		func(p *domain.WealthProfile) any { return &p.MortgageAPR },
		func(p *domain.WealthProfile) any { return ptrValue(p.MortgageAPR) }},
	{"mortgage_balance", "mortgage_balance",
		func(p *domain.WealthProfile) any { return &p.MortgageBalance },
		func(p *domain.WealthProfile) any { return ptrValue(p.MortgageBalance) }},
	{"pays_pmi", "pays_pmi",
		func(p *domain.WealthProfile) any { return &p.PaysPMI },
		func(p *domain.WealthProfile) any { return ptrValue(p.PaysPMI) }},
	{"student_loan_kind", "student_loan_kind",
		func(p *domain.WealthProfile) any { return &p.StudentLoanKind },
		func(p *domain.WealthProfile) any { return ptrValue(p.StudentLoanKind) }},
	{"student_loan_idr", "student_loan_idr",
		func(p *domain.WealthProfile) any { return &p.StudentLoanIDR },
		func(p *domain.WealthProfile) any { return ptrValue(p.StudentLoanIDR) }},
	{"student_loan_pslf", "student_loan_pslf",
		func(p *domain.WealthProfile) any { return &p.StudentLoanPSLF },
		func(p *domain.WealthProfile) any { return ptrValue(p.StudentLoanPSLF) }},

	// Projection assumptions.
	{"expected_return_apr", "expected_return_apr",
		func(p *domain.WealthProfile) any { return &p.ExpectedReturnAPR },
		func(p *domain.WealthProfile) any { return ptrValue(p.ExpectedReturnAPR) }},
	{"inflation_apr", "inflation_apr",
		func(p *domain.WealthProfile) any { return &p.InflationAPR },
		func(p *domain.WealthProfile) any { return ptrValue(p.InflationAPR) }},
	{"withdrawal_rate", "withdrawal_rate",
		func(p *domain.WealthProfile) any { return &p.WithdrawalRate },
		func(p *domain.WealthProfile) any { return ptrValue(p.WithdrawalRate) }},
	{"target_annual_spend", "target_annual_spend",
		func(p *domain.WealthProfile) any { return &p.TargetAnnualSpend },
		func(p *domain.WealthProfile) any { return ptrValue(p.TargetAnnualSpend) }},
}

// profileFieldByKey indexes the registry for the write path, which receives
// keys from the caller rather than iterating.
var profileFieldByKey = func() map[string]profileFieldDef {
	m := make(map[string]profileFieldDef, len(profileFieldDefs))
	for _, f := range profileFieldDefs {
		m[f.Key] = f
	}
	return m
}()

// ProfileFieldKeys returns every recognised profile field key.
// Used by the intake layer to work out what remains unanswered.
func ProfileFieldKeys() []string {
	keys := make([]string, 0, len(profileFieldDefs))
	for _, f := range profileFieldDefs {
		keys = append(keys, f.Key)
	}
	return keys
}

// IsProfileField reports whether a key names a real profile field, so an
// unknown key from a request body is rejected rather than silently dropped.
func IsProfileField(key string) bool {
	_, ok := profileFieldByKey[key]
	return ok
}
