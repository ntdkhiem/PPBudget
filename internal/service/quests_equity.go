package service

import (
	"fmt"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Phase 2 -- equity and concentration risk.
//
// The whole phase is gated on the employer being publicly traded. Private
// company equity is a different instrument with different actions: double
// trigger RSUs that vest but cannot be sold, no 10b5-1, no daily price. Handing
// those employees "sell within 48 hours of vest" is handing them an instruction
// they cannot follow, so the phase declines to apply rather than guessing.
//
// Everything dated here escapes phase gating by design. A vest 41 days out and
// a fixed ESPP purchase date happen on the calendar's schedule, not the plan's.

func equityQuests() []questDef {
	return []questDef{
		questEstablishSellOnVest(),
		questLiquidateVestedRSU(),
		questSellESPPAtPurchase(),
		questReserveEquityTaxGap(),
		questCalibrateW4Withholding(),
		questMaximizeESPPRate(),
	}
}

// hasPublicEquity reports whether the equity actions apply at all.
func hasPublicEquity(c questContext) (applies bool, missing []string) {
	if c.Profile.EmployerIsPublic == nil {
		return false, []string{"employer_is_public"}
	}
	if !*c.Profile.EmployerIsPublic {
		return false, nil // a real answer: this branch does not apply
	}
	return true, nil
}

func questEstablishSellOnVest() questDef {
	return questDef{
		Key:          "establish_sell_on_vest",
		Phase:        PhaseEquity,
		Priority:     0,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			ok, missing := hasPublicEquity(c)
			if len(missing) > 0 {
				return []questResult{{Missing: missing}}
			}
			if !ok || len(c.Grants) == 0 {
				return []questResult{{NotApplicable: true}}
			}

			// Already automated everywhere.
			automated := true
			var blackoutRisk bool
			for _, g := range c.Grants {
				if !boolOr(g.HasRule10b51, false) {
					automated = false
				}
				if g.BlackoutPolicy != nil && *g.BlackoutPolicy != "none" {
					blackoutRisk = true
				}
			}
			if automated {
				return []questResult{{
					Complete: true,
					Title:    "Selling on vest is already automatic",
					Detail:   "A 10b5-1 plan executes your sales regardless of what the trading window is doing.",
				}}
			}

			detail := "Selling each tranche as it vests removes the single-stock risk without any market call: " +
				"the shares are worth exactly what they were worth a moment before, and holding them is a " +
				"decision to buy your employer's stock with the proceeds."
			if blackoutRisk {
				detail += " You are subject to trading blackouts, which is the reason to automate rather than " +
					"act manually: a standing 10b5-1 instruction executes inside a closed window, while a " +
					"manual sale cannot. Without one, plan to sell in the first open window after each vest " +
					"and expect to hold the shares until then."
			}

			return []questResult{{
				Title:  "Set up automatic selling when equity vests",
				Detail: detail,
			}}
		},
	}
}

// questLiquidateVestedRSU produces one dated action per upcoming vest.
func questLiquidateVestedRSU() questDef {
	return questDef{
		Key:          "liquidate_vested_rsu",
		Phase:        PhaseEquity,
		Priority:     10,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			ok, missing := hasPublicEquity(c)
			if len(missing) > 0 {
				return []questResult{{Missing: missing}}
			}
			if !ok {
				return []questResult{{NotApplicable: true}}
			}

			var out []questResult
			for _, g := range c.Grants {
				if g.Kind != domain.EquityKindRSU || g.NextVestDate == nil {
					continue
				}
				due := *g.NextVestDate
				if due.Before(c.Now.AddDate(0, 0, -30)) {
					continue // long past; the schedule needs refreshing, not an action
				}

				label := "your RSU grant"
				if g.Label != nil && *g.Label != "" {
					label = *g.Label
				}
				days := int(due.Sub(c.Now).Hours() / 24)

				detail := fmt.Sprintf(
					"The next tranche of %s vests on %s", label, due.Format("2 January 2006"))
				if days >= 0 {
					detail += fmt.Sprintf(", in %d days", days)
				}
				detail += ". Sell it on arrival. Vested shares are ordinary income already taxed; " +
					"keeping them is an active decision to hold a concentrated position in the same " +
					"company that pays your salary."

				out = append(out, questResult{
					KeySuffix: g.ID,
					Title:     fmt.Sprintf("Sell the tranche of %s vesting %s", label, due.Format("2 Jan")),
					Detail:    detail,
					Due:       &due,
				})
			}
			return out
		},
	}
}

// questSellESPPAtPurchase produces one dated action per ESPP purchase date.
func questSellESPPAtPurchase() questDef {
	return questDef{
		Key:          "sell_espp_at_purchase",
		Phase:        PhaseEquity,
		Priority:     20,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			ok, missing := hasPublicEquity(c)
			if len(missing) > 0 {
				return []questResult{{Missing: missing}}
			}
			if !ok {
				return []questResult{{NotApplicable: true}}
			}

			var out []questResult
			for _, g := range c.Grants {
				if g.Kind != domain.EquityKindESPP || g.ESPPPurchaseDate == nil {
					continue
				}
				due := *g.ESPPPurchaseDate
				if due.Before(c.Now.AddDate(0, 0, -30)) {
					continue
				}

				detail := fmt.Sprintf("Shares are purchased on %s.", due.Format("2 January 2006"))
				if g.ESPPDiscountPct != nil && *g.ESPPDiscountPct > 0 {
					detail += fmt.Sprintf(
						" Selling immediately locks in the %s discount as a near-certain return. "+
							"Holding to qualify for long-term treatment risks the whole discount on the "+
							"share price, which is a much larger bet than the tax saving is worth.",
						pct(*g.ESPPDiscountPct, 0))
				}

				out = append(out, questResult{
					KeySuffix: g.ID,
					Title:     fmt.Sprintf("Sell the ESPP shares purchased %s", due.Format("2 Jan")),
					Detail:    detail,
					Due:       &due,
				})
			}
			return out
		},
	}
}

// questReserveEquityTaxGap covers the shortfall between flat supplemental
// withholding and the user's real marginal rate.
func questReserveEquityTaxGap() questDef {
	return questDef{
		Key:          "reserve_equity_tax_gap",
		Phase:        PhaseEquity,
		Priority:     30,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			ok, missing := hasPublicEquity(c)
			if len(missing) > 0 {
				return []questResult{{Missing: missing}}
			}
			if !ok || len(c.Grants) == 0 {
				return []questResult{{NotApplicable: true}}
			}

			var supplementalYTD money.Money
			if c.Paystub != nil {
				supplementalYTD = c.Paystub.SupplementalWages
			}
			withholding := SupplementalWithholdingRate(c.Limits, supplementalYTD)

			// The gap only exists if we know the marginal rate to compare
			// against, and that needs last year's return or a filing status.
			if c.Profile.FilingStatus == nil {
				return []questResult{{Missing: []string{"filing_status"}}}
			}

			detail := fmt.Sprintf(
				"Equity vests are withheld at a flat %s federal, which is below the marginal rate on a high "+
					"income. The difference is not forgiven, it is simply owed later. Set it aside as each "+
					"tranche lands rather than discovering it at filing.", pct(withholding, 0))

			if supplementalYTD > 0 {
				if threshold, ok := c.Limits.Amount(domain.LimitSupplementalThreshold); ok && supplementalYTD < threshold {
					detail += fmt.Sprintf(
						" You have had %s of supplemental wages this year; above %s the mandatory rate "+
							"steps up, which changes the gap mid-year.",
						usd(supplementalYTD), usd(threshold))
				}
			}

			return []questResult{{
				Title:  "Set aside the tax your equity withholding does not cover",
				Detail: detail,
			}}
		},
	}
}

// questCalibrateW4Withholding closes the loop the reserve action opens.
func questCalibrateW4Withholding() questDef {
	return questDef{
		Key:          "calibrate_w4_withholding",
		Phase:        PhaseEquity,
		Priority:     40,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"prior_year_tax_liability", "prior_year_agi"},
		Evaluate: func(c questContext) []questResult {
			ok, missing := hasPublicEquity(c)
			if len(missing) > 0 {
				return []questResult{{Missing: missing}}
			}
			if !ok {
				return []questResult{{NotApplicable: true}}
			}

			harbor := SafeHarborTarget(c.Limits, c.Profile.PriorYearTaxLiability, c.Profile.PriorYearAGI)
			if !harbor.Computable {
				return []questResult{{Missing: harbor.MissingFields}}
			}

			// Treating an absent paystub as zero withholding would overstate
			// the shortfall by the entire year's withholding and tell the user
			// to send the IRS money they have already paid.
			if c.Paystub == nil {
				return []questResult{{
					Title: fmt.Sprintf("Check your withholding against a %s safe-harbour target", usd(harbor.Target)),
					Detail: fmt.Sprintf(
						"Withholding %s of last year's tax -- %s -- avoids an underpayment penalty "+
							"whatever this year turns out to be. Enter the year-to-date figures from a "+
							"recent paystub and we can say how much of that is already covered.",
						pct(harbor.Rate, 0), usd(harbor.Target)),
					Missing: []string{"paystub_ytd"},
				}}
			}
			withheldYTD := c.Paystub.FederalWithheld
			shortfall := harbor.Target - withheldYTD

			if shortfall <= 0 {
				return []questResult{{
					Complete: true,
					Title:    "Your withholding already clears the safe harbour",
					Detail: fmt.Sprintf("%s withheld against a %s safe-harbour target.",
						usd(withheldYTD), usd(harbor.Target)),
					Target: &harbor.Target,
				}}
			}

			detail := fmt.Sprintf(
				"Withholding %s of last year's tax -- %s -- avoids an underpayment penalty whatever this "+
					"year turns out to be. You are at %s so far, leaving %s.",
				pct(harbor.Rate, 0), usd(harbor.Target), usd(withheldYTD), usd(shortfall))

			if left, ok := c.paychecksRemaining(); ok && left > 0 {
				perCheck := shortfall / money.Money(left)
				detail += fmt.Sprintf(
					" Adding %s to line 4(c) of your W-4 for each of the %d remaining paychecks closes it.",
					usd(perCheck), left)
			}

			due := endOfYear(c.Now)
			return []questResult{{
				Title:  fmt.Sprintf("Increase withholding by %s before year end", usd(shortfall)),
				Detail: detail,
				Target: &shortfall,
				Due:    &due,
			}}
		},
	}
}

// questMaximizeESPPRate is absent from the source document entirely, despite it
// collecting the discount and lookback that make the case.
func questMaximizeESPPRate() questDef {
	return questDef{
		Key:          "maximize_espp_rate",
		Phase:        PhaseEquity,
		Priority:     50,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			ok, missing := hasPublicEquity(c)
			if len(missing) > 0 {
				return []questResult{{Missing: missing}}
			}
			if !ok {
				return []questResult{{NotApplicable: true}}
			}

			for _, g := range c.Grants {
				if g.Kind != domain.EquityKindESPP {
					continue
				}
				if g.ESPPDiscountPct == nil || g.ESPPContributionPct == nil || g.ESPPPlanMaxPct == nil {
					return []questResult{{Missing: []string{"espp_terms"}}}
				}
				discount, current, max := *g.ESPPDiscountPct, *g.ESPPContributionPct, *g.ESPPPlanMaxPct
				if discount <= 0 || current >= max {
					return []questResult{{
						Complete: true,
						Title:    "You are contributing the maximum to your ESPP",
						Detail:   fmt.Sprintf("At %s of pay, the plan's ceiling.", pct(current, 0)),
					}}
				}

				// A lookback makes the effective return larger than the headline
				// discount, because the purchase price is struck against the
				// lower of two dates.
				effective := discount / (1 - discount)
				note := ""
				if boolOr(g.ESPPHasLookback, false) {
					note = " The lookback provision makes the real return higher still, since the purchase " +
						"price is struck against the lower of the offering and purchase dates."
				}

				gross, haveGross := c.grossAnnual()
				detail := fmt.Sprintf(
					"You contribute %s of pay against a plan maximum of %s. A %s discount is an effective "+
						"return of about %s on every dollar, earned in months rather than years, and sold "+
						"immediately it carries almost no market risk.%s",
					pct(current, 0), pct(max, 0), pct(discount, 0), pct(effective, 0), note)

				var target *money.Money
				if haveGross {
					extra := money.Money((max - current) * float64(gross))
					gain := money.Money(float64(extra) * effective)
					target = &gain
					detail += fmt.Sprintf(
						" Raising to the maximum puts a further %s a year through the plan, worth roughly %s.",
						usd(extra), usd(gain))
				}

				return []questResult{{
					KeySuffix:                    g.ID,
					Title:                        fmt.Sprintf("Raise your ESPP contribution from %s to %s of pay", pct(current, 0), pct(max, 0)),
					Detail:                       detail,
					Target:                       target,
					SuppressWhenCashFlowNegative: true,
				}}
			}
			return []questResult{{NotApplicable: true}}
		},
	}
}
