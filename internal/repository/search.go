package repository

import (
	"context"
	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

func (r *Repository) SearchTransactions(ctx context.Context, query string, limit int) ([]domain.Transaction, error) {
	q := `
        SELECT 
            t.id, t.account_id, t.category_id, t.amount, t.date, t.description, 
            t.notes, t.is_reviewed, t.is_reconciled, t.transfer_id, t.subscription_id, a.simplefin_id as simplefin_account_id
        FROM transactions t
        JOIN accounts a ON t.account_id = a.id
        WHERE t.deleted_at IS NULL AND t.search_vector @@ websearch_to_tsquery('english', $1)
		ORDER BY ts_rank(t.search_vector, websearch_to_tsquery('english', $1)) DESC
		LIMIT $2
    `
	rows, err := r.pool.Query(ctx, q, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		var amount int64
		var catID *string
		var notes *string
		var transferID *string
		var sfAccountID *string
		var subID *string

		err := rows.Scan(
			&t.ID, &t.AccountID, &catID, &amount, &t.Date, &t.Description,
			&notes, &t.IsReviewed, &t.IsReconciled, &transferID, &subID, &sfAccountID,
		)
		if err != nil {
			return nil, err
		}

		t.Amount = money.Money(amount)
		t.CategoryID = catID
		t.Notes = notes
		t.TransferID = transferID
		t.SimplefinAccountID = sfAccountID
		t.SubscriptionID = subID
		txns = append(txns, t)
	}
	return txns, nil
}

func (r *Repository) SearchCategories(ctx context.Context, query string, limit int) ([]domain.Category, error) {
	q := `
		SELECT id, name, type, 
			(SELECT count(*) FROM transactions WHERE category_id = categories.id) as transaction_count, 
			created_at
		FROM categories 
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY name <-> $1
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, q, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.TransactionCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (r *Repository) SearchAccounts(ctx context.Context, query string, limit int) ([]domain.Account, error) {
	q := `
		SELECT id, name, type, currency, initial_balance, simplefin_id, created_at, updated_at
		FROM accounts 
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY name <-> $1
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, q, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		var a domain.Account
		var bal int64
		var sfID *string
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Currency, &bal, &sfID, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.InitialBalance = money.Money(bal)
		a.SimplefinID = sfID
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (r *Repository) SearchSubscriptions(ctx context.Context, query string, limit int) ([]domain.Subscription, error) {
	q := `
		SELECT id, name, amount, billing_cycle, next_billing_date, category_id, created_at, updated_at
		FROM subscriptions 
		WHERE name ILIKE '%' || $1 || '%'
		ORDER BY name <-> $1
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, q, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		var amount int64
		if err := rows.Scan(&s.ID, &s.Name, &amount, &s.BillingCycle, &s.NextBillingDate, &s.CategoryID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.Amount = money.Money(amount)
		subs = append(subs, s)
	}
	return subs, nil
}
