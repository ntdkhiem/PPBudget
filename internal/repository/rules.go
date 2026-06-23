package repository

import (
	"context"
	"ntdkhiem/firefly-go/internal/domain"
	apperrors "ntdkhiem/firefly-go/internal/errors"

	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateRule(ctx context.Context, rule *domain.Rule) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	queryRule := `
		INSERT INTO rules (name, description, priority, is_active)
		VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at
	`
	err = tx.QueryRow(ctx, queryRule, rule.Name, rule.Description, rule.Priority, rule.IsActive).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
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
		UPDATE rules SET name = $1, description = $2, priority = $3, is_active = $4, updated_at = NOW()
		WHERE id = $5 RETURNING updated_at
	`
	err = tx.QueryRow(ctx, queryRule, rule.Name, rule.Description, rule.Priority, rule.IsActive, rule.ID).Scan(&rule.UpdatedAt)
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

func (r *Repository) GetRule(ctx context.Context, id string) (*domain.Rule, error) {
	rule := &domain.Rule{}
	queryRule := `SELECT id, name, description, priority, is_active, created_at, updated_at FROM rules WHERE id = $1`
	err := r.pool.QueryRow(ctx, queryRule, id).Scan(&rule.ID, &rule.Name, &rule.Description, &rule.Priority, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt)
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

func (r *Repository) ListRulesDetailed(ctx context.Context) ([]domain.Rule, error) {
	queryRule := `SELECT id, name, description, priority, is_active, created_at, updated_at FROM rules ORDER BY priority DESC, created_at DESC`
	rows, err := r.pool.Query(ctx, queryRule)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.Rule
	for rows.Next() {
		var rule domain.Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Description, &rule.Priority, &rule.IsActive, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	// for simplicity, just query them individually or group, but there are usually few rules
	for i := range rules {
		// Fetch conditions
		queryCond := `SELECT id, rule_id, field, operator, value FROM rule_conditions WHERE rule_id = $1`
		rowsCond, _ := r.pool.Query(ctx, queryCond, rules[i].ID)
		for rowsCond.Next() {
			var cond domain.RuleCondition
			rowsCond.Scan(&cond.ID, &cond.RuleID, &cond.Field, &cond.Operator, &cond.Value)
			rules[i].Conditions = append(rules[i].Conditions, cond)
		}
		rowsCond.Close()

		// Fetch actions
		queryAct := `SELECT id, rule_id, action_type, value FROM rule_actions WHERE rule_id = $1`
		rowsAct, _ := r.pool.Query(ctx, queryAct, rules[i].ID)
		for rowsAct.Next() {
			var act domain.RuleAction
			rowsAct.Scan(&act.ID, &act.RuleID, &act.ActionType, &act.Value)
			rules[i].Actions = append(rules[i].Actions, act)
		}
		rowsAct.Close()
	}

	return rules, nil
}

func (r *Repository) DeleteRule(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM rules WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
