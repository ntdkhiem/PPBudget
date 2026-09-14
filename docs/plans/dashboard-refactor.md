# Dashboard page cleanup

Findings from a review of `frontend/app/(dashboard)/page.tsx` (2026-09-14). Each item stands alone, so pick any subset. Line numbers are as of commit `a0867e2`; check them again before starting work.

Status: **Open** = not started · **Done** · **Skipped** = user decided against it.

Size: **S** < 30 min · **M** ~1–2 h · **L** half a day or more.

**Implementation notes (2026-09-14), where the work differed from the text below:**
- A1 reused the existing, previously unused `/reports/spending` endpoint instead of adding a new one. Its query now mirrors `out_period` (effective amounts, transfers excluded, uncategorized grouped), so Top Spending's categories sum exactly to "out". Covered by `internal/repository/reports_integration_test.go`.
- B1: the needs-review count is kept as a **Needs Review** card. It uses `is_reviewed = false` (the same data as `/review`), not "uncategorized expense".
- B2: fake click affordances were removed; the header links ("View all", "Manage") are the navigation.
- C1: components live in `app/(dashboard)/_components/` as `net-worth-card.tsx`, `kpi-cards.tsx`, `list-cards.tsx`, `transaction-list-item.tsx`, and `dashboard-data.ts` (shared hooks).

---

## A. Correctness

### A1. Top Spending uses a truncated transaction list (Done, M)
- `page.tsx:147` groups the output of `/transactions?start_date&end_date` by category on the client.
- `internal/repository/repository.go:232` caps that query at `LIMIT 100`, so any period with more than 100 transactions shows wrong totals, and nothing warns the user.
- Fix: add a server-side aggregate, either `spending_by_category` on `/reports/summary` or a new `/reports/spending-by-category?start_date&end_date`. It should exclude transfers and use `effective_amount` the same way the client does now. The card then reads the aggregate.
- **Verify:** for a period with more than 100 transactions, the totals match a SQL `SUM` grouped by category.

### A2. Recent Transactions downloads up to 100 rows to show 5 (Done, S)
- `page.tsx:235` slices `transactions` to 5.
- Fix: add a `limit` query param to the transactions handler/repository (default 100, max 100) and request `limit=5`. Once A1 ships, the dashboard no longer needs the full list.

### A3. Loading states look like empty states (Done, S)
- Only `loadingAccounts` gates the page (`page.tsx:223`). While their queries load, Top Spending says "No expenses found.", Savings Rate says "No income this period.", and Upcoming Bills says "No upcoming bills this week."
- Fix: render skeletons while `transactions`/`categories`, `summary` and `subscriptions` load. Stop gating the whole page on `accounts`, which is only used for the caption. This comes for free with C1.

### A4. Net-worth header edge cases (Done, S)
- `page.tsx:219`: `deltaPercent` is 0 whenever the starting value is ≤ 0, so users starting in debt never see a percentage. Use `Math.abs(prev)` as the denominator, or hide the percentage when `prev === 0`.
- `page.tsx:215`: with a single data point, the header shows `$0` and hides the value. Show the value without a delta.

### A5. Summary cards cover different periods without saying so (Done, S)
- "Safe to Spend" uses the budgets for the current calendar month (`page.tsx:111`, `241-242`). "In + Out" and "Savings Rate" use the global date range.
- Fix: label Safe to Spend with the month (e.g. "September budget"), or make it follow the selected range.

---

## B. Dead code & misleading UI

### B1. Remove unused code (Done, S)
- Unused values: `needsReviewTransactions`, `needsReviewTop`, `needsReviewCount` (`page.tsx:172-176`), `activeBudgets`, `budgetPercent`, `isBudgetOver` (`236-244`), `formatDescription` (`45-52`).
- Unused imports: `AnimatePresence`, `subMonths`, `Progress`, `Button`, `Dialog*`, `Label`, `Input`, `Select*`, and most of the lucide icons.
- Stale comments: "Left Column: Transactions & Budgets", "Mini Charts & Subscriptions", "(RESTORED)".
- **Decision needed:** delete the needs-review count, or render it as a "Needs review (N)" link to `/review`.

### B2. Rows look clickable but do nothing (Done, S)
- Recent transaction rows (`page.tsx:514`) and bill rows (`563`) have `cursor-pointer` and hover styles but no handler.
- Fix: link them (transaction → edit dialog or `/transactions`; bill → `/subscriptions`), or drop the pointer and hover styles.

### B3. Duplicated `dark:` classes (Done, S)
- `dark:bg-slate-900/80 dark:bg-slate-900/80` and `dark:border-slate-700/60 dark:border-slate-800/60` appear in `page.tsx:351,374`, `components/dashboard-card.tsx:14`, and `components/stat-card.tsx:33`. Only the last class in each pair takes effect, so keep the intended one.

---

## C. Structure

### C1. Split the page into per-card components (Done, L)
- Today `page.tsx` holds 7 queries, a mutation, 8 memos, and every widget's markup in about 600 lines.
- Target:
  ```
  app/(dashboard)/
    page.tsx                  # layout grid only
    _components/
      NetWorthCard.tsx        # range state, query, yDomain, caption
      CashFlowCard.tsx
      SafeToSpendCard.tsx
      SavingsRateCard.tsx
      TopSpendingCard.tsx
      RecentTransactionsCard.tsx
      UpcomingBillsCard.tsx
  ```
- Each card owns its query and its skeleton, which resolves A3. React Query dedupes shared keys such as `["categories"]`.
- It's easier to do after A1/A2 so the new components don't carry over the client-side aggregation.

### C2. Centralize token handling (Open, M, touches every page)
- Partial step taken: `getStoredToken()` in `lib/api.ts` is used by the new dashboard code; other pages still read `localStorage` directly.
- Every query repeats `localStorage.getItem("ppbudget_token")` and passes `token` to `apiFetch`. The dashboard page and `layout.tsx` both do this.
- Fix: have `apiFetch` read the token itself, or add a `useApi()` hook. A codebase-wide change, so it could go in its own PR.

### C3. Shared `useInsights()` hook (Done, S)
- `["reports","insights"]` is defined in both `page.tsx:124` and `layout.tsx:141`. Move it into one hook, along with the severity sort and the dismiss mutation.

### C4. Category lookups by id (Done, S)
- `categories.find(...)` runs inside loops (`page.tsx:152,158,521`). Build a `Map` once. The speedup is minor, but the code reads more cleanly.

---

## D. Visual consistency

### D1. Use `StatCard` for the hand-built KPI cards (Done, S)
- "In + Out" (`page.tsx:351`) and "Safe to Spend" (`374`) copy `StatCard`'s markup by hand. Replace them with `<StatCard>`.

### D2. Bring Upcoming Bills in line with the other cards (Done, S)
- `page.tsx:542` has its own gradient, `bg-white/60`, and hover lift that no other card uses. Switch it to `DashboardCard`, or give every card the same style.

### D3. Theme-aware chart colors (Done, S)
- The axis, tick, and series colors are hard-coded hex (`#94a3b8`, `#64748b`, `#6366f1`, `#cbd5e1`), and the tooltip `contentStyle` uses `var(--tw-prose-body)`, which isn't defined. In dark mode the grid adapts but these don't.
- Fix: use `currentColor` with Tailwind classes, or CSS variables from `globals.css`.

### D4. Consistent heading style (Done, S)
- `StatCard` titles are uppercase ("SAVINGS RATE"), while `DashboardCard` titles are title case with a bold, larger font ("Recent Transactions"). Choose one style.

---

## E. Layout

### E1. Rebalance the grid (Done, M)
- The 2-column summary grid holds two short number cards and two tall content cards (the donut and the bar list), so the rows are uneven. The bottom grid has one card per column.
- Proposed layout:
  1. Net worth (full width)
  2. A KPI row of 3 compact tiles: Net cash flow · Savings rate (number with a small bar instead of a donut) · Safe to spend
  3. Top Spending | Upcoming Bills
  4. Recent Transactions (full width, or beside a "Needs review" card if B1 keeps it)

### E2. Fewer entrance animations (Done, S)
- The page wrapper fades in (`page.tsx:255`), then each card staggers in with delays up to 0.7s, and the bars animate for 1s. This replays on every navigation back to the dashboard. Keep one level, e.g. only the page fade.

---

## Suggested order

1. **Quick wins:** B1, B3, A4, A5
2. **Data correctness:** A1, A2
3. **Structure:** C1 (includes A3, D1, C4), then C3
4. **Polish:** D2, D3, D4, B2, E2
5. **Layout redesign:** E1, best done after C1
6. **Separate PR:** C2
