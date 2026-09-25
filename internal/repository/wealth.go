package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
// above all, which no bank feed supplies. Without it a card gets no payoff
// action of its own, only a blocked request for the rate.
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

// UpsertRetirementAccountTerms records an account's tax treatment, and marks
// the account itself as an investment: a tax treatment is only meaningful on
// invested money, and the two must not disagree about what the account is.
func (r *Repository) UpsertRetirementAccountTerms(ctx context.Context, userID string, t domain.RetirementAccountTerms) error {
	if err := r.checkOwnership(ctx, nil, "accounts", t.AccountID, userID); err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx,
		`UPDATE accounts SET role = 'investment', updated_at = NOW()
		 WHERE id = $1 AND user_id = $2 AND type = 'asset'`, t.AccountID, userID)
	if err != nil {
		return fmt.Errorf("failed to mark account as an investment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: only an asset account can hold retirement savings", apperrors.ErrInvalidInput)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO retirement_account_terms (account_id, user_id, kind, monthly_contribution)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (account_id) DO UPDATE SET
			kind = EXCLUDED.kind,
			monthly_contribution = EXCLUDED.monthly_contribution,
			updated_at = NOW()`,
		t.AccountID, userID, t.Kind, t.MonthlyContribution.ToInt64()); err != nil {
		return constraintError(err, "retirement account terms")
	}
	return tx.Commit(ctx)
}

// MarkAccountsAsCash classifies the given asset accounts as savings unless
// they already count as cash. Used to carry a legacy hand-picked cash list
// over into roles.
func (r *Repository) MarkAccountsAsCash(ctx context.Context, userID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE accounts SET role = 'savings', updated_at = NOW()
		WHERE user_id = $1 AND id = ANY($2) AND type = 'asset'
		  AND (role IS NULL OR role NOT IN ('checking', 'savings'))`, userID, ids)
	if err != nil {
		return fmt.Errorf("failed to mark accounts as cash: %w", err)
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
//
// The action list itself is not stored: it is worked out on every read. What
// is kept is what cannot be recomputed -- the user's marks, the last status the
// engine saw for each action, and the history of both. See migration 0031.

// ListQuestState returns the last observed status of each action, by key.
func (r *Repository) ListQuestState(ctx context.Context, userID string) (map[string]domain.QuestState, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT catalog_key, status, achieved_at, changed_at FROM wealth_quest_state WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list quest state: %w", err)
	}
	defer rows.Close()

	out := make(map[string]domain.QuestState)
	for rows.Next() {
		var key string
		var s domain.QuestState
		if err := rows.Scan(&key, &s.Status, &s.AchievedAt, &s.ChangedAt); err != nil {
			return nil, err
		}
		out[key] = s
	}
	return out, rows.Err()
}

// ListQuestMarks returns what the user has said about each action, by key.
func (r *Repository) ListQuestMarks(ctx context.Context, userID string) (map[string]domain.QuestMark, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT catalog_key, mark, COALESCE(note, ''), created_at FROM wealth_quest_marks WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list quest marks: %w", err)
	}
	defer rows.Close()

	out := make(map[string]domain.QuestMark)
	for rows.Next() {
		var key string
		var m domain.QuestMark
		if err := rows.Scan(&key, &m.Mark, &m.Note, &m.CreatedAt); err != nil {
			return nil, err
		}
		out[key] = m
	}
	return out, rows.Err()
}

// RecordQuestTransition stores a newly observed status for one action and, if
// event is non-empty, appends it to the history -- but only when the stored
// status actually changes.
//
// The upsert's WHERE clause is the concurrency guard. Two page loads that both
// see the same change race on the row; the second waits for the first, then
// finds the status already written and updates nothing. Only the caller whose
// write changed the row appends the event, so a transition is recorded once
// however many tabs are open. Reports whether this call changed anything.
func (r *Repository) RecordQuestTransition(
	ctx context.Context, userID, key, status, event, source, note string,
) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		INSERT INTO wealth_quest_state (user_id, catalog_key, status, achieved_at, changed_at)
		VALUES ($1, $2, $3, CASE WHEN $3 = 'complete' THEN NOW() END, NOW())
		ON CONFLICT (user_id, catalog_key) DO UPDATE SET
			status = EXCLUDED.status,
			changed_at = NOW(),
			-- The first achievement is kept for good.
			achieved_at = COALESCE(wealth_quest_state.achieved_at, EXCLUDED.achieved_at)
		WHERE wealth_quest_state.status IS DISTINCT FROM EXCLUDED.status`,
		userID, key, status)
	if err != nil {
		return false, constraintError(err, fmt.Sprintf("quest state %q", key))
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}

	if event != "" {
		if err := appendQuestEvent(ctx, tx, userID, key, event, source, note); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}

// SetQuestMark records the user marking an action done or not for them.
//
// The stored status moves with the mark, in the same write, so the next read
// finds nothing new to record: the user's own event is the one that stands,
// rather than an engine event restating it.
func (r *Repository) SetQuestMark(ctx context.Context, userID, key, mark, note string) error {
	var event string
	switch mark {
	case domain.QuestStatusComplete:
		event = domain.QuestEventCompleted
	case domain.QuestStatusSkipped:
		event = domain.QuestEventSkipped
	default:
		return fmt.Errorf("%w: cannot mark an action %q", apperrors.ErrInvalidInput, mark)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO wealth_quest_marks (user_id, catalog_key, mark, note)
		VALUES ($1, $2, $3, NULLIF($4, ''))
		ON CONFLICT (user_id, catalog_key) DO UPDATE SET
			mark = EXCLUDED.mark, note = EXCLUDED.note, created_at = NOW()`,
		userID, key, mark, note); err != nil {
		return constraintError(err, fmt.Sprintf("quest mark %q", key))
	}
	if err := setQuestStatus(ctx, tx, userID, key, mark); err != nil {
		return err
	}
	if err := appendQuestEvent(ctx, tx, userID, key, event, domain.QuestSourceManual, note); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ClearQuestMark takes a mark back: the action reopens, and what it becomes
// is the engine's to say on the next read. Returns ErrNotFound when there was
// no mark to clear.
func (r *Repository) ClearQuestMark(ctx context.Context, userID, key string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var prior string
	err = tx.QueryRow(ctx,
		`DELETE FROM wealth_quest_marks WHERE user_id = $1 AND catalog_key = $2 RETURNING mark`,
		userID, key).Scan(&prior)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperrors.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to clear quest mark: %w", err)
	}

	event := domain.QuestEventUncompleted
	if prior == domain.QuestStatusSkipped {
		event = domain.QuestEventUnskipped
	}
	// Available is a placeholder until the next read works the status out;
	// any move from there is ordinary churn and records no event.
	if err := setQuestStatus(ctx, tx, userID, key, domain.QuestStatusAvailable); err != nil {
		return err
	}
	if err := appendQuestEvent(ctx, tx, userID, key, event, domain.QuestSourceManual, ""); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func setQuestStatus(ctx context.Context, tx pgx.Tx, userID, key, status string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO wealth_quest_state (user_id, catalog_key, status, achieved_at, changed_at)
		VALUES ($1, $2, $3, CASE WHEN $3 = 'complete' THEN NOW() END, NOW())
		ON CONFLICT (user_id, catalog_key) DO UPDATE SET
			status = EXCLUDED.status,
			changed_at = NOW(),
			achieved_at = COALESCE(wealth_quest_state.achieved_at, EXCLUDED.achieved_at)`,
		userID, key, status); err != nil {
		return constraintError(err, fmt.Sprintf("quest state %q", key))
	}
	return nil
}

func appendQuestEvent(ctx context.Context, tx pgx.Tx, userID, key, event, source, note string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO wealth_quest_events (user_id, catalog_key, event, source, note)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))`,
		userID, key, event, source, note); err != nil {
		return fmt.Errorf("failed to record quest event: %w", err)
	}
	return nil
}

// ListQuestEvents returns one action's history, newest first.
func (r *Repository) ListQuestEvents(ctx context.Context, userID, key string) ([]domain.QuestEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, catalog_key, event, source, COALESCE(note, ''), created_at
		 FROM wealth_quest_events WHERE user_id = $1 AND catalog_key = $2
		 ORDER BY created_at DESC, id`, userID, key)
	if err != nil {
		return nil, fmt.Errorf("failed to list quest events: %w", err)
	}
	defer rows.Close()

	out := []domain.QuestEvent{}
	for rows.Next() {
		var e domain.QuestEvent
		if err := rows.Scan(&e.ID, &e.CatalogKey, &e.Event, &e.Source, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
