package service

import (
	"testing"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Only the calendar month now falls in is partial. The old rule dropped the
// last month with any activity, so a quiet start to the month -- or accounts
// that stopped syncing -- cost the baseline a finished month.
func TestCompleteMonthsDropsOnlyTheCurrentMonth(t *testing.T) {
	now := time.Date(2026, time.September, 24, 18, 0, 0, 0, time.UTC)
	month := func(m time.Month, active bool) domain.PlanningMonth {
		pm := domain.PlanningMonth{Month: time.Date(2026, m, 1, 0, 0, 0, 0, time.UTC)}
		if active {
			pm.Income, pm.Outflow = 600_000, 450_000
		}
		return pm
	}
	names := func(months []domain.PlanningMonth) []string {
		out := []string{}
		for _, m := range months {
			out = append(out, m.Month.Month().String()[:3])
		}
		return out
	}

	cases := []struct {
		name   string
		months []domain.PlanningMonth
		want   []string
	}{
		{"this month under way", []domain.PlanningMonth{
			month(time.July, true), month(time.August, true), month(time.September, true),
		}, []string{"Jul", "Aug"}},
		{"nothing posted yet this month", []domain.PlanningMonth{
			month(time.July, true), month(time.August, true), month(time.September, false),
		}, []string{"Jul", "Aug"}},
		{"syncing stopped in June", []domain.PlanningMonth{
			month(time.May, true), month(time.June, true), month(time.July, false),
			month(time.August, false), month(time.September, false),
		}, []string{"May", "Jun"}},
		{"this month is all there is", []domain.PlanningMonth{
			month(time.August, false), month(time.September, true),
		}, []string{"Sep"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := names(completeMonths(c.months, now))
			if len(got) != len(c.want) {
				t.Fatalf("got %v want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("got %v want %v", got, c.want)
				}
			}
		})
	}
}

// The accounts behind the source document: three cash accounts, a card, and
// the long-term money the old liquid figure counted as an emergency fund.
func personaAccounts() []domain.PlanningAccount {
	role := func(r string) *string { return &r }
	return []domain.PlanningAccount{
		{ID: "checking", Name: "BofA Checking", Type: "asset", Role: role(domain.RoleChecking), Balance: 250_000},
		{ID: "savings", Name: "BofA Savings", Type: "asset", Role: role(domain.RoleSavings), Balance: 583_622},
		{ID: "hysa", Name: "Ally HYSA", Type: "asset", Role: role(domain.RoleSavings), Balance: 623_614},
		{ID: "401k", Name: "Fidelity 401(k)", Type: "asset", Role: role(domain.RoleInvestment), Balance: 4_000_000},
		{ID: "home", Name: "Home", Type: "asset", Role: role(domain.RoleProperty), Balance: 50_000_000},
		{ID: "chase", Name: "Chase Sapphire", Type: "liability", Role: role(domain.RoleCreditCard), Balance: -382_435},
	}
}

func liquidOf(accounts []domain.PlanningAccount, cash map[string]bool) money.Money {
	b := applyProfile(Baseline{}, &domain.WealthProfile{}, accounts, cash)
	return b.LiquidAssets
}

// Only checking and savings are cash. A 401(k) is not an emergency fund, and
// counting it called the cushion finished before a dollar of cash had been set
// aside and unlocked phase 2 on money that cannot be spent without a penalty.
func TestCashIsCheckingAndSavingsOnly(t *testing.T) {
	accounts := personaAccounts()
	cash := cashAccountIDs(accounts)

	for _, id := range []string{"checking", "savings", "hysa"} {
		if !cash[id] {
			t.Errorf("%s should count as cash", id)
		}
	}
	for _, id := range []string{"401k", "home", "chase"} {
		if cash[id] {
			t.Errorf("%s should not count as cash", id)
		}
	}
	if got, want := liquidOf(accounts, cash), money.Money(250_000+583_622+623_614); got != want {
		t.Errorf("liquid: got %d want %d", got, want)
	}
}

// An account nobody has classified counts as nothing: guessing it is cash is
// how retirement money became an emergency fund in the first place.
func TestUnclassifiedAccountsAreNotCash(t *testing.T) {
	accounts := append(personaAccounts(),
		domain.PlanningAccount{ID: "mystery", Name: "Homestead CU", Type: "asset", Balance: 900_000})
	if cashAccountIDs(accounts)["mystery"] {
		t.Error("an unclassified account should not count as cash")
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

// The keyword rules behind a new SimpleFin account, checked against the names
// the backfill was checked against.
func TestInferAccountKindFromNameAndBalance(t *testing.T) {
	tests := []struct {
		name     string
		balance  money.Money
		wantType string
		wantRole string // "" means unclassified
	}{
		{"BofA Advantage Checking", 250_000, "asset", domain.RoleChecking},
		{"Overdrawn Checking", -5_000, "asset", domain.RoleChecking},
		{"Ally Online Savings", 623_614, "asset", domain.RoleSavings},
		{"Ally HYSA", 623_614, "asset", domain.RoleSavings},
		{"Fidelity 401(k)", 4_000_000, "asset", domain.RoleInvestment},
		{"Roth IRA", 1_000_000, "asset", domain.RoleInvestment},
		{"Health Savings Account", 300_000, "asset", domain.RoleInvestment},
		{"Individual Brokerage", 1_500_000, "asset", domain.RoleInvestment},
		{"Irania Travel Fund", 90_000, "asset", ""},
		{"Homestead Credit Union", 90_000, "asset", ""},
		{"Home", 50_000_000, "asset", domain.RoleProperty},
		{"Chase Sapphire Preferred", -382_435, "liability", domain.RoleCreditCard},
		{"Chase Sapphire Preferred", 0, "asset", ""},
		{"Discover it Card", 0, "liability", domain.RoleCreditCard},
		{"Toyota Financial Services", -1_800_000, "liability", domain.RoleLoan},
		{"Nelnet Student Loan", -2_400_000, "liability", domain.RoleLoan},
		{"Rocket Mortgage", -40_000_000, "liability", domain.RoleMortgage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotRole := inferAccountKind(tc.name, tc.balance)
			role := ""
			if gotRole != nil {
				role = *gotRole
			}
			if gotType != tc.wantType || role != tc.wantRole {
				t.Errorf("got %s/%q, want %s/%q", gotType, role, tc.wantType, tc.wantRole)
			}
			if gotRole != nil {
				if side, _ := domain.AccountTypeForRole(*gotRole); side != gotType {
					t.Errorf("role %s belongs on the %s side, not %s", *gotRole, side, gotType)
				}
			}
		})
	}
}
