package domain

import (
	"time"

	"ntdkhiem/ppbudget-go/pkg/money"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Account struct {
	ID             string      `json:"id"`
	UserID         string      `json:"user_id"`
	Name           string      `json:"name"`
	Type           string      `json:"type"`
	Currency       string      `json:"currency"`
	InitialBalance money.Money `json:"initial_balance"`
	CurrentBalance money.Money `json:"current_balance"`
	SimplefinID    *string     `json:"simplefin_id,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type Transaction struct {
	ID                 string            `json:"id"`
	UserID             string            `json:"user_id"`
	AccountID          string            `json:"account_id"`
	CategoryID         *string           `json:"category_id,omitempty"`
	Amount             money.Money       `json:"amount"`
	Date               time.Time         `json:"date"`
	Description        string            `json:"description"`
	Notes              *string           `json:"notes,omitempty"`
	IsReviewed         bool              `json:"is_reviewed"`
	IsReconciled       bool              `json:"is_reconciled"`
	SimplefinAccountID *string           `json:"simplefin_account_id,omitempty"`
	SubscriptionID     *string           `json:"subscription_id,omitempty"`
	PaysFor            []TransactionLink `json:"pays_for,omitempty"`
	PaidBy             []TransactionLink `json:"paid_by,omitempty"`
	EffectiveAmount    money.Money       `json:"effective_amount"`
}

type TransactionLink struct {
	TransactionID string      `json:"transaction_id"`
	Amount        money.Money `json:"amount"`
	Description   string      `json:"description,omitempty"`
	Date          time.Time   `json:"date,omitempty"`
}

type TransactionWithBalance struct {
	Transaction
	RunningBalance money.Money `json:"running_balance"`
}

type Category struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Name             string    `json:"name"`
	Type             string    `json:"type"`
	TransactionCount int       `json:"transaction_count"`
	CreatedAt        time.Time `json:"created_at"`
}

type Rule struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	TriggerType string          `json:"trigger_type"`
	Strictness  string          `json:"strictness"`
	Priority    int             `json:"priority"`
	IsActive    bool            `json:"is_active"`
	Conditions  []RuleCondition `json:"conditions"`
	Actions     []RuleAction    `json:"actions"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type RuleCondition struct {
	ID       string `json:"id"`
	RuleID   string `json:"rule_id"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type RuleAction struct {
	ID         string `json:"id"`
	RuleID     string `json:"rule_id"`
	ActionType string `json:"action_type"`
	Value      string `json:"value"`
}

type Budget struct {
	ID         string      `json:"id"`
	UserID     string      `json:"user_id"`
	Name       string      `json:"name"`
	CategoryID string      `json:"category_id"`
	Amount     money.Money `json:"amount_cents"`
	PeriodType string      `json:"period_type"`
	StartDate  time.Time   `json:"start_date"`
	EndDate    time.Time   `json:"end_date"`
	Bucket     string      `json:"bucket"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

type BudgetSummary struct {
	Budget
	SpentTotal  money.Money `json:"spent_total"`
	SpentPerDay money.Money `json:"spent_per_day"`
	LeftTotal   money.Money `json:"left_total"`
	LeftPerDay  money.Money `json:"left_per_day"`
}

type CategorySpend struct {
	CategoryID string      `json:"category_id"`
	Name       string      `json:"name"`
	TotalSpent money.Money `json:"total_spent"`
}

type NetWorthPoint struct {
	Month       time.Time   `json:"month"`
	Assets      money.Money `json:"assets"`
	Liabilities money.Money `json:"liabilities"`
	NetWorth    money.Money `json:"net_worth"`
}

type ReportsSummary struct {
	InPeriod           money.Money `json:"in_period"`
	OutPeriod          money.Money `json:"out_period"`
	SubscriptionsToPay money.Money `json:"subscriptions_to_pay"`
	SubscriptionsPaid  money.Money `json:"subscriptions_paid"`
	LeftToSpend        money.Money `json:"left_to_spend"`
	NetWorth           money.Money `json:"net_worth"`
}

type Subscription struct {
	ID              string      `json:"id"`
	UserID          string      `json:"user_id"`
	Name            string      `json:"name"`
	Amount          money.Money `json:"amount"`
	BillingCycle    string      `json:"billing_cycle"`
	NextBillingDate time.Time   `json:"next_billing_date"`
	CategoryID      *string     `json:"category_id,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type SearchResult struct {
	Transactions  []Transaction  `json:"transactions"`
	Categories    []Category     `json:"categories"`
	Accounts      []Account      `json:"accounts"`
	Subscriptions []Subscription `json:"subscriptions"`
}
