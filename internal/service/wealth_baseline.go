package service

import (
	"math"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// The trailing baseline, ported from computeBaseline() in
// frontend/app/(dashboard)/cash-plan/_components/planning-math.ts.
//
// This is a DELIBERATE PORT, not a reimplementation. The Cash Plan page has
// shown these figures for a while and the action engine now decides against
// them, so the two must agree exactly -- a user who sees "essentials $4,435" on
// one screen and an emergency fund target built on a different number on
// another has no reason to trust either. The rounding below matches JavaScript
// Math.round rather than Go's for the same reason; see mean().
//
// Everything downstream of this file works in integer cents.

// Baseline is the averaged trailing window the generator reasons over.
type Baseline struct {
	// MonthlyIncome is the average across months with activity.
	MonthlyIncome money.Money
	// MonthlyOutflow is the average total outflow.
	MonthlyOutflow money.Money
	// EssentialMonthly is average 'needs' spend -- the emergency fund
	// denominator and the FI-number multiplicand.
	EssentialMonthly  money.Money
	MonthlyWants      money.Money
	MonthlySavings    money.Money
	MonthlyUnbucketed money.Money
	// MonthlySurplus may be negative, and the generator has to handle that:
	// when essentials exceed income, no savings action is valid.
	MonthlySurplus   money.Money
	LiquidAssets     money.Money
	TotalLiabilities money.Money // negative
	NetWorth         money.Money
	// BucketCoverage is the share of outflow attributed to a budget bucket,
	// 0..1. Below roughly 0.8 the derived expense figures are guesses, and the
	// intake should say so rather than present them confidently.
	BucketCoverage float64
	MonthsOfData   int
}

// mean averages cents, matching JavaScript Math.round.
//
// Math.round breaks ties toward positive infinity (-2.5 rounds to -2), while
// Go's math.Round breaks them away from zero (-2.5 rounds to -3). Floor(x+0.5)
// reproduces the JavaScript behaviour. The case is rare, but a one-cent
// divergence between the two implementations is exactly the kind of thing that
// costs an afternoon later.
func mean(values []money.Money) money.Money {
	if len(values) == 0 {
		return 0
	}
	var sum int64
	for _, v := range values {
		sum += v.ToInt64()
	}
	return money.Money(math.Floor(float64(sum)/float64(len(values)) + 0.5))
}

// activeMonths drops months with no activity at all.
//
// A month that has not happened yet, or that predates the user's data, would
// otherwise drag every average toward zero and overstate how long the
// emergency fund lasts.
func activeMonths(months []domain.PlanningMonth) []domain.PlanningMonth {
	out := make([]domain.PlanningMonth, 0, len(months))
	for _, m := range months {
		if m.Income != 0 || m.Outflow != 0 {
			out = append(out, m)
		}
	}
	return out
}

// completeMonths additionally drops the current, partial month.
//
// Including a half-finished month understates spending, which inflates the
// surplus and shortens every projected timeline. Kept only when it is the sole
// month of data, since some baseline beats none.
func completeMonths(months []domain.PlanningMonth) []domain.PlanningMonth {
	active := activeMonths(months)
	if len(active) <= 1 {
		return active
	}
	return active[:len(active)-1]
}

// ComputeBaseline averages the trailing window and folds in current balances.
//
// Unlike the TypeScript original it takes no plan overrides: those lived in the
// financial_plan blob, and their replacements are profile answers that
// applyProfile folds in afterwards. This stays a plain reading of the ledger
// because the intake's confirmation screen offers exactly these figures.
func ComputeBaseline(pb *domain.PlanningBaseline) Baseline {
	if pb == nil {
		return Baseline{BucketCoverage: 1}
	}
	months := completeMonths(pb.Months)

	income := make([]money.Money, 0, len(months))
	outflow := make([]money.Money, 0, len(months))
	needs := make([]money.Money, 0, len(months))
	wants := make([]money.Money, 0, len(months))
	savings := make([]money.Money, 0, len(months))
	unbucketed := make([]money.Money, 0, len(months))

	var totalOutflow, totalUnbucketed int64
	for _, m := range months {
		income = append(income, m.Income)
		outflow = append(outflow, m.Outflow)
		needs = append(needs, m.Needs)
		wants = append(wants, m.Wants)
		savings = append(savings, m.Savings)
		unbucketed = append(unbucketed, m.Unbucketed)
		totalOutflow += m.Outflow.ToInt64()
		totalUnbucketed += m.Unbucketed.ToInt64()
	}

	monthlyIncome := mean(income)
	monthlyOutflow := mean(outflow)

	coverage := 1.0
	if totalOutflow > 0 {
		coverage = 1 - float64(totalUnbucketed)/float64(totalOutflow)
	}

	var liquid money.Money
	for _, a := range pb.Accounts {
		if a.Type == "asset" {
			liquid += a.Balance
		}
	}

	return Baseline{
		MonthlyIncome:     monthlyIncome,
		MonthlyOutflow:    monthlyOutflow,
		EssentialMonthly:  mean(needs),
		MonthlyWants:      mean(wants),
		MonthlySavings:    mean(savings),
		MonthlyUnbucketed: mean(unbucketed),
		MonthlySurplus:    monthlyIncome - monthlyOutflow,
		LiquidAssets:      liquid,
		TotalLiabilities:  pb.TotalLiabilities,
		NetWorth:          pb.NetWorth,
		BucketCoverage:    coverage,
		MonthsOfData:      len(months),
	}
}

// cashAccountIDs names the accounts an emergency could actually draw on.
//
// The user's own choice wins outright. Without one, every asset account counts
// except those linked to a tax treatment on the Retirement tab: a 401(k) or a
// brokerage balance is not an emergency fund, and counting it declared the
// cushion finished before a dollar of cash had been set aside.
func cashAccountIDs(
	accounts []domain.PlanningAccount, chosen []string, invested []domain.RetirementAccountTerms,
) map[string]bool {
	cash := make(map[string]bool)

	if len(chosen) > 0 {
		picked := make(map[string]bool, len(chosen))
		for _, id := range chosen {
			picked[id] = true
		}
		for _, a := range accounts {
			if a.Type == "asset" && picked[a.ID] {
				cash[a.ID] = true
			}
		}
		return cash
	}

	skip := make(map[string]bool, len(invested))
	for _, r := range invested {
		skip[r.AccountID] = true
	}
	for _, a := range accounts {
		if a.Type == "asset" && !skip[a.ID] {
			cash[a.ID] = true
		}
	}
	return cash
}

// applyProfile folds the user's corrections into the ledger baseline.
//
// The plan is built on this version rather than on ComputeBaseline's: an
// override the engine never reads is one the user was shown and is ignored,
// which is worse than not offering it. Surplus is recomputed from an overridden
// income for the same reason the original did -- every timeline runs off it.
func applyProfile(
	b Baseline, p *domain.WealthProfile, accounts []domain.PlanningAccount, cash map[string]bool,
) Baseline {
	var liquid money.Money
	for _, a := range accounts {
		if cash[a.ID] {
			liquid += a.Balance
		}
	}
	b.LiquidAssets = liquid

	if p.OverrideMonthlyIncome != nil {
		b.MonthlyIncome = *p.OverrideMonthlyIncome
		b.MonthlySurplus = b.MonthlyIncome - b.MonthlyOutflow
	}
	if p.OverrideEssentialExpenses != nil {
		b.EssentialMonthly = *p.OverrideEssentialExpenses
	}
	return b
}
