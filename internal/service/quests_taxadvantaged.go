package service

import (
	"fmt"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Phase 3 -- tax-advantaged saving.
//
// Two rules here exist specifically because the source document got them
// wrong, and both wrong answers carry the same 6% annual excise tax:
//
//   - resolve_roth_route. The document automated a direct Roth IRA
//     contribution in three separate chapters while the user's income made
//     them ineligible. It even asked the eligibility question in its intake and
//     then never used the answer.
//   - max_hsa. The document assumed HDHP enrolment with no branch, and sized
//     the contribution from the self-only limit with no regard for coverage
//     tier or an employer seed.
//
// Everything that runs through payroll is dated to 31 December. A missed
// deferral has no do-over in January, so it cannot wait behind a phase.

func taxAdvantagedQuests() []questDef {
	return []questDef{
		questRaise401kDeferral(),
		questAutoEscalateDeferral(),
		questMaxHSA(),
		questResolveRothRoute(),
		questMegaBackdoorRoth(),
		questFundIRAToLimit(),
		questElectDependentCareFSA(),
	}
}

func questRaise401kDeferral() questDef {
	return questDef{
		Key:          "raise_401k_deferral",
		Phase:        PhaseTaxAdvantaged,
		Priority:     0,
		Verification: domain.QuestVerificationAuto,
		// See capture_employer_match: a paystub can supply gross pay too.
		Requires:    []string{"deferral_pct"},
		AlsoMissing: needsGrossPay,
		Evaluate: func(c questContext) []questResult {
			gross, ok := c.grossAnnual()
			if !ok {
				return []questResult{{Missing: []string{"gross_annual_income"}}}
			}
			limit := ElectiveDeferralLimit(c.Limits, c.Profile.DateOfBirth, c.TaxYear)

			// With no paystub we do not know what has already gone in, so the
			// honest action is about the RATE rather than a headroom figure
			// that would silently assume nothing had been contributed.
			if c.Paystub == nil {
				current := derefF(c.Profile.DeferralPct)
				detail := fmt.Sprintf(
					"Your limit this year is %s. At %s of pay you are on course for about %s, so there "+
						"is room to raise the rate.", usd(limit.Total), pct(current, 0),
					usd(money.Money(current*float64(gross))))
				if limit.Catchup > 0 {
					detail += fmt.Sprintf(" That limit includes a %s catch-up you are eligible for.", usd(limit.Catchup))
				}
				detail += " Enter a recent paystub and this can be stated as an exact amount remaining."
				due := endOfYear(c.Now)
				return []questResult{{
					Title:                        "Raise your 401(k) contribution rate",
					Detail:                       detail,
					Due:                          &due,
					SuppressWhenCashFlowNegative: true,
				}}
			}
			contributedYTD := c.Paystub.Pretax401k + c.Paystub.Roth401k
			headroom := limit.Total - contributedYTD
			if headroom <= 0 {
				return []questResult{{
					Complete: true,
					Title:    "You are on track to use your full 401(k) allowance",
					Detail:   fmt.Sprintf("%s contributed against a %s limit.", usd(contributedYTD), usd(limit.Total)),
					Target:   &limit.Total,
				}}
			}

			current := derefF(c.Profile.DeferralPct)
			annualAtCurrent := money.Money(current * float64(gross))

			detail := fmt.Sprintf(
				"Your limit this year is %s and you have contributed %s. At %s of pay you are on course for "+
					"about %s.", usd(limit.Total), usd(contributedYTD), pct(current, 0), usd(annualAtCurrent))

			// The catch-up tiers are not monotonic in age, so saying which one
			// applies is worth the sentence.
			if limit.Catchup > 0 {
				detail += fmt.Sprintf(" That includes a %s catch-up you are eligible for.", usd(limit.Catchup))
			} else if !limit.CatchupKnown {
				detail += " Add your date of birth and this will account for any catch-up you qualify for."
			}
			if left, ok := c.paychecksRemaining(); ok && left > 0 {
				detail += fmt.Sprintf(" %d paychecks remain this year.", left)
			}

			due := endOfYear(c.Now)
			var missing []string
			if !limit.CatchupKnown {
				missing = nil // not blocking: the base limit is still actionable
			}

			return []questResult{{
				Title:                        fmt.Sprintf("Raise your 401(k) contribution to use the remaining %s", usd(headroom)),
				Detail:                       detail,
				Target:                       &headroom,
				Due:                          &due,
				Missing:                      missing,
				SuppressWhenCashFlowNegative: true,
			}}
		},
	}
}

// questAutoEscalateDeferral is the cheapest behavioural lever in the catalog
// and needs no new information at all.
func questAutoEscalateDeferral() questDef {
	return questDef{
		Key:          "auto_escalate_deferral",
		Phase:        PhaseTaxAdvantaged,
		Priority:     10,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"deferral_pct"},
		Evaluate: func(c questContext) []questResult {
			current := derefF(c.Profile.DeferralPct)
			if current >= 0.15 {
				return []questResult{{NotApplicable: true}}
			}
			return []questResult{{
				Title: "Turn on automatic contribution increases",
				Detail: fmt.Sprintf(
					"Most plans will raise your contribution by a percentage point a year on their own. "+
						"From %s that reaches a serious savings rate without a single further decision, "+
						"and increases that land with a raise are the ones people do not notice.",
					pct(current, 0)),
			}}
		},
	}
}

// questMaxHSA branches on coverage rather than assuming it.
func questMaxHSA() questDef {
	return questDef{
		Key:          "max_hsa",
		Phase:        PhaseTaxAdvantaged,
		Priority:     20,
		Verification: domain.QuestVerificationAuto,
		Requires:     []string{"hdhp_enrolled"},
		Evaluate: func(c questContext) []questResult {
			// A "no" here is a real answer, and the whole action falls away.
			// The source document had no such branch and would have told a
			// user on a PPO to max an account they cannot legally hold.
			if !boolOr(c.Profile.HDHPEnrolled, false) {
				return []questResult{{NotApplicable: true}}
			}
			if c.Profile.HSACoverageTier == nil {
				return []questResult{{Missing: []string{"hsa_coverage_tier"}}}
			}

			var employer money.Money
			if c.Profile.HSAEmployerContribution != nil {
				employer = *c.Profile.HSAEmployerContribution
			}
			var contributedYTD money.Money
			if c.Paystub != nil {
				contributedYTD = c.Paystub.HSAContribution
			}

			room := HSAContributionRoom(c.Limits, c.Profile.HSACoverageTier,
				c.Profile.DateOfBirth, c.TaxYear, employer, contributedYTD)
			if !room.Computable {
				return []questResult{{Missing: []string{"hsa_coverage_tier"}}}
			}

			// Negative room is an over-contribution, which needs its own
			// instruction rather than being clamped to zero and hidden.
			if room.Remaining < 0 {
				excess := -room.Remaining
				due := endOfYear(c.Now)
				return []questResult{{
					Variant: domain.QuestVariantOverLimit,
					Title:   fmt.Sprintf("Withdraw %s of excess HSA contributions", usd(excess)),
					Detail: fmt.Sprintf(
						"Your contributions plus your employer's %s exceed this year's %s limit by %s. "+
							"Excess contributions are taxed at 6%% for every year they remain, so this needs "+
							"removing before the filing deadline.",
						usd(employer), usd(room.Limit+room.Catchup), usd(excess)),
					Target: &excess,
					Due:    &due,
				}}
			}
			if room.Remaining == 0 {
				return []questResult{{
					Complete: true,
					Title:    "Your HSA is fully funded for the year",
					Detail:   fmt.Sprintf("%s contributed against a %s ceiling.", usd(contributedYTD+employer), usd(room.Limit+room.Catchup)),
				}}
			}

			tier := "self-only"
			if *c.Profile.HSACoverageTier == "family" {
				tier = "family"
			}
			detail := fmt.Sprintf(
				"Your %s limit is %s. You have contributed %s", tier, usd(room.Limit), usd(contributedYTD))
			if employer > 0 {
				detail += fmt.Sprintf(" and your employer has added %s, which counts against the same ceiling", usd(employer))
			}
			detail += fmt.Sprintf(", leaving %s. An HSA is the only account that is untaxed going in, "+
				"growing, and coming out for medical costs.", usd(room.Remaining))
			if room.Catchup > 0 {
				detail += fmt.Sprintf(" That includes a %s catch-up.", usd(room.Catchup))
			}

			due := endOfYear(c.Now)
			return []questResult{{
				Title:                        fmt.Sprintf("Contribute the remaining %s to your HSA", usd(room.Remaining)),
				Detail:                       detail,
				Target:                       &room.Remaining,
				Due:                          &due,
				SuppressWhenCashFlowNegative: true,
			}}
		},
	}
}

// questResolveRothRoute is the correction the whole feature was built around.
func questResolveRothRoute() questDef {
	return questDef{
		Key:          "resolve_roth_route",
		Phase:        PhaseTaxAdvantaged,
		Priority:     30,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"filing_status"},
		Evaluate: func(c questContext) []questResult {
			magi, ok := c.householdMAGI()
			if !ok {
				return []questResult{{Missing: []string{"gross_annual_income"}}}
			}

			elig := EvaluateRothEligibility(c.Limits, c.Profile.FilingStatus, magi, c.Profile.SpouseGrossAnnual)
			if elig.Status == RothUnknown {
				return []questResult{{Missing: elig.MissingFields}}
			}

			// A derived income figure is not good enough to decide this on. The
			// consequence of getting it wrong is a penalty, so the user has to
			// have actually seen the number.
			if f, ok := c.Profile.Fields["gross_annual_income"]; ok && f.Source == domain.FieldSourceDerived {
				return []questResult{{
					Title: "Confirm your income so we can settle how to fund a Roth IRA",
					Detail: "Whether you may contribute directly depends on income, and an excess " +
						"contribution costs 6% a year until it is corrected. We have an estimate rather " +
						"than a figure you have checked, which is not a good enough basis for that call.",
					Missing: []string{"gross_annual_income"},
				}}
			}

			switch elig.Status {
			case RothEligible:
				return []questResult{{
					Title: "Fund a Roth IRA directly",
					Detail: fmt.Sprintf(
						"Household income of about %s is below the %s phase-out, so you can contribute "+
							"straight to a Roth IRA without any workaround.",
						usd(elig.MAGI), usd(elig.Start)),
				}}

			case RothPartial:
				return []questResult{{
					Title: "Your Roth IRA contribution is partly phased out",
					Detail: fmt.Sprintf(
						"Household income of about %s falls inside the %s to %s phase-out, so only part of "+
							"a full contribution is allowed. The backdoor route avoids having to compute "+
							"the reduced amount and is available regardless of income.",
						usd(elig.MAGI), usd(elig.Start), usd(elig.End)),
				}}
			}

			// Ineligible: the backdoor, and the pro-rata rule that can block it.
			detail := fmt.Sprintf(
				"Household income of about %s is above the %s cut-off, so a direct Roth IRA contribution "+
					"is not allowed -- and making one anyway costs 6%% a year until corrected. Contribute to "+
					"a traditional IRA and convert instead.",
				usd(elig.MAGI), usd(elig.End))

			if c.Profile.TraditionalIRABalance == nil {
				return []questResult{{
					Title:   "Use the backdoor route to fund a Roth IRA",
					Detail:  detail + " Before doing that, we need your existing traditional IRA balance: the pro-rata rule makes the conversion partly taxable if you hold one.",
					Missing: []string{"traditional_ira_balance"},
				}}
			}

			if *c.Profile.TraditionalIRABalance > 0 {
				detail += fmt.Sprintf(
					" Your existing %s traditional IRA balance complicates it: the pro-rata rule treats "+
						"any conversion as partly taxable in proportion to that balance.",
					usd(*c.Profile.TraditionalIRABalance))
				if boolOr(c.Profile.PlanAcceptsRollovers, false) {
					detail += " Your 401(k) accepts incoming rollovers, which is the fix -- moving the " +
						"traditional IRA into it empties the pro-rata denominator and clears the way."
				} else {
					detail += " Check whether your 401(k) accepts incoming rollovers: moving the balance " +
						"there empties the pro-rata denominator and clears the way."
				}
			} else {
				detail += " You hold no traditional IRA balance, so the pro-rata rule does not apply and " +
					"the conversion is clean."
			}

			return []questResult{{
				Title:  "Use the backdoor route to fund a Roth IRA",
				Detail: detail,
			}}
		},
	}
}

// questMegaBackdoorRoth is the largest single action available to a
// high-earning employee whose plan supports it, and was absent from the source
// document entirely.
func questMegaBackdoorRoth() questDef {
	return questDef{
		Key:          "mega_backdoor_roth",
		Phase:        PhaseTaxAdvantaged,
		Priority:     40,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"plan_allows_after_tax"},
		Evaluate: func(c questContext) []questResult {
			if !boolOr(c.Profile.PlanAllowsAfterTax, false) {
				return []questResult{{NotApplicable: true}}
			}
			if c.Profile.PlanAllowsInService == nil {
				return []questResult{{Missing: []string{"plan_allows_in_service"}}}
			}
			if !*c.Profile.PlanAllowsInService {
				return []questResult{{
					NotApplicable: true,
				}}
			}

			total, hasTotal := c.Limits.Amount(domain.LimitTotalAdditions415c)
			if !hasTotal {
				return []questResult{{NotApplicable: true}}
			}
			deferral := ElectiveDeferralLimit(c.Limits, c.Profile.DateOfBirth, c.TaxYear)

			// The after-tax space is what the overall annual-additions ceiling
			// leaves once elective deferrals and the full match are counted.
			var matchAnnual money.Money
			if saving, ok := c.workplaceSaving(); ok {
				matchAnnual = saving.MatchAvailable
			}
			space := total - deferral.Base - matchAnnual
			if space <= 0 {
				return []questResult{{NotApplicable: true}}
			}

			due := endOfYear(c.Now)
			return []questResult{{
				Title: fmt.Sprintf("Use up to %s of after-tax 401(k) space", usd(space)),
				Detail: fmt.Sprintf(
					"Your plan takes after-tax contributions and allows in-service conversion, which opens "+
						"roughly %s a year of additional tax-free growth beyond the %s ordinary limit. "+
						"Convert each contribution promptly so gains accrue inside the Roth rather than "+
						"becoming taxable on conversion.",
					usd(space), usd(deferral.Base)),
				Target:                       &space,
				Due:                          &due,
				SuppressWhenCashFlowNegative: true,
			}}
		},
	}
}

func questFundIRAToLimit() questDef {
	return questDef{
		Key:          "fund_ira_to_limit",
		Phase:        PhaseTaxAdvantaged,
		Priority:     50,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			limit := c.contributionLimits().IRA
			if limit <= 0 {
				return nil
			}

			// The IRA deadline is the filing date, not 31 December -- one of the
			// few pieces of good news in the tax calendar, and worth saying.
			due := time.Date(c.TaxYear+1, time.April, 15, 0, 0, 0, 0, time.UTC)
			return []questResult{{
				Title: fmt.Sprintf("Fund an IRA to the %s limit", usd(limit)),
				Detail: fmt.Sprintf(
					"Unlike payroll contributions, an IRA for %d can be funded right up to the filing "+
						"deadline in April %d, so this one does not expire with the calendar year.",
					c.TaxYear, c.TaxYear+1),
				Target:                       &limit,
				Due:                          &due,
				SuppressWhenCashFlowNegative: true,
			}}
		},
	}
}

// questElectDependentCareFSA only exists when there are dependents, and it is
// bounded by an enrollment window rather than the tax year.
func questElectDependentCareFSA() questDef {
	return questDef{
		Key:          "elect_dependent_care_fsa",
		Phase:        PhaseTaxAdvantaged,
		Priority:     60,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			if len(c.Dependents) == 0 {
				return []questResult{{NotApplicable: true}}
			}
			// The benefit applies to care costs, which in practice means young
			// children.
			var eligible int
			for _, d := range c.Dependents {
				if c.TaxYear-d.BirthYear < 13 {
					eligible++
				}
			}
			if eligible == 0 {
				return []questResult{{NotApplicable: true}}
			}

			cap, ok := c.Limits.Amount(domain.LimitDependentCareFSACap)
			if !ok {
				return nil
			}
			return []questResult{{
				Title: fmt.Sprintf("Elect up to %s of dependent care FSA at open enrollment", usd(cap)),
				Detail: fmt.Sprintf(
					"Care costs for %d dependent(s) can be paid with pre-tax money, which at a high marginal "+
						"rate is a substantial saving on money you are spending anyway. The election can only "+
						"be made during open enrollment, and unspent balances are forfeited, so size it "+
						"against what you actually expect to spend.", eligible),
				Target: &cap,
			}}
		},
	}
}
