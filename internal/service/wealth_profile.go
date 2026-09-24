package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/internal/repository"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Wealth profile: reading, writing, deriving and ageing.
//
// The intake principle this file implements is "never show a field the
// aggregator could have filled". Roughly half the old question set was already
// in the database -- housing, utilities, groceries, transport, subscriptions,
// balances -- and asking for them produced worse answers than computing them,
// because recalled spending runs systematically low. A user who types "$400" for
// transport when the real figure is $680 corrupts the emergency fund target,
// the independence number and the surplus at once, silently and with no flag.
//
// So: derive what we can, show it for confirmation, and only ask for what is
// genuinely unknowable from the ledger.

// BaselineMonths matches BASELINE_MONTHS in planning-data.ts, so the derived
// figures reconcile with what Cash Plan displays.
const BaselineMonths = 6

// bucketCoverageFloor is the point below which derived expense figures stop
// being trustworthy enough to present as fact.
//
// Coverage is the share of outflow attributed to a budget bucket. When a large
// slice is unbucketed, "essentials" is really "the part of essentials we
// happened to categorise", and presenting that confidently is how a plan ends
// up built on a number nobody checked.
const bucketCoverageFloor = 0.80

// staleAfter is how long each answer stays trustworthy.
//
// Cadence follows how fast the underlying fact actually moves, not a uniform
// timeout: a date of birth never goes stale, a brokerage balance does so within
// months, and salary and benefits move on an annual cycle. Fields absent from
// this map never expire.
var staleAfter = map[string]time.Duration{
	// Annual: pay, benefits elections and last year's return.
	"gross_annual_income":       365 * 24 * time.Hour,
	"spouse_gross_annual":       365 * 24 * time.Hour,
	"deferral_pct":              365 * 24 * time.Hour,
	"deferral_roth_share":       365 * 24 * time.Hour,
	"match_pct":                 365 * 24 * time.Hour,
	"match_limit_pct":           365 * 24 * time.Hour,
	"hsa_coverage_tier":         365 * 24 * time.Hour,
	"hsa_employer_contribution": 365 * 24 * time.Hour,
	"hdhp_enrolled":             365 * 24 * time.Hour,
	"prior_year_tax_liability":  365 * 24 * time.Hour,
	"prior_year_agi":            365 * 24 * time.Hour,
	"annual_bonus":              365 * 24 * time.Hour,
	"ltd_replacement_pct":       365 * 24 * time.Hour,
	"ltd_monthly_cap":           365 * 24 * time.Hour,
	"life_death_benefit":        365 * 24 * time.Hour,
	"filing_status":             365 * 24 * time.Hour,

	// Quarterly: balances that move on their own.
	"traditional_ira_balance": 90 * 24 * time.Hour,
	"taxable_brokerage_value": 90 * 24 * time.Hour,
	"taxable_unrealized_gain": 90 * 24 * time.Hour,
	"taxable_employer_stock":  90 * 24 * time.Hour,
	"mortgage_balance":        90 * 24 * time.Hour,
}

// StaleFields returns the answered field keys that are past their cadence.
//
// Actions built on these still render -- flagged rather than hidden. A plan
// quietly built on last year's salary is the failure to avoid, and silently
// dropping the action would hide it just as effectively as trusting it.
func StaleFields(p *domain.WealthProfile, now time.Time) []string {
	var out []string
	for key, maxAge := range staleAfter {
		f, ok := p.Fields[key]
		if !ok {
			continue // unanswered is not stale; it is simply unasked
		}
		if now.Sub(f.AnsweredAt) > maxAge {
			out = append(out, key)
		}
	}
	return out
}

// GetWealthProfile returns the profile, backfilling once from the legacy blobs
// if this user has never had one.
func (s *Service) GetWealthProfile(ctx context.Context, userID string) (*domain.WealthProfile, error) {
	p, err := s.repo.GetWealthProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(p.Fields) == 0 {
		if err := s.backfillFromLegacyBlobs(ctx, userID); err != nil {
			// A failed backfill must not block the page. The user can still
			// answer everything by hand, which is strictly better than a 500
			// on a read.
			s.logger.Warn("wealth profile backfill failed", "user_id", userID, "error", err)
		} else {
			if p, err = s.repo.GetWealthProfile(ctx, userID); err != nil {
				return nil, err
			}
		}
	}
	return p, nil
}

// SaveWealthProfile writes the named fields with the given provenance.
func (s *Service) SaveWealthProfile(
	ctx context.Context, userID string, p *domain.WealthProfile, setKeys []string, source string,
) (*domain.WealthProfile, error) {
	if err := s.repo.UpsertWealthProfile(ctx, userID, p, setKeys, source); err != nil {
		return nil, err
	}
	return s.repo.GetWealthProfile(ctx, userID)
}

// ---------------------------------------------------------------- derived

// DerivedValue is one computed answer offered for confirmation.
type DerivedValue struct {
	FieldKey string       `json:"field_key"`
	Amount   *money.Money `json:"amount,omitempty"`
	Number   *float64     `json:"number,omitempty"`
	Text     *string      `json:"text,omitempty"`
	// Basis explains where the figure came from, so the confirmation screen can
	// show its working rather than asserting a number.
	Basis string `json:"basis"`
	// AlreadyAnswered is true when the user has a value of their own. The UI
	// shows the derived figure alongside rather than overwriting it.
	AlreadyAnswered bool `json:"already_answered"`
}

// DerivedProfile is the payload behind the confirmation screen that replaces
// seven separate expense questions.
type DerivedProfile struct {
	Values []DerivedValue `json:"values"`
	// Breakdown is the per-category detail behind essential spend, so the user
	// can see what the total is made of before accepting it.
	Breakdown []domain.CategorySpend `json:"breakdown"`

	MonthsOfData   int     `json:"months_of_data"`
	BucketCoverage float64 `json:"bucket_coverage"`
	// LowConfidence is set when too little outflow is bucketed for the derived
	// figures to be presented as fact. The intake warns instead of asserting.
	LowConfidence bool `json:"low_confidence"`
	// NoSpendingData means there are no complete months to average at all.
	NoSpendingData bool `json:"no_spending_data"`
	// Unanswered lists every recognised field the user has not yet answered --
	// what progressive intake still has to ask.
	Unanswered []string `json:"unanswered"`
	Stale      []string `json:"stale"`
}

// DeriveProfileValues computes everything the ledger can answer.
//
// Nothing here is written. The user sees these, and confirming them is what
// promotes a value from 'derived' to 'confirmed' -- which matters because the
// generator refuses to run high-stakes branches off a figure nobody looked at.
func (s *Service) DeriveProfileValues(ctx context.Context, userID string) (*DerivedProfile, error) {
	pb, err := s.repo.GetPlanningBaseline(ctx, userID, BaselineMonths)
	if err != nil {
		return nil, fmt.Errorf("planning baseline: %w", err)
	}
	baseline := ComputeBaseline(pb)

	profile, err := s.repo.GetWealthProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	answered := func(key string) bool {
		_, ok := profile.Fields[key]
		return ok
	}
	out := &DerivedProfile{
		MonthsOfData:   baseline.MonthsOfData,
		BucketCoverage: baseline.BucketCoverage,
		LowConfidence:  baseline.BucketCoverage < bucketCoverageFloor || baseline.MonthsOfData < 2,
		// With no complete months there is nothing to be confident ABOUT, which
		// is a different message from "we have data but little of it is
		// categorised" -- the first asks for transactions, the second for
		// categories.
		NoSpendingData: baseline.MonthsOfData == 0,
	}

	// Retirement spending defaults to current essentials: the figure a plan
	// should be built on is what the household actually needs, not total spend
	// including the discretionary part that stops in retirement.
	if baseline.EssentialMonthly > 0 {
		annual := baseline.EssentialMonthly * 12
		out.Values = append(out.Values, DerivedValue{
			FieldKey: "target_annual_spend",
			Amount:   &annual,
			Basis: fmt.Sprintf("Essential spending averaged %s a month over %d complete months.",
				annual/12, baseline.MonthsOfData),
			AlreadyAnswered: answered("target_annual_spend"),
		})
	}

	// The emergency fund target is asked in MONTHS, never dollars: the dollar
	// figure is the planner's output, derived from essentials, and asking the
	// user for it asks them to do the job they came here for.
	months := suggestedEmergencyMonths(profile)
	monthsF := float64(months)
	basis := fmt.Sprintf("Suggested %d months. %s", months, emergencyMonthsRationale(profile))
	// Only quote a dollar figure once there is spending to quote. "That is $0
	// at your current essentials" is worse than saying nothing: it reads as a
	// computed answer when it is the absence of one.
	if baseline.EssentialMonthly > 0 {
		basis += fmt.Sprintf(" That is %s at your current essentials.",
			usd(baseline.EssentialMonthly*money.Money(months)))
	}
	out.Values = append(out.Values, DerivedValue{
		FieldKey:        "emergency_fund_target_months",
		Number:          &monthsF,
		Basis:           basis,
		AlreadyAnswered: answered("emergency_fund_target_months"),
	})

	// A per-category breakdown so the confirmation screen can show its working.
	end := time.Now().UTC()
	start := end.AddDate(0, -BaselineMonths, 0)
	if spend, err := s.repo.GetSpendingByCategory(ctx, userID, start, end); err == nil {
		out.Breakdown = spend
	} else {
		s.logger.Warn("category breakdown unavailable for intake", "user_id", userID, "error", err)
	}

	for _, key := range s.ProfileFieldKeys() {
		if !answered(key) && !isOverrideField(key) {
			out.Unanswered = append(out.Unanswered, key)
		}
	}
	out.Stale = StaleFields(profile, time.Now().UTC())

	return out, nil
}

// isOverrideField reports whether a profile key corrects a derived figure
// rather than asking a question.
//
// Overrides are left out of the unanswered list: an absent correction is the
// normal state, not a gap, and listing "override liquid account ids" among the
// questions invited answering an account list with a dollar amount. They are
// set from the figure they correct.
func isOverrideField(key string) bool {
	return strings.HasPrefix(key, "override_")
}

// ProfileFieldKeys exposes the canonical field list to handlers, so the
// registry in the repository stays the single definition of what a field is.
func (s *Service) ProfileFieldKeys() []string { return repository.ProfileFieldKeys() }

// IsProfileField reports whether a key names a real field, so an unrecognised
// key in a request body is rejected rather than silently dropped.
func (s *Service) IsProfileField(key string) bool { return repository.IsProfileField(key) }

// suggestedEmergencyMonths adjusts the conventional six months for the risks
// the profile actually describes.
//
// A single earner carries the whole household on one income stream; a second
// income is itself a cushion. Dependents raise the floor on what a bad month
// costs. Variable compensation means the income figure the plan is built on is
// the optimistic one.
func suggestedEmergencyMonths(p *domain.WealthProfile) int {
	months := 6
	if p.MaritalStatus != nil && *p.MaritalStatus != "single" &&
		p.SpouseGrossAnnual != nil && *p.SpouseGrossAnnual > 0 {
		months = 4
	}
	return months
}

func emergencyMonthsRationale(p *domain.WealthProfile) string {
	if p.MaritalStatus != nil && *p.MaritalStatus != "single" &&
		p.SpouseGrossAnnual != nil && *p.SpouseGrossAnnual > 0 {
		return "A second income shortens how long a gap has to be covered."
	}
	return "A single income stream means a gap is covered entirely from savings."
}

// ------------------------------------------------------- satellite tables

// ListAccountTerms returns per-account APR, APY and statement facts.
func (s *Service) ListAccountTerms(ctx context.Context, userID string) (map[string]domain.AccountTerms, error) {
	return s.repo.ListAccountTerms(ctx, userID)
}

// UpsertAccountTerms records the terms an aggregator does not return.
func (s *Service) UpsertAccountTerms(ctx context.Context, userID string, t domain.AccountTerms) error {
	return s.repo.UpsertAccountTerms(ctx, userID, t)
}

// ListRetirementAccountTerms maps the user's accounts to tax treatments.
func (s *Service) ListRetirementAccountTerms(ctx context.Context, userID string) ([]domain.RetirementAccountTerms, error) {
	return s.repo.ListRetirementAccountTerms(ctx, userID)
}

// ListPlannedExpenses returns near-term earmarked cash, which every sweep and
// invest rule subtracts before recommending anything.
func (s *Service) ListPlannedExpenses(ctx context.Context, userID string) ([]domain.PlannedExpense, error) {
	return s.repo.ListPlannedExpenses(ctx, userID)
}

// ListGoals returns the user's savings goals, in their own order.
func (s *Service) ListGoals(ctx context.Context, userID string) ([]domain.Goal, error) {
	return s.repo.ListGoals(ctx, userID)
}

// UpsertGoal creates or updates one goal.
func (s *Service) UpsertGoal(ctx context.Context, userID string, g domain.Goal) (string, error) {
	return s.repo.UpsertGoal(ctx, userID, g)
}

// DeleteGoal removes one goal.
func (s *Service) DeleteGoal(ctx context.Context, userID, id string) error {
	return s.repo.DeleteGoal(ctx, userID, id)
}

// UpsertRetirementAccountTerms records an account's tax treatment and monthly
// contribution.
func (s *Service) UpsertRetirementAccountTerms(ctx context.Context, userID string, t domain.RetirementAccountTerms) error {
	return s.repo.UpsertRetirementAccountTerms(ctx, userID, t)
}

// DeleteRetirementAccountTerms unmaps an account from its tax treatment.
func (s *Service) DeleteRetirementAccountTerms(ctx context.Context, userID, accountID string) error {
	return s.repo.DeleteRetirementAccountTerms(ctx, userID, accountID)
}

// ListEquityGrants returns the user's equity compensation.
func (s *Service) ListEquityGrants(ctx context.Context, userID string) ([]domain.EquityGrant, error) {
	return s.repo.ListEquityGrants(ctx, userID)
}

// GetLatestPaystub returns the most recent year-to-date payroll figures, or nil
// when none have been entered.
func (s *Service) GetLatestPaystub(ctx context.Context, userID string) (*domain.PaystubYTD, error) {
	return s.repo.GetLatestPaystub(ctx, userID)
}

// UpsertPaystub records one dated year-to-date reading.
func (s *Service) UpsertPaystub(ctx context.Context, userID string, p domain.PaystubYTD) error {
	return s.repo.UpsertPaystub(ctx, userID, p)
}

// --------------------------------------------------------------- backfill

// legacyFinancialPlan mirrors the 'financial_plan' blob written by
// planning-data.ts. Only the fields worth carrying over are declared.
type legacyFinancialPlan struct {
	Strategy  *string `json:"strategy"`
	Overrides struct {
		MonthlyIncomeCents     *int64   `json:"monthly_income_cents"`
		EssentialExpensesCents *int64   `json:"essential_expenses_cents"`
		LiquidAccountIDs       []string `json:"liquid_account_ids"`
	} `json:"overrides"`
	Goals []struct {
		Name            string  `json:"name"`
		TargetCents     int64   `json:"target_cents"`
		TargetDate      *string `json:"target_date"`
		LinkedAccountID *string `json:"linked_account_id"`
		CurrentCents    *int64  `json:"current_cents"`
		Priority        int     `json:"priority"`
	} `json:"goals"`
	EmergencyFund struct {
		TargetMonths *int `json:"target_months"`
	} `json:"emergency_fund"`
	Debts []struct {
		AccountID       string   `json:"account_id"`
		APR             *float64 `json:"apr"`
		MinPaymentCents *int64   `json:"min_payment_cents"`
	} `json:"debts"`
	Assumptions struct {
		InvestReturnAPR *float64 `json:"invest_return_apr"`
	} `json:"assumptions"`
}

// legacyRetirementPlan mirrors the 'retirement_plan' blob written by
// retirement-data.ts.
type legacyRetirementPlan struct {
	CurrentAge             *int     `json:"current_age"`
	TargetRetirementAge    *int     `json:"target_retirement_age"`
	GrossAnnualIncomeCents *int64   `json:"gross_annual_income_cents"`
	ExpectedReturnAPR      *float64 `json:"expected_return_apr"`
	InflationAPR           *float64 `json:"inflation_apr"`
	WithdrawalRate         *float64 `json:"withdrawal_rate"`
	TargetAnnualSpendCents *int64   `json:"target_annual_spend_cents"`
	Accounts               []struct {
		AccountID                string   `json:"account_id"`
		Kind                     string   `json:"kind"`
		MonthlyContributionCents int64    `json:"monthly_contribution_cents"`
		EmployerMatchPct         *float64 `json:"employer_match_pct"`
		EmployerMatchLimitPct    *float64 `json:"employer_match_limit_pct"`
	} `json:"accounts"`
}

// backfillFromLegacyBlobs seeds a new profile from the two user_settings JSON
// blobs, once.
//
// Runs on first read rather than as a migration step so it needs no downtime
// and costs nothing for users who never open the page. The blobs are left in
// place: this only ever reads them, which is what makes the 0024 rollback land
// somewhere usable.
//
// Everything carried over is marked 'derived', not 'entered'. The user never
// saw these values in this context, and one of them -- date of birth, inferred
// from a stored age -- is genuinely approximate. Marking them derived is what
// makes the intake ask for confirmation instead of quietly building a plan on
// an inference.
func (s *Service) backfillFromLegacyBlobs(ctx context.Context, userID string) error {
	financialRaw, _ := s.repo.GetUserSetting(ctx, userID, "financial_plan")
	retirementRaw, _ := s.repo.GetUserSetting(ctx, userID, "retirement_plan")
	if financialRaw == "" && retirementRaw == "" {
		return nil
	}

	p := &domain.WealthProfile{}
	var keys []string
	set := func(key string) { keys = append(keys, key) }

	if financialRaw != "" {
		var fp legacyFinancialPlan
		if err := json.Unmarshal([]byte(financialRaw), &fp); err != nil {
			s.logger.Warn("financial_plan blob unreadable, skipping", "user_id", userID, "error", err)
		} else {
			if fp.EmergencyFund.TargetMonths != nil {
				p.EmergencyFundTargetMonths = fp.EmergencyFund.TargetMonths
				set("emergency_fund_target_months")
			}
			if fp.Strategy != nil {
				p.Strategy = fp.Strategy
				set("strategy")
			}
			if fp.Overrides.MonthlyIncomeCents != nil {
				m := money.Money(*fp.Overrides.MonthlyIncomeCents)
				p.OverrideMonthlyIncome = &m
				set("override_monthly_income")
			}
			if fp.Overrides.EssentialExpensesCents != nil {
				m := money.Money(*fp.Overrides.EssentialExpensesCents)
				p.OverrideEssentialExpenses = &m
				set("override_essential_expenses")
			}
			if len(fp.Overrides.LiquidAccountIDs) > 0 {
				p.OverrideLiquidAccountIDs = fp.Overrides.LiquidAccountIDs
				set("override_liquid_account_ids")
			}

			// Goals are the one thing in the blob nothing can re-derive, so a
			// failure to carry one over loses it outright. Each is attempted
			// individually and logged on failure rather than abandoning the
			// whole backfill over one bad row.
			for _, g := range fp.Goals {
				if g.Name == "" || g.TargetCents <= 0 {
					continue
				}
				goal := domain.Goal{
					Name:         g.Name,
					TargetAmount: money.Money(g.TargetCents),
					Priority:     g.Priority,
				}
				if g.TargetDate != nil && *g.TargetDate != "" {
					if t, err := time.Parse("2006-01-02", *g.TargetDate); err == nil {
						goal.TargetDate = &t
					}
				}
				if g.LinkedAccountID != nil && *g.LinkedAccountID != "" {
					goal.LinkedAccountID = g.LinkedAccountID
				}
				if g.CurrentCents != nil {
					goal.CurrentAmount = money.Money(*g.CurrentCents)
				}
				if _, err := s.repo.UpsertGoal(ctx, userID, goal); err != nil {
					s.logger.Warn("could not carry over goal",
						"user_id", userID, "goal", g.Name, "error", err)
				}
			}
			// Per-account APR and minimum payment finally leave the blob.
			for _, d := range fp.Debts {
				if d.AccountID == "" {
					continue
				}
				terms := domain.AccountTerms{AccountID: d.AccountID, APR: d.APR}
				if d.MinPaymentCents != nil {
					m := money.Money(*d.MinPaymentCents)
					terms.MinPayment = &m
				}
				if err := s.repo.UpsertAccountTerms(ctx, userID, terms); err != nil {
					s.logger.Warn("could not carry over account terms",
						"user_id", userID, "account_id", d.AccountID, "error", err)
				}
			}
		}
	}

	if retirementRaw != "" {
		var rp legacyRetirementPlan
		if err := json.Unmarshal([]byte(retirementRaw), &rp); err != nil {
			s.logger.Warn("retirement_plan blob unreadable, skipping", "user_id", userID, "error", err)
		} else {
			// The old model stored an age, which rots. Reconstructing a date of
			// birth from it can only be accurate to the year, and the exact date
			// decides catch-up eligibility, so this is explicitly an
			// approximation the user is asked to confirm.
			if rp.CurrentAge != nil && *rp.CurrentAge > 0 {
				approx := time.Date(time.Now().UTC().Year()-*rp.CurrentAge, time.January, 1, 0, 0, 0, 0, time.UTC)
				p.DateOfBirth = &approx
				set("date_of_birth")
			}
			if rp.TargetRetirementAge != nil {
				p.TargetIndependenceAge = rp.TargetRetirementAge
				set("target_independence_age")
			}
			if rp.GrossAnnualIncomeCents != nil {
				m := money.Money(*rp.GrossAnnualIncomeCents)
				p.GrossAnnualIncome = &m
				set("gross_annual_income")
			}
			if rp.ExpectedReturnAPR != nil {
				p.ExpectedReturnAPR = rp.ExpectedReturnAPR
				set("expected_return_apr")
			}
			if rp.InflationAPR != nil {
				p.InflationAPR = rp.InflationAPR
				set("inflation_apr")
			}
			if rp.WithdrawalRate != nil {
				p.WithdrawalRate = rp.WithdrawalRate
				set("withdrawal_rate")
			}
			if rp.TargetAnnualSpendCents != nil {
				m := money.Money(*rp.TargetAnnualSpendCents)
				p.TargetAnnualSpend = &m
				set("target_annual_spend")
			}

			for _, a := range rp.Accounts {
				if a.AccountID == "" || a.Kind == "" {
					continue
				}
				if err := s.repo.UpsertRetirementAccountTerms(ctx, userID, domain.RetirementAccountTerms{
					AccountID:           a.AccountID,
					Kind:                a.Kind,
					MonthlyContribution: money.Money(a.MonthlyContributionCents),
				}); err != nil {
					s.logger.Warn("could not carry over retirement account terms",
						"user_id", userID, "account_id", a.AccountID, "error", err)
				}
				// Match terms were per-account in the blob but describe one
				// workplace plan, so they move to the profile. The first
				// matchable account with terms wins; a second workplace plan is
				// rare enough to be worth asking about rather than guessing.
				if p.MatchPct == nil && a.EmployerMatchPct != nil &&
					(a.Kind == domain.RetirementKind401k || a.Kind == domain.RetirementKindRoth401k) {
					p.MatchPct = a.EmployerMatchPct
					set("match_pct")
					if a.EmployerMatchLimitPct != nil {
						p.MatchLimitPct = a.EmployerMatchLimitPct
						set("match_limit_pct")
					}
				}
			}
		}
	}

	if len(keys) == 0 {
		return nil
	}
	return s.repo.UpsertWealthProfile(ctx, userID, p, keys, domain.FieldSourceDerived)
}
