package service

import (
	"testing"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// The accounts behind the source document: three cash accounts, a card, and
// the long-term money the old liquid figure counted as an emergency fund.
func personaAccounts() []domain.PlanningAccount {
	return []domain.PlanningAccount{
		{ID: "checking", Name: "BofA Checking", Type: "asset", Balance: 250_000},
		{ID: "savings", Name: "BofA Savings", Type: "asset", Balance: 583_622},
		{ID: "hysa", Name: "Ally HYSA", Type: "asset", Balance: 623_614},
		{ID: "401k", Name: "Fidelity 401(k)", Type: "asset", Balance: 4_000_000},
		{ID: "brokerage", Name: "Fidelity Investment", Type: "asset", Balance: 1_500_000},
		{ID: "chase", Name: "Chase Sapphire", Type: "liability", Balance: -382_435},
	}
}

func liquidOf(accounts []domain.PlanningAccount, cash map[string]bool) money.Money {
	b := applyProfile(Baseline{}, &domain.WealthProfile{}, accounts, cash)
	return b.LiquidAssets
}

// A 401(k) is not an emergency fund. Counting it called the cushion finished
// before a dollar of cash had been set aside, and unlocked phase 2 on the back
// of money that cannot be spent without a penalty.
func TestCashLeavesOutAccountsLinkedOnTheRetirementTab(t *testing.T) {
	accounts := personaAccounts()
	invested := []domain.RetirementAccountTerms{
		{AccountID: "401k", Kind: domain.RetirementKind401k},
		{AccountID: "brokerage", Kind: domain.RetirementKindTaxable},
	}

	cash := cashAccountIDs(accounts, nil, invested)

	for _, id := range []string{"checking", "savings", "hysa"} {
		if !cash[id] {
			t.Errorf("%s should count as cash", id)
		}
	}
	for _, id := range []string{"401k", "brokerage", "chase"} {
		if cash[id] {
			t.Errorf("%s should not count as cash", id)
		}
	}
	if got, want := liquidOf(accounts, cash), money.Money(250_000+583_622+623_614); got != want {
		t.Errorf("liquid: got %d want %d", got, want)
	}
}

// Without anything linked, every asset still counts. That is the old figure,
// and the Cash page says so beside the number.
func TestCashDefaultsToEveryAssetWhenNothingIsLinked(t *testing.T) {
	cash := cashAccountIDs(personaAccounts(), nil, nil)
	if len(cash) != 5 {
		t.Errorf("expected all five asset accounts, got %v", cash)
	}
	if cash["chase"] {
		t.Error("a liability is never cash")
	}
}

// The user's own pick replaces the default outright, and only picks asset
// accounts: a stale or mistaken card id cannot turn debt into a cushion.
func TestChosenCashAccountsReplaceTheDefault(t *testing.T) {
	accounts := personaAccounts()
	invested := []domain.RetirementAccountTerms{{AccountID: "401k", Kind: domain.RetirementKind401k}}

	cash := cashAccountIDs(accounts, []string{"hysa", "chase", "gone"}, invested)

	if len(cash) != 1 || !cash["hysa"] {
		t.Errorf("only the chosen asset account should count, got %v", cash)
	}
	if got := liquidOf(accounts, cash); got != 623_614 {
		t.Errorf("liquid: got %d want 623614", got)
	}
}

// Overrides the engine never read were stored and shown but changed nothing.
// Surplus follows an overridden income, since every timeline runs off it.
func TestIncomeAndEssentialOverridesReachThePlan(t *testing.T) {
	b := Baseline{
		MonthlyIncome:    700_000,
		MonthlyOutflow:   500_000,
		EssentialMonthly: 300_000,
		MonthlySurplus:   200_000,
	}

	t.Run("no overrides leaves the ledger figures alone", func(t *testing.T) {
		got := applyProfile(b, &domain.WealthProfile{}, nil, nil)
		if got.MonthlyIncome != 700_000 || got.EssentialMonthly != 300_000 || got.MonthlySurplus != 200_000 {
			t.Errorf("figures moved without an override: %+v", got)
		}
	})

	t.Run("income and essentials are replaced", func(t *testing.T) {
		p := &domain.WealthProfile{
			OverrideMonthlyIncome:     ptr(money.Money(682_360)),
			OverrideEssentialExpenses: ptr(money.Money(443_538)),
		}
		got := applyProfile(b, p, nil, nil)
		if got.MonthlyIncome != 682_360 {
			t.Errorf("income: got %d want 682360", got.MonthlyIncome)
		}
		if got.EssentialMonthly != 443_538 {
			t.Errorf("essentials: got %d want 443538", got.EssentialMonthly)
		}
		if got.MonthlySurplus != 682_360-500_000 {
			t.Errorf("surplus should follow the overridden income: got %d", got.MonthlySurplus)
		}
	})
}
