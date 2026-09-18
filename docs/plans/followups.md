# Follow-ups found during balance-snapshot planning

Issues discovered while planning [`balance-snapshots.md`](balance-snapshots.md) (2026-09-13). Each was verified in code at that date; re-check before acting, since code may have moved.

Status: **Open** = not started · **Covered** = handled inside the balance-snapshots plan (verify after it ships) · **Deferred** = user explicitly postponed.

---

## A. Correctness bugs

### A1. Net-worth sign convention differs between trend and summary — Covered
- `internal/repository/repository.go:684` (trend): `net_worth = assets + liabilities`
- `internal/repository/repository.go:829-831` (reports summary): `assets - liabilities`
- Same data gives two different net-worth numbers. The balance-snapshots plan (decision D2) standardizes on signed balances + SUM.
- **Verify after ship:** summary `net_worth` equals the last point of `/reports/net-worth` for the current month.

### A2. Transaction running balance is wrong under filters/pagination — Covered
- `repository.go:107` computes `running_balance` with a window function, which Postgres evaluates *after* `WHERE`. Filtering by `start_date`, search text, category, or unreviewed-only drops earlier rows from the sum.
- Visible on `frontend/app/(dashboard)/accounts/[id]/page.tsx:179`.
- The plan replaces it with a filter-independent `account_balance_at(...)` expression.
- **Verify after ship:** same transaction shows the same running balance with and without a date filter.

### A3. Concurrent SimpleFin imports for the same user — Open
- Auto-sync is triggered twice by design: in-process cron `cmd/api/main.go:52-68` (`@every 12h`) **and** GitHub Actions `importer-cron.yml` → `POST /import/simplefin/cron` → `internal/handler/import.go:183`. Manual "Sync Now" can also overlap.
- `SimpleFinExecute` (`internal/service/simplefin.go:205`) has no guard; it overwrites progress to `"running"` (line 275) and starts another goroutine.
- Impact: duplicate transaction inserts are mostly blocked by `simplefin_id` UNIQUE, but content-dedup checks can race, progress counters get clobbered, rules run twice, and **duplicate summary emails** can be sent.
- Fix idea: per-user in-process lock (`sync.Map` of mutexes / "already running → return 409"); for durability, a Postgres advisory lock (`pg_try_advisory_lock(hashtext(user_id))`). Consider removing the in-process cron entirely, since the GitHub ping exists because Render sleeps.

### A4. SimpleFin accounts are always created as `asset` — Open
- `internal/repository/simplefin.go:18` hardcodes `"asset"`. Credit cards/loans land in the Assets column on the Accounts page.
- Net worth stays correct under signed balances (negative asset = same as negative liability), so this is a labeling/UX issue.
- Fix idea: infer from SimpleFin metadata/negative balance on first import, or let the mapping wizard choose type; allow changing type later (already possible via PUT `/accounts/{id}`).

### A5. `simplefin_id` is globally unique, not per user — Open
- `internal/db/migrations/0001_init.up.sql:9` (accounts) and `:29` (transactions): `simplefin_id TEXT UNIQUE`; 0014 (multi-tenancy) never scoped it.
- Impact: two PPBudget users linking the same SimpleFin connection (e.g. a couple sharing an account) collide. `UpsertSimplefinAccount`'s `ON CONFLICT (simplefin_id) DO UPDATE` could even rename another user's account; `InsertIngestedTransaction`'s `ON CONFLICT DO NOTHING` would silently skip the second user's transactions.
- Fix idea: migration replacing both constraints with `UNIQUE (user_id, simplefin_id)` and updating the `ON CONFLICT` targets.

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

### D1. Duplicate panic recovery — Open
- `cmd/api/main.go:84-85` registers both `middleware.StructuredLogger` (which has its own `recover()`, `internal/middleware/middleware.go:28-35`) and chi `Recoverer`. Keep one.

### D2. `ADMIN_PASSWORD` is loaded but never used — Open
- `internal/config/config.go:13,29`; no other references. Vestigial from pre-multi-tenant auth. Remove from config, `.env.example`, `docker-compose.yml`, README deploy steps.

### D3. Insecure config defaults accepted silently — Open
- `config.go` falls back to placeholder `JWT_SECRET` / `INGEST_API_KEY`; `MustLoad` only enforces `DATABASE_URL`. A production deploy missing `JWT_SECRET` would sign tokens with a public default.
- Fix idea: fail startup when these equal their defaults and `FRONTEND_URL` isn't localhost (or add an `ENV=production` flag).

### D4. `github.com/jackc/pgx/v4` in go.mod but not imported — Open
- `go.mod:10`; no `.go` file imports it. Run `go mod tidy` and confirm it drops (it may be pulled in indirectly by a tool).

### D5. Hydration-mismatch warning on every page in dev — Open
- React warns that `<html>` className/style differ between server and client (`dark` class, `color-scheme`) — set by `next-themes` after SSR. Seen in the dev console on `/accounts` during balance-snapshot UI testing; pre-existing.
- Fix idea: add `suppressHydrationWarning` to `<html>` in `frontend/app/layout.tsx` (the documented `next-themes` setup).

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

### F4. Account conditions unresolvable after migration — Open (verify)
- Migration 0022 resolves old free-text account values to `accounts.id` by SimpleFIN id, then by a case-insensitive unique name match. A value matching neither (a renamed or deleted account, or an ambiguous name) is left as-is and will never match.
- **Verify after ship:** open Settings → Rules and check that every Account condition shows an account name rather than a raw string.
