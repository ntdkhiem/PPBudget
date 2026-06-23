package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ntdkhiem/firefly-go/internal/domain"
	"ntdkhiem/firefly-go/pkg/money"

	apperrors "ntdkhiem/firefly-go/internal/errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// BeginTx starts a new database transaction
func (r *Repository) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// GetAccountBySimplefinID fetches an account by its importer ID
func (r *Repository) GetAccountBySimplefinID(ctx context.Context, tx pgx.Tx, simplefinID string) (string, error) {
	var accountID string
	query := `SELECT id FROM accounts WHERE simplefin_id = $1`

	var row pgx.Row
	if tx != nil {
		row = tx.QueryRow(ctx, query, simplefinID)
	} else {
		row = r.pool.QueryRow(ctx, query, simplefinID)
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

// InsertIngestedTransaction inserts a transaction, returning true if created, false if it was a duplicate
func (r *Repository) InsertIngestedTransaction(ctx context.Context, tx pgx.Tx, accountID string, amount money.Money, date time.Time, description, simplefinTxID string, categoryID *string, isReviewed bool) (bool, error) {
	query := `
        INSERT INTO transactions (account_id, amount, date, description, simplefin_id, category_id, is_reviewed)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        ON CONFLICT (simplefin_id) DO NOTHING
        RETURNING id
    `
	var id string
	err := tx.QueryRow(ctx, query, accountID, amount.ToInt64(), date, description, simplefinTxID, categoryID, isReviewed).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // Duplicate safely ignored
		}
		return false, fmt.Errorf("failed to insert transaction: %w", err)
	}
	return true, nil
}

// CreateTransfer creates two linked transactions
func (r *Repository) CreateTransfer(ctx context.Context, fromAccountID, toAccountID string, amount money.Money, date time.Time, description string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	transferID := uuid.New().String()

	query := `
        INSERT INTO transactions (account_id, amount, date, description, transfer_id, is_reviewed)
        VALUES ($1, $2, $3, $4, $5, true), ($6, $7, $3, $4, $5, true)
    `
	// From account gets negative (outflow), To account gets positive (inflow)
	_, err = tx.Exec(ctx, query,
		fromAccountID, -amount.ToInt64(),
		transferID, description,
		toAccountID, amount.ToInt64())
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// ListTransactions fetches transactions for an account with running balances and cursor pagination
func (r *Repository) ListTransactions(ctx context.Context, accountID string, cursorDate *time.Time, cursorID *string, unreviewedOnly bool, startDate, endDate *time.Time) ([]domain.TransactionWithBalance, error) {
	query := `
        SELECT 
            t.id, t.account_id, t.category_id, t.amount, t.date, t.description, 
            t.is_reviewed, t.is_reconciled, t.transfer_id,
            (a.initial_balance + SUM(t.amount) OVER (PARTITION BY t.account_id ORDER BY t.date, t.id)) as running_balance
        FROM transactions t
        JOIN accounts a ON t.account_id = a.id
        WHERE ($1 = '' OR t.account_id = NULLIF($1, '')::uuid) AND t.deleted_at IS NULL
          AND ($2::date IS NULL OR (t.date, t.id) < ($2::date, $3::uuid))
          AND ($4::boolean = false OR t.is_reviewed = false)
          AND ($5::date IS NULL OR t.date >= $5::date)
          AND ($6::date IS NULL OR t.date <= $6::date)
        ORDER BY t.date DESC, t.id DESC
        LIMIT 50;
    `

	rows, err := r.pool.Query(ctx, query, accountID, cursorDate, cursorID, unreviewedOnly, startDate, endDate)
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
		var transferID *string

		err := rows.Scan(
			&t.ID, &t.AccountID, &catID, &amount, &t.Date, &t.Description,
			&t.IsReviewed, &t.IsReconciled, &transferID,
			&balance,
		)
		if err != nil {
			return nil, err
		}

		t.Amount = money.Money(amount)
		t.RunningBalance = money.Money(balance)
		t.CategoryID = catID
		t.TransferID = transferID
		txns = append(txns, t)
	}
	return txns, nil
}

// GetTransactionsByDateRange fetches all transactions (or bounded by dates) without limits
func (r *Repository) GetTransactionsByDateRange(ctx context.Context, startDate, endDate *time.Time) ([]domain.Transaction, error) {
	query := `
        SELECT 
            t.id, t.account_id, t.category_id, t.amount, t.date, t.description, 
            t.is_reviewed, t.is_reconciled, t.transfer_id, a.simplefin_id as simplefin_account_id
        FROM transactions t
        JOIN accounts a ON t.account_id = a.id
        WHERE t.deleted_at IS NULL
          AND ($1::date IS NULL OR t.date >= $1::date)
          AND ($2::date IS NULL OR t.date <= $2::date)
    `

	rows, err := r.pool.Query(ctx, query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var txns []domain.Transaction
	for rows.Next() {
		var t domain.Transaction
		var amount int64
		var catID *string
		var transferID *string
		var sfAccountID *string

		err := rows.Scan(
			&t.ID, &t.AccountID, &catID, &amount, &t.Date, &t.Description,
			&t.IsReviewed, &t.IsReconciled, &transferID, &sfAccountID,
		)
		if err != nil {
			return nil, err
		}

		t.Amount = money.Money(amount)
		t.CategoryID = catID
		t.TransferID = transferID
		t.SimplefinAccountID = sfAccountID
		txns = append(txns, t)
	}
	return txns, nil
}

// MarkReviewed updates the category and marks the transaction as reviewed
func (r *Repository) MarkReviewed(ctx context.Context, txnID string, categoryID *string) error {
	var query string
	var args []interface{}
	
	if categoryID != nil {
		query = `
			UPDATE transactions 
			SET is_reviewed = true, category_id = $2, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		args = []interface{}{txnID, categoryID}
	} else {
		query = `
			UPDATE transactions 
			SET is_reviewed = true, updated_at = NOW()
			WHERE id = $1 AND deleted_at IS NULL
		`
		args = []interface{}{txnID}
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

// ListAccounts fetches all accounts
func (r *Repository) ListAccounts(ctx context.Context) ([]domain.Account, error) {
	query := `
		SELECT id, name, type, currency, initial_balance, simplefin_id, created_at, updated_at
		FROM accounts ORDER BY name ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		var a domain.Account
		var balance int64
		err := rows.Scan(&a.ID, &a.Name, &a.Type, &a.Currency, &balance, &a.SimplefinID, &a.CreatedAt, &a.UpdatedAt)
		if err != nil {
			return nil, err
		}
		a.InitialBalance = money.Money(balance)
		accounts = append(accounts, a)
	}
	return accounts, nil
}

// Categories

func (r *Repository) CreateCategory(ctx context.Context, name, catType string) error {
	query := `INSERT INTO categories (name, type) VALUES ($1, $2)`
	_, err := r.pool.Exec(ctx, query, name, catType)
	return err
}

func (r *Repository) UpdateCategory(ctx context.Context, id, name, catType string) error {
	query := `UPDATE categories SET name = $1, type = $2 WHERE id = $3`
	tag, err := r.pool.Exec(ctx, query, name, catType, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteCategory(ctx context.Context, id string) error {
	query := `DELETE FROM categories WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) ListCategories(ctx context.Context) ([]domain.Category, error) {
	query := `SELECT id, name, type, created_at FROM categories ORDER BY name ASC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.CreatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

// Rules moved to rules.go

// Budgets moved to budgets.go

// Accounts

func (r *Repository) CreateAccount(ctx context.Context, name, accType, currency string, initialBalance int64) error {
	query := `INSERT INTO accounts (name, type, currency, initial_balance) VALUES ($1, $2, $3, $4)`
	_, err := r.pool.Exec(ctx, query, name, accType, currency, initialBalance)
	return err
}

func (r *Repository) GetAccount(ctx context.Context, id string) (*domain.Account, error) {
	query := `
		SELECT id, name, type, currency, initial_balance, simplefin_id, created_at, updated_at
		FROM accounts WHERE id = $1
	`
	var a domain.Account
	var balance int64
	err := r.pool.QueryRow(ctx, query, id).Scan(&a.ID, &a.Name, &a.Type, &a.Currency, &balance, &a.SimplefinID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	a.InitialBalance = money.Money(balance)
	return &a, nil
}

func (r *Repository) UpdateAccount(ctx context.Context, id, name, accType, currency string, initialBalance int64) error {
	query := `
		UPDATE accounts 
		SET name = $1, type = $2, currency = $3, initial_balance = $4, updated_at = NOW()
		WHERE id = $5
	`
	tag, err := r.pool.Exec(ctx, query, name, accType, currency, initialBalance, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteAccount(ctx context.Context, id string) error {
	query := `DELETE FROM accounts WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// Transactions (Manual)

func (r *Repository) CreateTransaction(ctx context.Context, accountID string, amount int64, date time.Time, description string, categoryID *string) error {
	query := `
		INSERT INTO transactions (account_id, amount, date, description, category_id, is_reviewed)
		VALUES ($1, $2, $3, $4, $5, true)
	`
	// Manual transactions are considered reviewed automatically.
	_, err := r.pool.Exec(ctx, query, accountID, amount, date, description, categoryID)
	return err
}

func (r *Repository) DeleteTransaction(ctx context.Context, id string) error {
	query := `UPDATE transactions SET deleted_at = NOW() WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateTransaction(ctx context.Context, id, accountID string, amount int64, date time.Time, description string, categoryID *string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		UPDATE transactions 
		SET account_id = $1, amount = $2, date = $3, description = $4, category_id = $5, updated_at = NOW()
		WHERE id = $6 AND deleted_at IS NULL
	`
	tag, err := tx.Exec(ctx, query, accountID, amount, date, description, categoryID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}

	return tx.Commit(ctx)
}

// Reports

func (r *Repository) GetNetWorthTrend(ctx context.Context, startDate, endDate time.Time) ([]domain.NetWorthPoint, error) {
	query := `
		WITH dates AS (
			SELECT (date_trunc('month', d) + INTERVAL '1 month - 1 day')::date AS end_of_month
			FROM generate_series(date_trunc('month', $1::timestamp), date_trunc('month', $2::timestamp), '1 month'::interval) d
		),
		balances AS (
			SELECT 
				d.end_of_month as month,
				a.type,
				a.initial_balance + COALESCE(SUM(t.amount), 0) as balance
			FROM dates d
			CROSS JOIN accounts a
			LEFT JOIN transactions t ON t.account_id = a.id AND t.date <= d.end_of_month AND t.deleted_at IS NULL
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
	rows, err := r.pool.Query(ctx, query, startDate, endDate)
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

func (r *Repository) GetSpendingByCategory(ctx context.Context, startDate, endDate time.Time) ([]domain.CategorySpend, error) {
	query := `
		SELECT c.id, c.name, SUM(ABS(t.amount)) as total_spent
		FROM transactions t
		JOIN categories c ON t.category_id = c.id
		WHERE t.date >= $1 AND t.date <= $2
		  AND c.type = 'expense'
		  AND t.deleted_at IS NULL
		GROUP BY c.id, c.name
		ORDER BY total_spent DESC
	`
	rows, err := r.pool.Query(ctx, query, startDate, endDate)
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

func (r *Repository) GetReportsSummary(ctx context.Context, startDate, endDate time.Time) (*domain.ReportsSummary, error) {
	var summary domain.ReportsSummary

	queryInOut := `
		SELECT 
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0) as in_period,
			COALESCE(SUM(CASE WHEN amount < 0 THEN ABS(amount) ELSE 0 END), 0) as out_period
		FROM transactions
		WHERE date >= $1 AND date <= $2 AND deleted_at IS NULL AND transfer_id IS NULL
	`
	var inPeriod, outPeriod int64
	err := r.pool.QueryRow(ctx, queryInOut, startDate, endDate).Scan(&inPeriod, &outPeriod)
	if err != nil {
		return nil, err
	}
	summary.InPeriod = money.Money(inPeriod)
	summary.OutPeriod = money.Money(outPeriod)

	querySubs := `
		SELECT COALESCE(SUM(amount), 0)
		FROM subscriptions
		WHERE next_billing_date >= $1 AND next_billing_date <= $2
	`
	var subs int64
	err = r.pool.QueryRow(ctx, querySubs, startDate, endDate).Scan(&subs)
	// If the table doesn't exist or other err, we might just ignore or return 0, but let's assume it exists
	if err != nil {
		subs = 0 // fallback
	}
	summary.SubscriptionsToPay = money.Money(subs)

	summary.LeftToSpend = money.Money(inPeriod - outPeriod)

	queryNetWorth := `
		WITH acc_balances AS (
			SELECT a.id, a.type, a.initial_balance + COALESCE(SUM(t.amount), 0) as balance
			FROM accounts a
			LEFT JOIN transactions t ON t.account_id = a.id AND t.deleted_at IS NULL
			GROUP BY a.id, a.type, a.initial_balance
		)
		SELECT 
			COALESCE(SUM(CASE WHEN type = 'asset' THEN balance ELSE 0 END), 0) -
			COALESCE(SUM(CASE WHEN type = 'liability' THEN balance ELSE 0 END), 0) as net_worth
		FROM acc_balances
	`
	var netWorth int64
	err = r.pool.QueryRow(ctx, queryNetWorth).Scan(&netWorth)
	if err != nil {
		return nil, err
	}
	summary.NetWorth = money.Money(netWorth)

	return &summary, nil
}
