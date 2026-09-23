package service

import (
	"fmt"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Phase 5 -- debt elimination and independence -- plus the cross-cutting
// protection actions.
//
// The student loan rule here carries a hard stop that the source document
// lacked entirely. Recommending extra principal on a loan headed for
// forgiveness is a pure, unrecoverable loss, and it is the worst output this
// catalog could produce: the user follows good-faith advice and destroys money
// that was about to be written off.
//
// The pre-59-and-a-half bridge is the other gap. The document defined success
// as 25x essential spend without ever asking how the user reaches the money.
// Someone aiming to stop work at 45 who put everything in a 401(k) arrives at
// their number with almost all of it locked for fourteen years.

func independenceQuests() []questDef {
	return []questDef{
		questEliminateConsumerDebt(),
		questEvaluateStudentLoanPayoff(),
		questRemovePMI(),
		questEvaluateMortgagePrepay(),
		questBuildPre59Bridge(),
		questScaleSavingsRate(),
		questReachFINumber(),
	}
}

func crossCuttingQuests() []questDef {
	return []questDef{
		questSecureDisabilityCoverage(),
		questSecureLifeCoverage(),
	}
}

func questEliminateConsumerDebt() questDef {
	return questDef{
		Key:          "eliminate_consumer_debt",
		Phase:        PhaseIndependence,
		Priority:     0,
		Verification: domain.QuestVerificationAuto,
		Evaluate: func(c questContext) []questResult {
			threshold := highAPRThreshold(c.Profile)
			var remaining money.Money
			for _, acct := range c.Accounts {
				if acct.Type != "liability" {
					continue
				}
				balance := absMoney(acct.Balance)
				if balance <= 0 {
					continue
				}
				// Mortgages and high-rate balances are handled by their own
				// actions; this is what is left in the middle.
				if t, ok := c.Terms[acct.ID]; ok && t.APR != nil && *t.APR >= threshold {
					continue
				}
				remaining += balance
			}
			if remaining <= 0 {
				return []questResult{{
					Complete: true,
					Title:    "You carry no remaining consumer debt",
				}}
			}
			return []questResult{{
				Title: fmt.Sprintf("Clear the remaining %s of consumer debt", usd(remaining)),
				Detail: "These balances sit below the rate that makes clearing them urgent, so they come " +
					"after the tax-advantaged accounts are working. Pay them to zero and keep the accounts " +
					"open: closing them raises utilisation and shortens your credit history.",
				Target:                       &remaining,
				SuppressWhenCashFlowNegative: true,
			}}
		},
	}
}

// questEvaluateStudentLoanPayoff refuses to recommend prepayment on a loan
// headed for forgiveness.
func questEvaluateStudentLoanPayoff() questDef {
	return questDef{
		Key:          "evaluate_student_loan_payoff",
		Phase:        PhaseIndependence,
		Priority:     10,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"student_loan_kind"},
		Evaluate: func(c questContext) []questResult {
			kind := ""
			if c.Profile.StudentLoanKind != nil {
				kind = *c.Profile.StudentLoanKind
			}
			if kind == "" || kind == "none" {
				return []questResult{{NotApplicable: true}}
			}

			// Federal loans need the forgiveness answer before anything else
			// can be said. This is the hard stop.
			if kind == "federal" || kind == "both" {
				if c.Profile.StudentLoanPSLF == nil || c.Profile.StudentLoanIDR == nil {
					return []questResult{{
						Title: "Check your student loans before paying anything extra",
						Detail: "Federal loans can be on a path to forgiveness, and extra principal on a " +
							"loan about to be written off is money destroyed rather than saved. Nothing " +
							"should be prepaid until this is settled.",
						Missing: []string{"student_loan_pslf", "student_loan_idr"},
					}}
				}
				if *c.Profile.StudentLoanPSLF || *c.Profile.StudentLoanIDR {
					return []questResult{{
						Title: "Do not pay extra toward your student loans",
						Detail: "You are on a forgiveness or income-driven path, where the balance is " +
							"written off at the end of the term and the payment depends on income rather " +
							"than balance. Extra principal buys you nothing at all -- it reduces a number " +
							"that was going to be cancelled. Pay the minimum and invest the difference.",
						Complete: true,
					}}
				}
			}

			threshold := highAPRThreshold(c.Profile)
			return []questResult{{
				Title: "Decide whether to pay student loans down early",
				Detail: fmt.Sprintf(
					"With no forgiveness in play this is an arithmetic question: above about %s the "+
						"guaranteed return from paying down beats the expected return from investing, and "+
						"below it the reverse holds. Rates below that are worth carrying to the end of "+
						"their term.", pct(threshold, 1)),
			}}
		},
	}
}

// questRemovePMI is a dated action worth real money and easy to forget.
func questRemovePMI() questDef {
	return questDef{
		Key:          "remove_pmi",
		Phase:        PhaseIndependence,
		Priority:     20,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"housing_tenure"},
		Evaluate: func(c questContext) []questResult {
			if c.Profile.HousingTenure == nil || *c.Profile.HousingTenure != "own" {
				return []questResult{{NotApplicable: true}}
			}
			if c.Profile.PaysPMI == nil {
				return []questResult{{Missing: []string{"pays_pmi"}}}
			}
			if !*c.Profile.PaysPMI {
				return []questResult{{NotApplicable: true}}
			}
			return []questResult{{
				Title: "Ask your servicer to drop mortgage insurance",
				Detail: "Mortgage insurance protects the lender, not you, and can be removed once the " +
					"loan falls below 80% of the home's value. Servicers do not usually volunteer this: " +
					"it typically takes a written request and sometimes an appraisal, and it is worth " +
					"one to three thousand a year for as long as it would otherwise have run.",
			}}
		},
	}
}

func questEvaluateMortgagePrepay() questDef {
	return questDef{
		Key:          "evaluate_mortgage_prepay",
		Phase:        PhaseIndependence,
		Priority:     30,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"housing_tenure"},
		Evaluate: func(c questContext) []questResult {
			if c.Profile.HousingTenure == nil || *c.Profile.HousingTenure != "own" {
				return []questResult{{NotApplicable: true}}
			}
			if c.Profile.MortgageAPR == nil {
				return []questResult{{Missing: []string{"mortgage_apr"}}}
			}
			rate := *c.Profile.MortgageAPR
			threshold := highAPRThreshold(c.Profile)

			if rate < threshold {
				return []questResult{{
					Complete: true,
					Title:    fmt.Sprintf("Your %s mortgage is worth carrying, not prepaying", pct(rate, 2)),
					Detail: fmt.Sprintf(
						"At %s the loan costs less than the %s you expect investments to return after tax. "+
							"Paying it down early converts a cheap, fixed, inflation-eroding debt into "+
							"illiquid home equity at a loss.", pct(rate, 2), pct(threshold, 1)),
				}}
			}
			var target *money.Money
			if c.Profile.MortgageBalance != nil {
				target = c.Profile.MortgageBalance
			}
			return []questResult{{
				Title: fmt.Sprintf("Consider paying down your %s mortgage", pct(rate, 2)),
				Detail: fmt.Sprintf(
					"At %s the loan costs more than the %s you expect to earn after tax, so extra principal "+
						"is a guaranteed return at that rate. Do this only after the tax-advantaged accounts "+
						"are full, since those cannot be back-filled later.", pct(rate, 2), pct(threshold, 1)),
				Target:                       target,
				SuppressWhenCashFlowNegative: true,
			}}
		},
	}
}

// questBuildPre59Bridge is the structural gap in the source document.
func questBuildPre59Bridge() questDef {
	return questDef{
		Key:          "build_pre59_bridge",
		Phase:        PhaseIndependence,
		Priority:     40,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"target_independence_age"},
		Evaluate: func(c questContext) []questResult {
			if c.Profile.TargetIndependenceAge == nil {
				return []questResult{{Missing: []string{"target_independence_age"}}}
			}
			target := *c.Profile.TargetIndependenceAge
			if target >= 60 {
				return []questResult{{
					NotApplicable: true,
				}}
			}
			gap := 60 - target

			// Without spending history the bridge has no size, and "you need
			// about $0" reads as success. Zero is not the answer here; it is
			// the absence of one.
			if c.Baseline.EssentialMonthly <= 0 {
				return []questResult{{
					Title: "Work out how to reach your money before 59 and a half",
					Detail: "Sizing the gap between stopping work and penalty-free withdrawals needs " +
						"to know what a year costs you, and there is not enough categorised spending yet.",
					Missing: []string{"transaction_history"},
				}}
			}

			var sheltered, taxable money.Money
			for _, ra := range c.Retirement {
				acct := c.accountByID(ra.AccountID)
				if acct == nil {
					continue
				}
				if ra.Kind == domain.RetirementKindTaxable {
					taxable += acct.Balance
				} else {
					sheltered += acct.Balance
				}
			}

			needed := c.Baseline.EssentialMonthly * 12 * money.Money(gap)
			detail := fmt.Sprintf(
				"You are aiming to stop working at %d, roughly %d years before retirement accounts open "+
					"without penalty. Covering that span needs about %s reachable before then -- from a "+
					"taxable account, a Roth conversion ladder started five years ahead, or a 72(t) "+
					"schedule. You currently hold %s in taxable against %s in sheltered accounts.",
				target, gap, usd(needed), usd(taxable), usd(sheltered))

			if taxable >= needed {
				return []questResult{{
					Complete: true,
					Title:    "You can reach enough money before 60 to bridge the gap",
					Detail:   detail,
					Target:   &needed,
				}}
			}
			shortfall := needed - taxable
			return []questResult{{
				Title:  fmt.Sprintf("Build a %s bridge you can reach before 60", usd(shortfall)),
				Detail: detail + " Without that bridge you can hit your number and still be unable to spend it.",
				Target: &shortfall,
			}}
		},
	}
}

func questScaleSavingsRate() questDef {
	return questDef{
		Key:          "scale_savings_rate",
		Phase:        PhaseIndependence,
		Priority:     50,
		Verification: domain.QuestVerificationAuto,
		Evaluate: func(c questContext) []questResult {
			income := c.Baseline.MonthlyIncome
			if income <= 0 {
				return nil
			}
			rate := float64(c.Baseline.MonthlySurplus) / float64(income)
			if rate >= 0.35 {
				return []questResult{{
					Complete: true,
					Title:    fmt.Sprintf("You are saving %s of income", pct(rate, 0)),
					Detail:   "At that rate the timeline is driven by returns rather than by contributions.",
				}}
			}
			return []questResult{{
				Title: fmt.Sprintf("Lift your savings rate from %s toward 35%%", pct(rate, 0)),
				Detail: "Savings rate moves the independence date far more than investment returns do, and " +
					"it is the one input you control. The cheapest place to find it is future raises: " +
					"committing half of each one before it reaches your account costs nothing you are " +
					"currently spending.",
				SuppressWhenCashFlowNegative: true,
			}}
		},
	}
}

// questReachFINumber states the target the source document implied but never
// wrote down.
func questReachFINumber() questDef {
	return questDef{
		Key:          "reach_fi_number",
		Phase:        PhaseIndependence,
		Priority:     60,
		Verification: domain.QuestVerificationAuto,
		Evaluate: func(c questContext) []questResult {
			essentials := c.Baseline.EssentialMonthly
			if essentials <= 0 {
				return nil
			}
			rate := 0.04
			if c.Profile.WithdrawalRate != nil && *c.Profile.WithdrawalRate > 0 {
				rate = *c.Profile.WithdrawalRate
			}
			annual := essentials * 12
			target := money.Money(float64(annual) / rate)
			netWorth := c.Baseline.NetWorth

			detail := fmt.Sprintf(
				"Essential spending of %s a year at a %s withdrawal rate implies %s. You are at %s.",
				usd(annual), pct(rate, 0), usd(target), usd(netWorth))

			if netWorth >= target {
				return []questResult{{
					Complete: true,
					Title:    fmt.Sprintf("You have reached your %s independence number", usd(target)),
					Detail:   detail,
					Target:   &target,
				}}
			}
			return []questResult{{
				Title:  fmt.Sprintf("Your financial independence number is %s", usd(target)),
				Detail: detail + " This is measured against essentials rather than total spending, so it is the point at which work becomes optional rather than the point at which nothing changes.",
				Target: &target,
			}}
		},
	}
}

// ----------------------------------------------------- cross-cutting

// questSecureDisabilityCoverage protects the income stream that funds every
// other action in the catalog. The source document had nothing on it.
func questSecureDisabilityCoverage() questDef {
	return questDef{
		Key:          "secure_disability_coverage",
		Phase:        PhaseCrossCutting,
		Priority:     0,
		Verification: domain.QuestVerificationManual,
		// See capture_employer_match: a paystub can supply gross pay too.
		Requires:    []string{"ltd_replacement_pct"},
		AlsoMissing: needsGrossPay,
		Evaluate: func(c questContext) []questResult {
			gross, ok := c.grossAnnual()
			if !ok {
				return []questResult{{Missing: []string{"gross_annual_income"}}}
			}
			replacement := derefF(c.Profile.LTDReplacementPct)

			// Group cover is usually a share of BASE salary only, and capped.
			// For someone whose compensation is substantially equity, the real
			// replacement share is far below the headline percentage.
			covered := money.Money(replacement * float64(gross))
			if c.Profile.LTDMonthlyCap != nil && *c.Profile.LTDMonthlyCap > 0 {
				capped := *c.Profile.LTDMonthlyCap * 12
				if capped < covered {
					covered = capped
				}
			}

			// Employer-paid premiums make the benefit taxable, which cuts what
			// actually arrives by roughly a third.
			taxNote := ""
			effective := covered
			if !boolOr(c.Profile.LTDPremiumPretax, true) {
				taxNote = " Your employer pays the premium, which makes any benefit taxable -- so what " +
					"actually arrives is materially less than the headline figure."
				effective = money.Money(float64(covered) * 0.70)
			}

			realShare := float64(effective) / float64(gross)
			if realShare >= 0.60 {
				return []questResult{{
					Complete: true,
					Title:    "Your disability cover replaces a realistic share of income",
					Detail:   fmt.Sprintf("About %s of %s.", pct(realShare, 0), usd(gross)),
				}}
			}

			gap := money.Money(float64(gross)*0.60) - effective
			return []questResult{{
				Title: fmt.Sprintf("Close the %s gap in your disability cover", usd(gap)),
				Detail: fmt.Sprintf(
					"Your cover works out at roughly %s of a %s income once caps are applied.%s Nothing "+
						"else in this plan protects the earnings that fund all of it, and the risk of being "+
						"unable to work is considerably higher than most people assume. Supplemental "+
						"own-occupation cover closes the difference; if your employer offers a post-tax "+
						"premium option, taking it makes the benefit tax-free.",
					pct(realShare, 0), usd(gross), taxNote),
				Target: &gap,
			}}
		},
	}
}

func questSecureLifeCoverage() questDef {
	return questDef{
		Key:          "secure_life_coverage",
		Phase:        PhaseCrossCutting,
		Priority:     10,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			// No dependents, no one to protect.
			hasDependents := len(c.Dependents) > 0 ||
				(c.Profile.MaritalStatus != nil && *c.Profile.MaritalStatus != "single")
			if !hasDependents {
				return []questResult{{NotApplicable: true}}
			}
			if c.Profile.LifeDeathBenefit == nil {
				return []questResult{{Missing: []string{"life_death_benefit"}}}
			}

			// A needs-based figure: replace essential spending for long enough
			// for the household to restructure, plus clear the mortgage.
			years := 10
			need := c.Baseline.EssentialMonthly * 12 * money.Money(years)
			if c.Profile.MortgageBalance != nil {
				need += *c.Profile.MortgageBalance
			}
			held := *c.Profile.LifeDeathBenefit

			if held >= need {
				return []questResult{{
					Complete: true,
					Title:    "Your life cover meets your household's need",
					Detail:   fmt.Sprintf("%s held against roughly %s needed.", usd(held), usd(need)),
					Target:   &need,
				}}
			}
			gap := need - held
			return []questResult{{
				Title: fmt.Sprintf("Add about %s of term life cover", usd(gap)),
				Detail: fmt.Sprintf(
					"Replacing %d years of essential spending and clearing the mortgage comes to roughly "+
						"%s; you hold %s. Note that employer-provided cover is typically one or two times "+
						"salary and disappears the day you leave, so it should not be counted on for a "+
						"need that outlasts the job.", years, usd(need), usd(held)),
				Target: &gap,
			}}
		},
	}
}
