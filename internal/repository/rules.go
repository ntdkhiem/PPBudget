package repository

import (
	"context"
	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateRule(ctx context.Context, rule *domain.Rule) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	queryRule := `
		INSERT INTO rules (name, description, trigger_type, strictness, priority, is_active, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, created_at, updated_at
	`
	err = tx.QueryRow(ctx, queryRule, rule.Name, rule.Description, rule.TriggerType, rule.Strictness, rule.Priority, rule.IsActive, rule.UserID).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return err
	}

	for i := range rule.Conditions {
		queryCond := `INSERT INTO rule_conditions (rule_id, field, operator, value) VALUES ($1, $2, $3, $4) RETURNING id`
		err = tx.QueryRow(ctx, queryCond, rule.ID, rule.Conditions[i].Field, rule.Conditions[i].Operator, rule.Conditions[i].Value).Scan(&rule.Conditions[i].ID)
		if err != nil {
			return err
		}
		rule.Conditions[i].RuleID = rule.ID
	}

	for i := range rule.Actions {
		queryAct := `INSERT INTO rule_actions (rule_id, action_type, value) VALUES ($1, $2, $3) RETURNING id`
		err = tx.QueryRow(ctx, queryAct, rule.ID, rule.Actions[i].ActionType, rule.Actions[i].Value).Scan(&rule.Actions[i].ID)
		if err != nil {
			return err
		}
		rule.Actions[i].RuleID = rule.ID
	}

	return tx.Commit(ctx)
}

func (r *Repository) UpdateRule(ctx context.Context, rule *domain.Rule) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	queryRule := `
		UPDATE rules SET name = $1, description = $2, trigger_type = $3, strictness = $4, priority = $5, is_active = $6, updated_at = NOW()
		WHERE id = $7 AND user_id = $8 RETURNING updated_at
	`
	err = tx.QueryRow(ctx, queryRule, rule.Name, rule.Description, rule.TriggerType, rule.Strictness, rule.Priority, rule.IsActive, rule.ID, rule.UserID).Scan(&rule.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.ErrNotFound
		}
		return err
	}

	// Delete old conditions and actions
	_, err = tx.Exec(ctx, `DELETE FROM rule_conditions WHERE rule_id = $1`, rule.ID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `DELETE FROM rule_actions WHERE rule_id = $1`, rule.ID)
	if err != nil {
		return err
	}

	for i := range rule.Conditions {
		queryCond := `INSERT INTO rule_conditions (rule_id, field, operator, value) VALUES ($1, $2, $3, $4) RETURNING id`
		err = tx.QueryRow(ctx, queryCond, rule.ID, rule.Conditions[i].Field, rule.Conditions[i].Operator, rule.Conditions[i].Value).Scan(&rule.Conditions[i].ID)
		if err != nil {
			return err
		}
		rule.Conditions[i].RuleID = rule.ID
	}

	for i := range rule.Actions {
		queryAct := `INSERT INTO rule_actions (rule_id, action_type, value) VALUES ($1, $2, $3) RETURNING id`
		err = tx.QueryRow(ctx, queryAct, rule.ID, rule.Actions[i].ActionType, rule.Actions[i].Value).Scan(&rule.Actions[i].ID)
		if err != nil {
			return err
		}
		rule.Actions[i].RuleID = rule.ID
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetRule(ctx context.Context, userID, id string) (*domain.Rule, error) {
	rule := &domain.Rule{}
	queryRule := `SELECT id, user_id, name, description, trigger_type, strictness, priority, is_active, created_at, updated_at FROM rules WHERE id = $1 AND user_id = $2`
	err := r.pool.QueryRow(ctx, queryRule, id, userID).Scan(&rule.ID, &rule.UserID, &rule.Name, &rule.Description, &rule.TriggerType, &rule.Strictness, &rule.Priority, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}

	// Fetch conditions
	queryCond := `SELECT id, rule_id, field, operator, value FROM rule_conditions WHERE rule_id = $1`
	rowsCond, err := r.pool.Query(ctx, queryCond, id)
	if err != nil {
		return nil, err
	}
	defer rowsCond.Close()
	for rowsCond.Next() {
		var cond domain.RuleCondition
		if err := rowsCond.Scan(&cond.ID, &cond.RuleID, &cond.Field, &cond.Operator, &cond.Value); err != nil {
			return nil, err
		}
		rule.Conditions = append(rule.Conditions, cond)
	}

	// Fetch actions
	queryAct := `SELECT id, rule_id, action_type, value FROM rule_actions WHERE rule_id = $1`
	rowsAct, err := r.pool.Query(ctx, queryAct, id)
	if err != nil {
		return nil, err
	}
	defer rowsAct.Close()
	for rowsAct.Next() {
		var act domain.RuleAction
		if err := rowsAct.Scan(&act.ID, &act.RuleID, &act.ActionType, &act.Value); err != nil {
			return nil, err
		}
		rule.Actions = append(rule.Actions, act)
	}

	return rule, nil
}

func (r *Repository) ListRulesDetailed(ctx context.Context, userID string) ([]domain.Rule, error) {
	queryRule := `SELECT id, user_id, name, description, trigger_type, strictness, priority, is_active, created_at, updated_at FROM rules WHERE user_id = $1 ORDER BY priority DESC, created_at DESC`
	rows, err := r.pool.Query(ctx, queryRule, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.Rule
	var ids []string
	byID := map[string]int{}
	for rows.Next() {
		var rule domain.Rule
		if err := rows.Scan(&rule.ID, &rule.UserID, &rule.Name, &rule.Description, &rule.TriggerType, &rule.Strictness, &rule.Priority, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		byID[rule.ID] = len(rules)
		ids = append(ids, rule.ID)
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return rules, nil
	}

	// Two bulk queries rather than two per rule. Errors here are returned, not
	// discarded: an empty condition list is indistinguishable from a rule with
	// no conditions, and silently dropping conditions changes what a rule matches.
	queryCond := `SELECT id, rule_id, field, operator, value FROM rule_conditions WHERE rule_id = ANY($1) ORDER BY id`
	rowsCond, err := r.pool.Query(ctx, queryCond, ids)
	if err != nil {
		return nil, err
	}
	defer rowsCond.Close()
	for rowsCond.Next() {
		var cond domain.RuleCondition
		if err := rowsCond.Scan(&cond.ID, &cond.RuleID, &cond.Field, &cond.Operator, &cond.Value); err != nil {
			return nil, err
		}
		if i, ok := byID[cond.RuleID]; ok {
			rules[i].Conditions = append(rules[i].Conditions, cond)
		}
	}
	if err := rowsCond.Err(); err != nil {
		return nil, err
	}

	queryAct := `SELECT id, rule_id, action_type, value FROM rule_actions WHERE rule_id = ANY($1) ORDER BY id`
	rowsAct, err := r.pool.Query(ctx, queryAct, ids)
	if err != nil {
		return nil, err
	}
	defer rowsAct.Close()
	for rowsAct.Next() {
		var act domain.RuleAction
		if err := rowsAct.Scan(&act.ID, &act.RuleID, &act.ActionType, &act.Value); err != nil {
			return nil, err
		}
		if i, ok := byID[act.RuleID]; ok {
			rules[i].Actions = append(rules[i].Actions, act)
		}
	}
	if err := rowsAct.Err(); err != nil {
		return nil, err
	}

	return rules, nil
}

// ApplyRuleUpdates writes the rules engine's pending changes in one
// transaction. It touches only the three columns a rule may set, so applying a
// category does not churn the transaction's links the way UpdateTransaction
// does. Ownership is enforced by the WHERE clause plus the existing foreign
// keys; a row that fails either is skipped rather than failing the batch.
func (r *Repository) ApplyRuleUpdates(ctx context.Context, userID string, updates []domain.TransactionRuleUpdate) (int, error) {
	if len(updates) == 0 {
		return 0, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	const query = `
		UPDATE transactions t
		SET category_id = $1, subscription_id = $2, account_id = $3, updated_at = NOW()
		FROM accounts a
		WHERE t.id = $4 AND t.user_id = $5 AND t.deleted_at IS NULL
		  AND a.id = $3 AND a.user_id = $5 AND a.balance_only = FALSE
		  AND ($1::uuid IS NULL OR EXISTS (SELECT 1 FROM categories c WHERE c.id = $1 AND c.user_id = $5))
		  AND ($2::uuid IS NULL OR EXISTS (SELECT 1 FROM subscriptions s WHERE s.id = $2 AND s.user_id = $5))
	`

	updated := 0
	batch := &pgx.Batch{}
	for _, u := range updates {
		batch.Queue(query, u.CategoryID, u.SubscriptionID, u.AccountID, u.ID, userID)
	}

	br := tx.SendBatch(ctx, batch)
	for range updates {
		tag, err := br.Exec()
		if err != nil {
			br.Close()
			return 0, err
		}
		updated += int(tag.RowsAffected())
	}
	if err := br.Close(); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return updated, nil
}

func (r *Repository) DeleteRule(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM rules WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
