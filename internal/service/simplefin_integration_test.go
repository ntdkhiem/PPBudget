package service

// Integration test for the SimpleFin sync balance-snapshot write path
// (docs/plans/balance-snapshots.md §4.3). Runs against a real Postgres
// instance, gated on TEST_DATABASE_URL, and a local httptest.Server standing
// in for the SimpleFin bridge -- no real network calls are made.
//
//	TEST_DATABASE_URL="postgres://ppbudget:ppbudget_password@localhost:55432/ppbudget?sslmode=disable" \
//	  go test -count=1 -run Integration ./internal/service/...

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ntdkhiem/ppbudget-go/internal/config"
	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/internal/repository"
)

var sfEmailCounter int64

func sfUniqueID(prefix string) string {
	n := atomic.AddInt64(&sfEmailCounter, 1)
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), n)
}

func sfSetupTestDB(t *testing.T) *pgxpool.Pool {
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

func sfCreateTestUser(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ($1, 'dummy-hash-not-real') RETURNING id`,
		sfUniqueID("simplefin-integration")+"@balances-integration.test",
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

// newSimplefinServer serves a SimpleFin-shaped GET /accounts response.
func newSimplefinServer(t *testing.T, accounts []SFAccount) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/accounts" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(sfResponse{Accounts: accounts}); err != nil {
			t.Errorf("failed to encode test simplefin response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func waitForImportDone(t *testing.T, userID string) ImportStatus {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st := GetImportProgress(userID)
		if st.Status != "running" {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("import did not finish within timeout; last status: %+v", GetImportProgress(userID))
	return ImportStatus{}
}

func TestSimpleFinExecuteBalanceSnapshotsIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	repo := repository.New(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// No SMTP/Resend config: sendImportNotification either short-circuits
	// (zero imported transactions, as in this test) or -- even with
	// transactions -- returns immediately without making a network call,
	// since s.cfg.SMTPHost and s.cfg.ResendAPIKey are both empty.
	cfg := &config.Config{FrontendURL: "http://localhost:3000"}
	svc := New(repo, logger, cfg)

	userID := sfCreateTestUser(t, pool)

	existingAccountID, err := repo.CreateAccount(ctx, userID, "Existing PP Account", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	newGoodSFID := sfUniqueID("sf-new-good")
	existingMappedSFID := sfUniqueID("sf-existing-mapped")
	badBalanceSFID := sfUniqueID("sf-bad-balance")

	// Single `now` for both derived values -- taking two separate
	// time.Now().UTC() calls risked a midnight flake where todayUnix and
	// todayDate landed on different calendar days.
	now := time.Now().UTC()
	todayUnix := now.Unix()
	todayDate := now.Format("2006-01-02")

	accounts := []SFAccount{
		{ID: newGoodSFID, Name: "New Good", Currency: "USD", Balance: "1000.00", BalanceDate: todayUnix},
		{ID: existingMappedSFID, Name: "Existing Mapped", Currency: "USD", Balance: "2500.50", BalanceDate: todayUnix},
		{ID: badBalanceSFID, Name: "Bad Balance", Currency: "USD", Balance: "not-a-number", BalanceDate: todayUnix},
	}
	srv := newSimplefinServer(t, accounts)

	mapping := map[string]string{
		newGoodSFID:        "new",
		existingMappedSFID: existingAccountID,
		badBalanceSFID:     "new",
	}

	if err := svc.SimpleFinExecute(ctx, userID, SimplefinExecuteRequest{
		AccessURL:      srv.URL,
		AccountMapping: mapping,
	}); err != nil {
		t.Fatalf("SimpleFinExecute: %v", err)
	}

	final := waitForImportDone(t, userID)
	if final.Status != "completed" {
		t.Fatalf("import finished with status %q, want %q (error: %s)", final.Status, "completed", final.Error)
	}

	t.Run("mapped_new_account_gets_snapshot_with_zero_transactions", func(t *testing.T) {
		newAccountID, err := repo.GetAccountBySimplefinID(ctx, nil, userID, newGoodSFID)
		if err != nil {
			t.Fatalf("GetAccountBySimplefinID(new good): %v", err)
		}
		snapshots, err := repo.ListBalanceSnapshots(ctx, userID, newAccountID)
		if err != nil {
			t.Fatalf("ListBalanceSnapshots(new good): %v", err)
		}
		var sfSnap *domain.BalanceSnapshot
		for i := range snapshots {
			if snapshots[i].Source == domain.BalanceSourceSimplefin {
				sfSnap = &snapshots[i]
			}
		}
		if sfSnap == nil {
			t.Fatalf("no simplefin snapshot found for new good account")
		}
		if sfSnap.Balance.ToInt64() != 100000 {
			t.Errorf("balance = %d, want 100000", sfSnap.Balance.ToInt64())
		}
		if sfSnap.AsOfDate == nil || sfSnap.AsOfDate.Format("2006-01-02") != todayDate {
			t.Errorf("as_of_date = %v, want %s", sfSnap.AsOfDate, todayDate)
		}
	})

	t.Run("mapped_existing_account_gets_snapshot_with_zero_transactions", func(t *testing.T) {
		snapshots, err := repo.ListBalanceSnapshots(ctx, userID, existingAccountID)
		if err != nil {
			t.Fatalf("ListBalanceSnapshots(existing mapped): %v", err)
		}
		var sfSnap *domain.BalanceSnapshot
		for i := range snapshots {
			if snapshots[i].Source == domain.BalanceSourceSimplefin {
				sfSnap = &snapshots[i]
			}
		}
		if sfSnap == nil {
			t.Fatalf("no simplefin snapshot found for existing mapped account")
		}
		if sfSnap.Balance.ToInt64() != 250050 {
			t.Errorf("balance = %d, want 250050", sfSnap.Balance.ToInt64())
		}
		if sfSnap.AsOfDate == nil || sfSnap.AsOfDate.Format("2006-01-02") != todayDate {
			t.Errorf("as_of_date = %v, want %s", sfSnap.AsOfDate, todayDate)
		}
	})

	t.Run("unparseable_balance_gets_no_snapshot_but_account_and_others_still_import", func(t *testing.T) {
		badAccountID, err := repo.GetAccountBySimplefinID(ctx, nil, userID, badBalanceSFID)
		if err != nil {
			t.Fatalf("GetAccountBySimplefinID(bad balance): %v (account should still be created)", err)
		}
		snapshots, err := repo.ListBalanceSnapshots(ctx, userID, badAccountID)
		if err != nil {
			t.Fatalf("ListBalanceSnapshots(bad balance): %v", err)
		}
		for _, s := range snapshots {
			if s.Source == domain.BalanceSourceSimplefin {
				t.Errorf("expected no simplefin snapshot for unparseable-balance account, found one: %+v", s)
			}
		}
		// The other two accounts still got their snapshots (checked above), proving
		// one bad account doesn't abort the rest of the import.
	})

	t.Run("resync_same_day_updates_snapshot_instead_of_inserting", func(t *testing.T) {
		updatedAccounts := []SFAccount{
			{ID: existingMappedSFID, Name: "Existing Mapped", Currency: "USD", Balance: "2600.75", BalanceDate: time.Now().UTC().Unix()},
		}
		srv2 := newSimplefinServer(t, updatedAccounts)

		if err := svc.SimpleFinExecute(ctx, userID, SimplefinExecuteRequest{
			AccessURL:      srv2.URL,
			AccountMapping: map[string]string{existingMappedSFID: existingAccountID},
		}); err != nil {
			t.Fatalf("SimpleFinExecute (resync): %v", err)
		}
		final := waitForImportDone(t, userID)
		if final.Status != "completed" {
			t.Fatalf("resync finished with status %q, want %q (error: %s)", final.Status, "completed", final.Error)
		}

		snapshots, err := repo.ListBalanceSnapshots(ctx, userID, existingAccountID)
		if err != nil {
			t.Fatalf("ListBalanceSnapshots (after resync): %v", err)
		}
		var sfSnaps []domain.BalanceSnapshot
		for _, s := range snapshots {
			if s.Source == domain.BalanceSourceSimplefin {
				sfSnaps = append(sfSnaps, s)
			}
		}
		if len(sfSnaps) != 1 {
			t.Fatalf("expected exactly 1 simplefin snapshot after same-day resync, got %d: %+v", len(sfSnaps), sfSnaps)
		}
		if sfSnaps[0].Balance.ToInt64() != 260075 {
			t.Errorf("balance after resync = %d, want 260075 (updated, not appended)", sfSnaps[0].Balance.ToInt64())
		}
	})
}

// TestSimpleFinExecuteBalanceOnlyIntegration covers the balance-only account
// path end to end: an account already flagged balance_only, one newly
// requested via BalanceOnlyAccounts on a "new" mapping, one mapped to "skip",
// and one normal account -- all in a single sync -- to confirm transactions
// are skipped only for the balance-only/skip accounts while the snapshot
// write and the normal account's import both proceed unaffected.
func TestSimpleFinExecuteBalanceOnlyIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	repo := repository.New(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{FrontendURL: "http://localhost:3000"}
	svc := New(repo, logger, cfg)

	userID := sfCreateTestUser(t, pool)

	// (a) An existing PP account already flagged balance_only, mapped to an
	// SF account that reports 3 transactions.
	flaggedAccountID, err := repo.CreateAccount(ctx, userID, "Roth IRA", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount(flagged): %v", err)
	}
	if _, err := repo.SetAccountBalanceOnly(ctx, userID, flaggedAccountID, true); err != nil {
		t.Fatalf("SetAccountBalanceOnly(flagged): %v", err)
	}

	flaggedSFID := sfUniqueID("sf-balance-only-existing")
	newBalanceOnlySFID := sfUniqueID("sf-balance-only-new")
	skipSFID := sfUniqueID("sf-skip")
	normalSFID := sfUniqueID("sf-normal")

	now := time.Now().UTC()
	todayUnix := now.Unix()
	todayDate := now.Format("2006-01-02")

	txn := func(id, amount string) sfTransaction {
		return sfTransaction{ID: id, Posted: todayUnix, Amount: amount, Description: "test txn " + id, Pending: false}
	}

	accounts := []SFAccount{
		{
			ID: flaggedSFID, Name: "Roth IRA", Currency: "USD", Balance: "50000.00", BalanceDate: todayUnix,
			Transactions: []sfTransaction{txn("f1", "-10.00"), txn("f2", "-20.00"), txn("f3", "-30.00")},
		},
		{
			ID: newBalanceOnlySFID, Name: "401k", Currency: "USD", Balance: "75000.00", BalanceDate: todayUnix,
			Transactions: []sfTransaction{txn("n1", "-5.00")},
		},
		{
			ID: skipSFID, Name: "Skip Me", Currency: "USD", Balance: "999.00", BalanceDate: todayUnix,
			Transactions: []sfTransaction{txn("s1", "-1.00")},
		},
		{
			ID: normalSFID, Name: "Checking", Currency: "USD", Balance: "1500.00", BalanceDate: todayUnix,
			Transactions: []sfTransaction{txn("c1", "-100.00"), txn("c2", "-200.00")},
		},
	}
	srv := newSimplefinServer(t, accounts)

	mapping := map[string]string{
		flaggedSFID:        flaggedAccountID,
		newBalanceOnlySFID: "new",
		skipSFID:           "skip",
		normalSFID:         "new",
	}

	if err := svc.SimpleFinExecute(ctx, userID, SimplefinExecuteRequest{
		AccessURL:           srv.URL,
		AccountMapping:      mapping,
		BalanceOnlyAccounts: []string{newBalanceOnlySFID},
	}); err != nil {
		t.Fatalf("SimpleFinExecute: %v", err)
	}

	final := waitForImportDone(t, userID)
	if final.Status != "completed" {
		t.Fatalf("import finished with status %q, want %q (error: %s)", final.Status, "completed", final.Error)
	}

	countLiveTxns := func(t *testing.T, accountID string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM transactions WHERE account_id = $1 AND deleted_at IS NULL`, accountID).Scan(&n); err != nil {
			t.Fatalf("count transactions for %s: %v", accountID, err)
		}
		return n
	}

	simplefinSnapshot := func(t *testing.T, accountID string) *domain.BalanceSnapshot {
		t.Helper()
		snapshots, err := repo.ListBalanceSnapshots(ctx, userID, accountID)
		if err != nil {
			t.Fatalf("ListBalanceSnapshots(%s): %v", accountID, err)
		}
		for i := range snapshots {
			if snapshots[i].Source == domain.BalanceSourceSimplefin {
				return &snapshots[i]
			}
		}
		return nil
	}

	t.Run("already_flagged_account_gets_no_transactions_but_gets_snapshot", func(t *testing.T) {
		if n := countLiveTxns(t, flaggedAccountID); n != 0 {
			t.Errorf("transaction count = %d, want 0", n)
		}
		snap := simplefinSnapshot(t, flaggedAccountID)
		if snap == nil {
			t.Fatalf("no simplefin snapshot found for already-flagged account")
		}
		if snap.Balance.ToInt64() != 5000000 {
			t.Errorf("balance = %d, want 5000000", snap.Balance.ToInt64())
		}
		if snap.AsOfDate == nil || snap.AsOfDate.Format("2006-01-02") != todayDate {
			t.Errorf("as_of_date = %v, want %s", snap.AsOfDate, todayDate)
		}
	})

	t.Run("new_mapping_requested_balance_only_creates_flagged_account_with_no_transactions", func(t *testing.T) {
		newAccountID, err := repo.GetAccountBySimplefinID(ctx, nil, userID, newBalanceOnlySFID)
		if err != nil {
			t.Fatalf("GetAccountBySimplefinID(new balance-only): %v", err)
		}
		acc, err := repo.GetAccount(ctx, userID, newAccountID)
		if err != nil {
			t.Fatalf("GetAccount(new balance-only): %v", err)
		}
		if !acc.BalanceOnly {
			t.Errorf("BalanceOnly = false, want true")
		}
		if n := countLiveTxns(t, newAccountID); n != 0 {
			t.Errorf("transaction count = %d, want 0", n)
		}
		snap := simplefinSnapshot(t, newAccountID)
		if snap == nil {
			t.Fatalf("no simplefin snapshot found for newly-flagged account")
		}
		if snap.Balance.ToInt64() != 7500000 {
			t.Errorf("balance = %d, want 7500000", snap.Balance.ToInt64())
		}
	})

	t.Run("skip_mapping_creates_no_account_and_no_error", func(t *testing.T) {
		if _, err := repo.GetAccountBySimplefinID(ctx, nil, userID, skipSFID); !errors.Is(err, apperrors.ErrNotFound) {
			t.Errorf("GetAccountBySimplefinID(skip) error = %v, want ErrNotFound", err)
		}
		if final.Error != "" {
			t.Errorf("import error = %q, want empty", final.Error)
		}
	})

	t.Run("normal_account_still_imports_its_transactions", func(t *testing.T) {
		newAccountID, err := repo.GetAccountBySimplefinID(ctx, nil, userID, normalSFID)
		if err != nil {
			t.Fatalf("GetAccountBySimplefinID(normal): %v", err)
		}
		if n := countLiveTxns(t, newAccountID); n != 2 {
			t.Errorf("transaction count = %d, want 2", n)
		}
		snap := simplefinSnapshot(t, newAccountID)
		if snap == nil {
			t.Fatalf("no simplefin snapshot found for normal account")
		}
		if snap.Balance.ToInt64() != 150000 {
			t.Errorf("balance = %d, want 150000", snap.Balance.ToInt64())
		}
	})

	t.Run("progress_total_counts_only_normal_account_transactions", func(t *testing.T) {
		if final.Total != 2 {
			t.Errorf("Total = %d, want 2 (only the normal account's transactions)", final.Total)
		}
	})
}

// RecordManualBalance's date bound: clients may send their local calendar
// date, which can be up to one day ahead of UTC, so today and tomorrow (UTC)
// must succeed, today+2 must fail with ErrInvalidInput, and a nil asOf must
// be stored as today's UTC date.
func TestRecordManualBalanceDateBoundsIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	repo := repository.New(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{FrontendURL: "http://localhost:3000"}
	svc := New(repo, logger, cfg)

	userID := sfCreateTestUser(t, pool)

	accID, err := repo.CreateAccount(ctx, userID, "Manual Balance Bounds", "asset", "USD", 10000)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	tomorrow := today.AddDate(0, 0, 1)
	dayAfterTomorrow := today.AddDate(0, 0, 2)

	// manualBalanceOn returns the manual snapshot balance stored for date, if any.
	manualBalanceOn := func(t *testing.T, date time.Time) (int64, bool) {
		t.Helper()
		snapshots, err := repo.ListBalanceSnapshots(ctx, userID, accID)
		if err != nil {
			t.Fatalf("ListBalanceSnapshots: %v", err)
		}
		for _, s := range snapshots {
			if s.Source == domain.BalanceSourceManual && s.AsOfDate != nil && s.AsOfDate.Equal(date) {
				return s.Balance.ToInt64(), true
			}
		}
		return 0, false
	}

	t.Run("today_utc_succeeds", func(t *testing.T) {
		if err := svc.RecordManualBalance(ctx, userID, accID, 11100, &today); err != nil {
			t.Fatalf("RecordManualBalance(today): %v", err)
		}
		if got, ok := manualBalanceOn(t, today); !ok || got != 11100 {
			t.Errorf("manual snapshot on %s = (%d, %v), want (11100, true)", today.Format("2006-01-02"), got, ok)
		}
	})

	t.Run("utc_tomorrow_succeeds", func(t *testing.T) {
		if err := svc.RecordManualBalance(ctx, userID, accID, 11200, &tomorrow); err != nil {
			t.Fatalf("RecordManualBalance(tomorrow): %v", err)
		}
		if got, ok := manualBalanceOn(t, tomorrow); !ok || got != 11200 {
			t.Errorf("manual snapshot on %s = (%d, %v), want (11200, true)", tomorrow.Format("2006-01-02"), got, ok)
		}
	})

	t.Run("utc_today_plus_2_rejected", func(t *testing.T) {
		err := svc.RecordManualBalance(ctx, userID, accID, 99999, &dayAfterTomorrow)
		if !errors.Is(err, apperrors.ErrInvalidInput) {
			t.Errorf("RecordManualBalance(today+2) error = %v, want ErrInvalidInput", err)
		}
		if _, ok := manualBalanceOn(t, dayAfterTomorrow); ok {
			t.Errorf("a snapshot was stored for a rejected date")
		}
	})

	t.Run("nil_date_stored_as_today_utc", func(t *testing.T) {
		if err := svc.RecordManualBalance(ctx, userID, accID, 11300, nil); err != nil {
			t.Fatalf("RecordManualBalance(nil): %v", err)
		}
		// Same-day upsert: today's snapshot now holds the new balance.
		if got, ok := manualBalanceOn(t, today); !ok || got != 11300 {
			t.Errorf("manual snapshot on today = (%d, %v), want (11300, true)", got, ok)
		}
	})
}

// ---------------------------------------------------------------------------
// Balance-only accounts in SimpleFinExecute
// ---------------------------------------------------------------------------

// sfInsertTxn inserts a manually-entered (non-SimpleFin) live transaction
// directly, mirroring the repository package's insertTxn helper.
func sfInsertTxn(t *testing.T, pool *pgxpool.Pool, userID, accountID string, amountCents int64, dateStr, description string) string {
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

func sfCountLiveTxns(t *testing.T, pool *pgxpool.Pool, accountID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE account_id = $1 AND deleted_at IS NULL`, accountID,
	).Scan(&n); err != nil {
		t.Fatalf("count live txns: %v", err)
	}
	return n
}

func sfSimplefinSnapshot(t *testing.T, repo *repository.Repository, userID, accountID string) *domain.BalanceSnapshot {
	t.Helper()
	snapshots, err := repo.ListBalanceSnapshots(context.Background(), userID, accountID)
	if err != nil {
		t.Fatalf("ListBalanceSnapshots(%s): %v", accountID, err)
	}
	for i := range snapshots {
		if snapshots[i].Source == domain.BalanceSourceSimplefin {
			return &snapshots[i]
		}
	}
	return nil
}

// An existing PP account that is NOT yet flagged balance-only, holding 2
// manually-entered live transactions, gets flagged via BalanceOnlyAccounts
// during a sync mapped to an SF account reporting 3 transactions: the 2 old
// transactions must be soft-deleted, none of the 3 SF transactions imported,
// and the balance snapshot still written.
func TestSimpleFinExecuteBalanceOnlyExistingAccountDeletesTransactionsIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	repo := repository.New(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{FrontendURL: "http://localhost:3000"}
	svc := New(repo, logger, cfg)

	userID := sfCreateTestUser(t, pool)

	existingAccountID, err := repo.CreateAccount(ctx, userID, "Existing Not Yet Flagged", "asset", "USD", 0)
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	sfInsertTxn(t, pool, userID, existingAccountID, -1000, "2025-01-05", "manual 1")
	sfInsertTxn(t, pool, userID, existingAccountID, -2000, "2025-01-06", "manual 2")

	sfID := sfUniqueID("sf-existing-becomes-balance-only")
	todayUnix := time.Now().UTC().Unix()

	accounts := []SFAccount{
		{
			ID: sfID, Name: "Becomes Balance Only", Currency: "USD", Balance: "9000.00", BalanceDate: todayUnix,
			Transactions: []sfTransaction{
				{ID: "bo1", Posted: todayUnix, Amount: "-10.00", Description: "t1"},
				{ID: "bo2", Posted: todayUnix, Amount: "-20.00", Description: "t2"},
				{ID: "bo3", Posted: todayUnix, Amount: "-30.00", Description: "t3"},
			},
		},
	}
	srv := newSimplefinServer(t, accounts)

	if err := svc.SimpleFinExecute(ctx, userID, SimplefinExecuteRequest{
		AccessURL:           srv.URL,
		AccountMapping:      map[string]string{sfID: existingAccountID},
		BalanceOnlyAccounts: []string{sfID},
	}); err != nil {
		t.Fatalf("SimpleFinExecute: %v", err)
	}

	final := waitForImportDone(t, userID)
	if final.Status != "completed" {
		t.Fatalf("import finished with status %q, want completed (error: %s)", final.Status, final.Error)
	}

	acc, err := repo.GetAccount(ctx, userID, existingAccountID)
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if !acc.BalanceOnly {
		t.Error("BalanceOnly = false, want true")
	}

	if n := sfCountLiveTxns(t, pool, existingAccountID); n != 0 {
		t.Errorf("live txns = %d, want 0 (2 old soft-deleted, 0 SF txns imported)", n)
	}

	snap := sfSimplefinSnapshot(t, repo, userID, existingAccountID)
	if snap == nil {
		t.Fatal("no simplefin snapshot found")
	}
	if snap.Balance.ToInt64() != 900000 {
		t.Errorf("snapshot balance = %d, want 900000", snap.Balance.ToInt64())
	}
}

// A sync using a saved "new" mapping (no BalanceOnlyAccounts in the request)
// against an account that was flagged balance-only after being created by an
// earlier sync must still resolve the mapping to that existing account, skip
// importing its transactions, and finish with Current == Total, Total == 0
// (this being the only mapped account) -- while still updating the balance
// snapshot.
func TestSimpleFinExecuteSavedNewMappingOnFlaggedAccountIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	repo := repository.New(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{FrontendURL: "http://localhost:3000"}
	svc := New(repo, logger, cfg)

	userID := sfCreateTestUser(t, pool)
	sfID := sfUniqueID("sf-saved-new-then-flagged")
	todayUnix := time.Now().UTC().Unix()

	firstAccounts := []SFAccount{
		{
			ID: sfID, Name: "Later Flagged", Currency: "USD", Balance: "100.00", BalanceDate: todayUnix,
			Transactions: []sfTransaction{
				{ID: "sn1", Posted: todayUnix, Amount: "-1.00", Description: "t1"},
				{ID: "sn2", Posted: todayUnix, Amount: "-2.00", Description: "t2"},
			},
		},
	}
	srv1 := newSimplefinServer(t, firstAccounts)
	mapping := map[string]string{sfID: "new"}

	if err := svc.SimpleFinExecute(ctx, userID, SimplefinExecuteRequest{AccessURL: srv1.URL, AccountMapping: mapping}); err != nil {
		t.Fatalf("SimpleFinExecute (first sync): %v", err)
	}
	first := waitForImportDone(t, userID)
	if first.Status != "completed" {
		t.Fatalf("first sync finished with status %q, want completed (error: %s)", first.Status, first.Error)
	}

	accountID, err := repo.GetAccountBySimplefinID(ctx, nil, userID, sfID)
	if err != nil {
		t.Fatalf("GetAccountBySimplefinID: %v", err)
	}
	if n := sfCountLiveTxns(t, pool, accountID); n != 2 {
		t.Fatalf("live txns after first sync = %d, want 2", n)
	}

	if _, err := repo.SetAccountBalanceOnly(ctx, userID, accountID, true); err != nil {
		t.Fatalf("SetAccountBalanceOnly: %v", err)
	}
	if n := sfCountLiveTxns(t, pool, accountID); n != 0 {
		t.Fatalf("live txns after enabling balance-only = %d, want 0", n)
	}

	secondAccounts := []SFAccount{
		{
			ID: sfID, Name: "Later Flagged", Currency: "USD", Balance: "150.00", BalanceDate: todayUnix,
			Transactions: []sfTransaction{
				{ID: "sn1", Posted: todayUnix, Amount: "-1.00", Description: "t1"},
				{ID: "sn2", Posted: todayUnix, Amount: "-2.00", Description: "t2"},
				{ID: "sn3", Posted: todayUnix, Amount: "-3.00", Description: "t3 (new)"},
			},
		},
	}
	srv2 := newSimplefinServer(t, secondAccounts)

	if err := svc.SimpleFinExecute(ctx, userID, SimplefinExecuteRequest{AccessURL: srv2.URL, AccountMapping: mapping}); err != nil {
		t.Fatalf("SimpleFinExecute (second sync): %v", err)
	}
	second := waitForImportDone(t, userID)
	if second.Status != "completed" {
		t.Fatalf("second sync finished with status %q, want completed (error: %s)", second.Status, second.Error)
	}

	if n := sfCountLiveTxns(t, pool, accountID); n != 0 {
		t.Errorf("live txns after second sync = %d, want 0 (account is balance-only)", n)
	}
	if second.Current != second.Total {
		t.Errorf("Current = %d, Total = %d, want equal", second.Current, second.Total)
	}
	if second.Total != 0 {
		t.Errorf("Total = %d, want 0 (only mapped account is balance-only)", second.Total)
	}

	snap := sfSimplefinSnapshot(t, repo, userID, accountID)
	if snap == nil {
		t.Fatal("no simplefin snapshot found")
	}
	if snap.Balance.ToInt64() != 15000 {
		t.Errorf("snapshot balance = %d, want 15000 (updated by second sync)", snap.Balance.ToInt64())
	}
}

// After a normal sync imports transactions, enabling then disabling
// balance-only (which soft-deletes but never restores), and re-syncing with
// the same SimpleFin transactions plus one genuinely new one: the two old
// ids must stay soft-deleted (ON CONFLICT (user_id, simplefin_id) DO NOTHING, not
// resurrected), only the new transaction ends up live, and the balance
// snapshot still updates.
func TestSimpleFinReimportAfterDisableIntegration(t *testing.T) {
	pool := sfSetupTestDB(t)
	ctx := context.Background()
	repo := repository.New(pool)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{FrontendURL: "http://localhost:3000"}
	svc := New(repo, logger, cfg)

	userID := sfCreateTestUser(t, pool)
	sfID := sfUniqueID("sf-reimport-after-disable")
	todayUnix := time.Now().UTC().Unix()

	mkAccounts := func(balance string, txns []sfTransaction) []SFAccount {
		return []SFAccount{{ID: sfID, Name: "Reimport Test", Currency: "USD", Balance: balance, BalanceDate: todayUnix, Transactions: txns}}
	}

	srv1 := newSimplefinServer(t, mkAccounts("100.00", []sfTransaction{
		{ID: "ri1", Posted: todayUnix, Amount: "-1.00", Description: "t1"},
		{ID: "ri2", Posted: todayUnix, Amount: "-2.00", Description: "t2"},
	}))
	mapping := map[string]string{sfID: "new"}

	if err := svc.SimpleFinExecute(ctx, userID, SimplefinExecuteRequest{AccessURL: srv1.URL, AccountMapping: mapping}); err != nil {
		t.Fatalf("SimpleFinExecute (initial sync): %v", err)
	}
	first := waitForImportDone(t, userID)
	if first.Status != "completed" {
		t.Fatalf("initial sync finished with status %q, want completed (error: %s)", first.Status, first.Error)
	}

	accountID, err := repo.GetAccountBySimplefinID(ctx, nil, userID, sfID)
	if err != nil {
		t.Fatalf("GetAccountBySimplefinID: %v", err)
	}
	if n := sfCountLiveTxns(t, pool, accountID); n != 2 {
		t.Fatalf("live txns after initial sync = %d, want 2", n)
	}

	if _, err := repo.SetAccountBalanceOnly(ctx, userID, accountID, true); err != nil {
		t.Fatalf("SetAccountBalanceOnly(enable): %v", err)
	}
	if n := sfCountLiveTxns(t, pool, accountID); n != 0 {
		t.Fatalf("live txns after enabling balance-only = %d, want 0", n)
	}
	if _, err := repo.SetAccountBalanceOnly(ctx, userID, accountID, false); err != nil {
		t.Fatalf("SetAccountBalanceOnly(disable): %v", err)
	}

	srv2 := newSimplefinServer(t, mkAccounts("140.00", []sfTransaction{
		{ID: "ri1", Posted: todayUnix, Amount: "-1.00", Description: "t1"},
		{ID: "ri2", Posted: todayUnix, Amount: "-2.00", Description: "t2"},
		{ID: "ri3", Posted: todayUnix, Amount: "-4.00", Description: "t3 (genuinely new)"},
	}))

	if err := svc.SimpleFinExecute(ctx, userID, SimplefinExecuteRequest{AccessURL: srv2.URL, AccountMapping: mapping}); err != nil {
		t.Fatalf("SimpleFinExecute (reimport): %v", err)
	}
	second := waitForImportDone(t, userID)
	if second.Status != "completed" {
		t.Fatalf("reimport finished with status %q, want completed (error: %s)", second.Status, second.Error)
	}

	if n := sfCountLiveTxns(t, pool, accountID); n != 1 {
		t.Errorf("live txns after reimport = %d, want 1 (only the genuinely new transaction)", n)
	}

	for _, oldSFID := range []string{"ri1", "ri2"} {
		var deletedAt *time.Time
		if err := pool.QueryRow(ctx, `SELECT deleted_at FROM transactions WHERE account_id = $1 AND simplefin_id = $2`, accountID, oldSFID).Scan(&deletedAt); err != nil {
			t.Fatalf("select old txn %s: %v", oldSFID, err)
		}
		if deletedAt == nil {
			t.Errorf("txn with simplefin_id=%s is live, want soft-deleted", oldSFID)
		}
	}

	var newLiveSFID string
	if err := pool.QueryRow(ctx, `SELECT simplefin_id FROM transactions WHERE account_id = $1 AND deleted_at IS NULL`, accountID).Scan(&newLiveSFID); err != nil {
		t.Fatalf("select new live txn: %v", err)
	}
	if newLiveSFID != "ri3" {
		t.Errorf("live txn simplefin_id = %s, want ri3", newLiveSFID)
	}

	snap := sfSimplefinSnapshot(t, repo, userID, accountID)
	if snap == nil {
		t.Fatal("no simplefin snapshot found")
	}
	if snap.Balance.ToInt64() != 14000 {
		t.Errorf("snapshot balance = %d, want 14000 (updated by reimport)", snap.Balance.ToInt64())
	}
}
