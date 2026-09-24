package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ntdkhiem/ppbudget-go/internal/domain"
	apperrors "ntdkhiem/ppbudget-go/internal/errors"
	"ntdkhiem/ppbudget-go/pkg/money"
)

// Wealth Strategy persistence.
//
// Replaces the two JSON blobs in user_settings ('financial_plan',
// 'retirement_plan') that PUT /settings/values/{key} accepted unvalidated. The
// blobs stay readable so the one-time backfill in the service layer can seed
// from them; nothing here writes back to them.

// constraintError maps a Postgres integrity violation onto an app error.
//
// The schema carries real domain constraints -- filing statuses, account kinds,
// rates bounded to 0..1, a promo APR that must come with an expiry. When a
// caller sends a value outside one of those, that is BAD INPUT, not a server
// fault. Without this mapping it surfaces as a 500 with nothing useful for the
// client and a spurious error in the logs, which is how a validation gap hides
// in plain sight.
//
// The constraint name is included because Postgres derives it from the column
// (wealth_profiles_filing_status_check), which is usually enough to say what
// was wrong without a hand-maintained table of messages.
func constraintError(err error, context string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23514": // check_violation
			return fmt.Errorf("%w: %s violates %s", apperrors.ErrInvalidInput, context, pgErr.ConstraintName)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: %s references a row that does not exist", apperrors.ErrInvalidInput, context)
		case "23502": // not_null_violation
			return fmt.Errorf("%w: %s is missing a required value (%s)", apperrors.ErrInvalidInput, context, pgErr.ColumnName)
		case "23505": // unique_violation
			return fmt.Errorf("%w: %s already exists", apperrors.ErrConflict, context)
		}
	}
	return fmt.Errorf("%s: %w", context, err)
}

// ---------------------------------------------------------------- profile

// GetWealthProfile returns the user's profile with per-field provenance.
//
// A user with no row is NOT an error: it is someone who has not started
// intake, and the zero profile is exactly what the generator should see. It
// reads every field as unanswered and locks the actions that need them, which
// is the correct behaviour for a new account.
func (r *Repository) GetWealthProfile(ctx context.Context, userID string) (*domain.WealthProfile, error) {
	cols := make([]string, 0, len(profileFieldDefs)+2)
	for _, f := range profileFieldDefs {
		cols = append(cols, f.Column)
	}
	cols = append(cols, "created_at", "updated_at")

	p := &domain.WealthProfile{UserID: userID}

	dests := make([]any, 0, len(cols))
	for _, f := range profileFieldDefs {
		dests = append(dests, f.Dest(p))
	}
	dests = append(dests, &p.CreatedAt, &p.UpdatedAt)

	query := fmt.Sprintf(`SELECT %s FROM wealth_profiles WHERE user_id = $1`, strings.Join(cols, ", "))
	err := r.pool.QueryRow(ctx, query, userID).Scan(dests...)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to get wealth profile: %w", err)
	}

	fields, err := r.getProfileFields(ctx, userID)
	if err != nil {
		return nil, err
	}
	p.Fields = fields
	return p, nil
}

func (r *Repository) getProfileFields(ctx context.Context, userID string) (map[string]domain.ProfileField, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT field_key, source, answered_at FROM wealth_profile_fields WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile field provenance: %w", err)
	}
	defer rows.Close()

	out := make(map[string]domain.ProfileField)
	for rows.Next() {
		var f domain.ProfileField
		if err := rows.Scan(&f.FieldKey, &f.Source, &f.AnsweredAt); err != nil {
			return nil, err
		}
		out[f.FieldKey] = f
	}
	return out, rows.Err()
}

// UpsertWealthProfile writes only the fields named in setKeys, and records the
// provenance of each.
//
// setKeys rather than "every non-nil field" because the two are different
// questions. A nil pointer means "not mentioned in this request"; a key present
// in setKeys with a nil value means "the user cleared this". Inferring intent
// from nil-ness would make it impossible to ever unset an answer, and would
// silently re-write every field on every partial save.
//
// source is how the values arrived -- 'entered' when typed, 'confirmed' when a
// derived value was shown and accepted, 'derived' when computed without being
// seen. The generator refuses to run high-stakes branches off 'derived', so
// passing the wrong one here is a correctness bug, not a cosmetic one.
func (r *Repository) UpsertWealthProfile(
	ctx context.Context, userID string, p *domain.WealthProfile, setKeys []string, source string,
) error {
	if len(setKeys) == 0 {
		return nil
	}
	switch source {
	case domain.FieldSourceDerived, domain.FieldSourceConfirmed,
		domain.FieldSourceEntered, domain.FieldSourceDefault:
	default:
		return fmt.Errorf("%w: unknown profile field source %q", apperrors.ErrInvalidInput, source)
	}

	cols := make([]string, 0, len(setKeys))
	placeholders := make([]string, 0, len(setKeys))
	updates := make([]string, 0, len(setKeys))
	args := []any{userID}

	// A key whose value is NULL is a request to take an answer BACK, which is
	// the opposite of recording one. Tracked separately because the two halves
	// of this function then have to do opposite things with it.
	cleared := make([]string, 0)

	for _, key := range setKeys {
		def, ok := profileFieldByKey[key]
		if !ok {
			return fmt.Errorf("%w: unknown profile field %q", apperrors.ErrInvalidInput, key)
		}
		v := def.Value(p)
		if v == nil {
			cleared = append(cleared, key)
		}
		args = append(args, v)
		cols = append(cols, def.Column)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		updates = append(updates, fmt.Sprintf("%s = EXCLUDED.%s", def.Column, def.Column))
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := fmt.Sprintf(
		`INSERT INTO wealth_profiles (user_id, %s) VALUES ($1, %s)
		 ON CONFLICT (user_id) DO UPDATE SET %s, updated_at = NOW()`,
		strings.Join(cols, ", "), strings.Join(placeholders, ", "), strings.Join(updates, ", "),
	)
	if _, err := tx.Exec(ctx, query, args...); err != nil {
		return constraintError(err, "wealth profile")
	}

	// Provenance is rewritten for exactly the keys touched, so answered_at
	// tracks when this answer was last affirmed rather than when the row was
	// created. Staleness depends on that distinction.
	//
	// Cleared keys lose their row instead of gaining one. The whole feature
	// reads "answered" as "a provenance row exists" (see missingFields in
	// quests_generate.go), so leaving the row behind on a NULL value would
	// report the question as answered while the value is gone -- the generator
	// would stop asking for it and run on nothing.
	for _, key := range setKeys {
		if _, err := tx.Exec(ctx,
			`INSERT INTO wealth_profile_fields (user_id, field_key, source, answered_at)
			 VALUES ($1, $2, $3, NOW())
			 ON CONFLICT (user_id, field_key)
			 DO UPDATE SET source = EXCLUDED.source, answered_at = EXCLUDED.answered_at`,
			userID, key, source,
		); err != nil {
			return fmt.Errorf("failed to record provenance for %q: %w", key, err)
		}
	}

	if len(cleared) > 0 {
		if _, err := tx.Exec(ctx,
			`DELETE FROM wealth_profile_fields WHERE user_id = $1 AND field_key = ANY($2)`,
			userID, cleared,
		); err != nil {
			return fmt.Errorf("failed to clear provenance: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// ---------------------------------------------------------- account terms

// ListAccountTerms returns per-account terms keyed by account id, for every
// account the user owns that has any recorded.
func (r *Repository) ListAccountTerms(ctx context.Context, userID string) (map[string]domain.AccountTerms, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT account_id, apy, apr, promo_apr, promo_expires_on,
		       statement_balance, min_payment, due_day
		FROM account_terms WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list account terms: %w", err)
	}
	defer rows.Close()

	out := make(map[string]domain.AccountTerms)
	for rows.Next() {
		var t domain.AccountTerms
		if err := rows.Scan(&t.AccountID, &t.APY, &t.APR, &t.PromoAPR, &t.PromoExpiresOn,
			&t.StatementBalance, &t.MinPayment, &t.DueDay); err != nil {
			return nil, err
		}
		out[t.AccountID] = t
	}
	return out, rows.Err()
}

// UpsertAccountTerms records the facts an aggregator does not return -- APR
// above all, which currently lives in the financial_plan blob and is why the
// cash-plan waterfall has a "you have liabilities but no interest rates
// entered" blocked state.
func (r *Repository) UpsertAccountTerms(ctx context.Context, userID string, t domain.AccountTerms) error {
	if err := r.checkOwnership(ctx, nil, "accounts", t.AccountID, userID); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO account_terms
			(account_id, user_id, apy, apr, promo_apr, promo_expires_on,
			 statement_balance, min_payment, due_day)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (account_id) DO UPDATE SET
			apy = EXCLUDED.apy,
			apr = EXCLUDED.apr,
			promo_apr = EXCLUDED.promo_apr,
			promo_expires_on = EXCLUDED.promo_expires_on,
			statement_balance = EXCLUDED.statement_balance,
			min_payment = EXCLUDED.min_payment,
			due_day = EXCLUDED.due_day,
			updated_at = NOW()`,
		t.AccountID, userID, t.APY, t.APR, t.PromoAPR, t.PromoExpiresOn,
		t.StatementBalance, t.MinPayment, t.DueDay)
	if err != nil {
		return constraintError(err, "account terms")
	}
	return nil
}

// ------------------------------------------------- retirement account terms

func (r *Repository) ListRetirementAccountTerms(ctx context.Context, userID string) ([]domain.RetirementAccountTerms, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT account_id, kind, monthly_contribution
		FROM retirement_account_terms WHERE user_id = $1 ORDER BY kind`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list retirement account terms: %w", err)
	}
	defer rows.Close()

	var out []domain.RetirementAccountTerms
	for rows.Next() {
		var t domain.RetirementAccountTerms
		var contribution int64
		if err := rows.Scan(&t.AccountID, &t.Kind, &contribution); err != nil {
			return nil, err
		}
		t.MonthlyContribution = money.Money(contribution)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertRetirementAccountTerms(ctx context.Context, userID string, t domain.RetirementAccountTerms) error {
	if err := r.checkOwnership(ctx, nil, "accounts", t.AccountID, userID); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO retirement_account_terms (account_id, user_id, kind, monthly_contribution)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_id) DO UPDATE SET
			kind = EXCLUDED.kind,
			monthly_contribution = EXCLUDED.monthly_contribution,
			updated_at = NOW()`,
		t.AccountID, userID, t.Kind, t.MonthlyContribution.ToInt64())
	if err != nil {
		return constraintError(err, "retirement account terms")
	}
	return nil
}

func (r *Repository) DeleteRetirementAccountTerms(ctx context.Context, userID, accountID string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM retirement_account_terms WHERE account_id = $1 AND user_id = $2`, accountID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete retirement account terms: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ------------------------------------------------------------- dependents

func (r *Repository) ListDependents(ctx context.Context, userID string) ([]domain.Dependent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, birth_year, COALESCE(label, '') FROM dependents WHERE user_id = $1 ORDER BY birth_year`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list dependents: %w", err)
	}
	defer rows.Close()

	var out []domain.Dependent
	for rows.Next() {
		var d domain.Dependent
		if err := rows.Scan(&d.ID, &d.BirthYear, &d.Label); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ReplaceDependents swaps the whole set in one transaction.
//
// Wholesale replacement rather than per-row CRUD because the intake screen
// edits the list as a unit, and a partial failure that left half a household
// recorded would silently change the Dependent Care FSA and life-insurance
// calculations.
func (r *Repository) ReplaceDependents(ctx context.Context, userID string, deps []domain.Dependent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM dependents WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("failed to clear dependents: %w", err)
	}
	for _, d := range deps {
		var label *string
		if d.Label != "" {
			label = &d.Label
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO dependents (user_id, birth_year, label) VALUES ($1, $2, $3)`,
			userID, d.BirthYear, label,
		); err != nil {
			return constraintError(err, "dependent")
		}
	}
	return tx.Commit(ctx)
}

// -------------------------------------------------------- planned expenses

// ListPlannedExpenses returns near-term earmarked cash, soonest first.
// The generator subtracts these from anything it would otherwise sweep or
// invest.
func (r *Repository) ListPlannedExpenses(ctx context.Context, userID string) ([]domain.PlannedExpense, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, label, amount, target_date FROM planned_expenses
		 WHERE user_id = $1 ORDER BY target_date`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list planned expenses: %w", err)
	}
	defer rows.Close()

	var out []domain.PlannedExpense
	for rows.Next() {
		var e domain.PlannedExpense
		var amount int64
		if err := rows.Scan(&e.ID, &e.Label, &amount, &e.TargetDate); err != nil {
			return nil, err
		}
		e.Amount = money.Money(amount)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) CreatePlannedExpense(ctx context.Context, userID string, e domain.PlannedExpense) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx,
		`INSERT INTO planned_expenses (user_id, label, amount, target_date)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, e.Label, e.Amount.ToInt64(), e.TargetDate).Scan(&id)
	if err != nil {
		return "", constraintError(err, "planned expense")
	}
	return id, nil
}

func (r *Repository) DeletePlannedExpense(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM planned_expenses WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete planned expense: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ------------------------------------------------------------------ goals

func (r *Repository) ListGoals(ctx context.Context, userID string) ([]domain.Goal, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, target_amount, target_date, linked_account_id, current_amount, priority
		FROM wealth_goals WHERE user_id = $1 ORDER BY priority, created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list goals: %w", err)
	}
	defer rows.Close()

	var out []domain.Goal
	for rows.Next() {
		var g domain.Goal
		var target, current int64
		if err := rows.Scan(&g.ID, &g.Name, &target, &g.TargetDate,
			&g.LinkedAccountID, &current, &g.Priority); err != nil {
			return nil, err
		}
		g.TargetAmount = money.Money(target)
		g.CurrentAmount = money.Money(current)
		out = append(out, g)
	}
	return out, rows.Err()
}

// UpsertGoal creates or updates one goal. An empty ID creates.
func (r *Repository) UpsertGoal(ctx context.Context, userID string, g domain.Goal) (string, error) {
	if g.LinkedAccountID != nil && *g.LinkedAccountID != "" {
		if err := r.checkOwnership(ctx, nil, "accounts", *g.LinkedAccountID, userID); err != nil {
			return "", err
		}
	}

	if g.ID == "" {
		var id string
		err := r.pool.QueryRow(ctx, `
			INSERT INTO wealth_goals
				(user_id, name, target_amount, target_date, linked_account_id, current_amount, priority)
			VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
			userID, g.Name, g.TargetAmount.ToInt64(), g.TargetDate,
			g.LinkedAccountID, g.CurrentAmount.ToInt64(), g.Priority).Scan(&id)
		if err != nil {
			return "", constraintError(err, "goal")
		}
		return id, nil
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE wealth_goals SET
			name = $3, target_amount = $4, target_date = $5,
			linked_account_id = $6, current_amount = $7, priority = $8,
			updated_at = NOW()
		WHERE id = $2 AND user_id = $1`,
		userID, g.ID, g.Name, g.TargetAmount.ToInt64(), g.TargetDate,
		g.LinkedAccountID, g.CurrentAmount.ToInt64(), g.Priority)
	if err != nil {
		return "", constraintError(err, "goal")
	}
	if tag.RowsAffected() == 0 {
		return "", apperrors.ErrNotFound
	}
	return g.ID, nil
}

func (r *Repository) DeleteGoal(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM wealth_goals WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete goal: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ------------------------------------------------------------ paystub YTD

// GetLatestPaystub returns the most recent year-to-date figures, or nil when
// none have been entered. nil is a normal state, not an error.
func (r *Repository) GetLatestPaystub(ctx context.Context, userID string) (*domain.PaystubYTD, error) {
	var p domain.PaystubYTD
	var gross, fed, state, pretax401k, roth401k, hsa, espp, benefits, supplemental int64

	err := r.pool.QueryRow(ctx, `
		SELECT id, as_of_date, gross, federal_withheld, state_withheld,
		       pretax_401k, roth_401k, hsa_contribution, espp_contribution,
		       pretax_benefits, supplemental_wages, paychecks_ytd, paychecks_per_year
		FROM paystub_ytd WHERE user_id = $1 ORDER BY as_of_date DESC LIMIT 1`, userID,
	).Scan(&p.ID, &p.AsOfDate, &gross, &fed, &state, &pretax401k, &roth401k,
		&hsa, &espp, &benefits, &supplemental, &p.PaychecksYTD, &p.PaychecksPerYear)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest paystub: %w", err)
	}

	p.Gross = money.Money(gross)
	p.FederalWithheld = money.Money(fed)
	p.StateWithheld = money.Money(state)
	p.Pretax401k = money.Money(pretax401k)
	p.Roth401k = money.Money(roth401k)
	p.HSAContribution = money.Money(hsa)
	p.ESPPContribution = money.Money(espp)
	p.PretaxBenefits = money.Money(benefits)
	p.SupplementalWages = money.Money(supplemental)
	return &p, nil
}

// UpsertPaystub records one dated YTD reading. Re-entering the same date
// replaces it; a different date adds a row, preserving the history that makes
// a mid-year deferral change visible.
func (r *Repository) UpsertPaystub(ctx context.Context, userID string, p domain.PaystubYTD) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO paystub_ytd
			(user_id, as_of_date, gross, federal_withheld, state_withheld,
			 pretax_401k, roth_401k, hsa_contribution, espp_contribution,
			 pretax_benefits, supplemental_wages, paychecks_ytd, paychecks_per_year)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (user_id, as_of_date) DO UPDATE SET
			gross = EXCLUDED.gross,
			federal_withheld = EXCLUDED.federal_withheld,
			state_withheld = EXCLUDED.state_withheld,
			pretax_401k = EXCLUDED.pretax_401k,
			roth_401k = EXCLUDED.roth_401k,
			hsa_contribution = EXCLUDED.hsa_contribution,
			espp_contribution = EXCLUDED.espp_contribution,
			pretax_benefits = EXCLUDED.pretax_benefits,
			supplemental_wages = EXCLUDED.supplemental_wages,
			paychecks_ytd = EXCLUDED.paychecks_ytd,
			paychecks_per_year = EXCLUDED.paychecks_per_year`,
		userID, p.AsOfDate, p.Gross.ToInt64(), p.FederalWithheld.ToInt64(),
		p.StateWithheld.ToInt64(), p.Pretax401k.ToInt64(), p.Roth401k.ToInt64(),
		p.HSAContribution.ToInt64(), p.ESPPContribution.ToInt64(),
		p.PretaxBenefits.ToInt64(), p.SupplementalWages.ToInt64(),
		p.PaychecksYTD, p.PaychecksPerYear)
	if err != nil {
		return constraintError(err, "paystub")
	}
	return nil
}

// ---------------------------------------------------------- equity grants

const equityGrantColumns = `
	id, kind, label, grant_date, total_shares, shares_vested, next_vest_date,
	vest_frequency, vest_share, espp_discount_pct, espp_has_lookback,
	espp_contribution_pct, espp_plan_max_pct, espp_offering_start,
	espp_purchase_date, strike_price, expiration_date, has_10b5_1,
	blackout_policy, supplemental_withholding_pct`

func scanEquityGrant(row pgx.Row) (domain.EquityGrant, error) {
	var g domain.EquityGrant
	err := row.Scan(&g.ID, &g.Kind, &g.Label, &g.GrantDate, &g.TotalShares,
		&g.SharesVested, &g.NextVestDate, &g.VestFrequency, &g.VestShare,
		&g.ESPPDiscountPct, &g.ESPPHasLookback, &g.ESPPContributionPct,
		&g.ESPPPlanMaxPct, &g.ESPPOfferingStart, &g.ESPPPurchaseDate,
		&g.StrikePrice, &g.ExpirationDate, &g.HasRule10b51,
		&g.BlackoutPolicy, &g.SupplementalWithholdingPct)
	return g, err
}

func (r *Repository) ListEquityGrants(ctx context.Context, userID string) ([]domain.EquityGrant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+equityGrantColumns+` FROM equity_grants WHERE user_id = $1 ORDER BY grant_date NULLS LAST, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list equity grants: %w", err)
	}
	defer rows.Close()

	var out []domain.EquityGrant
	for rows.Next() {
		g, err := scanEquityGrant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func (r *Repository) UpsertEquityGrant(ctx context.Context, userID string, g domain.EquityGrant) (string, error) {
	var strike *int64
	if g.StrikePrice != nil {
		v := g.StrikePrice.ToInt64()
		strike = &v
	}

	args := []any{
		userID, g.Kind, g.Label, g.GrantDate, g.TotalShares, g.SharesVested,
		g.NextVestDate, g.VestFrequency, g.VestShare, g.ESPPDiscountPct,
		g.ESPPHasLookback, g.ESPPContributionPct, g.ESPPPlanMaxPct,
		g.ESPPOfferingStart, g.ESPPPurchaseDate, strike, g.ExpirationDate,
		g.HasRule10b51, g.BlackoutPolicy, g.SupplementalWithholdingPct,
	}

	if g.ID == "" {
		var id string
		err := r.pool.QueryRow(ctx, `
			INSERT INTO equity_grants
				(user_id, kind, label, grant_date, total_shares, shares_vested,
				 next_vest_date, vest_frequency, vest_share, espp_discount_pct,
				 espp_has_lookback, espp_contribution_pct, espp_plan_max_pct,
				 espp_offering_start, espp_purchase_date, strike_price,
				 expiration_date, has_10b5_1, blackout_policy,
				 supplemental_withholding_pct)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
			RETURNING id`, args...).Scan(&id)
		if err != nil {
			return "", constraintError(err, "equity grant")
		}
		return id, nil
	}

	args = append(args, g.ID)
	tag, err := r.pool.Exec(ctx, `
		UPDATE equity_grants SET
			kind = $2, label = $3, grant_date = $4, total_shares = $5,
			shares_vested = $6, next_vest_date = $7, vest_frequency = $8,
			vest_share = $9, espp_discount_pct = $10, espp_has_lookback = $11,
			espp_contribution_pct = $12, espp_plan_max_pct = $13,
			espp_offering_start = $14, espp_purchase_date = $15,
			strike_price = $16, expiration_date = $17, has_10b5_1 = $18,
			blackout_policy = $19, supplemental_withholding_pct = $20,
			updated_at = NOW()
		WHERE id = $21 AND user_id = $1`, args...)
	if err != nil {
		return "", constraintError(err, "equity grant")
	}
	if tag.RowsAffected() == 0 {
		return "", apperrors.ErrNotFound
	}
	return g.ID, nil
}

func (r *Repository) DeleteEquityGrant(ctx context.Context, userID, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM equity_grants WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete equity grant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ------------------------------------------------------------- tax limits

// GetTaxLimits loads one year's statutory figures.
//
// Returns ErrNotFound when the year has not been seeded rather than an empty
// set, because an empty set would let every contribution rule compute headroom
// against zero and quietly tell the user they have nothing left to contribute.
// A missing year is an operator problem and should surface as one.
func (r *Repository) GetTaxLimits(ctx context.Context, taxYear int) (domain.TaxLimits, error) {
	limits := domain.TaxLimits{
		TaxYear: taxYear,
		Amounts: make(map[string]money.Money),
		Rates:   make(map[string]float64),
	}

	rows, err := r.pool.Query(ctx,
		`SELECT key, amount, rate FROM tax_limits WHERE tax_year = $1`, taxYear)
	if err != nil {
		return limits, fmt.Errorf("failed to get tax limits: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var amount *int64
		var rate *float64
		if err := rows.Scan(&key, &amount, &rate); err != nil {
			return limits, err
		}
		switch {
		case amount != nil:
			limits.Amounts[key] = money.Money(*amount)
		case rate != nil:
			limits.Rates[key] = *rate
		}
	}
	if err := rows.Err(); err != nil {
		return limits, err
	}

	if len(limits.Amounts) == 0 && len(limits.Rates) == 0 {
		return limits, fmt.Errorf("%w: no tax limits seeded for %d", apperrors.ErrNotFound, taxYear)
	}
	return limits, nil
}

// ----------------------------------------------------------------- quests

const questColumns = `
	id, catalog_key, phase, priority, status, title, COALESCE(detail, ''),
	target_amount, due_date, verification, stale, missing_fields,
	completed_at, COALESCE(completed_source, ''), generated_at`

func scanQuest(row pgx.Row) (domain.Quest, error) {
	var q domain.Quest
	var target *int64
	err := row.Scan(&q.ID, &q.CatalogKey, &q.Phase, &q.Priority, &q.Status,
		&q.Title, &q.Detail, &target, &q.DueDate, &q.Verification, &q.Stale,
		&q.MissingFields, &q.CompletedAt, &q.CompletedSource, &q.GeneratedAt)
	if err != nil {
		return q, err
	}
	if target != nil {
		m := money.Money(*target)
		q.TargetAmount = &m
	}
	return q, nil
}

// ListQuests returns the user's generated actions in execution order.
//
// Ordered by phase then priority, which is the order the engine intends them to
// be worked. Callers that surface dated actions ahead of their phase filter on
// DueDate rather than re-sorting.
func (r *Repository) ListQuests(ctx context.Context, userID string) ([]domain.Quest, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+questColumns+` FROM wealth_quests WHERE user_id = $1 ORDER BY phase, priority, catalog_key`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list quests: %w", err)
	}
	defer rows.Close()

	var out []domain.Quest
	for rows.Next() {
		q, err := scanQuest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// ReplaceQuests writes a freshly generated set, preserving completion.
//
// Regeneration must not undo work the USER has done, but it must be free to
// revise what the ENGINE concluded. A manual completion is a claim about
// something the app cannot observe -- "I set up the 10b5-1" -- and it has no
// standing to contradict that. A computed completion is only a reading of the
// data, and it has to move when the data does; leaving it sticky lets a plan go
// on reporting success built on a balance that has since fallen, or on spending
// history that was never there in the first place.
//
// So: skips and manual completions survive. Everything else -- auto-completions
// included -- is recomputed, along with the interpolated text, targets and due
// dates that go stale as balances move.
func (r *Repository) ReplaceQuests(ctx context.Context, userID string, quests []domain.Quest) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	seen := make([]string, 0, len(quests))
	for _, q := range quests {
		seen = append(seen, q.CatalogKey)

		var target *int64
		if q.TargetAmount != nil {
			v := q.TargetAmount.ToInt64()
			target = &v
		}
		missing := q.MissingFields
		if missing == nil {
			missing = []string{}
		}
		// Anything the generator closes is a computed conclusion, never a user
		// claim, so it is stamped as such and stays revisable.
		var completedSource *string
		if q.Status == domain.QuestStatusComplete {
			src := domain.QuestSourceAuto
			completedSource = &src
		}

		var questID string
		err := tx.QueryRow(ctx, `
			INSERT INTO wealth_quests
				(user_id, catalog_key, phase, priority, status, title, detail,
				 target_amount, due_date, verification, stale, missing_fields,
				 completed_at, completed_source, generated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NOW())
			ON CONFLICT (user_id, catalog_key) DO UPDATE SET
				phase = EXCLUDED.phase,
				priority = EXCLUDED.priority,
				title = EXCLUDED.title,
				detail = EXCLUDED.detail,
				target_amount = EXCLUDED.target_amount,
				due_date = EXCLUDED.due_date,
				verification = EXCLUDED.verification,
				missing_fields = EXCLUDED.missing_fields,
				generated_at = NOW(),
				-- Sticky for a skip, or for a manual completion the engine has
				-- no standing to overturn. An AUTO-verifiable action whose data
				-- now says otherwise is reopened: the app can see the balance,
				-- so a stale claim that it was funded is worse than no claim.
				status = CASE WHEN (
					wealth_quests.status = 'skipped'
					OR (
						wealth_quests.status = 'complete'
						AND wealth_quests.completed_source = 'manual'
						AND (
							-- The app cannot see this, so the user's word is the
							-- only evidence there is.
							EXCLUDED.verification = 'manual'
							-- Or it can see it but could not work it out this
							-- time. "We do not know" is not grounds to
							-- contradict someone.
							OR EXCLUDED.status IN ('blocked', 'locked')
						)
					)
				)
					THEN wealth_quests.status ELSE EXCLUDED.status END,
				completed_at = CASE WHEN (
					wealth_quests.status = 'skipped'
					OR (
						wealth_quests.status = 'complete'
						AND wealth_quests.completed_source = 'manual'
						AND (
							-- The app cannot see this, so the user's word is the
							-- only evidence there is.
							EXCLUDED.verification = 'manual'
							-- Or it can see it but could not work it out this
							-- time. "We do not know" is not grounds to
							-- contradict someone.
							OR EXCLUDED.status IN ('blocked', 'locked')
						)
					)
				)
					THEN wealth_quests.completed_at ELSE EXCLUDED.completed_at END,
				completed_source = CASE WHEN (
					wealth_quests.status = 'skipped'
					OR (
						wealth_quests.status = 'complete'
						AND wealth_quests.completed_source = 'manual'
						AND (
							-- The app cannot see this, so the user's word is the
							-- only evidence there is.
							EXCLUDED.verification = 'manual'
							-- Or it can see it but could not work it out this
							-- time. "We do not know" is not grounds to
							-- contradict someone.
							OR EXCLUDED.status IN ('blocked', 'locked')
						)
					)
				)
					THEN wealth_quests.completed_source ELSE EXCLUDED.completed_source END,
				stale = CASE WHEN (
					wealth_quests.status = 'skipped'
					OR (
						wealth_quests.status = 'complete'
						AND wealth_quests.completed_source = 'manual'
						AND (
							-- The app cannot see this, so the user's word is the
							-- only evidence there is.
							EXCLUDED.verification = 'manual'
							-- Or it can see it but could not work it out this
							-- time. "We do not know" is not grounds to
							-- contradict someone.
							OR EXCLUDED.status IN ('blocked', 'locked')
						)
					)
				)
					THEN wealth_quests.stale ELSE EXCLUDED.stale END,
				updated_at = NOW()
			RETURNING id`,
			userID, q.CatalogKey, q.Phase, q.Priority, q.Status, q.Title, q.Detail,
			target, q.DueDate, q.Verification, q.Stale, missing, q.CompletedAt,
			completedSource,
		).Scan(&questID)
		if err != nil {
			return constraintError(err, fmt.Sprintf("quest %q", q.CatalogKey))
		}
	}

	// Actions the catalog no longer produces for this profile are removed --
	// the user changed something that made them irrelevant (sold the house,
	// left the HDHP). Completed ones are kept so the history survives.
	if len(seen) > 0 {
		if _, err := tx.Exec(ctx,
			`DELETE FROM wealth_quests
			 WHERE user_id = $1 AND catalog_key <> ALL($2) AND status NOT IN ('complete','skipped')`,
			userID, seen,
		); err != nil {
			return fmt.Errorf("failed to prune stale quests: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// SetQuestStatus transitions one action and appends the matching event.
//
// The event log is the reason this is not a bare UPDATE. Duration conditions
// ("under the discretionary cap for 90 consecutive days") are queries over
// history, and a mutable status column cannot answer them.
func (r *Repository) SetQuestStatus(ctx context.Context, userID, questID, status, source, note string) (*domain.Quest, error) {
	var event string
	switch status {
	case domain.QuestStatusComplete:
		event = domain.QuestEventCompleted
	case domain.QuestStatusSkipped:
		event = domain.QuestEventSkipped
	case domain.QuestStatusAvailable:
		event = domain.QuestEventUncompleted
	default:
		return nil, fmt.Errorf("%w: cannot transition to status %q", apperrors.ErrInvalidInput, status)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var completedAt *time.Time
	if status == domain.QuestStatusComplete {
		now := time.Now().UTC()
		completedAt = &now
	}

	// A user-driven completion is stamped with its source so regeneration
	// leaves it alone; the engine may revise its own conclusions but not theirs.
	var completedSource *string
	if status == domain.QuestStatusComplete {
		completedSource = &source
	}

	tag, err := tx.Exec(ctx,
		`UPDATE wealth_quests SET status = $1, completed_at = $2, completed_source = $3, updated_at = NOW()
		 WHERE id = $4 AND user_id = $5`,
		status, completedAt, completedSource, questID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to set quest status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, apperrors.ErrNotFound
	}

	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO wealth_quest_events (quest_id, user_id, event, source, note)
		 VALUES ($1, $2, $3, $4, $5)`,
		questID, userID, event, source, notePtr,
	); err != nil {
		return nil, fmt.Errorf("failed to record quest event: %w", err)
	}

	q, err := scanQuest(tx.QueryRow(ctx,
		`SELECT `+questColumns+` FROM wealth_quests WHERE id = $1`, questID))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &q, nil
}

// AppendQuestEvent records something that happened to an action without
// changing it.
//
// Separate from SetQuestStatus because the evaluator observes transitions it
// did not cause: regeneration writes the new status, and this records that it
// happened. Without it an auto-completion leaves no trace at all, and the
// history reads as though the user did everything by hand.
func (r *Repository) AppendQuestEvent(ctx context.Context, userID, questID, event, source, note string) error {
	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO wealth_quest_events (quest_id, user_id, event, source, note)
		 VALUES ($1, $2, $3, $4, $5)`,
		questID, userID, event, source, notePtr)
	if err != nil {
		return fmt.Errorf("failed to append quest event: %w", err)
	}
	return nil
}

// ListQuestEvents returns one action's history, newest first.
func (r *Repository) ListQuestEvents(ctx context.Context, userID, questID string) ([]domain.QuestEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, quest_id, event, source, COALESCE(note, ''), created_at
		 FROM wealth_quest_events WHERE user_id = $1 AND quest_id = $2
		 ORDER BY created_at DESC`, userID, questID)
	if err != nil {
		return nil, fmt.Errorf("failed to list quest events: %w", err)
	}
	defer rows.Close()

	var out []domain.QuestEvent
	for rows.Next() {
		var e domain.QuestEvent
		if err := rows.Scan(&e.ID, &e.QuestID, &e.Event, &e.Source, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
