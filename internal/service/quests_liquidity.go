package service

import (
	"fmt"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Phase 1 -- liquidity and high-cost debt.
//
// The ordering here is the substance of the phase and is deliberately NOT the
// order the source document used:
//
//	match -> starter buffer -> high-APR debt -> full emergency fund
//
// Two departures from that document, each worth money:
//
//   - Employer match leads, at priority 0. The source plan left the deferral at
//     2% for a year while paying down a card. A 50% match is an instant 50%
//     return; it outranks even a 20% APR balance, and every paycheck it is
//     deferred is gone permanently.
//   - The starter buffer comes BEFORE high-APR debt. Without a small cushion
//     the paid-down card gets re-borrowed on the next car repair, and the user
//     concludes the plan does not work. This also matches computeWaterfall() in
//     planning-math.ts, which this catalog replaces and must not regress.

func liquidityQuests() []questDef {
	return []questDef{
		questCaptureEmployerMatch(),
		questStarterEmergencyFund(),
		questClearPromoBalance(),
		questClearHighAPRBalance(),
		questSweepIdleCash(),
		questDiscretionaryCap(),
		questFullEmergencyFund(),
	}
}

// questCaptureEmployerMatch is the highest-priority action in the catalog.
func questCaptureEmployerMatch() questDef {
	return questDef{
		Key:          "capture_employer_match",
		Phase:        PhaseLiquidity,
		Priority:     0,
		Verification: domain.QuestVerificationAuto,
		// gross_annual_income is deliberately absent: grossAnnual() also
		// annualises a paystub, and Evaluate reports it missing only when
		// neither source can supply it.
		Requires:    []string{"match_pct", "match_limit_pct", "deferral_pct"},
		AlsoMissing: needsGrossPay,
		Evaluate: func(c questContext) []questResult {
			matchPct := derefF(c.Profile.MatchPct)
			limitPct := derefF(c.Profile.MatchLimitPct)
			deferral := derefF(c.Profile.DeferralPct)

			// A plan with no match is an answer, not a gap.
			if matchPct <= 0 || limitPct <= 0 {
				return []questResult{{NotApplicable: true}}
			}
			gross, ok := c.grossAnnual()
			if !ok {
				return []questResult{{Missing: []string{"gross_annual_income"}}}
			}

			maxMatch := money.Money(limitPct * matchPct * float64(gross))
			earnedRate := deferral
			if earnedRate > limitPct {
				earnedRate = limitPct
			}
			earned := money.Money(earnedRate * matchPct * float64(gross))
			forgone := maxMatch - earned

			if forgone <= 0 {
				return []questResult{{
					Complete: true,
					Title:    "You are capturing the full employer match",
					Detail: fmt.Sprintf("Contributing %s of salary earns the whole %s a year your employer offers.",
						pct(limitPct, 0), usd(maxMatch)),
					Target: &maxMatch,
				}}
			}

			detail := fmt.Sprintf(
				"Raising your contribution from %s to %s of salary claims the %s a year you are currently leaving behind. "+
					"No other action in this plan returns as much for as little.",
				pct(deferral, 0), pct(limitPct, 0), usd(forgone))

			if left, ok := c.paychecksRemaining(); ok && left > 0 {
				detail += fmt.Sprintf(" You have %d paychecks left this year.", left)
			}

			// The front-loading trap. A plan that funds the match per paycheck
			// with no true-up stops matching the moment the annual deferral cap
			// is hit, so racing to the cap in August forfeits the rest of the
			// year's match outright -- typically thousands, unrecoverable.
			//
			// This keys on the true-up answer ALONE rather than also requiring
			// match_per_paycheck. A plan that matches on annual totals has
			// nothing to true up, so the question only arises for per-paycheck
			// plans in the first place -- and requiring both meant the answer we
			// actually collect could never fire the warning it was collected
			// for. An explicit "no" is the signal; nil still stays silent.
			if c.Profile.MatchHasTrueUp != nil && !*c.Profile.MatchHasTrueUp {
				detail += " Spread it across every remaining paycheck rather than front-loading. " +
					"Your plan does not true up at year end, so if it funds the match per paycheck " +
					"-- as most do -- hitting the annual cap early would forfeit the match on every " +
					"paycheck after it."
			}

			return []questResult{{
				Title:  fmt.Sprintf("Claim the %s of employer match you are leaving behind", usd(forgone)),
				Detail: detail,
				Target: &forgone,
			}}
		},
	}
}

// questStarterEmergencyFund is the small cushion that makes debt payoff stick.
func questStarterEmergencyFund() questDef {
	return questDef{
		Key:          "starter_emergency_fund",
		Phase:        PhaseLiquidity,
		Priority:     10,
		Verification: domain.QuestVerificationAuto,
		Evaluate: func(c questContext) []questResult {
			target := c.Baseline.EssentialMonthly
			if target <= 0 {
				// The gap here is spending history, not an unanswered question.
				// Naming a profile field the user has already filled in makes
				// the app look broken and sends them to re-answer something
				// that was never the problem.
				return []questResult{{
					Title: "Connect or categorise some spending first",
					Detail: "A cushion is sized against what a month actually costs you, and there is " +
						"not enough categorised spending yet to work that out.",
					Missing: []string{"transaction_history"},
				}}
			}
			// Cash already spoken for is not a cushion.
			available := c.Baseline.LiquidAssets - c.plannedWithin(12)
			gap := target - available

			if gap <= 0 {
				return []questResult{{
					Complete: true,
					Title:    "You have a one-month cushion in cash",
					Detail: fmt.Sprintf("%s covers a month of essentials at %s.",
						usd(available), usd(target)),
					Target: &target,
				}}
			}
			return []questResult{{
				Title: fmt.Sprintf("Put %s aside as a starter cushion", usd(gap)),
				Detail: fmt.Sprintf(
					"One month of essentials is %s and you have %s available. "+
						"This comes before clearing card balances on purpose: without a buffer the next "+
						"unexpected bill goes straight back onto the card you just paid off.",
					usd(target), usd(available)),
				Target:                       &gap,
				SuppressWhenCashFlowNegative: true,
			}}
		},
	}
}

// questClearPromoBalance handles 0% promotional balances, which invert the
// usual ordering.
func questClearPromoBalance() questDef {
	return questDef{
		Key:          "clear_promo_balance_before_expiry",
		Phase:        PhaseLiquidity,
		Priority:     15,
		Verification: domain.QuestVerificationAuto,
		Evaluate: func(c questContext) []questResult {
			var out []questResult
			for _, acct := range c.Accounts {
				if acct.Type != "liability" {
					continue
				}
				terms, ok := c.Terms[acct.ID]
				if !ok || terms.PromoExpiresOn == nil {
					continue
				}
				balance := absMoney(acct.Balance)
				if balance <= 0 {
					continue
				}
				due := *terms.PromoExpiresOn
				if due.Before(c.Now) {
					continue // the promotion has already lapsed; ordinary rules apply
				}

				months := monthsBetween(c.Now, due)
				perMonth := balance
				if months > 0 {
					perMonth = balance / money.Money(months)
				}

				out = append(out, questResult{
					KeySuffix: acct.ID,
					Title:     fmt.Sprintf("Clear %s on %s before the promotional rate ends", usd(balance), acct.Name),
					Detail: fmt.Sprintf(
						"The promotional rate expires on %s. Clearing about %s a month gets there. "+
							"This is excluded from the ordinary lowest-rate-last ordering deliberately: "+
							"deferred-interest promotions charge ALL the interest accrued since day one if "+
							"any balance is left at expiry, so a 0%% balance is not free money past that date.",
						due.Format("2 January 2006"), usd(perMonth)),
					Target: &balance,
					Due:    &due,
				})
			}
			return out
		},
	}
}

// questClearHighAPRBalance fans out one action per expensive balance.
func questClearHighAPRBalance() questDef {
	return questDef{
		Key:          "clear_high_apr_balance",
		Phase:        PhaseLiquidity,
		Priority:     20,
		Verification: domain.QuestVerificationAuto,
		Evaluate: func(c questContext) []questResult {
			threshold := highAPRThreshold(c.Profile)

			var out []questResult
			var unratedCount int

			for _, acct := range c.Accounts {
				if acct.Type != "liability" {
					continue
				}
				balance := absMoney(acct.Balance)
				if balance <= 0 {
					continue
				}
				terms, ok := c.Terms[acct.ID]
				if !ok || terms.APR == nil {
					// A rate the user has not given is not a rate of zero. The
					// old waterfall skipped these silently; naming them is how
					// the balance gets a rate instead of being ignored.
					unratedCount++
					continue
				}
				// A live promotional rate is handled by its own dated action.
				if terms.PromoExpiresOn != nil && terms.PromoExpiresOn.After(c.Now) {
					continue
				}
				if *terms.APR < threshold {
					continue
				}

				annualInterest := money.Money(float64(balance) * *terms.APR)
				out = append(out, questResult{
					KeySuffix: acct.ID,
					Title:     fmt.Sprintf("Clear the %s balance on %s", usd(balance), acct.Name),
					Detail: fmt.Sprintf(
						"At %s APR this balance costs about %s a year, more than the %s you expect "+
							"investments to return after tax. Pay it to zero and KEEP THE ACCOUNT OPEN: "+
							"closing it raises your utilisation and shortens your average account age.",
						pct(*terms.APR, 2), usd(annualInterest), pct(threshold, 1)),
					Target:                       &balance,
					SuppressWhenCashFlowNegative: true,
				})
			}

			if len(out) == 0 && unratedCount > 0 {
				return []questResult{{
					Title: "Add interest rates to your debts",
					Detail: fmt.Sprintf(
						"%d of your balances have no interest rate recorded, so they cannot be ordered "+
							"against each other or against investing. A rate takes a moment to find on a statement.",
						unratedCount),
					Missing: []string{"account_apr"},
				}}
			}
			return out
		},
	}
}

// questSweepIdleCash moves dead money into yield.
func questSweepIdleCash() questDef {
	return questDef{
		Key:          "sweep_idle_cash",
		Phase:        PhaseLiquidity,
		Priority:     30,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			// The best rate the user actually holds sets the bar: telling
			// someone to chase a rate they have no account for is advice they
			// cannot act on today.
			var bestAPY float64
			for _, acct := range c.Accounts {
				if acct.Type != "asset" {
					continue
				}
				if t, ok := c.Terms[acct.ID]; ok && t.APY != nil && *t.APY > bestAPY {
					bestAPY = *t.APY
				}
			}
			if bestAPY <= 0 {
				return nil
			}

			// A month of essentials stays in checking regardless -- sweeping the
			// float is how you produce an overdraft in pursuit of a few dollars
			// of interest.
			float := c.Baseline.EssentialMonthly
			earmarked := c.plannedWithin(12)

			var out []questResult
			for _, acct := range c.Accounts {
				if acct.Type != "asset" || acct.Balance <= 0 {
					continue
				}
				apy := 0.0
				if t, ok := c.Terms[acct.ID]; ok && t.APY != nil {
					apy = *t.APY
				}
				if apy >= bestAPY {
					continue
				}
				movable := acct.Balance - float - earmarked
				if movable <= 0 {
					continue
				}
				gain := money.Money(float64(movable) * (bestAPY - apy))
				if gain < 1000 { // under ten dollars a year is not worth an action
					continue
				}

				detail := fmt.Sprintf(
					"%s earns %s while you hold an account paying %s. Moving %s earns about %s more a year. "+
						"That leaves a month of essentials in place as working cash",
					acct.Name, pct(apy, 2), pct(bestAPY, 2), usd(movable), usd(gain))
				if earmarked > 0 {
					detail += fmt.Sprintf(", and holds back the %s you have earmarked for near-term plans", usd(earmarked))
				}
				detail += "."

				out = append(out, questResult{
					KeySuffix: acct.ID,
					Title:     fmt.Sprintf("Move %s out of %s into higher yield", usd(movable), acct.Name),
					Detail:    detail,
					Target:    &movable,
				})
			}
			return out
		},
	}
}

// discretionaryCapShare is the share of income above which discretionary
// spending is worth an action rather than a shrug.
const discretionaryCapShare = 0.10

// discretionaryStreakMonths is how long the cap has to hold before the action
// counts as done.
//
// Three complete months, not an average over six. The two are different
// questions and they disagree in exactly the case that matters: a single bad
// month hidden by five good ones passes an average and fails a streak, and the
// user who is told they have held a limit they broke in March has been
// misinformed by their own plan.
const discretionaryStreakMonths = 3

// questDiscretionaryCap is the one phase 1 action about behaviour rather than
// placement, and the only one whose condition is about a PERIOD rather than a
// present balance.
func questDiscretionaryCap() questDef {
	return questDef{
		Key:          "discretionary_cap",
		Phase:        PhaseLiquidity,
		Priority:     40,
		Verification: domain.QuestVerificationAuto,
		Evaluate: func(c questContext) []questResult {
			income := c.Baseline.MonthlyIncome
			if income <= 0 || c.Baseline.MonthsOfData == 0 {
				return nil
			}
			wants := c.Baseline.MonthlyWants
			if wants <= 0 {
				return nil
			}

			cap := money.Money(float64(income) * discretionaryCapShare)
			streak := monthsUnderCap(c.Months, cap)

			// Not enough history to claim a streak either way. Saying so beats
			// both alternatives: calling it done would be a guess, and calling
			// it outstanding would be nagging about something unmeasured.
			if len(c.Months) < discretionaryStreakMonths {
				return []questResult{{
					Title: fmt.Sprintf("Hold discretionary spending under %s a month", usd(cap)),
					Detail: fmt.Sprintf(
						"A tenth of income is %s a month. There is not yet enough history to say "+
							"whether you are holding it -- this needs %d complete months and there "+
							"%s %d.",
						usd(cap), discretionaryStreakMonths,
						pluralIs(len(c.Months)), len(c.Months)),
					Target: &cap,
				}}
			}

			if streak >= discretionaryStreakMonths {
				return []questResult{{
					Complete: true,
					Title:    fmt.Sprintf("You have held discretionary spending under %s", usd(cap)),
					Detail: fmt.Sprintf(
						"%d consecutive months at or below a tenth of income.", streak),
					Target: &cap,
				}}
			}

			detail := fmt.Sprintf(
				"Wants have averaged %s a month, %s of income. A tenth of income is %s, which frees "+
					"%s a month for everything above.",
				usd(wants), pct(float64(wants)/float64(income), 0), usd(cap), usd(wants-cap))
			if streak > 0 {
				detail += fmt.Sprintf(" You are %d %s in; this counts as held after %d.",
					streak, pluralMonth(streak), discretionaryStreakMonths)
			}

			return []questResult{{
				Title:  fmt.Sprintf("Hold discretionary spending under %s a month", usd(cap)),
				Detail: detail,
				Target: &cap,
			}}
		},
	}
}

func pluralIs(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

func pluralMonth(n int) string {
	if n == 1 {
		return "month"
	}
	return "months"
}

// questFullEmergencyFund tops the cushion up to the user's own target.
func questFullEmergencyFund() questDef {
	return questDef{
		Key:          "full_emergency_fund",
		Phase:        PhaseLiquidity,
		Priority:     50,
		Verification: domain.QuestVerificationAuto,
		Requires:     []string{"emergency_fund_target_months"},
		Evaluate: func(c questContext) []questResult {
			months := 6
			if c.Profile.EmergencyFundTargetMonths != nil {
				months = *c.Profile.EmergencyFundTargetMonths
			}
			essentials := c.Baseline.EssentialMonthly
			if essentials <= 0 {
				return nil
			}
			target := essentials * money.Money(months)
			available := c.Baseline.LiquidAssets - c.plannedWithin(12)
			gap := target - available

			if gap <= 0 {
				return []questResult{{
					Complete: true,
					Title:    fmt.Sprintf("Your emergency fund covers %d months", months),
					Detail:   fmt.Sprintf("%s against a %s target.", usd(available), usd(target)),
					Target:   &target,
				}}
			}

			detail := fmt.Sprintf("%d months of essentials is %s and you have %s set aside.",
				months, usd(target), usd(available))
			if surplus := c.Baseline.MonthlySurplus; surplus > 0 {
				detail += fmt.Sprintf(" At your current surplus of %s a month that takes about %d months.",
					usd(surplus), monthsToFill(gap, surplus))
			}

			return []questResult{{
				Title:                        fmt.Sprintf("Build your emergency fund to %s", usd(target)),
				Detail:                       detail,
				Target:                       &gap,
				SuppressWhenCashFlowNegative: true,
			}}
		},
	}
}

// ---------------------------------------------------------------- helpers

func derefF(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func boolOr(p *bool, fallback bool) bool {
	if p == nil {
		return fallback
	}
	return *p
}

// absMoney gives a liability's magnitude: balances are stored signed, with
// liabilities negative.
func absMoney(m money.Money) money.Money {
	if m < 0 {
		return -m
	}
	return m
}

func monthsToFill(gap, perMonth money.Money) int {
	if perMonth <= 0 {
		return 0
	}
	n := int(gap / perMonth)
	if gap%perMonth != 0 {
		n++
	}
	return n
}

func monthsBetween(from, to time.Time) int {
	if !to.After(from) {
		return 0
	}
	months := int(to.Sub(from).Hours() / 24 / 30.44)
	if months < 1 {
		return 1
	}
	return months
}
