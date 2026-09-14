# Balance Snapshots

## Overview

Instead of deriving account balances purely from transactions, PPBudget anchors them to **balance snapshots** — discrete points-in-time records that reset the starting point. The current balance combines the latest snapshot with transactions recorded after that snapshot date.

This design makes investment accounts (brokerage, IRA, 401k) track market value (via SimpleFin), and ensures net-worth trends reflect real-world account states rather than drifting due to missed transactions.

## Snapshot Sources

Accounts created in PPBudget (and all accounts that existed before snapshots were introduced) have an **opening snapshot**. Accounts created by linking a new SimpleFin account start with a SimpleFin snapshot instead. Snapshots come from three sources:

1. **Opening Balance** (`source='opening'`)  
   Set when you create an account, or when you edit its opening balance. It represents the balance before any transactions, so it anchors all history up to the first dated snapshot.

2. **SimpleFin Sync** (`source='simplefin'`)  
   Recorded every time the SimpleFin importer runs (~12 hours). Captures the bank-reported balance for synced accounts (checking, savings, investments, credit cards). Investment accounts automatically reflect market value at each sync.

3. **Manual Update** (`source='manual'`)  
   Created when you use the "Update balance" action on the account page. Useful for manually tracked investments or cash accounts that don't sync via SimpleFin.

## Computing Balance at a Date

Given an account and a specific date, PPBudget calculates the balance in three cases:

**Case 1: Snapshot on or before the date**  
Find the most recent snapshot at or before that date. Add all transactions posted after the snapshot and on or before the target date.
```
balance = snapshot.balance + SUM(transactions from after snapshot to target date)
```

**Example:** Opening balance $1,000. Transactions: +$100 on 2024-01-10, −$50 on 2024-01-20, +$30 on 2024-02-05. A SimpleFin sync on 2024-01-31 reports $5,000 (market gains that never appeared as transactions).
- Balance on 2024-01-15: latest snapshot is the opening one → $1,000 + $100 = **$1,100**.
- Balance on 2024-01-31: the SimpleFin snapshot → **$5,000**.
- Balance on 2024-02-10: $5,000 + $30 (transaction after the snapshot) = **$5,030**.

**Case 2: No snapshot before the date, but one exists after**  
Find the earliest snapshot after the date. Work backwards: subtract transactions dated after the target date, up to and including the snapshot date.
```
balance = snapshot.balance - SUM(transactions after target date through snapshot date)
```
This handles freshly linked SimpleFin accounts whose first snapshot is newer than their imported transaction history.

**Case 3: No snapshots at all**  
Sum all transactions on or before the date (behaves like the old system). This should not occur after migration.

## Sign Convention

**Liabilities are stored as negative amounts.**

- Asset account (checking, savings, investments): positive balance = you own it.
- Liability account (credit card, loan): negative balance = you owe it.
- **Net Worth** = SUM of all asset balances + SUM of all liability balances.

Example: Checking +$5,000, Savings +$2,000, Credit Card −$800 → Net Worth = $5,000 + $2,000 + (−$800) = $6,200.

## What Changes (and Doesn't) Affect Balance

**Does NOT change the current balance:**
- Editing or deleting a transaction dated on or before the latest snapshot date (the bank's reported balance is treated as truth).
- Recategorizing, linking, or reviewing any transaction (amount unchanged).

**Does change the current balance:**
- Adding a new transaction dated after the latest snapshot (until the next sync re-anchors).
- Creating a new manual snapshot (immediately anchors at that amount).
- SimpleFin sync (records a new snapshot, re-anchoring the balance).

**Historical balances** (used in net-worth trend charts for past dates) between two snapshots are still derived from transactions, so editing an old transaction can shift the trend line — but never changes a snapshot value itself.

## Same-Day Updates

When multiple snapshots exist for the same date (e.g., two SimpleFin syncs in one day), the latest one wins. PPBudget uses `UNIQUE (account_id, as_of_date)` with an `ON CONFLICT UPDATE` strategy, so re-syncing the same day updates the snapshot rather than creating a duplicate.
