# Phantom Budgets Architecture

## Overview
The "Phantom Budgets" architecture is a system designed to efficiently manage recurring budgets without needing to eagerly populate database records for all future months. It combines physical persistence for historical and current data with dynamic projection for future timelines.

## `GetBudgetsSummary`
The `GetBudgetsSummary` function retrieves budget information and handles the distinction between physical and projected budgets:
- **Physical Database Rows:** For the current month and any past months, the budgets are represented by physical rows stored directly in the database.
- **Dynamic Projection (Future Months):** For future months, there are no physical rows. Instead, the system dynamically projects the budgets. It takes the most recent physical budget structure (e.g., the current month's categories and limits) and applies it to the future periods. 

This mechanism allows the system to provide seamless budget forecasting without bloated storage, as future "phantom" budgets only exist in memory when queried.

## `UpdateBudget`
The `UpdateBudget` function handles user modifications to their budget allocations:
- **Cascading Updates:** For budgets configured with a 'monthly' recurrence, updates are designed to cascade. When the most recent physical budget is updated, all future "phantom" projections automatically reflect this change since they are dynamically generated from this row. If the system needs to persist changes that affect already-instantiated physical rows in future months, `UpdateBudget` applies cascading updates to ensure consistency across the user's forward-looking timeline, while safely preserving historical data.
