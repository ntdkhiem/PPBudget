package service

import (
	"regexp"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Account role inference for accounts created by a SimpleFin sync.
//
// SimpleFin reports a name and a balance and nothing about what the account
// is, which is how every card used to arrive as an asset. The keywords are the
// ones migration 0030 backfilled existing accounts with, and the balance sign
// stands in for the type a feed does not report. One deliberate difference:
// the backfill kept unplaceable assets counting as cash, as they already did,
// while a new account nothing matches is left unclassified and the plan asks
// about it rather than guessing.

var (
	investmentName = regexp.MustCompile(`(?i)(401|403\s*\(?b|457|\bira\b|roth|\bhsa\b|health\s*sav|brokerage|invest|retire|pension|\btsp\b|529)`)
	checkingName   = regexp.MustCompile(`(?i)(checking|\bchk\b)`)
	savingsName    = regexp.MustCompile(`(?i)(saving|hysa|high[\s-]*yield|money\s*market|\bcd\b)`)
	propertyName   = regexp.MustCompile(`(?i)(\bhouse\b|\bhome\b|property|real\s*estate|vehicle|\bcar\b)`)
	mortgageName   = regexp.MustCompile(`(?i)(mortgage|home\s*loan|heloc)`)
	loanName       = regexp.MustCompile(`(?i)(loan|lending|financ|\bauto\b|\bcar\b|student)`)
	// "credit card", not "credit": a credit union is a bank.
	cardName = regexp.MustCompile(`(?i)(credit\s*card|\bcard\b|visa|mastercard|amex|american express|discover)`)
)

// inferAccountKind guesses an account's type and role from its name and
// balance.
//
// Names win over the balance sign, since an overdrawn checking account is
// still a checking account. A negative balance on a name nothing matches is
// taken as a card, the commonest debt a bank feed carries. Investment
// keywords are checked before savings: a "Health Savings Account" is an HSA.
func inferAccountKind(name string, balance money.Money) (accType string, role *string) {
	pick := func(t, r string) (string, *string) { return t, &r }

	switch {
	case mortgageName.MatchString(name):
		return pick("liability", domain.RoleMortgage)
	case investmentName.MatchString(name):
		return pick("asset", domain.RoleInvestment)
	case checkingName.MatchString(name):
		return pick("asset", domain.RoleChecking)
	case savingsName.MatchString(name):
		return pick("asset", domain.RoleSavings)
	case cardName.MatchString(name):
		return pick("liability", domain.RoleCreditCard)
	case balance < 0 && loanName.MatchString(name):
		return pick("liability", domain.RoleLoan)
	case propertyName.MatchString(name):
		return pick("asset", domain.RoleProperty)
	case balance < 0:
		return pick("liability", domain.RoleCreditCard)
	}
	return "asset", nil
}
