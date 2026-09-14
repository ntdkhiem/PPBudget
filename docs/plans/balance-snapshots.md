# Plan: Replace transaction-derived balances with balance snapshots

Repo: `C:\Users\kevin\Documents\go\PPBudget` (Go API + Next.js 16 frontend, Postgres).
Goal: an account's balance is anchored to **snapshots** (SimpleFin-reported balance at each sync, manual "update balance" entries, or an opening balance), with transactions only filling the gap between anchors. This makes investment accounts (Fidelity brokerage / Roth IRA / 401k) track market value, and makes "Net Worth Progression" truthful.

---

## 0. Current state (verified in code)

Balance today is always `accounts.initial_balance + SUM(transactions.amount)`, computed in 6 places:

| # | Location | Purpose |
|---|---|---|
| 1 | `internal/repository/repository.go:101-243` `ListTransactions` | `running_balance` window fn (line 107). **Already buggy**: window runs after WHERE/LIMIT, so date/search filters corrupt it. |
| 2 | `repository.go:362-391` `ListAccounts` | `current_balance` |
| 3 | `repository.go:458-479` `GetAccount` | `current_balance` |
| 4 | `repository.go:644-688` `GetNetWorthTrend` | monthly trend; `net_worth = assets + liabilities` |
| 5 | `repository.go:821-839` (in reports summary) | `net_worth = assets - liabilities` ← **sign convention contradicts #4** |
| 6 | `internal/repository/search.go:82-110` `SearchAccounts` | `current_balance` |

Writes of `initial_balance`: `repository.go:452` `CreateAccount`, `repository.go:481` `UpdateAccount`, `repository/simplefin.go:9-23` `UpsertSimplefinAccount` (set once on INSERT; ON CONFLICT never refreshes it — root cause of investment drift).

SimpleFin parsing: `internal/service/simplefin.go:36-50` `SFAccount` only has `id,name,currency,balance,transactions` (drops `balance-date`, `available-balance`, `holdings`). Import loop: `simplefin.go:295-374` (goroutine, per-account DB tx at 324/371). Auto-sync: `simplefin.go:612-667` (30-day window).

API surface: `internal/handler/handlers.go:528-616` (Create/Get/UpdateAccount bodies carry `initial_balance`), routes in `cmd/api/main.go` (~line 163 `/reports/net-worth`).

Frontend consumers:
- `frontend/lib/api.ts:36-52` (`Account.initial_balance`, `current_balance?`, `running_balance?`), `:123-141` (`NetWorthDataPoint`, `DashboardSummary.net_worth`).
- `frontend/app/(dashboard)/accounts/page.tsx:32-109,145,188-227` (create/edit forms with "Initial Balance", totals with `?? initial_balance` fallback).
- `frontend/app/(dashboard)/accounts/[id]/page.tsx:179` (`running_balance`).
- `frontend/app/(dashboard)/page.tsx:222-224` — dead `netWorth` variable using `initial_balance` (unused); net worth card at `:254-338`.

Migrations: `internal/db/migrations/NNNN_name.{up,down}.sql`, latest `0018_drop_transfer_id`. **Not run on startup** (README claim is false); applied with golang-migrate CLI (`make migrate-up`). Local tooling verified present: docker, migrate, psql, go 1.25.5, node 22.

Dependencies: **no Go or npm package additions, upgrades, or downgrades are required.** (pgx/v5 already supports everything below.)

---

## 1. Decisions (approved by user 2026-09-13)

**D1 – APPROVED. Drop `accounts.initial_balance`; replace with an "opening" snapshot.**
Single source of truth. API `initial_balance` field becomes `opening_balance`.

**D2 – APPROVED. Signed balance convention: liabilities are stored negative; net worth = SUM(asset balances) + SUM(liability balances).**
Matches SimpleFin (credit cards report negative) and the trend query; fixes the summary query (#5). User has created no manual liability accounts, so no data needs flipping; still run `SELECT name, initial_balance FROM accounts WHERE type='liability';` at rollout step 2 as a sanity check (expect 0 rows).

**D3 – Two-step migration for zero-downtime deploys.**
`0019` is additive (old backend keeps working); `0020` drops `initial_balance` after the new backend is live.

**D4 – Integration tests against real Postgres, gated on `TEST_DATABASE_URL`, plus a Makefile target. CI Postgres service: SKIPPED by user** — do not modify `.github/workflows/ci.yml`; tests `t.Skip` in CI and run locally only.

**Out of scope** — tracked in [`followups.md`](followups.md). Do not implement here.

### Balance semantics (user FAQ)

*"Will the balance only come from the SimpleFin snapshot, so transaction changes won't affect it?"* — Not quite:

- **Balance = latest snapshot on/before the date + transactions dated after that snapshot.** Transactions still matter; the snapshot just resets the starting point.
- **SimpleFin-synced accounts:** every sync (~12h) writes a new snapshot, so the current balance is essentially the bank-reported number. Adding, editing or deleting a transaction dated *on or before* the latest snapshot does not change the current balance (the bank figure is treated as the truth — e.g. deleting a duplicate import won't distort it). A transaction dated *after* the latest snapshot does change it until the next sync re-anchors.
- **Historical balances** (net-worth trend for past months) between two snapshots are still derived from transactions, so editing an old transaction can change past points on the chart, but never a snapshot value itself.
- **Manual accounts** (never synced) only have an opening snapshot, so they behave exactly like today: opening balance + all transactions. Using "Update balance" adds a manual snapshot that anchors the same way a sync does.
- Recategorizing, linking, or reviewing transactions never affects balances (amount unchanged).

---

## 2. Target design (the contract every agent codes against)

### 2.1 Schema — migration `0019_balance_snapshots.up.sql`

```sql
CREATE TABLE account_balance_snapshots (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id        UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    as_of_date        DATE NOT NULL,        -- end-of-day balance, UTC; '-infinity' = opening balance
    balance           BIGINT NOT NULL,      -- cents, signed (D2)
    available_balance BIGINT,               -- cents, nullable (SimpleFin available-balance)
    source            TEXT NOT NULL CHECK (source IN ('opening','simplefin','manual')),
    reported_at       TIMESTAMPTZ,          -- SimpleFin balance-date, or NOW() for manual
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, as_of_date),
    CHECK ((source = 'opening') = (as_of_date = '-infinity'::date))
);
CREATE INDEX idx_balance_snapshots_account_date ON account_balance_snapshots (account_id, as_of_date DESC);
CREATE INDEX idx_balance_snapshots_user ON account_balance_snapshots (user_id);
CREATE TRIGGER update_balance_snapshot_modtime BEFORE UPDATE ON account_balance_snapshots
    FOR EACH ROW EXECUTE PROCEDURE update_modified_column();

-- Seed: every existing account gets an opening snapshot equal to initial_balance.
INSERT INTO account_balance_snapshots (user_id, account_id, as_of_date, balance, source)
SELECT user_id, id, '-infinity', initial_balance, 'opening' FROM accounts;
```
(`users(id UUID)` and `update_modified_column()` already exist — verified in migrations 0014 and 0001.)

### 2.2 Balance function (same migration)

`account_balance_at(p_account_id UUID, p_as_of DATE) RETURNS BIGINT`, `LANGUAGE plpgsql STABLE`. End-of-day inclusive. Algorithm:

1. `prev` = snapshot with greatest `as_of_date <= p_as_of` (opening at `-infinity` always qualifies if present).
   → return `prev.balance + SUM(t.amount)` for non-deleted txns of the account with `prev.as_of_date < t.date <= p_as_of`.
2. Else `next` = snapshot with smallest `as_of_date` (i.e. account only has future-dated anchors, e.g. a freshly connected SimpleFin account whose 30 days of history predate its first snapshot).
   → return `next.balance - SUM(t.amount)` for non-deleted txns with `p_as_of < t.date <= next.as_of_date`.
3. Else (no snapshots at all — should not happen after migration) → `SUM(t.amount)` for txns with `t.date <= p_as_of`, COALESCE 0.

"Current balance" everywhere = `account_balance_at(id, 'infinity'::date)` (preserves today's behaviour of counting future-dated txns).

Consequences to preserve/accept:
- Manual accounts with only an opening snapshot behave **exactly** as today (opening + all txns). Migration parity must be 100%.
- For SimpleFin accounts, each sync re-anchors the balance → market moves, dividends, fees and missed txns self-correct.
- Transactions dated on/before the latest snapshot no longer change the current balance (they are assumed already reflected in the bank-reported number). Expected and correct.

### 2.3 Migration `0019_balance_snapshots.down.sql`
`DROP FUNCTION account_balance_at; DROP TABLE account_balance_snapshots;`

### 2.4 Migration `0020_drop_initial_balance.up.sql` (applied only after new backend is deployed)
1. Re-seed opening snapshots for any account missing one (accounts created by old code between 0019 and deploy): `INSERT ... SELECT ... WHERE NOT EXISTS (opening snapshot)`, using `initial_balance`.
2. `ALTER TABLE accounts DROP COLUMN initial_balance;`

Down: add column back `BIGINT NOT NULL DEFAULT 0`, backfill from opening snapshots.

### 2.5 Go domain (`internal/domain/domain.go`)

```go
type Account struct {
    ID, UserID, Name, Type, Currency string (existing tags)
    CurrentBalance money.Money  `json:"current_balance"`
    BalanceAsOf    *time.Time   `json:"balance_as_of,omitempty"`    // latest non-opening snapshot date
    BalanceSource  *string      `json:"balance_source,omitempty"`   // "simplefin" | "manual"; nil = transaction-derived only
    SimplefinID    *string      `json:"simplefin_id,omitempty"`
    CreatedAt, UpdatedAt time.Time
}
// InitialBalance field REMOVED.

type BalanceSnapshot struct {
    ID               string       `json:"id"`
    AccountID        string       `json:"account_id"`
    AsOfDate         *time.Time   `json:"as_of_date"`          // nil for opening
    Balance          money.Money  `json:"balance"`
    AvailableBalance *money.Money `json:"available_balance,omitempty"`
    Source           string       `json:"source"`
    ReportedAt       *time.Time   `json:"reported_at,omitempty"`
    CreatedAt        time.Time    `json:"created_at"`
}
```
⚠️ pgx gotcha: never scan `as_of_date = '-infinity'` into `time.Time` — select `CASE WHEN as_of_date = '-infinity' THEN NULL ELSE as_of_date END`.

### 2.6 Repository contract (new file `internal/repository/balances.go` + edits)

```go
// Upsert on (account_id, as_of_date); last write wins. Verifies account ownership (use checkOwnership).
func (r *Repository) UpsertBalanceSnapshot(ctx context.Context, q Querier, userID, accountID string,
    asOf time.Time, balance money.Money, available *money.Money, source string, reportedAt *time.Time) error
func (r *Repository) SetOpeningBalance(ctx context.Context, q Querier, userID, accountID string, balance money.Money) error
func (r *Repository) ListBalanceSnapshots(ctx context.Context, userID, accountID string) ([]domain.BalanceSnapshot, error) // newest first, opening last
func (r *Repository) DeleteBalanceSnapshot(ctx context.Context, userID, accountID, snapshotID string) error // ErrInvalidInput if source='opening'
```
Follow the existing repo convention instead of a new interface: the `q Querier` params above are `tx pgx.Tx`, where `nil` means "use `r.pool`" — exactly how `checkOwnership(ctx, tx pgx.Tx, table, id, userID)` works today (`repository.go:27-46`). Call `checkOwnership(ctx, tx, "accounts", accountID, userID)` first in each write.

Changed signatures:
- `CreateAccount(ctx, userID, name, accType, currency string, openingBalance int64) (string, error)` — inserts account + opening snapshot **in one tx**, returns id.
- `UpdateAccount(ctx, userID, id, name, accType, currency string, openingBalance *int64) error` — nil = don't touch opening snapshot.
- `UpsertSimplefinAccount(ctx, userID, sfID, name, currency string) (string, error)` — no balance param, no opening snapshot.

Rewritten reads (all must keep `user_id` filters):
- `ListAccounts`, `GetAccount`, `SearchAccounts`: `current_balance = account_balance_at(a.id,'infinity')`; `balance_as_of`/`balance_source` from a `LEFT JOIN LATERAL` on latest non-opening snapshot. Drop the `LEFT JOIN transactions ... GROUP BY`.
- `GetNetWorthTrend`: `dates × accounts(type IN ('asset','liability'))`, `balance = account_balance_at(a.id, end_of_month)`; `NetWorth = assets + liabilities` (D2). For the current (partial) month use `LEAST(end_of_month, CURRENT_DATE)`.
- Reports summary net worth (`repository.go:821-839`): `SUM(account_balance_at(id,'infinity'))` over asset+liability accounts (D2 — removes the subtraction).
- `ListTransactions` running balance, filter-independent:
  `running_balance = account_balance_at(t.account_id, t.date) - COALESCE((SELECT SUM(x.amount) FROM transactions x WHERE x.account_id = t.account_id AND x.date = t.date AND x.id > t.id AND x.deleted_at IS NULL), 0)`
  (last txn of a day equals that day's end-of-day balance). Remove the window function and the `JOIN accounts a` if no longer needed.

### 2.7 Service contract

- `internal/service/service.go`: `CreateAccount(... openingBalance int64) (string, error)`, `UpdateAccount(... openingBalance *int64) error`.
- New `internal/service/balances.go`:
  - `RecordManualBalance(ctx, userID, accountID string, balance int64, asOf *time.Time) error` (asOf nil → today UTC; reject future dates with `ErrInvalidInput`)
  - `ListBalanceSnapshots(ctx, userID, accountID) ([]domain.BalanceSnapshot, error)`
  - `DeleteBalanceSnapshot(ctx, userID, accountID, snapshotID) error`
- `internal/service/simplefin.go`:
  - `SFAccount` gains `BalanceDate int64 \`json:"balance-date"\`` and `AvailableBalance string \`json:"available-balance,omitempty"\``.
  - Pure helper (unit-testable): `func snapshotFromSFAccount(acc SFAccount, now time.Time) (balance money.Money, available *money.Money, asOf time.Time, reportedAt time.Time, err error)` — `asOf` = UTC date of `balance-date` (fallback `now` if 0); error if `balance` unparseable (then **skip** the snapshot and log; do not write 0).
  - In the import goroutine (`simplefin.go:300-374`): for **every mapped account** (both `"new"` and existing mappings), **after a successful `tx.Commit`**, call `UpsertBalanceSnapshot(bgCtx, nil, ..., domain.BalanceSourceSimplefin, ...)`. Not inside the tx: in pgx a failed statement aborts the whole tx, so a snapshot error would discard the imported transactions. Snapshot failure is logged and the import continues. Must run even when the account has zero new transactions.
  - Remove the balance parsing at `:308-311` that fed `UpsertSimplefinAccount`.

### 2.8 HTTP API

| Method | Route (under JWT group in `cmd/api/main.go`) | Body / notes |
|---|---|---|
| POST | `/accounts` | `{name,type,currency,opening_balance}` (replaces `initial_balance`); response `{status:"ok", id}` |
| PUT | `/accounts/{id}` | `{name,type,currency,opening_balance?}` — omitted = unchanged |
| GET | `/accounts`, `/accounts/{id}` | response gains `balance_as_of`, `balance_source`; loses `initial_balance` |
| GET | `/accounts/{id}/balances` | list snapshots |
| POST | `/accounts/{id}/balances` | `{balance (cents, signed), as_of_date? "YYYY-MM-DD"}` → 201 |
| DELETE | `/accounts/{id}/balances/{snapshotId}` | 400 for opening, 404 not found, 403 foreign |

Error mapping via existing `apperrors` + `writeError` conventions (`handlers.go:154-165`). New handlers go in `internal/handler/balances.go`.

### 2.9 Frontend

- `lib/api.ts`: `Account` — remove `initial_balance`, make `current_balance: number` required, add `balance_as_of?: string`, `balance_source?: "simplefin" | "manual"`. Add `BalanceSnapshot` interface.
- `accounts/page.tsx`: create form label "Opening Balance ($)" → `opening_balance`; edit dialog keeps an "Opening Balance" field but sends it only if changed (fetch opening from `/accounts/{id}/balances` or omit field from edit and rely on new action — agent's call, prefer omitting); add **"Update balance"** action per account (dialog: amount + optional date, POST `/accounts/{id}/balances`, invalidate `["accounts"]`, `["reports"]`); remove all `?? a.initial_balance` fallbacks (106-109, 145); show "as of {date} · {SimpleFin|Manual}" under balance when `balance_as_of` present; show a stale badge when a `simplefin` account's `balance_as_of` is > 3 days old.
- `accounts/[id]/page.tsx`: add a "Balance history" section listing snapshots (date, source, balance, delete button for non-opening). Running balance column unchanged (now correct server-side).
- `(dashboard)/page.tsx`: delete dead `netWorth` (222-224); under the Net Worth Progression title add a muted caption "Based on N accounts · balances as of {oldest latest-snapshot date among synced accounts}" using the already-fetched `accounts` query.
- Per `frontend/AGENTS.md`: Next.js 16.2.9 has breaking changes — read relevant guides in `frontend/node_modules/next/dist/docs/` before editing. Use existing primitives (`components/ui/dialog`, `confirm-dialog`, `PageHeader`, `DashboardCard`, `formatCurrency`, `formatDate`).

### 2.10 Docs
- `README.md`: fix "backend will automatically run migrations on startup" → document `make migrate-up`; update Net Worth Tracking feature bullet to describe snapshots.
- New `docs/BALANCES.md` (same style/length as `docs/BUDGETS.md`): snapshot model, `account_balance_at` algorithm, sign convention, opening vs simplefin vs manual.
- `docs/simplefin-importer.html`: "Background sync" section — each sync records a balance snapshot per mapped account; investment accounts track market value.
- `docs/getting-started.html` step 4 (Add Accounts): "Initial balance" → "Opening balance"; mention "Update balance" for manually tracked investment/other accounts; liabilities entered as negative.

---

## 3. Execution — waves, ownership, models

All agents work in the **same working tree** with **disjoint file ownership** (no worktrees → no merges). An agent must not edit files it doesn't own; if it needs a contract change, it reports back instead.

### Wave 0 — Contracts scaffold (orchestrator does this directly, ~10 min)
Files: `internal/domain/domain.go`, `internal/repository/balances.go` (stubs), changed repository/service signatures with stub bodies returning `errors.New("not implemented")`, callers updated so `go build ./...` passes. Commit nothing yet. This lets Wave 1 agents compile independently.

### Wave 1 — Parallel implementation

| ID | Task | Owns (exclusive) | Model | Done when |
|---|---|---|---|---|
| **A** | Migrations 0019 + 0020 (§2.1–2.4) + parity script | `internal/db/migrations/0019_*`, `0020_*`, `scripts/verify_balance_parity.sql` | **opus** (SQL correctness-critical) | `migrate up` / `down 1` / `up` succeed on local docker DB seeded with test data; parity script shows 0 diffs between old formula and `account_balance_at(...,'infinity')` for every account; function cases in §4.1 pass via psql |
| **B** | Repository reads & writes (§2.6) | `internal/repository/repository.go`, `search.go`, `balances.go`, `simplefin.go` | **opus** (running-balance + trend SQL, tenancy) | `go build ./...` + `go vet ./...` pass; every query retains `user_id` scoping |
| **C** | SimpleFin sync write path (§2.7 simplefin bullets) + unit tests | `internal/service/simplefin.go`, new `internal/service/simplefin_test.go` | **sonnet** | table-driven tests for `snapshotFromSFAccount` (normal, negative, missing balance-date, unparseable balance, available-balance absent) pass; snapshot written for accounts with 0 new txns |
| **D** | Service + handlers + routes (§2.7 non-simplefin, §2.8) | `internal/service/service.go`, `internal/service/balances.go`, `internal/handler/handlers.go`, `internal/handler/balances.go`, `cmd/api/main.go` | **sonnet** | build passes; handler validation (future date, bad JSON, opening delete) returns documented codes |
| **E** | Frontend (§2.9) | `frontend/lib/api.ts`, `frontend/app/(dashboard)/accounts/page.tsx`, `accounts/[id]/page.tsx`, `(dashboard)/page.tsx` | **sonnet** | `npm run build` and `npm run lint` pass in `frontend/` |
| **F** | Docs (§2.10) | `README.md`, `docs/BALANCES.md`, `docs/simplefin-importer.html`, `docs/getting-started.html` | **haiku** | content matches this plan; no claims beyond implemented behaviour |

Dependencies: B, C, D compile against Wave 0 stubs; A is independent. E codes against §2.8 contract. F against the plan.

### Wave 2 — Integration & verification (sequential)

| ID | Task | Owns | Model |
|---|---|---|---|
| **G** | Integration tests + end-to-end check | `internal/repository/balances_integration_test.go`, `Makefile` (add `test` + `test-integration` recipes — `test` is declared `.PHONY` but has no recipe today) | **sonnet** |
| **H** | Independent review of full diff against this plan | read-only | **opus** |

G details: `docker compose up -d db` → `make migrate-up` → seed fixtures (1 manual checking acct, 1 manual liability, 1 simplefin-style investment acct with snapshots) → `TEST_DATABASE_URL=... go test ./internal/repository/ -run Balance` covering §4.1–4.3 → run API locally (`go run ./cmd/api`) and exercise §2.8 endpoints with curl → `npm run build`. Tests `t.Skip` when `TEST_DATABASE_URL` unset so CI stays green.

H focus list: `account_balance_at` edge cases; `-infinity` scanning; every new/changed query filters by `user_id` or uses `checkOwnership`; sign convention consistent across trend/summary/frontend; snapshot written inside the per-account tx; no leftover `initial_balance` references (`grep -r initial_balance` must only hit migrations 0001/0019/0020 and docs history).

Orchestrator after each wave: read every diff (don't trust summaries), run `go build ./... && go vet ./... && go test ./...` and `npm run build`, fix integration seams.

---

## 4. Test cases

### 4.1 `account_balance_at` (SQL, run by A and G)
Account X: opening 1000; txns: +100 on 01-10, −50 on 01-20, +30 on 02-05.
1. No other snapshots: at 01-15 → 1100; at 'infinity' → 1080 (parity with old formula).
2. Add simplefin snapshot 5000 @ 01-31: at 01-31 → 5000; at 02-10 → 5030; at 01-15 → 1100 (opening still anchors earlier dates).
3. Account Y, **no opening**, snapshot 2000 @ 03-01, txns +100 on 02-20, −40 on 02-25: at 02-22 → 2000 − (−40) = 2040; at 02-10 → 2000 − (−40 + 100) = 1940; at 03-05 → 2000.
4. Soft-deleted txns (`deleted_at` set) ignored in all branches.
5. Same-day upsert: second snapshot on 01-31 replaces first (UNIQUE + ON CONFLICT).
6. Opening/source CHECK rejects `source='manual'` at `-infinity` and `source='opening'` at a real date.

### 4.2 Repository
- ListAccounts for user A never returns user B's accounts/snapshots; `UpsertBalanceSnapshot` on B's account from A → `ErrForbidden`.
- `balance_as_of`/`balance_source` null for opening-only accounts.
- Trend: liability −500 + asset 1500 → `net_worth` 1000; summary net worth identical to last trend point for the current month.
- Running balance on page filtered by `start_date` equals unfiltered value for the same txn.

### 4.3 Sync
- Mapped account with zero transactions in window still gets a snapshot.
- Re-sync same day updates snapshot rather than inserting.
- Unparseable balance → no snapshot, error logged, import continues.

---

## 5. Rollout (user performs; orchestrator provides commands)

1. Trigger `db-backup` workflow manually (workflow_dispatch) and confirm artifact exists.
2. Run the D2 liability sign check on prod; fix any positive liability `initial_balance` values first.
3. Apply `0019` to Supabase via **session pooler (port 5432, without `?default_query_exec_mode=exec`)** — same URL rewrite the backup workflow does: `migrate -path internal/db/migrations -database "<session-pooler-url>" up 1`.
4. Run parity script against prod → must be 0 diffs.
5. Merge → Render deploys backend, Vercel deploys frontend.
6. Apply `0020` (`up 1`).
7. Trigger `importer-cron` workflow manually → confirm `SELECT source, count(*) FROM account_balance_snapshots GROUP BY 1` shows `simplefin` rows; compare Fidelity accounts' `current_balance` with Fidelity's site.
8. Re-check after the next market day with no contributions: balance should move.

Rollback: before step 6 → `migrate down 1` (0019) + redeploy previous backend. After step 6 → `down 1` twice (0020 restores `initial_balance` from opening snapshots).

**Rollback is lossy for data created after the new backend went live** (accepted, per review 2026-09-13):
- Accounts created by the new backend have `initial_balance = 0` until 0020 and, after `down` of 0020, only get it back from their opening snapshot — fine for manual accounts, but accounts created by linking a new SimpleFin account have no opening snapshot and come back with 0.
- All SimpleFin/manual snapshots are discarded by `down` of 0019, so balances revert to the transaction-derived (drifting) numbers.
- Mitigation: take the backup in step 1 and treat rollback as "restore from backup" if more than a day has passed.

Implementation notes added after review:
- 0019 adds a transition trigger `sync_opening_snapshot` (AFTER UPDATE OF `initial_balance`) so opening-balance edits made by the *old* backend between steps 3 and 5 reach the opening snapshot; 0020 drops it (its down migration restores it).
- 0019 adds covering index `idx_transactions_account_date_amount_live (account_id, date) INCLUDE (amount) WHERE deleted_at IS NULL` (~4× faster `account_balance_at` on a 50k-transaction account), and the function is `STRICT`.
- `ListTransactions` computes the page first (CTE), then calls `account_balance_at` once per distinct (account, date).
- `RecordManualBalance` accepts dates up to UTC today + 1 day, because the frontend sends the user's local date.

---

## 6. Risks / notes
- SimpleFin `balance-date` semantics vary by institution; a stale bank-reported balance anchors a stale value. Mitigated by the stale badge (§2.9).
- Stale balance-date on the same day: if a sync anchors 1000 as of Feb 10 and a later sync imports a −50 transaction dated Feb 10 while the bank still reports the same Feb 10 balance, the current balance shows 1000 (the transaction is on/before the anchor) until the bank's balance-date advances. Self-correcting; accepted.
- Running balance "jumps" on snapshot days (e.g. 1030 → 4997 on a −7 transaction) because the snapshot re-anchors. Correct by design; a visual marker is a follow-up.
- Pending transactions: bank balance may exclude them; they're dated ≤ snapshot date so they won't be double-counted, but may be missing until posted. Acceptable.
- UTC date boundaries: server runs UTC; snapshots and txn dates both UTC-derived.
- `simplefin_id` UNIQUE is global, not per-user (pre-existing; out of scope).
- Performance: trend = months × accounts function calls, each two index lookups — fine at personal scale (≤ ~50 accounts × 120 months).
