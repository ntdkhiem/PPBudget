package repository

import (
	"context"
	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"
)

func (r *Repository) SearchTransactions(ctx context.Context, userID string, query string, limit int) ([]domain.Transaction, error) {
	q := `
        SELECT 
            t.id, t.user_id, t.account_id, t.category_id, t.amount, t.date, t.description, 
            t.notes, t.is_reviewed, t.is_reconciled, t.transfer_id, t.subscription_id, a.simplefin_id as simplefin_account_id
        FROM transactions t
        JOIN accounts a ON t.account_id = a.id
        WHERE t.user_id = $1 AND t.deleted_at IS NULL AND t.search_vector @@ websearch_to_tsquery('english', $2)
		ORDER BY ts_rank(t.search_vector, websearch_to_tsquery('english', $2)) DESC
		LIMIT $3
    `
	rows, err := r.pool.Query(ctx, q, userID, query, limit)
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
			&t.ID, &t.UserID, &t.AccountID, &catID, &amount, &t.Date, &t.Description,
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

func (r *Repository) SearchCategories(ctx context.Context, userID string, query string, limit int) ([]domain.Category, error) {
	q := `
		SELECT id, user_id, name, type, 
			(SELECT count(*) FROM transactions WHERE category_id = categories.id AND deleted_at IS NULL AND user_id = $1) as transaction_count, 
			created_at
		FROM categories 
		WHERE user_id = $1 AND name ILIKE '%' || $2 || '%'
		ORDER BY name <-> $2
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, q, userID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.TransactionCount, &c.CreatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (r *Repository) SearchAccounts(ctx context.Context, userID string, query string, limit int) ([]domain.Account, error) {
	q := `
		SELECT a.id, a.user_id, a.name, a.type, a.currency, a.initial_balance, a.simplefin_id, a.created_at, a.updated_at,
		       COALESCE(SUM(t.amount), 0) + a.initial_balance as current_balance
		FROM accounts a
		LEFT JOIN transactions t ON a.id = t.account_id AND t.deleted_at IS NULL AND t.user_id = $1
		WHERE a.user_id = $1 AND a.name ILIKE '%' || $2 || '%'
		GROUP BY a.id
		ORDER BY a.name <-> $2
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, q, userID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		var a domain.Account
		var balance, currentBalance int64
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.Currency, &balance, &a.SimplefinID, &a.CreatedAt, &a.UpdatedAt, &currentBalance); err != nil {
			return nil, err
		}
		a.InitialBalance = money.Money(balance)
		a.CurrentBalance = money.Money(currentBalance)
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (r *Repository) SearchSubscriptions(ctx context.Context, userID string, query string, limit int) ([]domain.Subscription, error) {
	q := `
		SELECT id, user_id, name, amount, billing_cycle, next_billing_date, category_id, created_at, updated_at
		FROM subscriptions 
		WHERE user_id = $1 AND name ILIKE '%' || $2 || '%'
		ORDER BY name <-> $2
		LIMIT $3
	`
	rows, err := r.pool.Query(ctx, q, userID, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []domain.Subscription
	for rows.Next() {
		var s domain.Subscription
		var amount int64
		if err := rows.Scan(&s.ID, &s.UserID, &s.Name, &amount, &s.BillingCycle, &s.NextBillingDate, &s.CategoryID, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		s.Amount = money.Money(amount)
		subs = append(subs, s)
	}
	return subs, nil
}
