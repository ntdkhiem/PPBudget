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
