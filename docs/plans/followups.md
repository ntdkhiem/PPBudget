# Follow-ups found during balance-snapshot planning

Issues discovered while planning [`balance-snapshots.md`](balance-snapshots.md) (2026-09-13). Each was verified in code at that date; re-check before acting, since code may have moved.

Status: **Open** = not started · **Done** = shipped and verified · **Covered** = handled inside the balance-snapshots plan (verify after it ships) · **Deferred** = user explicitly postponed.

---

## A. Correctness bugs

### A1. Net-worth sign convention differs between trend and summary — Done (2026-09-19)
- `internal/repository/repository.go:684` (trend): `net_worth = assets + liabilities`
- `internal/repository/repository.go:829-831` (reports summary): `assets - liabilities`
- Same data gives two different net-worth numbers. The balance-snapshots plan (decision D2) standardizes on signed balances + SUM.
- **Verified 2026-09-19:** both paths now go through `account_balance_at`; `net_worth` is a single `SUM(account_balance_at(a.id, 'infinity'))` (`repository.go:971`). One convention, one code path.

### A2. Transaction running balance is wrong under filters/pagination — Done (2026-09-19)
- `repository.go:107` computes `running_balance` with a window function, which Postgres evaluates *after* `WHERE`. Filtering by `start_date`, search text, category, or unreviewed-only drops earlier rows from the sum.
- Visible on `frontend/app/(dashboard)/accounts/[id]/page.tsx:179`.
- The plan replaces it with a filter-independent `account_balance_at(...)` expression.
- **Verified 2026-09-19:** the window function is gone; `running_balance` derives from `account_balance_at(d.account_id, d.date)` (`repository.go:244`), which is independent of the `WHERE` clause.

### A3. Concurrent SimpleFin imports for the same user — Done (2026-09-19)
- Auto-sync is triggered twice by design: in-process cron `cmd/api/main.go:52-68` (`@every 12h`) **and** GitHub Actions `importer-cron.yml` → `POST /import/simplefin/cron` → `internal/handler/import.go:183`. Manual "Sync Now" can also overlap.
- `SimpleFinExecute` (`internal/service/simplefin.go:205`) has no guard; it overwrites progress to `"running"` (line 275) and starts another goroutine.
- Impact: duplicate transaction inserts are mostly blocked by `simplefin_id` UNIQUE, but content-dedup checks can race, progress counters get clobbered, rules run twice, and **duplicate summary emails** can be sent.
- **Fixed:** per-user in-process lock in `internal/service/simplefin.go` (`tryAcquireImportLock`/`releaseImportLock`). `SimpleFinExecute` returns `apperrors.ErrConflict` when one is already running; the handler maps that to **409**, and auto-sync logs a skip at Info rather than Error. Covered by `internal/service/import_lock_test.go`, including a 50-goroutine contention test run under `-race`.
- **Limitation:** process-local, which matches the single-instance Render deployment. More than one backend instance would need a Postgres advisory lock held on one pinned connection for the whole import.
- Still open: removing the redundant in-process cron now that the GitHub ping exists.

### A4. SimpleFin accounts are always created as `asset` — Open
- `internal/repository/simplefin.go:18` hardcodes `"asset"`. Credit cards/loans land in the Assets column on the Accounts page.
- Net worth stays correct under signed balances (negative asset = same as negative liability), so this is a labeling/UX issue.
- Fix idea: infer from SimpleFin metadata/negative balance on first import, or let the mapping wizard choose type; allow changing type later (already possible via PUT `/accounts/{id}`).

### A5. `simplefin_id` is globally unique, not per user — Done (2026-09-19)
- `internal/db/migrations/0001_init.up.sql:9` (accounts) and `:29` (transactions): `simplefin_id TEXT UNIQUE`; 0014 (multi-tenancy) never scoped it.
- Impact: two PPBudget users linking the same SimpleFin connection (e.g. a couple sharing an account) collide. `UpsertSimplefinAccount`'s `ON CONFLICT (simplefin_id) DO UPDATE` could even rename another user's account; `InsertIngestedTransaction`'s `ON CONFLICT DO NOTHING` would silently skip the second user's transactions.
- **Worse than first described:** `UpsertSimplefinAccount` had no `user_id` filter on its `DO UPDATE`, so a second user's sync matched the *first* user's row, renamed it, and returned that account's id — the second user's transactions were then written against someone else's account.
- **Fixed:** migration `0023_user_scoped_simplefin_id` swaps both constraints to `UNIQUE (user_id, simplefin_id)`; `ON CONFLICT` targets updated in `repository/simplefin.go` and `repository/repository.go`, plus a `WHERE accounts.user_id = EXCLUDED.user_id` guard on the upsert.
- **Verified** against a throwaway database: two users linking the same SimpleFIN id get separate accounts with names intact, same-user re-sync still upserts in place, and same-user transaction dedup still suppresses.
- The down migration deliberately fails if two users have linked the same account (rolling back would have to delete someone's data); recovery steps are in the `.down.sql` header.

---

## B. Documentation drift

### B1. README says migrations run automatically on startup — Covered (docs task F)
- `README.md:104` ("The backend will automatically run migrations on startup"). No migration code exists in `cmd/api/main.go` or `internal/db/pool.go`; migrations only run via `make migrate-up` (golang-migrate CLI).
- Decide later whether to *add* startup migrations (e.g. embed with `golang-migrate` + `iofs`) instead of just fixing the doc.

### B2. Backup schedule/encryption docs vs workflow — Deferred
- README says nightly + `openssl aes-256-cbc`; `.github/workflows/db-backup.yml` runs monthly (`0 0 1 * *`) with `gpg --symmetric AES256`.
- Pick one: change the cron to nightly, or fix README lines 19, 53, 156, 160, 168.

---

## C. Architecture limitations

### C1. Import progress is in-memory / process-local — Deferred
- `internal/service/simplefin.go:182-202` (`userProgress` map). Lost on restart; wrong with >1 instance. Pairs naturally with A3 (a persisted `import_runs` table would solve both).

### C2. SimpleFin `holdings` are discarded — Open (feature)
- `SFAccount` (`internal/service/simplefin.go:36-42`) has no `Holdings` field. Snapshots fix balance accuracy; holdings would enable per-position views, allocation charts, and cost basis.

### C3. Net-worth trend is monthly only — Open (feature)
- `GetNetWorthTrend` generates month-ends only. With 12-hourly snapshots, a daily/weekly granularity option for the 1M/3M ranges on the dashboard becomes meaningful.

---

## D. Hygiene

### D1. Duplicate panic recovery — Done (2026-09-19)
- `cmd/api/main.go` registered both `middleware.StructuredLogger` (which has its own `recover()`) and chi `Recoverer`.
- **Fixed:** dropped `chimw.Recoverer`. `StructuredLogger` is the outer middleware, so chi's would never have seen a handler panic anyway, and the structured one logs through slog with a stack trace.

### D2. `ADMIN_PASSWORD` is loaded but never used — Done (2026-09-19)
- Worse than vestigial: `README.md` stated "Your `ADMIN_PASSWORD` secures the frontend UI via JWT tokens", which is false. Someone could set a strong value, believe the instance was protected, and have changed nothing.
- **Fixed:** removed from `internal/config/config.go`, `.env.example`, `docker-compose.yml`, and both README references; the security section now describes how JWT sessions actually work.

### D3. Insecure config defaults accepted silently — Done (2026-09-19)
- `config.go` fell back to placeholder `JWT_SECRET` / `INGEST_API_KEY`; `MustLoad` only enforced `DATABASE_URL`. Since `RequireJWT` trusts the `user_id` claim it carries, a deploy missing `JWT_SECRET` would accept tokens anyone could mint from the public default.
- **Root cause was the docs:** the README's Render deploy steps never listed `JWT_SECRET` at all, so following them produced exactly this state.
- **Fixed:** `MustLoad` became `Validate() error`; startup refuses (exit 1, logged at ERROR) when either secret equals its committed default and `FRONTEND_URL` is not a localhost address. Local development still runs on the defaults, with a warning. README now lists both as required, with a generation command. Covered by `internal/config/config_test.go`; the refuse / allow / local paths were verified by actually booting the binary.
- **Action for the operator:** confirm `JWT_SECRET` is set in the Render environment. If it was never set, sessions have been signed with the public default — rotate it, which invalidates all existing sessions.

### D4. `github.com/jackc/pgx/v4` in go.mod but not imported — Done (2026-09-19)
- **Fixed:** `go mod tidy` dropped `pgx/v4` and eight transitive dependencies (`jackc/pgconn`, `pgtype`, `pgproto3/v2`, `chunkreader/v2`, `pgio`, `puddle` v1 and others); `golang.org/x/crypto` became a direct requirement. Full suite still green.

### D5. Hydration-mismatch warning on every page in dev — Done (2026-09-19)
- React warns that `<html>` className/style differ between server and client (`dark` class, `color-scheme`) — set by `next-themes` after SSR. Seen in the dev console on `/accounts` during balance-snapshot UI testing; pre-existing.
- **Fixed:** added `suppressHydrationWarning` to `<html>` in `frontend/app/layout.tsx`, the documented `next-themes` setup.

---

## E. From the balance-snapshot review (2026-09-13)

### E1. Opening balance can't be edited in the UI — Open
- The edit-account dialog dropped the balance field; `PUT /accounts/{id}` still accepts `opening_balance`, but no UI sends it. A typo in the opening balance can only be corrected by adding a manual snapshot.
- Fix idea: an "Edit" action on the "Opening balance" row in Balance History.

### E2. Data export omits balance snapshots — Open
- `internal/service/settings.go` `ExportAllData` exports accounts without opening balances or snapshots, so an export can't reconstruct balances.
- Fix idea: add a `balance_snapshots` type to the export.

### E3. Running balance jumps on snapshot days with no visual cue — Open
- See plan §6. Add a small marker/tooltip on the transaction row where a snapshot re-anchors.

### E4. History step for SimpleFin accounts created before snapshots — Open
- Their opening balance (old `initial_balance`) was the balance at first import, which already included the imported 30 days of transactions, so net-worth history before their first snapshot double-counts those transactions. Everything from the first post-deploy sync onward is correct.
- Fix idea: one-off script per affected account — set the opening snapshot to `first_snapshot.balance − SUM(txns ≤ first_snapshot date)`, or delete the opening snapshot so history back-derives from the first SimpleFin snapshot.

### E5. Balance function cost on very large accounts — Open (monitor)
- `account_balance_at` sums from the anchor; manual accounts anchor at `-infinity`, so cost grows with history. Measured with the covering index: 50k transactions → ~0.5s per 100-row transactions page, ~0.35s for a 10-year trend on one account. Fine at personal scale.
- Fix idea if needed: periodic automatic snapshots for manual accounts (e.g. monthly), which bound every sum to one month.

---

## F. From the rules-engine fixes (2026-09-18)

Discovered while fixing the UI/engine vocabulary drift. See migration
`0022_rules_canonical_vocabulary` and `internal/service/rules_vocabulary.go`.

### F1. Legacy action rows left in place by migration 0022 — Open
- The rules UI used to offer `add_tag`, `set_budget`, and `link_as_card_payment`. None were ever implemented by the engine, and there is no `tags` table in the schema. Rules using only those actions have always been no-ops.
- Migration 0022 deliberately leaves those `rule_actions` rows alone rather than deleting user data. The rules page renders them as an amber "Unsupported" badge so they stay visible.
- `ValidateRule` now rejects them, so **editing and saving** such a rule forces you to replace the action. A rule left untouched keeps its dead row.
- Fix idea: a one-off cleanup once you've confirmed none of your rules still carry them, or implement `add_tag` properly (needs a `tags` table plus a transaction_tags join).

### F2. Rules still only run on the SimpleFin sync path — Open
- `docs/rules-engine.html` previously claimed rules run on SimpleFin, CSV upload, and manual creation. Only the SimpleFin path calls them (`internal/service/simplefin.go`); the doc has been corrected rather than the behavior.
- Fix idea: call `ApplyActiveRules` (scoped to the new transaction ids) from the CSV import and the manual-create handler. Wants a transaction-id-scoped variant so it doesn't rescan the ledger to categorize one row.

### F3. Retroactive apply has no undo — Open
- `POST /rules/{id}/apply` overwrites `category_id` / `subscription_id` / `account_id` with no record of the previous values. The new preview endpoint makes the blast radius visible beforehand, but a mistaken apply is still unrecoverable.
- Fix idea: record the prior values in an `apply_runs` table and offer "undo last apply", or reuse the same table to satisfy C1's need for durable run history.

### F5. Recurring Payments showed every subscription unpaid — Done (2026-09-21)
- Reported as "no subscription gets linked by the rules". The rules engine was fine; the page never got the transactions it compares against.
- `frontend/app/(dashboard)/subscriptions/page.tsx` walked the cursor-paginated `/transactions` endpoint with `while (res.length < 50)` as its stop condition, but the server pages at 100 (`internal/repository/repository.go`) and serialized an empty page as `null`, not `[]`. So any range whose transaction count left a final page of 0 or 50-99 rows triggered one request past the end, `null.length` threw, and with `retry: false` in `providers.tsx` the query failed silently — `transactions` stayed `undefined`, every `t.subscription_id === sub.id` check was skipped, and "Expected and not yet paid" reported the full expected total. Roughly half of all transaction counts hit it, and only for a busy enough month, which is why it looked intermittent.
- **Fixed:** the walk now coalesces the response and stops on an empty page instead of guessing a page size, and `ListTransactions` builds a non-nil slice so the endpoint returns `[]`.
- Covered by `internal/service/rules_apply_test.go`, which pins the previously untested `link_to_subscription` action (set, combined with a category, no-op when already linked, priority resolution).

### F4. Account conditions unresolvable after migration — Open (verify)
- Migration 0022 resolves old free-text account values to `accounts.id` by SimpleFIN id, then by a case-insensitive unique name match. A value matching neither (a renamed or deleted account, or an ambiguous name) is left as-is and will never match.
- **Verify after ship:** open Settings → Rules and check that every Account condition shows an account name rather than a raw string.
