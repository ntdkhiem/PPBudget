package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ntdkhiem/ppbudget-go/internal/domain"
	"ntdkhiem/ppbudget-go/pkg/money"

	apperrors "ntdkhiem/ppbudget-go/internal/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) checkOwnership(ctx context.Context, tx pgx.Tx, table, id, userID string) error {
	if id == "" {
		return nil
	}
	query := fmt.Sprintf("SELECT 1 FROM %s WHERE id = $1 AND user_id = $2", table)
	var dummy int
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, query, id, userID).Scan(&dummy)
	} else {
		err = r.pool.QueryRow(ctx, query, id, userID).Scan(&dummy)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return apperrors.ErrForbidden
		}
		return err
	}
	return nil
}

// BeginTx starts a new database transaction
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// GetAccountBySimplefinID fetches an account by its importer ID scoped by userID
func (r *Repository) GetAccountBySimplefinID(ctx context.Context, tx pgx.Tx, userID, simplefinID string) (string, error) {
	var accountID string
	query := `SELECT id FROM accounts WHERE simplefin_id = $1 AND user_id = $2`

	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, query, simplefinID, userID)
	} else {
		row = r.pool.QueryRow(ctx, query, simplefinID, userID)
	}

	err := row.Scan(&accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.ErrNotFound
		}
		return "", fmt.Errorf("failed to get account: %w", err)
	}
	return accountID, nil
}

// InsertIngestedTransaction inserts a transaction, returning the ID, and true if created, false if it was a duplicate
func (r *Repository) InsertIngestedTransaction(ctx context.Context, tx pgx.Tx, userID, accountID string, amount money.Money, date time.Time, description, simplefinTxID string, categoryID *string, subscriptionID *string, isReviewed bool) (string, bool, error) {
	query := `
		INSERT INTO transactions (account_id, amount, date, description, simplefin_id, category_id, subscription_id, is_reviewed, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (simplefin_id) DO NOTHING
		RETURNING id
	`
	args := []interface{}{accountID, amount.ToInt64(), date, description, simplefinTxID, categoryID, subscriptionID, isReviewed, userID}

	var id string
	var err error
	if tx != nil {
		err = tx.QueryRow(ctx, query, args...).Scan(&id)
	} else {
		err = r.pool.QueryRow(ctx, query, args...).Scan(&id)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", false, nil // Duplicate safely ignored
		}
		return "", false, fmt.Errorf("failed to insert transaction: %w", err)
	}
	return id, true, nil
}

func (r *Repository) ListTransactions(ctx context.Context, f domain.TransactionFilter) ([]domain.TransactionWithBalance, error) {
	// Base query
	query := `
        SELECT 
            t.id, t.user_id, t.account_id, t.category_id, t.amount, t.date, t.description, 
            t.notes, t.is_reviewed, t.is_reconciled, t.subscription_id,
            (a.initial_balance + SUM(t.amount) OVER (PARTITION BY t.account_id ORDER BY t.date, t.id)) as running_balance,
            (t.amount 
             - COALESCE(pays_for_agg.total_amount, 0)
             + COALESCE(paid_by_agg.total_amount, 0)
            ) as effective_amount,
            COALESCE(pays_for_agg.json_data, '[]'::json) as pays_for,
            COALESCE(paid_by_agg.json_data, '[]'::json) as paid_by
        FROM transactions t
        JOIN accounts a ON t.account_id = a.id
        LEFT JOIN LATERAL (
            SELECT SUM(l.amount) as total_amount,
                   json_agg(json_build_object(
                       'transaction_id', l.target_transaction_id, 
                       'amount', l.amount,
                       'description', tt.description,
                       'date', to_char(tt.date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
                   )) as json_data
            FROM transaction_links l
            JOIN transactions tt ON l.target_transaction_id = tt.id
            WHERE l.source_transaction_id = t.id
        ) pays_for_agg ON true
        LEFT JOIN LATERAL (
            SELECT SUM(l.amount) as total_amount,
                   json_agg(json_build_object(
                       'transaction_id', l.source_transaction_id, 
                       'amount', l.amount,
                       'description', st.description,
                       'date', to_char(st.date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
                   )) as json_data
            FROM transaction_links l
            JOIN transactions st ON l.source_transaction_id = st.id
            WHERE l.target_transaction_id = t.id
        ) paid_by_agg ON true
        WHERE t.user_id = $1 AND t.deleted_at IS NULL
    `
	args := []interface{}{f.UserID}
	argID := 2

	if f.AccountID != "" {
		query += fmt.Sprintf(" AND t.account_id = $%d", argID)
		args = append(args, f.AccountID)
		argID++
	} else if len(f.AccountIDs) > 0 {
		query += fmt.Sprintf(" AND t.account_id = ANY($%d)", argID)
		args = append(args, f.AccountIDs)
		argID++
	}

	if len(f.CategoryIDs) > 0 {
		query += fmt.Sprintf(" AND t.category_id = ANY($%d)", argID)
		args = append(args, f.CategoryIDs)
		argID++
	}

	if f.Type == "Income" {
		query += ` AND t.amount > 0 AND (t.category_id IS NULL OR t.category_id NOT IN (SELECT id FROM categories WHERE type = 'transfer'))`
	} else if f.Type == "Expense" {
		query += ` AND t.amount < 0 AND (t.category_id IS NULL OR t.category_id NOT IN (SELECT id FROM categories WHERE type = 'transfer'))`
	} else if f.Type == "Transfer" {
		query += ` AND t.category_id IN (SELECT id FROM categories WHERE type = 'transfer')`
	}

	if f.CursorDate != nil && f.CursorID != nil {
		query += fmt.Sprintf(" AND (t.date, t.id) < ($%d, $%d)", argID, argID+1)
		args = append(args, *f.CursorDate, *f.CursorID)
		argID += 2
	}

	if f.UnreviewedOnly {
		query += " AND t.is_reviewed = false"
	}

	if f.StartDate != nil {
		query += fmt.Sprintf(" AND t.date >= $%d", argID)
		args = append(args, *f.StartDate)
		argID++
	}

	if f.EndDate != nil {
		query += fmt.Sprintf(" AND t.date <= $%d", argID)
		args = append(args, *f.EndDate)
		argID++
	}

	if f.Search != "" {
		query += fmt.Sprintf(" AND (t.search_vector @@ websearch_to_tsquery('english', $%d) OR t.description ILIKE '%%' || $%d || '%%' OR t.notes ILIKE '%%' || $%d || '%%' OR t.amount::text ILIKE '%%' || $%d || '%%')", argID, argID, argID, argID)
		args = append(args, f.Search)
		argID++
	}

	query += " ORDER BY t.date DESC, t.id DESC LIMIT 100"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []domain.TransactionWithBalance
	for rows.Next() {
		var t domain.TransactionWithBalance
		var amount int64
		var balance int64
		var catID *string
		var notes *string

		var subID *string
		var effectiveAmount int64
		var paysForJson []byte
		var paidByJson []byte

		err := rows.Scan(
			&t.ID, &t.UserID, &t.AccountID, &catID, &amount, &t.Date, &t.Description,
			&notes, &t.IsReviewed, &t.IsReconciled, &subID,
			&balance, &effectiveAmount, &paysForJson, &paidByJson,
		)
		if err != nil {
			return nil, err
		}

		t.Amount = money.Money(amount)
		t.RunningBalance = money.Money(balance)
		t.CategoryID = catID
		t.Notes = notes

		t.SubscriptionID = subID
		t.EffectiveAmount = money.Money(effectiveAmount)
		if err := json.Unmarshal(paysForJson, &t.PaysFor); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pays_for: %w", err)
		}
		if err := json.Unmarshal(paidByJson, &t.PaidBy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal paid_by: %w", err)
		}
		txns = append(txns, t)
	}
	return txns, nil
}

// GetTransactionsByDateRange fetches all transactions (or bounded by dates) without limits
func (r *Repository) GetTransactionsByDateRange(ctx context.Context, userID string, startDate, endDate *time.Time) ([]domain.Transaction, error) {
	query := `
        SELECT 
            t.id, t.user_id, t.account_id, t.category_id, t.amount, t.date, t.description, 
            t.notes, t.is_reviewed, t.is_reconciled, t.subscription_id, a.simplefin_id as simplefin_account_id,
            (t.amount 
             - COALESCE((SELECT SUM(amount) FROM transaction_links WHERE source_transaction_id = t.id), 0)
             + COALESCE((SELECT SUM(amount) FROM transaction_links WHERE target_transaction_id = t.id), 0)
            ) as effective_amount,
            COALESCE((
                SELECT json_agg(json_build_object(
                    'transaction_id', l.target_transaction_id, 
                    'amount', l.amount,
                    'description', tt.description,
                    'date', to_char(tt.date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
                ))
                FROM transaction_links l
                JOIN transactions tt ON l.target_transaction_id = tt.id
                WHERE l.source_transaction_id = t.id
            ), '[]'::json) as pays_for,
            COALESCE((
                SELECT json_agg(json_build_object(
                    'transaction_id', l.source_transaction_id, 
                    'amount', l.amount,
                    'description', st.description,
                    'date', to_char(st.date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
                ))
                FROM transaction_links l
                JOIN transactions st ON l.source_transaction_id = st.id
                WHERE l.target_transaction_id = t.id
            ), '[]'::json) as paid_by
        FROM transactions t
        JOIN accounts a ON t.account_id = a.id
        WHERE t.user_id = $1 AND t.deleted_at IS NULL
          AND ($2::date IS NULL OR t.date >= $2::date)
          AND ($3::date IS NULL OR t.date <= $3::date)
    `

	rows, err := r.pool.Query(ctx, query, userID, startDate, endDate)
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

		var sfAccountID *string
		var subID *string
		var effectiveAmount int64
		var paysForJson []byte
		var paidByJson []byte

		err := rows.Scan(
			&t.ID, &t.UserID, &t.AccountID, &catID, &amount, &t.Date, &t.Description,
			&notes, &t.IsReviewed, &t.IsReconciled, &subID, &sfAccountID,
			&effectiveAmount, &paysForJson, &paidByJson,
		)
		if err != nil {
			return nil, err
		}

		t.Amount = money.Money(amount)
		t.CategoryID = catID
		t.Notes = notes

		t.SimplefinAccountID = sfAccountID
		t.SubscriptionID = subID
		t.EffectiveAmount = money.Money(effectiveAmount)
		if err := json.Unmarshal(paysForJson, &t.PaysFor); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pays_for: %w", err)
		}
		if err := json.Unmarshal(paidByJson, &t.PaidBy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal paid_by: %w", err)
		}
		txns = append(txns, t)
	}
	return txns, nil
}

// MarkReviewed updates the category and marks the transaction as reviewed
func (r *Repository) MarkReviewed(ctx context.Context, userID, txnID string, categoryID *string) error {
	var query string
	var args []interface{}

	if categoryID != nil {
		query = `
			UPDATE transactions 
			SET is_reviewed = true, category_id = $3, updated_at = NOW()
			WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		`
		args = []interface{}{txnID, userID, categoryID}
	} else {
		query = `
			UPDATE transactions 
			SET is_reviewed = true, updated_at = NOW()
			WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		`
		args = []interface{}{txnID, userID}
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ListAccounts fetches all accounts for a user
func (r *Repository) ListAccounts(ctx context.Context, userID string) ([]domain.Account, error) {
	query := `
		SELECT a.id, a.user_id, a.name, a.type, a.currency, a.initial_balance, a.simplefin_id, a.created_at, a.updated_at,
		       COALESCE(SUM(t.amount), 0) + a.initial_balance as current_balance
		FROM accounts a
		LEFT JOIN transactions t ON a.id = t.account_id AND t.deleted_at IS NULL AND t.user_id = $1
		WHERE a.user_id = $1
		GROUP BY a.id
		ORDER BY a.name ASC
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		var a domain.Account
		var balance, currentBalance int64
		err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.Currency, &balance, &a.SimplefinID, &a.CreatedAt, &a.UpdatedAt, &currentBalance)
		if err != nil {
			return nil, err
		}
		a.InitialBalance = money.Money(balance)
		a.CurrentBalance = money.Money(currentBalance)
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// Categories

func (r *Repository) CreateCategory(ctx context.Context, userID, name, catType string) error {
	query := `INSERT INTO categories (name, type, user_id) VALUES ($1, $2, $3)`
	_, err := r.pool.Exec(ctx, query, name, catType, userID)
	return err
}

func (r *Repository) UpdateCategory(ctx context.Context, userID, id, name, catType string) error {
	query := `UPDATE categories SET name = $1, type = $2 WHERE id = $3 AND user_id = $4`
	tag, err := r.pool.Exec(ctx, query, name, catType, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteCategory(ctx context.Context, userID, id string) error {
	query := `DELETE FROM categories WHERE id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) ListCategories(ctx context.Context, userID string) ([]domain.Category, error) {
	query := `
		SELECT c.id, c.user_id, c.name, c.type, c.created_at, COUNT(t.id) as transaction_count
		FROM categories c
		LEFT JOIN transactions t ON c.id = t.category_id AND t.deleted_at IS NULL AND t.user_id = $1
		WHERE c.user_id = $1
		GROUP BY c.id
		ORDER BY c.name ASC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &c.CreatedAt, &c.TransactionCount); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

// Accounts

func (r *Repository) CreateAccount(ctx context.Context, userID, name, accType, currency string, initialBalance int64) error {
	query := `INSERT INTO accounts (name, type, currency, initial_balance, user_id) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.pool.Exec(ctx, query, name, accType, currency, initialBalance, userID)
	return err
}

func (r *Repository) GetAccount(ctx context.Context, userID, id string) (*domain.Account, error) {
	query := `
		SELECT a.id, a.user_id, a.name, a.type, a.currency, a.initial_balance, a.simplefin_id, a.created_at, a.updated_at,
		       COALESCE(SUM(t.amount), 0) + a.initial_balance as current_balance
		FROM accounts a
		LEFT JOIN transactions t ON a.id = t.account_id AND t.deleted_at IS NULL AND t.user_id = $2
		WHERE a.id = $1 AND a.user_id = $2
		GROUP BY a.id
	`
	var a domain.Account
	var balance, currentBalance int64
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.Currency, &balance, &a.SimplefinID, &a.CreatedAt, &a.UpdatedAt, &currentBalance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	a.InitialBalance = money.Money(balance)
	a.CurrentBalance = money.Money(currentBalance)
	return &a, nil
}

func (r *Repository) UpdateAccount(ctx context.Context, userID, id, name, accType, currency string, initialBalance int64) error {
	query := `
		UPDATE accounts 
		SET name = $1, type = $2, currency = $3, initial_balance = $4, updated_at = NOW()
		WHERE id = $5 AND user_id = $6
	`
	tag, err := r.pool.Exec(ctx, query, name, accType, currency, initialBalance, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteAccount(ctx context.Context, userID, id string) error {
	query := `DELETE FROM accounts WHERE id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// Transactions (Manual)

func (r *Repository) CreateTransaction(ctx context.Context, userID, accountID string, amount int64, date time.Time, description string, notes *string, categoryID *string, subscriptionID *string) error {
	if err := r.checkOwnership(ctx, nil, "accounts", accountID, userID); err != nil {
		return err
	}
	if categoryID != nil {
		if err := r.checkOwnership(ctx, nil, "categories", *categoryID, userID); err != nil {
			return err
		}
	}
	if subscriptionID != nil {
		if err := r.checkOwnership(ctx, nil, "subscriptions", *subscriptionID, userID); err != nil {
			return err
		}
	}
	query := `
		INSERT INTO transactions (account_id, amount, date, description, notes, category_id, subscription_id, is_reviewed, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8)
	`
	// Manual transactions are considered reviewed automatically.
	_, err := r.pool.Exec(ctx, query, accountID, amount, date, description, notes, categoryID, subscriptionID, userID)
	return err
}

func (r *Repository) DeleteTransaction(ctx context.Context, userID, id string) error {
	query := `UPDATE transactions SET deleted_at = NOW() WHERE id = $1 AND user_id = $2`
	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) BulkDeleteTransactions(ctx context.Context, userID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query := `UPDATE transactions SET deleted_at = NOW() WHERE id = ANY($1) AND user_id = $2`
	_, err := r.pool.Exec(ctx, query, ids, userID)
	return err
}

func (r *Repository) BulkUpdateTransactionsCategory(ctx context.Context, userID string, ids []string, categoryID string) error {
	if len(ids) == 0 {
		return nil
	}
	var catID *string
	if categoryID != "" {
		if err := r.checkOwnership(ctx, nil, "categories", categoryID, userID); err != nil {
			return err
		}
		catID = &categoryID
	}
	query := `UPDATE transactions SET category_id = $1 WHERE id = ANY($2) AND user_id = $3 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, query, catID, ids, userID)
	return err
}

func (r *Repository) UpdateTransaction(ctx context.Context, userID, id, accountID string, amount int64, date time.Time, description string, notes *string, categoryID *string, subscriptionID *string, paysFor []domain.TransactionLink, paidBy []domain.TransactionLink) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := r.checkOwnership(ctx, tx, "accounts", accountID, userID); err != nil {
		return err
	}
	if categoryID != nil {
		if err := r.checkOwnership(ctx, tx, "categories", *categoryID, userID); err != nil {
			return err
		}
	}
	if subscriptionID != nil {
		if err := r.checkOwnership(ctx, tx, "subscriptions", *subscriptionID, userID); err != nil {
			return err
		}
	}

	query := `
		UPDATE transactions 
		SET account_id = $1, amount = $2, date = $3, description = $4, notes = $5, category_id = $6, subscription_id = $7, updated_at = NOW()
		WHERE id = $8 AND user_id = $9 AND deleted_at IS NULL
	`
	tag, err := tx.Exec(ctx, query, accountID, amount, date, description, notes, categoryID, subscriptionID, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	if paysFor != nil {
		clearQuery := `DELETE FROM transaction_links WHERE source_transaction_id = $1`
		if _, err := tx.Exec(ctx, clearQuery, id); err != nil {
			return err
		}

		for _, link := range paysFor {
			if err := r.checkOwnership(ctx, tx, "transactions", link.TransactionID, userID); err != nil {
				return err
			}
			setQuery := `INSERT INTO transaction_links (source_transaction_id, target_transaction_id, amount) VALUES ($1, $2, $3)`
			if _, err := tx.Exec(ctx, setQuery, id, link.TransactionID, link.Amount.ToInt64()); err != nil {
				return err
			}
		}
	}

	if paidBy != nil {
		clearQuery := `DELETE FROM transaction_links WHERE target_transaction_id = $1`
		if _, err := tx.Exec(ctx, clearQuery, id); err != nil {
			return err
		}

		for _, link := range paidBy {
			if err := r.checkOwnership(ctx, tx, "transactions", link.TransactionID, userID); err != nil {
				return err
			}
			setQuery := `INSERT INTO transaction_links (source_transaction_id, target_transaction_id, amount) VALUES ($1, $2, $3)`
			if _, err := tx.Exec(ctx, setQuery, link.TransactionID, id, link.Amount.ToInt64()); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

// Reports

func (r *Repository) GetNetWorthTrend(ctx context.Context, userID string, startDate, endDate time.Time) ([]domain.NetWorthPoint, error) {
	query := `
		WITH dates AS (
			SELECT (date_trunc('month', d) + INTERVAL '1 month - 1 day')::date AS end_of_month
			FROM generate_series(date_trunc('month', $2::timestamp), date_trunc('month', $3::timestamp), '1 month'::interval) d
		),
		balances AS (
			SELECT 
				d.end_of_month as month,
				a.type,
				a.initial_balance + COALESCE(SUM(t.amount), 0) as balance
			FROM dates d
			CROSS JOIN (SELECT * FROM accounts WHERE user_id = $1) a
			LEFT JOIN transactions t ON t.account_id = a.id AND t.date <= d.end_of_month AND t.deleted_at IS NULL AND t.user_id = $1
			GROUP BY d.end_of_month, a.id, a.type, a.initial_balance
		)
		SELECT 
			month,
			SUM(CASE WHEN type = 'asset' THEN balance ELSE 0 END) as assets,
			SUM(CASE WHEN type = 'liability' THEN balance ELSE 0 END) as liabilities
		FROM balances
		GROUP BY month
		ORDER BY month ASC
	`
	rows, err := r.pool.Query(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []domain.NetWorthPoint
	for rows.Next() {
		var p domain.NetWorthPoint
		var assets int64
		var liabilities int64
		if err := rows.Scan(&p.Month, &assets, &liabilities); err != nil {
			return nil, err
		}
		p.Assets = money.Money(assets)
		p.Liabilities = money.Money(liabilities)
		p.NetWorth = money.Money(assets + liabilities)
		points = append(points, p)
	}
	return points, nil
}

func (r *Repository) GetSpendingByCategory(ctx context.Context, userID string, startDate, endDate time.Time) ([]domain.CategorySpend, error) {
	query := `
		SELECT c.id, c.name, SUM(ABS(t.amount)) as total_spent
		FROM transactions t
		JOIN categories c ON t.category_id = c.id
		WHERE t.user_id = $1 AND c.user_id = $1
		  AND t.date >= $2 AND t.date <= $3
		  AND c.type = 'expense'
		  AND t.deleted_at IS NULL
		GROUP BY c.id, c.name
		ORDER BY total_spent DESC
	`
	rows, err := r.pool.Query(ctx, query, userID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spending []domain.CategorySpend
	for rows.Next() {
		var s domain.CategorySpend
		var total int64
		if err := rows.Scan(&s.CategoryID, &s.Name, &total); err != nil {
			return nil, err
		}
		s.TotalSpent = money.Money(total)
		spending = append(spending, s)
	}
	return spending, nil
}

func (r *Repository) GetReportsSummary(ctx context.Context, userID string, startDate, endDate time.Time) (*domain.ReportsSummary, error) {
	// Auto-rollover past due subscriptions
	r.RolloverSubscriptions(ctx, userID)

	var summary domain.ReportsSummary

	queryInOut := `
		WITH linked_amounts AS (
			SELECT 
				t.id,
				t.amount,
				(t.amount 
				 - COALESCE(pays_for_agg.total_amount, 0)
				 + COALESCE(paid_by_agg.total_amount, 0)
				) as effective_amount,
				c.type as category_type
			FROM transactions t
			LEFT JOIN categories c ON t.category_id = c.id
			LEFT JOIN LATERAL (
				SELECT SUM(l.amount) as total_amount
				FROM transaction_links l
				WHERE l.source_transaction_id = t.id
			) pays_for_agg ON true
			LEFT JOIN LATERAL (
				SELECT SUM(l.amount) as total_amount
				FROM transaction_links l
				WHERE l.target_transaction_id = t.id
			) paid_by_agg ON true
			WHERE t.user_id = $1 AND t.date >= $2 AND t.date <= $3 
			  AND t.deleted_at IS NULL
		)
		SELECT 
			COALESCE(SUM(CASE WHEN effective_amount > 0 THEN effective_amount ELSE 0 END), 0) as in_period,
			COALESCE(SUM(CASE WHEN effective_amount < 0 THEN ABS(effective_amount) ELSE 0 END), 0) as out_period
		FROM linked_amounts
		WHERE category_type IS NULL OR category_type != 'transfer'
	`
	var inPeriod, outPeriod int64
	err := r.pool.QueryRow(ctx, queryInOut, userID, startDate, endDate).Scan(&inPeriod, &outPeriod)
	if err != nil {
		return nil, err
	}
	summary.InPeriod = money.Money(inPeriod)
	summary.OutPeriod = money.Money(outPeriod)

	querySubs := `
		SELECT COALESCE(SUM(amount), 0)
		FROM subscriptions
		WHERE user_id = $1 AND next_billing_date >= $2 AND next_billing_date <= $3
	`
	var subs int64
	err = r.pool.QueryRow(ctx, querySubs, userID, startDate, endDate).Scan(&subs)
	if err != nil {
		subs = 0 // fallback
	}
	summary.SubscriptionsToPay = money.Money(subs)

	querySubsPaid := `
		WITH sub_payments AS (
			SELECT DISTINCT t.id, ABS(t.amount) as amount
			FROM transactions t
			JOIN subscriptions s ON t.subscription_id = s.id AND t.deleted_at IS NULL AND s.user_id = $1
			WHERE t.user_id = $1 AND t.date >= $2 AND t.date <= $3 AND t.amount < 0
		)
		SELECT COALESCE(SUM(amount), 0) FROM sub_payments
	`
	var subsPaid int64
	err = r.pool.QueryRow(ctx, querySubsPaid, userID, startDate, endDate).Scan(&subsPaid)
	if err != nil {
		subsPaid = 0
	}
	summary.SubscriptionsPaid = money.Money(subsPaid)

	queryBudgets := `
		WITH budget_spent AS (
			SELECT b.id, COALESCE(SUM(ABS(t.amount)), 0) as spent_total
			FROM budgets b
			LEFT JOIN transactions t ON t.category_id = b.category_id 
				AND t.date >= b.start_date 
				AND t.date <= b.end_date
				AND t.deleted_at IS NULL
				AND t.user_id = $1
			WHERE b.user_id = $1 AND b.start_date <= $3 AND b.end_date >= $2
			GROUP BY b.id
		)
		SELECT 
			COALESCE(SUM(b.amount), 0) as allocated,
			COALESCE(SUM(s.spent_total), 0) as spent
		FROM budgets b
		JOIN budget_spent s ON b.id = s.id
		WHERE b.user_id = $1 AND b.start_date <= $3 AND b.end_date >= $2
	`
	var allocated, spent int64
	err = r.pool.QueryRow(ctx, queryBudgets, userID, startDate, endDate).Scan(&allocated, &spent)
	if err != nil {
		allocated = 0
		spent = 0
	}
	summary.LeftToSpend = money.Money(allocated - spent)

	queryNetWorth := `
		WITH acc_balances AS (
			SELECT a.id, a.type, a.initial_balance + COALESCE(SUM(t.amount), 0) as balance
			FROM accounts a
			LEFT JOIN transactions t ON t.account_id = a.id AND t.deleted_at IS NULL AND t.user_id = $1
			WHERE a.user_id = $1
			GROUP BY a.id, a.type, a.initial_balance
		)
		SELECT 
			COALESCE(SUM(CASE WHEN type = 'asset' THEN balance ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN type = 'liability' THEN balance ELSE 0 END), 0) as net_worth
		FROM acc_balances
	`
	var netWorth int64
	err = r.pool.QueryRow(ctx, queryNetWorth, userID).Scan(&netWorth)
	if err != nil {
		return nil, err
	}
	summary.NetWorth = money.Money(netWorth)

	return &summary, nil
}

func (r *Repository) GetTransaction(ctx context.Context, userID, id string) (*domain.Transaction, error) {
	q := `
		SELECT t.id, t.user_id, t.account_id, t.category_id, t.amount, t.date, t.description,
		       t.is_reviewed, 
		       t.is_reconciled, t.notes, t.subscription_id,
		       a.simplefin_id as simplefin_account_id,
		       (t.amount 
		        - COALESCE(pays_for_agg.total_amount, 0)
		        + COALESCE(paid_by_agg.total_amount, 0)
		       ) as effective_amount,
		       COALESCE(pays_for_agg.json_data, '[]'::json) as pays_for,
		       COALESCE(paid_by_agg.json_data, '[]'::json) as paid_by
		FROM transactions t
		JOIN accounts a ON t.account_id = a.id
		LEFT JOIN LATERAL (
		    SELECT SUM(l.amount) as total_amount,
		           json_agg(json_build_object(
		               'transaction_id', l.target_transaction_id, 
		               'amount', l.amount,
		               'description', tt.description,
		               'date', to_char(tt.date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		           )) as json_data
		    FROM transaction_links l
		    JOIN transactions tt ON l.target_transaction_id = tt.id
		    WHERE l.source_transaction_id = t.id
		) pays_for_agg ON true
		LEFT JOIN LATERAL (
		    SELECT SUM(l.amount) as total_amount,
		           json_agg(json_build_object(
		               'transaction_id', l.source_transaction_id, 
		               'amount', l.amount,
		               'description', st.description,
		               'date', to_char(st.date, 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		           )) as json_data
		    FROM transaction_links l
		    JOIN transactions st ON l.source_transaction_id = st.id
		    WHERE l.target_transaction_id = t.id
		) paid_by_agg ON true
		WHERE t.id = $1 AND t.user_id = $2 AND t.deleted_at IS NULL
	`
	row := r.pool.QueryRow(ctx, q, id, userID)
	var t domain.Transaction
	var amount int64
	var effectiveAmount int64
	var paysForJSON, paidByJSON []byte
	var catID, notes, sfAccountID, subID *string
	err := row.Scan(
		&t.ID, &t.UserID, &t.AccountID, &catID, &amount, &t.Date, &t.Description,
		&t.IsReviewed, &t.IsReconciled,
		&notes, &subID, &sfAccountID,
		&effectiveAmount, &paysForJSON, &paidByJSON,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	t.Amount = money.Money(amount)
	t.EffectiveAmount = money.Money(effectiveAmount)

	if len(paysForJSON) > 0 {
		if err := json.Unmarshal(paysForJSON, &t.PaysFor); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pays_for: %w", err)
		}
	}
	if len(paidByJSON) > 0 {
		if err := json.Unmarshal(paidByJSON, &t.PaidBy); err != nil {
			return nil, fmt.Errorf("failed to unmarshal paid_by: %w", err)
		}
	}

	t.CategoryID = catID

	t.Notes = notes
	t.SubscriptionID = subID
	t.SimplefinAccountID = sfAccountID
	return &t, nil
}
