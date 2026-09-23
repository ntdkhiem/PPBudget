package service

import (
	"context"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Statutory limit resolution.
//
// Every figure comes from the tax_limits table by year; nothing here is a
// constant. The companion discipline, enforced by the shapes below, is that the
// app stores year-to-date CONTRIBUTIONS and computes headroom at evaluation
// time. It never stores "the user's limit", which is wrong every January.
//
// Functions that cannot answer confidently say so rather than guessing. An
// unanswered date of birth means the catch-up tier is unknown, and a limit that
// silently omits a catch-up the user is entitled to would under-state their
// headroom by thousands.

// GetTaxLimits loads one tax year's figures.
func (s *Service) GetTaxLimits(ctx context.Context, taxYear int) (domain.TaxLimits, error) {
	return s.repo.GetTaxLimits(ctx, taxYear)
}

// ageAtYearEnd is the age used for every catch-up test.
//
// Contribution catch-ups turn on age at the END of the calendar year, not on
// the birthday itself: someone who turns 50 on 31 December is eligible for the
// whole of that year. Using current age would wrongly withhold the catch-up for
// most of the year from everyone with a late birthday.
//
// Because 31 December is the last day of the year, every birthday in that year
// has necessarily already passed, so the age is just the difference in years --
// no month or day comparison is needed. Comparing day-of-year here would be
// worse than redundant: year days do not line up across leap boundaries, so a
// 31 December birthday in a leap year would come out a year young.
func ageAtYearEnd(dob time.Time, taxYear int) int {
	return taxYear - dob.Year()
}

// DeferralLimit is the 402(g) elective deferral ceiling plus any catch-up the
// user's age entitles them to.
type DeferralLimit struct {
	Base    money.Money
	Catchup money.Money
	Total   money.Money
	// CatchupKnown is false when date of birth is unanswered. Callers must not
	// present Total as authoritative in that case -- a 52-year-old shown only
	// the base limit is being told to under-contribute.
	CatchupKnown bool
}

// ElectiveDeferralLimit resolves the 401(k) ceiling for a tax year.
//
// The tiering is not monotonic in age: SECURE 2.0 gives 60-to-63-year-olds a
// larger "super" catch-up that then REVERTS to the ordinary one at 64. Writing
// this as a simple threshold ladder would quietly over-state the limit for
// anyone 64 or older.
func ElectiveDeferralLimit(limits domain.TaxLimits, dob *time.Time, taxYear int) DeferralLimit {
	base, _ := limits.Amount(domain.LimitElectiveDeferral402g)
	out := DeferralLimit{Base: base, Total: base}
	if dob == nil {
		return out
	}
	out.CatchupKnown = true

	age := ageAtYearEnd(*dob, taxYear)
	switch {
	case age >= 60 && age <= 63:
		out.Catchup, _ = limits.Amount(domain.LimitCatchup60To63)
	case age >= 50:
		out.Catchup, _ = limits.Amount(domain.LimitCatchup50Plus)
	}
	out.Total = out.Base + out.Catchup
	return out
}

// HSARoom is what the user may still contribute this year.
type HSARoom struct {
	Limit    money.Money
	Catchup  money.Money
	Employer money.Money
	// Remaining is Limit + Catchup - Employer - already contributed. It can go
	// negative, which is the excess-contribution case and needs its own action
	// rather than a clamped zero.
	Remaining money.Money
	// Computable is false when coverage tier is unanswered. "Max your HSA" is
	// NOT derivable from an HDHP flag alone: family coverage is thousands
	// higher than self-only, and getting it wrong produces an excess
	// contribution carrying a 6% annual excise tax.
	Computable bool
}

// HSAContributionRoom computes remaining HSA headroom for the year.
//
// employerContribution is subtracted because the statutory limit is a combined
// ceiling across employee and employer money. Ignoring an employer seed is one
// of the two ways this function could hand someone a penalty.
func HSAContributionRoom(
	limits domain.TaxLimits,
	coverageTier *string,
	dob *time.Time,
	taxYear int,
	employerContribution money.Money,
	contributedYTD money.Money,
) HSARoom {
	out := HSARoom{Employer: employerContribution}
	if coverageTier == nil {
		return out
	}

	var key string
	switch *coverageTier {
	case "family":
		key = domain.LimitHSAFamily
	case "self_only":
		key = domain.LimitHSASelfOnly
	default:
		return out
	}
	limit, ok := limits.Amount(key)
	if !ok {
		return out
	}
	out.Limit = limit
	out.Computable = true

	if dob != nil && ageAtYearEnd(*dob, taxYear) >= 55 {
		out.Catchup, _ = limits.Amount(domain.LimitHSACatchup55Plus)
	}
	out.Remaining = out.Limit + out.Catchup - employerContribution - contributedYTD
	return out
}

// Roth IRA direct-contribution eligibility.
const (
	RothEligible   = "eligible"
	RothPartial    = "partial"
	RothIneligible = "ineligible"
	// RothUnknown means the inputs to decide are missing. It is NOT a synonym
	// for eligible: the engine must block the action rather than assume.
	RothUnknown = "unknown"
)

// RothEligibility is the answer to a question the old intake asked the USER --
// "do your income levels permit direct Roth IRA contributions?" -- which they
// cannot reliably answer and the app can compute.
type RothEligibility struct {
	Status string
	MAGI   money.Money
	Start  money.Money
	End    money.Money
	// MissingFields names what would make this computable. The UI turns these
	// into "answer 2 questions to unlock".
	MissingFields []string
}

// EvaluateRothEligibility decides whether a direct Roth IRA contribution is
// permitted, from filing status and household MAGI.
//
// This is the fix for the flaw that motivated the feature. A high base salary
// plus equity vests pushes MAGI past the phase-out, and an automated direct
// contribution then becomes an excess contribution carrying a 6% excise tax per
// year until corrected.
//
// Spouse income is required for joint filers and is not a nicety. Asking filing
// status while not asking spouse income is worse than asking neither: an MFJ
// user who supplies only their own income looks comfortably eligible while the
// household is not.
func EvaluateRothEligibility(
	limits domain.TaxLimits,
	filingStatus *string,
	ownMAGI money.Money,
	spouseIncome *money.Money,
) RothEligibility {
	out := RothEligibility{Status: RothUnknown}

	if filingStatus == nil {
		out.MissingFields = append(out.MissingFields, "filing_status")
		return out
	}

	var startKey, endKey string
	magi := ownMAGI

	switch *filingStatus {
	case "single", "hoh", "qss":
		startKey, endKey = domain.LimitRothMAGISingleStart, domain.LimitRothMAGISingleEnd
	case "mfj":
		startKey, endKey = domain.LimitRothMAGIMFJStart, domain.LimitRothMAGIMFJEnd
		if spouseIncome == nil {
			// Deliberately unknown rather than optimistic.
			out.MissingFields = append(out.MissingFields, "spouse_gross_annual")
			return out
		}
		magi += *spouseIncome
	case "mfs":
		startKey, endKey = "roth_magi_phaseout_mfs_start", "roth_magi_phaseout_mfs_end"
	default:
		out.MissingFields = append(out.MissingFields, "filing_status")
		return out
	}

	start, okStart := limits.Amount(startKey)
	end, okEnd := limits.Amount(endKey)
	if !okStart || !okEnd {
		return out
	}

	out.MAGI, out.Start, out.End = magi, start, end
	switch {
	case magi < start:
		out.Status = RothEligible
	case magi >= end:
		out.Status = RothIneligible
	default:
		out.Status = RothPartial
	}
	return out
}

// FICA is the mechanical payroll tax split.
//
// Computed rather than asked. The old intake asked for "total estimated tax
// withheld across federal, state and FICA combined", which is the wrong shape:
// bundling a mechanical tax with two discretionary ones makes the number
// useless for every downstream calculation and impossible to split back out.
type FICA struct {
	SocialSecurity money.Money
	Medicare       money.Money
	Additional     money.Money
	Total          money.Money
}

// ComputeFICA derives the employee share from year-to-date gross wages.
func ComputeFICA(limits domain.TaxLimits, grossYTD money.Money, filingStatus *string) FICA {
	ssRate, _ := limits.Rate(domain.RateFICASocialSecurity)
	medRate, _ := limits.Rate(domain.RateFICAMedicare)
	addlRate, _ := limits.Rate(domain.RateFICAAddlMedicare)
	wageBase, _ := limits.Amount(domain.LimitSocialSecurityWageBase)

	ssWages := grossYTD
	if wageBase > 0 && ssWages > wageBase {
		ssWages = wageBase
	}

	var out FICA
	out.SocialSecurity = money.Money(float64(ssWages) * ssRate)
	out.Medicare = money.Money(float64(grossYTD) * medRate)

	threshold, ok := limits.Amount(domain.LimitAddlMedicareSingle)
	if filingStatus != nil && *filingStatus == "mfj" {
		threshold, ok = limits.Amount(domain.LimitAddlMedicareMFJ)
	}
	if ok && grossYTD > threshold {
		out.Additional = money.Money(float64(grossYTD-threshold) * addlRate)
	}

	out.Total = out.SocialSecurity + out.Medicare + out.Additional
	return out
}

// SafeHarbor is the withholding floor that avoids an underpayment penalty.
type SafeHarbor struct {
	Target money.Money
	// Rate applied: the standard share of prior-year tax, or the higher one
	// for high prior-year AGI.
	Rate float64
	// Computable is false without prior-year figures. Without them the equity
	// tax-gap action can only say "set some money aside" rather than naming a
	// dated amount -- and for someone whose vests are withheld at a flat 22%
	// against a much higher marginal rate, under-withholding is the default
	// state, not the exception.
	Computable    bool
	MissingFields []string
}

// SafeHarborTarget computes total withholding that satisfies the prior-year
// safe harbour.
func SafeHarborTarget(limits domain.TaxLimits, priorYearTax, priorYearAGI *money.Money) SafeHarbor {
	var out SafeHarbor
	if priorYearTax == nil {
		out.MissingFields = append(out.MissingFields, "prior_year_tax_liability")
	}
	if priorYearAGI == nil {
		out.MissingFields = append(out.MissingFields, "prior_year_agi")
	}
	if len(out.MissingFields) > 0 {
		return out
	}

	rate, _ := limits.Rate(domain.RateSafeHarborStandard)
	if threshold, ok := limits.Amount(domain.LimitSafeHarborAGIThreshold); ok && *priorYearAGI > threshold {
		if high, ok := limits.Rate(domain.RateSafeHarborHighAGI); ok {
			rate = high
		}
	}

	out.Rate = rate
	out.Target = money.Money(float64(*priorYearTax) * rate)
	out.Computable = true
	return out
}

// SupplementalWithholdingRate is the federal rate applied to a vest or bonus.
//
// Returned as a DEFAULT rather than an intake question: the statutory rate is
// knowable, and asking users to recall their employer's supplemental rate
// produces guesses. An employer that withholds at some other rate is recorded
// per grant and overrides this.
//
// The step from the low rate to the high one happens once aggregate
// supplemental wages cross the threshold WITHIN the year, which makes it a
// dated, predictable event rather than a static property -- hence taking
// year-to-date supplemental wages rather than just a flag.
func SupplementalWithholdingRate(limits domain.TaxLimits, supplementalYTD money.Money) float64 {
	threshold, ok := limits.Amount(domain.LimitSupplementalThreshold)
	if ok && supplementalYTD > threshold {
		if high, ok := limits.Rate(domain.RateSupplementalHigh); ok {
			return high
		}
	}
	low, _ := limits.Rate(domain.RateSupplementalLow)
	return low
}
