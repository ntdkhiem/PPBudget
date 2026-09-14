package repository

// Integration tests for balance snapshots (docs/plans/balance-snapshots.md §4.1, §4.2).
//
// These run against a real Postgres instance, gated on TEST_DATABASE_URL. Never
// point this at a production or shared dev database -- these tests create and
// delete real rows (users, accounts, transactions, snapshots). Use the
// throwaway container described in the plan.
//
//	TEST_DATABASE_URL="postgres://ppbudget:ppbudget_password@localhost:55432/ppbudget?sslmode=disable" \
//	  go test -count=1 -run Integration ./internal/repository/...

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

var emailCounter int64

// uniqueEmail returns an email address unique to this test run/process, so
// repeated runs never collide with leftover or concurrently-running data.
func uniqueEmail(prefix string) string {
	n := atomic.AddInt64(&emailCounter, 1)
	return fmt.Sprintf("%s-%d-%d@balances-integration.test", prefix, time.Now().UnixNano(), n)
}

// setupTestDB skips the test when TEST_DATABASE_URL is unset (so plain `go
// test ./...` stays green), otherwise returns a pool connected to it.
func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatalf("failed to ping test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// createTestUser inserts a user directly (dummy password hash -- these tests
// never authenticate) and deletes it (cascading to owned rows) on cleanup.
func createTestUser(t *testing.T, pool *pgxpool.Pool, emailPrefix string) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, 'dummy-hash-not-real') RETURNING id`,
		uniqueEmail(emailPrefix),
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Logf("cleanup: failed to delete test user %s: %v", id, err)
		}
	})
	return id
}

// insertTxn inserts a transaction directly so tests have full control over
// date and amount, returning its id. dateStr is YYYY-MM-DD.
func insertTxn(t *testing.T, pool *pgxpool.Pool, userID, accountID string, amountCents int64, dateStr, description string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO transactions (account_id, user_id, amount, date, description, is_reviewed)
		 VALUES ($1, $2, $3, $4::date, $5, true) RETURNING id`,
		accountID, userID, amountCents, dateStr, description,
	).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test transaction: %v", err)
	}
	return id
}

func softDeleteTxn(t *testing.T, pool *pgxpool.Pool, txnID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `UPDATE transactions SET deleted_at = NOW() WHERE id = $1`, txnID); err != nil {
		t.Fatalf("failed to soft-delete test transaction: %v", err)
	}
}

// balanceAt runs SELECT account_balance_at(...) directly, mirroring §4.1's
// "direct SELECT account_balance_at(...) where needed" instruction. asOf is
// either "infinity"/"-infinity" or a YYYY-MM-DD date.
func balanceAt(t *testing.T, pool *pgxpool.Pool, accountID, asOf string) int64 {
	t.Helper()
	var balance int64
	err := pool.QueryRow(context.Background(),
		`SELECT account_balance_at($1, $2::date)`, accountID, asOf,
	).Scan(&balance)
	if err != nil {
		t.Fatalf("account_balance_at(%s, %s) failed: %v", accountID, asOf, err)
	}
	return balance
}

func assertBalanceAt(t *testing.T, pool *pgxpool.Pool, accountID, asOf string, want int64) {
	t.Helper()
	got := balanceAt(t, pool, accountID, asOf)
	if got != want {
		t.Errorf("account_balance_at(%s) = %d, want %d", asOf, got, want)
	}
}

func mustParseDate(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return tm
}

// ---------------------------------------------------------------------------
// §4.1 account_balance_at
// ---------------------------------------------------------------------------

func TestAccountBalanceAtIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "balance-at")

	// Account X: opening $1000.00 (100000 cents).
	accX, err := repo.CreateAccount(ctx, userID, "Account X", "asset", "USD", 100000)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// Transactions: +100 on 01-10, -50 on 01-20, +30 on 02-05.
	insertTxn(t, pool, userID, accX, 10000, "2025-01-10", "tx1 +100")
	insertTxn(t, pool, userID, accX, -5000, "2025-01-20", "tx2 -50")
	insertTxn(t, pool, userID, accX, 3000, "2025-02-05", "tx3 +30")

	t.Run("case1_opening_plus_transactions_no_other_snapshots", func(t *testing.T) {
		assertBalanceAt(t, pool, accX, "2025-01-15", 110000) // 1000 + 100 = 1100
		assertBalanceAt(t, pool, accX, "infinity", 108000)   // 1000 + 100 - 50 + 30 = 1080
	})

	// Add a simplefin snapshot of $5000.00 @ 2025-01-31.
	if err := repo.UpsertBalanceSnapshot(ctx, nil, userID, accX, mustParseDate(t, "2025-01-31"), money.Money(500000), nil, domain.BalanceSourceSimplefin, nil); err != nil {
		t.Fatalf("UpsertBalanceSnapshot: %v", err)
	}

	t.Run("case2_simplefin_snapshot_reanchors_later_dates_only", func(t *testing.T) {
		assertBalanceAt(t, pool, accX, "2025-01-31", 500000) // snapshot value itself
		assertBalanceAt(t, pool, accX, "2025-02-10", 503000) // 5000 + 30 (tx3 after snapshot)
		assertBalanceAt(t, pool, accX, "2025-01-15", 110000) // opening still anchors earlier dates
	})

	// Same-day upsert on 2025-01-31 should replace, not add.
	if err := repo.UpsertBalanceSnapshot(ctx, nil, userID, accX, mustParseDate(t, "2025-01-31"), money.Money(600000), nil, domain.BalanceSourceSimplefin, nil); err != nil {
		t.Fatalf("UpsertBalanceSnapshot (replace): %v", err)
	}

	t.Run("case5_same_day_upsert_replaces", func(t *testing.T) {
		assertBalanceAt(t, pool, accX, "2025-01-31", 600000)

		var count int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM account_balance_snapshots WHERE account_id = $1 AND as_of_date = $2::date`,
			accX, "2025-01-31",
		).Scan(&count); err != nil {
			t.Fatalf("count query failed: %v", err)
		}
		if count != 1 {
			t.Errorf("expected exactly 1 snapshot row for 2025-01-31, got %d", count)
		}
	})

	// Soft-deleted transaction dated after the latest snapshot must be ignored.
	deletedTxnID := insertTxn(t, pool, userID, accX, 999999, "2025-02-01", "should be ignored once deleted")
	softDeleteTxn(t, pool, deletedTxnID)

	t.Run("case4_soft_deleted_transactions_ignored", func(t *testing.T) {
		// anchor = 600000 (01-31 snapshot) + tx3 (+3000 on 02-05) = 603000.
		// The soft-deleted 999999-cent txn on 02-01 must NOT be included.
		assertBalanceAt(t, pool, accX, "infinity", 603000)
	})

	t.Run("case3_no_opening_only_future_snapshot", func(t *testing.T) {
		sfID := uniqueEmail("sf-acct-y")
		accY, err := repo.UpsertSimplefinAccount(ctx, userID, sfID, "Account Y", "USD")
		if err != nil {
			t.Fatalf("UpsertSimplefinAccount: %v", err)
		}
		// UpsertSimplefinAccount must not create an opening snapshot.
		var openingCount int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM account_balance_snapshots WHERE account_id = $1 AND source = 'opening'`,
			accY,
		).Scan(&openingCount); err != nil {
			t.Fatalf("opening count query failed: %v", err)
		}
		if openingCount != 0 {
			t.Fatalf("expected Account Y to have no opening snapshot, found %d", openingCount)
		}

		if err := repo.UpsertBalanceSnapshot(ctx, nil, userID, accY, mustParseDate(t, "2025-03-01"), money.Money(200000), nil, domain.BalanceSourceSimplefin, nil); err != nil {
			t.Fatalf("UpsertBalanceSnapshot on Account Y: %v", err)
		}
		insertTxn(t, pool, userID, accY, 10000, "2025-02-20", "y tx1 +100")
		insertTxn(t, pool, userID, accY, -4000, "2025-02-25", "y tx2 -40")

		assertBalanceAt(t, pool, accY, "2025-02-22", 204000) // 2000 - (-40) = 2040
		assertBalanceAt(t, pool, accY, "2025-02-10", 194000) // 2000 - (-40 + 100) = 1940
		assertBalanceAt(t, pool, accY, "2025-03-05", 200000) // anchor itself, no txns after
	})

	t.Run("case6_check_constraint_rejects_invalid_source_date_pairing", func(t *testing.T) {
		// source='manual' at '-infinity' must be rejected.
		_, err := pool.Exec(ctx,
			`INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
			 VALUES ($1, $2, '-infinity'::date, 100, 'manual')`,
			userID, accX,
		)
		if err == nil {
			t.Error("expected CHECK violation inserting source='manual' at -infinity, got no error")
		}

		// source='opening' at a real date must be rejected.
		_, err = pool.Exec(ctx,
			`INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
			 VALUES ($1, $2, '2025-06-01'::date, 100, 'opening')`,
			userID, accX,
		)
		if err == nil {
			t.Error("expected CHECK violation inserting source='opening' at a real date, got no error")
		}
	})
}

// ---------------------------------------------------------------------------
// §4.2 Repository
// ---------------------------------------------------------------------------

// Opening-only account: BalanceAsOf/BalanceSource are nil until a non-opening
// snapshot is written.
func TestOpeningOnlyAccountBalanceFieldsIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "opening-only")

	accID, err := repo.CreateAccount(ctx, userID, "Opening Only", "asset", "USD", 50000)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	acc, err := repo.GetAccount(ctx, userID, accID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acc.BalanceAsOf != nil {
		t.Errorf("BalanceAsOf = %v, want nil for opening-only account", acc.BalanceAsOf)
	}
	if acc.BalanceSource != nil {
		t.Errorf("BalanceSource = %v, want nil for opening-only account", acc.BalanceSource)
	}
	if acc.CurrentBalance.ToInt64() != 50000 {
		t.Errorf("CurrentBalance = %d, want 50000", acc.CurrentBalance.ToInt64())
	}

	reportedAt := time.Now().UTC()
	if err := repo.UpsertBalanceSnapshot(ctx, nil, userID, accID, time.Now().UTC(), money.Money(60000), nil, domain.BalanceSourceManual, &reportedAt); err != nil {
		t.Fatalf("UpsertBalanceSnapshot: %v", err)
	}

	acc2, err := repo.GetAccount(ctx, userID, accID)
	if err != nil {
		t.Fatalf("GetAccount (after snapshot): %v", err)
	}
	if acc2.BalanceAsOf == nil {
		t.Error("BalanceAsOf = nil, want set after a manual snapshot")
	}
	if acc2.BalanceSource == nil || *acc2.BalanceSource != domain.BalanceSourceManual {
		t.Errorf("BalanceSource = %v, want %q", acc2.BalanceSource, domain.BalanceSourceManual)
	}
}

// Tenancy: user A cannot list, upsert, or delete snapshots on user B's
// account, and ListAccounts(A) never returns B's accounts.
func TestBalanceSnapshotTenancyIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userA := createTestUser(t, pool, "tenancy-a")
	userB := createTestUser(t, pool, "tenancy-b")

	accB, err := repo.CreateAccount(ctx, userB, "B's Account", "asset", "USD", 10000)
	if err != nil {
		t.Fatalf("CreateAccount(B): %v", err)
	}
	accA, err := repo.CreateAccount(ctx, userA, "A's Account", "asset", "USD", 20000)
	if err != nil {
		t.Fatalf("CreateAccount(A): %v", err)
	}

	if _, err := repo.ListBalanceSnapshots(ctx, userA, accB); !errors.Is(err, apperrors.ErrForbidden) {
		t.Errorf("ListBalanceSnapshots(A, B's account) error = %v, want ErrForbidden", err)
	}

	reportedAt := time.Now().UTC()
	if err := repo.UpsertBalanceSnapshot(ctx, nil, userA, accB, time.Now().UTC(), money.Money(99999), nil, domain.BalanceSourceManual, &reportedAt); !errors.Is(err, apperrors.ErrForbidden) {
		t.Errorf("UpsertBalanceSnapshot(A, B's account) error = %v, want ErrForbidden", err)
	}

	// Grab a real (non-opening) snapshot id on B's account to attempt to delete as A.
	if err := repo.UpsertBalanceSnapshot(ctx, nil, userB, accB, time.Now().UTC(), money.Money(11111), nil, domain.BalanceSourceManual, &reportedAt); err != nil {
		t.Fatalf("UpsertBalanceSnapshot(B, own account): %v", err)
	}
	snapshotsB, err := repo.ListBalanceSnapshots(ctx, userB, accB)
	if err != nil {
		t.Fatalf("ListBalanceSnapshots(B): %v", err)
	}
	if len(snapshotsB) == 0 {
		t.Fatalf("expected at least one snapshot for B's account")
	}
	var nonOpeningID string
	for _, s := range snapshotsB {
		if s.Source != domain.BalanceSourceOpening {
			nonOpeningID = s.ID
			break
		}
	}
	if nonOpeningID == "" {
		t.Fatalf("expected a non-opening snapshot on B's account")
	}

	if err := repo.DeleteBalanceSnapshot(ctx, userA, accB, nonOpeningID); !errors.Is(err, apperrors.ErrForbidden) {
		t.Errorf("DeleteBalanceSnapshot(A, B's snapshot) error = %v, want ErrForbidden", err)
	}

	accountsA, err := repo.ListAccounts(ctx, userA)
	if err != nil {
		t.Fatalf("ListAccounts(A): %v", err)
	}
	for _, a := range accountsA {
		if a.ID == accB {
			t.Errorf("ListAccounts(A) returned B's account %s", accB)
		}
	}
	// Sanity: A's own account is present.
	found := false
	for _, a := range accountsA {
		if a.ID == accA {
			found = true
		}
	}
	if !found {
		t.Errorf("ListAccounts(A) did not include A's own account %s", accA)
	}
}

// ListBalanceSnapshots order: newest first, opening last, opening AsOfDate == nil.
func TestListBalanceSnapshotsOrderIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "list-order")

	accID, err := repo.CreateAccount(ctx, userID, "Order Test", "asset", "USD", 10000)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	dates := []string{"2025-01-10", "2025-03-05", "2025-02-01"}
	for i, d := range dates {
		if err := repo.UpsertBalanceSnapshot(ctx, nil, userID, accID, mustParseDate(t, d), money.Money(int64(1000*(i+1))), nil, domain.BalanceSourceManual, nil); err != nil {
			t.Fatalf("UpsertBalanceSnapshot(%s): %v", d, err)
		}
	}

	snapshots, err := repo.ListBalanceSnapshots(ctx, userID, accID)
	if err != nil {
		t.Fatalf("ListBalanceSnapshots: %v", err)
	}
	if len(snapshots) != 4 { // 3 manual + 1 opening
		t.Fatalf("expected 4 snapshots, got %d", len(snapshots))
	}

	// Newest first: 2025-03-05, 2025-02-01, 2025-01-10, then opening last.
	wantOrder := []string{"2025-03-05", "2025-02-01", "2025-01-10"}
	for i, want := range wantOrder {
		if snapshots[i].AsOfDate == nil {
			t.Fatalf("snapshot[%d] AsOfDate is nil, want %s", i, want)
		}
		got := snapshots[i].AsOfDate.Format("2006-01-02")
		if got != want {
			t.Errorf("snapshot[%d] AsOfDate = %s, want %s", i, got, want)
		}
	}

	last := snapshots[len(snapshots)-1]
	if last.Source != domain.BalanceSourceOpening {
		t.Errorf("last snapshot source = %q, want %q", last.Source, domain.BalanceSourceOpening)
	}
	if last.AsOfDate != nil {
		t.Errorf("opening snapshot AsOfDate = %v, want nil", last.AsOfDate)
	}
}

// DeleteBalanceSnapshot: opening -> ErrInvalidInput; unknown id -> ErrNotFound.
func TestDeleteBalanceSnapshotIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "delete-snapshot")

	accID, err := repo.CreateAccount(ctx, userID, "Delete Test", "asset", "USD", 10000)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	snapshots, err := repo.ListBalanceSnapshots(ctx, userID, accID)
	if err != nil {
		t.Fatalf("ListBalanceSnapshots: %v", err)
	}
	var openingID string
	for _, s := range snapshots {
		if s.Source == domain.BalanceSourceOpening {
			openingID = s.ID
		}
	}
	if openingID == "" {
		t.Fatalf("expected an opening snapshot")
	}

	if err := repo.DeleteBalanceSnapshot(ctx, userID, accID, openingID); !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("DeleteBalanceSnapshot(opening) error = %v, want ErrInvalidInput", err)
	}

	if err := repo.DeleteBalanceSnapshot(ctx, userID, accID, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, apperrors.ErrNotFound) {
		t.Errorf("DeleteBalanceSnapshot(unknown id) error = %v, want ErrNotFound", err)
	}
}

// CreateAccount + UpdateAccount opening-balance semantics, and empty currency
// keeping the existing value.
func TestCreateUpdateAccountOpeningBalanceIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "create-update")

	accID, err := repo.CreateAccount(ctx, userID, "CU Test", "asset", "USD", 100000)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	acc, err := repo.GetAccount(ctx, userID, accID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acc.CurrentBalance.ToInt64() != 100000 {
		t.Fatalf("initial CurrentBalance = %d, want 100000", acc.CurrentBalance.ToInt64())
	}

	// UpdateAccount with openingBalance = nil (unchanged) and currency = "" (keeps existing).
	if err := repo.UpdateAccount(ctx, userID, accID, "CU Test Renamed", "asset", "", nil); err != nil {
		t.Fatalf("UpdateAccount (nil opening balance): %v", err)
	}
	acc2, err := repo.GetAccount(ctx, userID, accID)
	if err != nil {
		t.Fatalf("GetAccount (after nil update): %v", err)
	}
	if acc2.CurrentBalance.ToInt64() != 100000 {
		t.Errorf("CurrentBalance after nil openingBalance update = %d, want unchanged 100000", acc2.CurrentBalance.ToInt64())
	}
	if acc2.Name != "CU Test Renamed" {
		t.Errorf("Name = %q, want %q", acc2.Name, "CU Test Renamed")
	}
	if acc2.Currency != "USD" {
		t.Errorf("Currency = %q, want unchanged %q (empty string should keep existing)", acc2.Currency, "USD")
	}

	// UpdateAccount with openingBalance = non-nil (changed).
	newBalance := int64(250000)
	if err := repo.UpdateAccount(ctx, userID, accID, "CU Test Renamed", "asset", "", &newBalance); err != nil {
		t.Fatalf("UpdateAccount (non-nil opening balance): %v", err)
	}
	acc3, err := repo.GetAccount(ctx, userID, accID)
	if err != nil {
		t.Fatalf("GetAccount (after non-nil update): %v", err)
	}
	if acc3.CurrentBalance.ToInt64() != 250000 {
		t.Errorf("CurrentBalance after openingBalance update = %d, want 250000", acc3.CurrentBalance.ToInt64())
	}
}

// Net worth trend + summary: asset 1500 + liability -500 = net worth 1000;
// summary matches the last trend point only in the absence of future-dated
// transactions.
func TestNetWorthTrendAndSummaryIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "networth")

	assetID, err := repo.CreateAccount(ctx, userID, "Asset Acct", "asset", "USD", 150000) // $1500.00
	if err != nil {
		t.Fatalf("CreateAccount(asset): %v", err)
	}
	liabilityID, err := repo.CreateAccount(ctx, userID, "Liability Acct", "liability", "USD", -50000) // -$500.00
	if err != nil {
		t.Fatalf("CreateAccount(liability): %v", err)
	}

	now := time.Now().UTC()
	startDate := now.AddDate(0, -2, 0)
	endDate := now

	points, err := repo.GetNetWorthTrend(ctx, userID, startDate, endDate)
	if err != nil {
		t.Fatalf("GetNetWorthTrend: %v", err)
	}
	if len(points) == 0 {
		t.Fatalf("expected at least one trend point")
	}
	last := points[len(points)-1]
	if last.NetWorth.ToInt64() != 100000 {
		t.Errorf("last trend point NetWorth = %d, want 100000", last.NetWorth.ToInt64())
	}

	summary, err := repo.GetReportsSummary(ctx, userID, startDate, endDate)
	if err != nil {
		t.Fatalf("GetReportsSummary: %v", err)
	}
	if summary.NetWorth.ToInt64() != last.NetWorth.ToInt64() {
		t.Errorf("summary NetWorth = %d, want to match last trend point %d (no future-dated txns)", summary.NetWorth.ToInt64(), last.NetWorth.ToInt64())
	}
	if summary.NetWorth.ToInt64() != 100000 {
		t.Errorf("summary NetWorth = %d, want 100000", summary.NetWorth.ToInt64())
	}

	// Now add a future-dated transaction on the asset account.
	futureDate := now.AddDate(0, 0, 30).Format("2006-01-02")
	insertTxn(t, pool, userID, assetID, 20000, futureDate, "future txn +200")
	_ = liabilityID

	points2, err := repo.GetNetWorthTrend(ctx, userID, startDate, endDate)
	if err != nil {
		t.Fatalf("GetNetWorthTrend (after future txn): %v", err)
	}
	last2 := points2[len(points2)-1]
	if last2.NetWorth.ToInt64() != 100000 {
		t.Errorf("trend's current-month point after future txn = %d, want unchanged 100000 (capped at CURRENT_DATE)", last2.NetWorth.ToInt64())
	}

	summary2, err := repo.GetReportsSummary(ctx, userID, startDate, endDate)
	if err != nil {
		t.Fatalf("GetReportsSummary (after future txn): %v", err)
	}
	if summary2.NetWorth.ToInt64() != 120000 {
		t.Errorf("summary NetWorth after future txn = %d, want 120000 (includes future-dated txn via 'infinity')", summary2.NetWorth.ToInt64())
	}
	if summary2.NetWorth.ToInt64() == last2.NetWorth.ToInt64() {
		t.Errorf("summary NetWorth (%d) should now differ from the trend's current-month point (%d) because of the future-dated txn", summary2.NetWorth.ToInt64(), last2.NetWorth.ToInt64())
	}
}

// ListTransactions running balance: exact per-row values, including two
// same-day transactions (where id order -- not insertion order, since ids
// are random v4 UUIDs -- decides the split of that day's total), a lone
// earlier-day transaction, and a transaction anchored to a mid-range
// simplefin snapshot. Also keeps the filtered-vs-unfiltered equality check
// (§4.2 "running balance on page filtered by start_date equals unfiltered
// value for the same txn"), now on this richer fixture.
func TestListTransactionsRunningBalanceIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "running-balance")

	accID, err := repo.CreateAccount(ctx, userID, "Running Balance Test", "asset", "USD", 100000) // opening $1000.00
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	// Earlier day, alone: day-end = opening + 20000 = 120000.
	earlyID := insertTxn(t, pool, userID, accID, 20000, "2025-01-05", "early +200")

	// Two transactions on the same day: day-end = 120000 + 5000 - 2000 = 123000.
	// Postgres uuid ordering matches Go string comparison of the canonical
	// lowercase hex form (hyphens align, hex digits compare in byte order),
	// so we resolve which id is "higher" in Go rather than assuming insertion
	// order -- ids from uuid_generate_v4() are random, not sequential.
	sameDayAID := insertTxn(t, pool, userID, accID, 5000, "2025-01-10", "same-day +50")
	sameDayBID := insertTxn(t, pool, userID, accID, -2000, "2025-01-10", "same-day -20")

	var higherSameDayID, lowerSameDayID string
	var higherSameDayAmt int64
	if sameDayAID > sameDayBID {
		higherSameDayID, higherSameDayAmt = sameDayAID, 5000
		lowerSameDayID = sameDayBID
	} else {
		higherSameDayID, higherSameDayAmt = sameDayBID, -2000
		lowerSameDayID = sameDayAID
	}

	// Mid-range simplefin snapshot re-anchors everything after it.
	if err := repo.UpsertBalanceSnapshot(ctx, nil, userID, accID, mustParseDate(t, "2025-02-01"), money.Money(500000), nil, domain.BalanceSourceSimplefin, nil); err != nil {
		t.Fatalf("UpsertBalanceSnapshot: %v", err)
	}

	// After the snapshot, alone: day-end = 500000 + 7500 = 507500 (anchored
	// to the snapshot, NOT opening + all transactions, which would be 130500).
	afterSnapshotID := insertTxn(t, pool, userID, accID, 7500, "2025-02-10", "after-snapshot +75")

	const (
		wantEarly         = int64(120000)
		wantSameDayHigh   = int64(123000) // day-end itself
		wantAfterSnapshot = int64(507500)
	)
	wantSameDayLow := wantSameDayHigh - higherSameDayAmt

	assertRunningBalances := func(t *testing.T, txns []domain.TransactionWithBalance, label string, includesEarly bool) {
		t.Helper()
		balances := make(map[string]int64, len(txns))
		for _, tx := range txns {
			balances[tx.ID] = tx.RunningBalance.ToInt64()
		}

		check := func(id string, want int64, name string) {
			got, ok := balances[id]
			if !ok {
				t.Fatalf("[%s] %s (id=%s) missing from results", label, name, id)
			}
			if got != want {
				t.Errorf("[%s] %s running_balance = %d, want %d", label, name, got, want)
			}
		}
		if includesEarly {
			check(earlyID, wantEarly, "early txn")
		}
		check(higherSameDayID, wantSameDayHigh, "higher-id same-day txn (last shown for that day)")
		check(lowerSameDayID, wantSameDayLow, "lower-id same-day txn")
		check(afterSnapshotID, wantAfterSnapshot, "txn after snapshot")
	}

	unfiltered, err := repo.ListTransactions(ctx, domain.TransactionFilter{UserID: userID, AccountID: accID})
	if err != nil {
		t.Fatalf("ListTransactions (unfiltered): %v", err)
	}
	assertRunningBalances(t, unfiltered, "unfiltered", true)

	// Filtered page: start_date excludes the earliest transaction, but must
	// not change the running balance of any row that's still on the page.
	start := mustParseDate(t, "2025-01-06")
	filtered, err := repo.ListTransactions(ctx, domain.TransactionFilter{UserID: userID, AccountID: accID, StartDate: &start})
	if err != nil {
		t.Fatalf("ListTransactions (filtered): %v", err)
	}
	for _, tx := range filtered {
		if tx.ID == earlyID {
			t.Fatalf("early txn should have been excluded by start_date filter")
		}
	}
	assertRunningBalances(t, filtered, "filtered(start_date=2025-01-06)", false)

	findBalance := func(txns []domain.TransactionWithBalance, id string) (money.Money, bool) {
		for _, tx := range txns {
			if tx.ID == id {
				return tx.RunningBalance, true
			}
		}
		return 0, false
	}
	for _, id := range []string{higherSameDayID, lowerSameDayID, afterSnapshotID} {
		balUnfiltered, ok := findBalance(unfiltered, id)
		if !ok {
			t.Fatalf("txn %s not found in unfiltered results", id)
		}
		balFiltered, ok := findBalance(filtered, id)
		if !ok {
			t.Fatalf("txn %s not found in filtered results", id)
		}
		if balUnfiltered != balFiltered {
			t.Errorf("running balance differs for %s: unfiltered=%d filtered(start_date)=%d", id, balUnfiltered, balFiltered)
		}
	}
}

// GetNetWorthTrend across a mid-range simplefin snapshot: months before the
// snapshot must come from opening + transactions; months on/after it must
// come from the snapshot + only the transactions after it.
func TestNetWorthTrendAcrossSnapshotIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "trend-across-snapshot")

	accID, err := repo.CreateAccount(ctx, userID, "Trend Snapshot Test", "asset", "USD", 100000) // opening $1000
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	insertTxn(t, pool, userID, accID, 10000, "2025-01-15", "jan +100")
	insertTxn(t, pool, userID, accID, 5000, "2025-02-10", "feb +50")

	if err := repo.UpsertBalanceSnapshot(ctx, nil, userID, accID, mustParseDate(t, "2025-03-01"), money.Money(300000), nil, domain.BalanceSourceSimplefin, nil); err != nil {
		t.Fatalf("UpsertBalanceSnapshot: %v", err)
	}

	insertTxn(t, pool, userID, accID, 2000, "2025-03-20", "mar (post-snapshot) +20")
	insertTxn(t, pool, userID, accID, -1000, "2025-04-05", "apr (post-snapshot) -10")

	points, err := repo.GetNetWorthTrend(ctx, userID, mustParseDate(t, "2025-01-01"), mustParseDate(t, "2025-04-01"))
	if err != nil {
		t.Fatalf("GetNetWorthTrend: %v", err)
	}
	if len(points) != 4 {
		t.Fatalf("expected 4 trend points (Jan-Apr 2025), got %d", len(points))
	}

	// Jan, Feb: derived from opening (100000) + transactions to date --
	// nothing to do with the March snapshot.
	wantJan := int64(110000) // 100000 + 10000
	wantFeb := int64(115000) // 100000 + 10000 + 5000
	// Mar, Apr: anchored to the snapshot (300000) + only transactions after it.
	// If these were (wrongly) still opening-derived, Mar would be 117000 and
	// Apr 116000 -- clearly different from the snapshot-anchored values below.
	wantMar := int64(302000) // 300000 + 2000
	wantApr := int64(301000) // 300000 + 2000 - 1000

	want := []int64{wantJan, wantFeb, wantMar, wantApr}
	monthLabels := []string{"Jan", "Feb", "Mar", "Apr"}
	for i, w := range want {
		got := points[i].NetWorth.ToInt64()
		if got != w {
			t.Errorf("%s net worth = %d, want %d (month=%s)", monthLabels[i], got, w, points[i].Month.Format("2006-01-02"))
		}
	}
}

// GetNetWorthTrend for an account with only a future/mid-range snapshot and
// no opening balance (e.g. a newly linked SimpleFin account): past
// month-ends must be derived backwards from that snapshot.
func TestNetWorthTrendOnlyFutureSnapshotIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "trend-future-only")

	sfID := uniqueEmail("sf-trend-future-only")
	accID, err := repo.UpsertSimplefinAccount(ctx, userID, sfID, "Future Snapshot Only", "USD")
	if err != nil {
		t.Fatalf("UpsertSimplefinAccount: %v", err)
	}
	var openingCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM account_balance_snapshots WHERE account_id = $1 AND source = 'opening'`, accID).Scan(&openingCount); err != nil {
		t.Fatalf("opening count query failed: %v", err)
	}
	if openingCount != 0 {
		t.Fatalf("expected no opening snapshot for a freshly-linked account, found %d", openingCount)
	}

	if err := repo.UpsertBalanceSnapshot(ctx, nil, userID, accID, mustParseDate(t, "2025-03-01"), money.Money(300000), nil, domain.BalanceSourceSimplefin, nil); err != nil {
		t.Fatalf("UpsertBalanceSnapshot: %v", err)
	}
	insertTxn(t, pool, userID, accID, 2000, "2025-02-10", "pre-snapshot +20")

	points, err := repo.GetNetWorthTrend(ctx, userID, mustParseDate(t, "2025-01-01"), mustParseDate(t, "2025-03-01"))
	if err != nil {
		t.Fatalf("GetNetWorthTrend: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 trend points (Jan-Mar 2025), got %d", len(points))
	}

	// Jan: walked backwards from the snapshot, minus the txn that falls
	// between Jan-end and the snapshot: 300000 - 2000 = 298000.
	// Feb: walked backwards too, but the txn (02-10) is not after Feb-end,
	// so nothing to subtract: 300000 - 0 = 300000.
	// Mar: the snapshot date itself is now <= month-end, so it's the normal
	// forward branch: 300000 + 0 = 300000.
	want := []int64{298000, 300000, 300000}
	monthLabels := []string{"Jan", "Feb", "Mar"}
	for i, w := range want {
		got := points[i].NetWorth.ToInt64()
		if got != w {
			t.Errorf("%s net worth = %d, want %d (month=%s)", monthLabels[i], got, w, points[i].Month.Format("2006-01-02"))
		}
	}
}

// account_balance_at is STRICT: a NULL date argument returns NULL rather
// than raising an error or falling through to a branch.
func TestAccountBalanceAtNullDateIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "null-date")

	accID, err := repo.CreateAccount(ctx, userID, "Null Date Test", "asset", "USD", 10000)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	var balance *int64
	if err := pool.QueryRow(ctx, `SELECT account_balance_at($1, NULL)`, accID).Scan(&balance); err != nil {
		t.Fatalf("SELECT account_balance_at(id, NULL) failed: %v", err)
	}
	if balance != nil {
		t.Errorf("account_balance_at(id, NULL) = %d, want NULL", *balance)
	}
}

// ---------------------------------------------------------------------------
// Balance-only accounts
// ---------------------------------------------------------------------------

func countLiveTxns(t *testing.T, pool *pgxpool.Pool, accountID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE account_id = $1 AND deleted_at IS NULL`, accountID,
	).Scan(&n); err != nil {
		t.Fatalf("count live txns: %v", err)
	}
	return n
}

// Enabling balance_only soft-deletes live transactions, removes their links
// (including cross-account ones), and leaves the balance to snapshots.
func TestSetAccountBalanceOnlyEnableIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "balance-only-enable")

	accID, err := repo.CreateAccount(ctx, userID, "Roth IRA", "asset", "USD", 100000)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	otherID, err := repo.CreateAccount(ctx, userID, "Checking", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount(other): %v", err)
	}

	buyID := insertTxn(t, pool, userID, accID, -20000, "2025-01-10", "buy")
	insertTxn(t, pool, userID, accID, 500, "2025-01-15", "dividend")
	oldID := insertTxn(t, pool, userID, accID, 999, "2025-01-05", "already deleted")
	softDeleteTxn(t, pool, oldID)

	otherTxnID := insertTxn(t, pool, userID, otherID, -20000, "2025-01-09", "transfer out")
	if _, err := pool.Exec(ctx,
		`INSERT INTO transaction_links (source_transaction_id, target_transaction_id, amount) VALUES ($1, $2, $3)`,
		otherTxnID, buyID, 20000,
	); err != nil {
		t.Fatalf("insert link: %v", err)
	}

	n, err := repo.SetAccountBalanceOnly(ctx, userID, accID, true)
	if err != nil {
		t.Fatalf("SetAccountBalanceOnly: %v", err)
	}
	if n != 2 {
		t.Errorf("deleted = %d, want 2", n)
	}

	txns, err := repo.ListTransactions(ctx, domain.TransactionFilter{UserID: userID, AccountID: accID})
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	if len(txns) != 0 {
		t.Errorf("ListTransactions(balance-only account) returned %d rows, want 0", len(txns))
	}

	var links int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transaction_links WHERE source_transaction_id = $1 OR target_transaction_id = $1`, otherTxnID,
	).Scan(&links); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if links != 0 {
		t.Errorf("links remaining = %d, want 0", links)
	}

	otherTxns, err := repo.ListTransactions(ctx, domain.TransactionFilter{UserID: userID, AccountID: otherID})
	if err != nil {
		t.Fatalf("ListTransactions(other): %v", err)
	}
	if len(otherTxns) != 1 {
		t.Fatalf("other account txns = %d, want 1", len(otherTxns))
	}
	if len(otherTxns[0].PaysFor) != 0 || len(otherTxns[0].PaidBy) != 0 {
		t.Errorf("other txn pays_for=%v paid_by=%v, want both empty", otherTxns[0].PaysFor, otherTxns[0].PaidBy)
	}
	if otherTxns[0].EffectiveAmount.ToInt64() != -20000 {
		t.Errorf("other txn effective_amount = %d, want -20000", otherTxns[0].EffectiveAmount.ToInt64())
	}

	acc, err := repo.GetAccount(ctx, userID, accID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if !acc.BalanceOnly {
		t.Error("BalanceOnly = false, want true")
	}
	if acc.CurrentBalance.ToInt64() != 100000 {
		t.Errorf("CurrentBalance = %d, want 100000 (opening only)", acc.CurrentBalance.ToInt64())
	}

	if err := repo.UpsertBalanceSnapshot(ctx, nil, userID, accID, time.Now().UTC(), money.Money(500000), nil, domain.BalanceSourceSimplefin, nil); err != nil {
		t.Fatalf("UpsertBalanceSnapshot: %v", err)
	}
	acc2, err := repo.GetAccount(ctx, userID, accID)
	if err != nil {
		t.Fatalf("GetAccount (after snapshot): %v", err)
	}
	if acc2.CurrentBalance.ToInt64() != 500000 {
		t.Errorf("CurrentBalance after snapshot = %d, want 500000", acc2.CurrentBalance.ToInt64())
	}
}

// Re-enabling returns 0; disabling returns 0, clears the flag, restores nothing.
func TestSetAccountBalanceOnlyIdempotentAndDisableIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "balance-only-toggle")
	accID, err := repo.CreateAccount(ctx, userID, "Brokerage", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	insertTxn(t, pool, userID, accID, 1000, "2025-01-10", "t1")

	if n, err := repo.SetAccountBalanceOnly(ctx, userID, accID, true); err != nil || n != 1 {
		t.Fatalf("enable: n=%d err=%v, want 1, nil", n, err)
	}
	if n, err := repo.SetAccountBalanceOnly(ctx, userID, accID, true); err != nil || n != 0 {
		t.Errorf("re-enable: n=%d err=%v, want 0, nil", n, err)
	}
	if n, err := repo.SetAccountBalanceOnly(ctx, userID, accID, false); err != nil || n != 0 {
		t.Errorf("disable: n=%d err=%v, want 0, nil", n, err)
	}

	acc, err := repo.GetAccount(ctx, userID, accID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acc.BalanceOnly {
		t.Error("BalanceOnly = true after disable, want false")
	}
	if got := countLiveTxns(t, pool, accID); got != 0 {
		t.Errorf("live txns after disable = %d, want 0 (nothing restored)", got)
	}
}

// User B cannot toggle user A's account.
func TestSetAccountBalanceOnlyTenancyIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userA := createTestUser(t, pool, "balance-only-tenancy-a")
	userB := createTestUser(t, pool, "balance-only-tenancy-b")

	accA, err := repo.CreateAccount(ctx, userA, "A 401k", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	insertTxn(t, pool, userA, accA, 1000, "2025-01-10", "t1")

	if _, err := repo.SetAccountBalanceOnly(ctx, userB, accA, true); !errors.Is(err, apperrors.ErrForbidden) {
		t.Errorf("SetAccountBalanceOnly(B, account of A) error = %v, want ErrForbidden", err)
	}

	acc, err := repo.GetAccount(ctx, userA, accA)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acc.BalanceOnly {
		t.Error("BalanceOnly changed by foreign user")
	}
	if got := countLiveTxns(t, pool, accA); got != 1 {
		t.Errorf("live txns = %d, want 1 (untouched)", got)
	}
}

// CreateTransaction / UpdateTransaction into a balance-only account are rejected.
func TestBalanceOnlyTransactionGuardIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "balance-only-guard")
	boID, err := repo.CreateAccount(ctx, userID, "IRA", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount(bo): %v", err)
	}
	normalID, err := repo.CreateAccount(ctx, userID, "Checking", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount(normal): %v", err)
	}
	if _, err := repo.SetAccountBalanceOnly(ctx, userID, boID, true); err != nil {
		t.Fatalf("SetAccountBalanceOnly: %v", err)
	}

	date := mustParseDate(t, "2025-01-10")
	if err := repo.CreateTransaction(ctx, userID, boID, 1000, date, "blocked", nil, nil, nil); !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("CreateTransaction(balance-only) error = %v, want ErrInvalidInput", err)
	}
	if got := countLiveTxns(t, pool, boID); got != 0 {
		t.Errorf("balance-only live txns = %d, want 0", got)
	}

	if err := repo.CreateTransaction(ctx, userID, normalID, 1000, date, "ok", nil, nil, nil); err != nil {
		t.Fatalf("CreateTransaction(normal): %v", err)
	}

	txnID := insertTxn(t, pool, userID, normalID, 2000, "2025-01-11", "to move")
	if err := repo.UpdateTransaction(ctx, userID, txnID, boID, 2000, date, "moved", nil, nil, nil, nil, nil); !errors.Is(err, apperrors.ErrInvalidInput) {
		t.Errorf("UpdateTransaction(move into balance-only) error = %v, want ErrInvalidInput", err)
	}
	var accountID string
	if err := pool.QueryRow(ctx, `SELECT account_id FROM transactions WHERE id = $1`, txnID).Scan(&accountID); err != nil {
		t.Fatalf("select txn: %v", err)
	}
	if accountID != normalID {
		t.Errorf("txn account = %s, want unchanged %s", accountID, normalID)
	}

	if err := repo.UpdateTransaction(ctx, userID, txnID, normalID, 2500, date, "edited", nil, nil, nil, nil, nil); err != nil {
		t.Errorf("UpdateTransaction(normal): %v", err)
	}
	if got := countLiveTxns(t, pool, normalID); got != 2 {
		t.Errorf("normal live txns = %d, want 2", got)
	}
}

// BalanceOnlyAccountIDs returns only the flagged accounts of that user.
func TestBalanceOnlyAccountIDsIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userA := createTestUser(t, pool, "balance-only-ids-a")
	userB := createTestUser(t, pool, "balance-only-ids-b")

	flagged, err := repo.CreateAccount(ctx, userA, "Flagged", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	if _, err := repo.CreateAccount(ctx, userA, "Plain", "asset", "USD", 0); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	flaggedB, err := repo.CreateAccount(ctx, userB, "B Flagged", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount(B): %v", err)
	}
	for _, p := range []struct{ user, acc string }{{userA, flagged}, {userB, flaggedB}} {
		if _, err := repo.SetAccountBalanceOnly(ctx, p.user, p.acc, true); err != nil {
			t.Fatalf("SetAccountBalanceOnly: %v", err)
		}
	}

	ids, err := repo.BalanceOnlyAccountIDs(ctx, userA)
	if err != nil {
		t.Fatalf("BalanceOnlyAccountIDs: %v", err)
	}
	if len(ids) != 1 || !ids[flagged] {
		t.Errorf("BalanceOnlyAccountIDs(A) = %v, want only %s", ids, flagged)
	}

	empty, err := repo.BalanceOnlyAccountIDs(ctx, createTestUser(t, pool, "balance-only-ids-c"))
	if err != nil {
		t.Fatalf("BalanceOnlyAccountIDs(C): %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("BalanceOnlyAccountIDs(C) = %v, want empty non-nil map", empty)
	}
}

// ---------------------------------------------------------------------------
// Balance-only concurrency (the FOR SHARE race fix)
// ---------------------------------------------------------------------------

// A concurrent importer holding AccountBalanceOnlyForShare's row lock (via an
// open transaction that read the flag and inserted a transaction) must block
// SetAccountBalanceOnly's UPDATE until that transaction ends -- otherwise the
// enable could finish (and soft-delete nothing) before the import commits its
// new row, leaving a live transaction on a balance-only account.
func TestSetAccountBalanceOnlyWaitsForConcurrentImportIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "balance-only-race-import")
	accID, err := repo.CreateAccount(ctx, userID, "Race Import", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	t1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin T1: %v", err)
	}
	defer t1.Rollback(ctx)

	balanceOnly, err := repo.AccountBalanceOnlyForShare(ctx, t1, userID, accID)
	if err != nil {
		t.Fatalf("AccountBalanceOnlyForShare: %v", err)
	}
	if balanceOnly {
		t.Fatalf("AccountBalanceOnlyForShare = true, want false before enabling")
	}

	if _, _, err := repo.InsertIngestedTransaction(ctx, t1, userID, accID, money.Money(-500), mustParseDate(t, "2025-01-10"), "concurrent import", uniqueEmail("race-import-sf"), nil, nil, false); err != nil {
		t.Fatalf("InsertIngestedTransaction: %v", err)
	}

	type result struct {
		deleted int64
		err     error
	}
	done := make(chan result, 1)
	go func() {
		n, err := repo.SetAccountBalanceOnly(ctx, userID, accID, true)
		done <- result{n, err}
	}()

	select {
	case r := <-done:
		t.Fatalf("SetAccountBalanceOnly returned early (deleted=%d err=%v) before T1 committed -- it should have blocked on the share lock", r.deleted, r.err)
	case <-time.After(300 * time.Millisecond):
		// still blocked, as expected
	}

	if err := t1.Commit(ctx); err != nil {
		t.Fatalf("commit T1: %v", err)
	}

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("SetAccountBalanceOnly: %v", r.err)
		}
		if r.deleted != 1 {
			t.Errorf("deleted = %d, want 1", r.deleted)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SetAccountBalanceOnly did not return after T1 committed")
	}

	if got := countLiveTxns(t, pool, accID); got != 0 {
		t.Errorf("live txns = %d, want 0", got)
	}
}

// The reverse ordering: a transaction that already ran the UPDATE to set
// balance_only = true, but hasn't committed yet, must block
// AccountBalanceOnlyForShare's SELECT ... FOR SHARE until it does -- so a
// concurrent import can never observe a stale "false" flag while an enable is
// still in flight.
func TestAccountBalanceOnlyForShareWaitsForEnableIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "balance-only-race-enable")
	accID, err := repo.CreateAccount(ctx, userID, "Race Enable", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	t2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin T2: %v", err)
	}
	defer t2.Rollback(ctx)

	if _, err := t2.Exec(ctx, `UPDATE accounts SET balance_only = true WHERE id = $1`, accID); err != nil {
		t.Fatalf("update balance_only in T2: %v", err)
	}

	type result struct {
		balanceOnly bool
		err         error
	}
	done := make(chan result, 1)
	go func() {
		t3, err := pool.Begin(ctx)
		if err != nil {
			done <- result{false, err}
			return
		}
		defer t3.Rollback(ctx)
		b, err := repo.AccountBalanceOnlyForShare(ctx, t3, userID, accID)
		done <- result{b, err}
	}()

	select {
	case r := <-done:
		t.Fatalf("AccountBalanceOnlyForShare returned early (balanceOnly=%v err=%v) before T2 committed -- it should have blocked", r.balanceOnly, r.err)
	case <-time.After(300 * time.Millisecond):
		// still blocked, as expected
	}

	if err := t2.Commit(ctx); err != nil {
		t.Fatalf("commit T2: %v", err)
	}

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("AccountBalanceOnlyForShare: %v", r.err)
		}
		if !r.balanceOnly {
			t.Errorf("balanceOnly = false, want true")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AccountBalanceOnlyForShare did not return after T2 committed")
	}
}

// Enabling balance_only must also clean up a link between two transactions
// that both belong to the account being flagged (not just cross-account
// links), soft-deleting both sides and leaving no dangling link row.
func TestSetAccountBalanceOnlySameAccountLinkIntegration(t *testing.T) {
	pool := setupTestDB(t)
	ctx := context.Background()
	repo := New(pool)

	userID := createTestUser(t, pool, "balance-only-same-account-link")
	accID, err := repo.CreateAccount(ctx, userID, "Self-Linked", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	txn1ID := insertTxn(t, pool, userID, accID, -1000, "2025-01-10", "source")
	txn2ID := insertTxn(t, pool, userID, accID, 1000, "2025-01-11", "target")
	if _, err := pool.Exec(ctx,
		`INSERT INTO transaction_links (source_transaction_id, target_transaction_id, amount) VALUES ($1, $2, $3)`,
		txn1ID, txn2ID, 500,
	); err != nil {
		t.Fatalf("insert link: %v", err)
	}

	n, err := repo.SetAccountBalanceOnly(ctx, userID, accID, true)
	if err != nil {
		t.Fatalf("SetAccountBalanceOnly: %v", err)
	}
	if n != 2 {
		t.Errorf("deleted = %d, want 2", n)
	}

	var links int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transaction_links WHERE source_transaction_id IN ($1, $2) OR target_transaction_id IN ($1, $2)`,
		txn1ID, txn2ID,
	).Scan(&links); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if links != 0 {
		t.Errorf("links remaining = %d, want 0", links)
	}
}
