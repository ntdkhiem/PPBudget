package service

import (
	"fmt"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Phase 4 -- investment allocation.
//
// Two departures from the source document, both about tax:
//
//   - consolidate_into_index_core is bounded by a TAX BUDGET. "Move everything
//     into index funds" against a large embedded gain is a recommendation to
//     realise it; the right action there is to redirect new money and harvest
//     opportunistically, which is a different instruction entirely.
//   - enable_drip is restricted to tax-advantaged accounts. The document said
//     to turn it on everywhere. In a taxable account reinvestment fragments the
//     cost basis and, worse, a reinvestment within 30 days of a loss sale of the
//     same security creates a wash sale -- so the document's own DRIP advice
//     would sabotage its own harvesting.

// employerConcentrationCeiling is the share of net worth above which a single
// employer position stops being an investment and becomes a second bet on the
// same paycheck.
const employerConcentrationCeiling = 0.05

func allocationQuests() []questDef {
	return []questDef{
		questReduceEmployerConcentration(),
		questConsolidateIntoIndexCore(),
		questHarvestTaxLosses(),
		questEnableDRIP(),
		questAutomateSurplusSweep(),
	}
}

func questReduceEmployerConcentration() questDef {
	return questDef{
		Key:          "reduce_employer_concentration",
		Phase:        PhaseAllocation,
		Priority:     0,
		Verification: domain.QuestVerificationAuto,
		Requires:     []string{"taxable_employer_stock"},
		Evaluate: func(c questContext) []questResult {
			held := money.Money(0)
			if c.Profile.TaxableEmployerStock != nil {
				held = *c.Profile.TaxableEmployerStock
			}
			netWorth := c.Baseline.NetWorth
			if netWorth <= 0 {
				return nil
			}
			share := float64(held) / float64(netWorth)
			ceiling := money.Money(employerConcentrationCeiling * float64(netWorth))

			if held <= ceiling {
				return []questResult{{
					Complete: true,
					Title:    "Your employer stock is within a sensible share of net worth",
					Detail: fmt.Sprintf("%s is %s of %s.",
						usd(held), pct(share, 1), usd(netWorth)),
					Target: &ceiling,
				}}
			}

			excess := held - ceiling
			return []questResult{{
				Title: fmt.Sprintf("Reduce employer stock by %s", usd(excess)),
				Detail: fmt.Sprintf(
					"%s of employer stock is %s of your %s net worth, against a %s ceiling. The risk is "+
						"not just volatility: your salary, your equity compensation and this position all "+
						"depend on the same company, so a bad year arrives in three places at once.",
					usd(held), pct(share, 1), usd(netWorth), pct(employerConcentrationCeiling, 0)),
				Target: &excess,
			}}
		},
	}
}

func questConsolidateIntoIndexCore() questDef {
	return questDef{
		Key:          "consolidate_into_index_core",
		Phase:        PhaseAllocation,
		Priority:     10,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"taxable_brokerage_value"},
		Evaluate: func(c questContext) []questResult {
			value := money.Money(0)
			if c.Profile.TaxableBrokerageValue != nil {
				value = *c.Profile.TaxableBrokerageValue
			}
			if value <= 0 {
				return []questResult{{NotApplicable: true}}
			}
			// Without knowing the embedded gain we cannot say whether
			// consolidating is cheap or ruinous, and guessing is how someone
			// ends up with an unexpected tax bill on advice we gave.
			if c.Profile.TaxableUnrealizedGain == nil {
				return []questResult{{Missing: []string{"taxable_unrealized_gain"}}}
			}
			gain := *c.Profile.TaxableUnrealizedGain

			// A rough long-term capital gains estimate: enough to say whether
			// this is a small cost or a serious one.
			estTax := money.Money(float64(gain) * 0.20)

			if gain <= 0 {
				return []questResult{{
					Title: "Consolidate your taxable holdings into a broad-market core",
					Detail: fmt.Sprintf(
						"There is no embedded gain in your %s taxable account, so rearranging it costs "+
							"nothing in tax. A broad-market index core captures the market return at a "+
							"fraction of the cost of picking positions.", usd(value)),
					Target: &value,
				}}
			}

			detail := fmt.Sprintf(
				"Your %s taxable account holds about %s of unrealised gain, so selling to rearrange it "+
					"would cost roughly %s in tax.", usd(value), usd(gain), usd(estTax))
			if float64(gain)/float64(value) > 0.25 {
				detail += " That is large enough that liquidating is the wrong move: point NEW money at " +
					"a broad-market core instead, harvest losses when they appear, and let the " +
					"concentrated positions run down over time rather than paying to exit them at once."
			} else {
				detail += " That is modest enough to be worth paying once to get a clean broad-market core."
			}

			return []questResult{{
				Title:  "Move toward a broad-market index core",
				Detail: detail,
				Target: &value,
			}}
		},
	}
}

func questHarvestTaxLosses() questDef {
	return questDef{
		Key:          "harvest_tax_losses",
		Phase:        PhaseAllocation,
		Priority:     20,
		Verification: domain.QuestVerificationManual,
		Requires:     []string{"taxable_brokerage_value"},
		Evaluate: func(c questContext) []questResult {
			value := money.Money(0)
			if c.Profile.TaxableBrokerageValue != nil {
				value = *c.Profile.TaxableBrokerageValue
			}
			if value <= 0 {
				return []questResult{{NotApplicable: true}}
			}

			detail := "Positions trading below what you paid can be sold to bank the loss against gains " +
				"and income, then replaced with something similar but not identical. The deadline is " +
				"31 December; losses cannot be harvested retroactively."

			// The wash-sale interaction is the part that catches people with
			// employer equity, precisely because the two other actions in this
			// plan create the purchases that trigger it.
			hasESPP := false
			for _, g := range c.Grants {
				if g.Kind == domain.EquityKindESPP {
					hasESPP = true
				}
			}
			if hasESPP {
				detail += " Watch the 30-day wash-sale window around your ESPP purchases: buying the same " +
					"stock within 30 days either side of a loss sale disallows the loss, and an ESPP " +
					"purchase is a buy like any other."
			}

			due := endOfYear(c.Now)
			return []questResult{{
				Title:  "Harvest investment losses before year end",
				Detail: detail,
				Due:    &due,
			}}
		},
	}
}

func questEnableDRIP() questDef {
	return questDef{
		Key:          "enable_drip",
		Phase:        PhaseAllocation,
		Priority:     30,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			var sheltered []string
			for _, ra := range c.Retirement {
				if ra.Kind == domain.RetirementKindTaxable {
					continue
				}
				if acct := c.accountByID(ra.AccountID); acct != nil {
					sheltered = append(sheltered, acct.Name)
				}
			}
			if len(sheltered) == 0 {
				return []questResult{{NotApplicable: true}}
			}

			return []questResult{{
				Title: "Turn on dividend reinvestment in your tax-sheltered accounts",
				Detail: fmt.Sprintf(
					"Automatic reinvestment inside %d sheltered account(s) compounds without a decision or "+
						"a tax consequence. Leave it OFF in taxable accounts: there it fragments your cost "+
						"basis across dozens of tiny lots and can trigger a wash sale against any loss you "+
						"harvest, which costs more than the convenience is worth.", len(sheltered)),
			}}
		},
	}
}

func questAutomateSurplusSweep() questDef {
	return questDef{
		Key:          "automate_surplus_sweep",
		Phase:        PhaseAllocation,
		Priority:     40,
		Verification: domain.QuestVerificationManual,
		Evaluate: func(c questContext) []questResult {
			surplus := c.Baseline.MonthlySurplus
			if surplus <= 0 {
				return []questResult{{NotApplicable: true}}
			}
			earmarked := c.plannedWithin(12)
			monthlyEarmark := money.Money(0)
			if earmarked > 0 {
				monthlyEarmark = earmarked / 12
			}
			sweepable := surplus - monthlyEarmark
			if sweepable <= 0 {
				return []questResult{{NotApplicable: true}}
			}

			detail := fmt.Sprintf(
				"You run about %s a month of surplus. An automatic transfer on payday invests it before it "+
					"is available to spend, which is the whole mechanism.", usd(surplus))
			if monthlyEarmark > 0 {
				detail += fmt.Sprintf(
					" This holds back %s a month for the near-term plans you have recorded, leaving %s to sweep.",
					usd(monthlyEarmark), usd(sweepable))
			}

			return []questResult{{
				Title:  fmt.Sprintf("Automate a %s monthly sweep into investments", usd(sweepable)),
				Detail: detail,
				Target: &sweepable,
			}}
		},
	}
}
