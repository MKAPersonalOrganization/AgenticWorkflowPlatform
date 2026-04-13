package evaluation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// repository defines the storage interface for evaluation data.
type repository interface {
	// EvalSet operations
	CreateEvalSet(ctx context.Context, es EvalSet) error
	GetEvalSet(ctx context.Context, id uuid.UUID) (EvalSet, error)
	ListEvalSets(ctx context.Context, userID string) ([]EvalSet, error)
	UpdateEvalSet(ctx context.Context, id uuid.UUID, updates EvalSetUpdate) error
	DeleteEvalSet(ctx context.Context, id uuid.UUID, userID string) error

	// EvalCase operations
	CreateEvalCase(ctx context.Context, ec EvalCase) error
	GetEvalCase(ctx context.Context, id uuid.UUID) (EvalCase, error)
	ListEvalCases(ctx context.Context, evalSetID uuid.UUID) ([]EvalCase, error)
	UpdateEvalCase(ctx context.Context, id uuid.UUID, updates EvalCaseUpdate) error
	DeleteEvalCase(ctx context.Context, id uuid.UUID) error
	ReorderEvalCases(ctx context.Context, evalSetID uuid.UUID, caseIDs []uuid.UUID) error

	// Import/export operations
	CreateEvalSetWithCases(ctx context.Context, es EvalSet, cases []EvalCase) error
	ListEvalCasesWithSet(ctx context.Context, evalSetID uuid.UUID) (EvalSet, []EvalCase, error)

	// Audit operations
	LogEvalAudit(ctx context.Context, evalSetID uuid.UUID, action, actor string, changes json.RawMessage) error

	// Metric profile operations
	CreateMetricProfile(ctx context.Context, p MetricProfile) error
	GetMetricProfile(ctx context.Context, id uuid.UUID) (MetricProfile, error)
	ListMetricProfiles(ctx context.Context, userID string) ([]MetricProfile, error)
	UpdateMetricProfile(ctx context.Context, id uuid.UUID, updates MetricProfileUpdate) error
	DeleteMetricProfile(ctx context.Context, id uuid.UUID, userID string) error
	SetDefaultProfile(ctx context.Context, id uuid.UUID, userID string) error
}

// postgresEvalRepo implements repository using PostgreSQL.
type postgresEvalRepo struct {
	db *sqlx.DB
}

// newPostgresEvalRepo creates a new postgres evaluation repository.
func newPostgresEvalRepo(db *sqlx.DB) *postgresEvalRepo {
	return &postgresEvalRepo{db: db}
}

// ─── EvalSet ─────────────────────────────────────────────────────────────────

func (r *postgresEvalRepo) CreateEvalSet(ctx context.Context, es EvalSet) error {
	const q = `
INSERT INTO eval_sets (id, user_id, name, description, created_at, updated_at)
VALUES (:id, :user_id, :name, :description, :created_at, :updated_at)`

	if _, err := r.db.NamedExecContext(ctx, q, es); err != nil {
		return fmt.Errorf("inserting eval_set: %w", err)
	}
	return nil
}

func (r *postgresEvalRepo) GetEvalSet(ctx context.Context, id uuid.UUID) (EvalSet, error) {
	const q = `
SELECT id, user_id, name, description, created_at, updated_at
FROM eval_sets WHERE id = $1`

	var es EvalSet
	if err := r.db.GetContext(ctx, &es, q, id); err != nil {
		if err == sql.ErrNoRows {
			return EvalSet{}, fmt.Errorf("eval_set not found")
		}
		return EvalSet{}, fmt.Errorf("selecting eval_set: %w", err)
	}
	return es, nil
}

func (r *postgresEvalRepo) ListEvalSets(ctx context.Context, userID string) ([]EvalSet, error) {
	const q = `
SELECT es.id, es.user_id, es.name, es.description,
       COUNT(ec.id) AS case_count,
       es.created_at, es.updated_at
FROM eval_sets es
LEFT JOIN eval_cases ec ON ec.eval_set_id = es.id
WHERE es.user_id = $1
GROUP BY es.id
ORDER BY es.updated_at DESC`

	var sets []EvalSet
	if err := r.db.SelectContext(ctx, &sets, q, userID); err != nil {
		return nil, fmt.Errorf("listing eval_sets: %w", err)
	}
	return sets, nil
}

func (r *postgresEvalRepo) UpdateEvalSet(ctx context.Context, id uuid.UUID, updates EvalSetUpdate) error {
	q := `UPDATE eval_sets SET updated_at = now(), version = version + 1`
	args := []any{}
	argIdx := 1

	if updates.Name != nil {
		q += fmt.Sprintf(", name = $%d", argIdx)
		args = append(args, *updates.Name)
		argIdx++
	}
	if updates.Description != nil {
		q += fmt.Sprintf(", description = $%d", argIdx)
		args = append(args, *updates.Description)
		argIdx++
	}
	if updates.UpdatedBy != nil {
		q += fmt.Sprintf(", updated_by = $%d", argIdx)
		args = append(args, *updates.UpdatedBy)
		argIdx++
	}

	// Optimistic concurrency: only update if version matches
	if updates.Version > 0 {
		q += fmt.Sprintf(" WHERE id = $%d AND version = $%d", argIdx, argIdx+1)
		args = append(args, id, updates.Version)
	} else {
		q += fmt.Sprintf(" WHERE id = $%d", argIdx)
		args = append(args, id)
	}

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("updating eval_set: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		if updates.Version > 0 {
			return fmt.Errorf("eval_set not found or concurrent modification")
		}
		return fmt.Errorf("eval_set not found")
	}
	return nil
}

func (r *postgresEvalRepo) DeleteEvalSet(ctx context.Context, id uuid.UUID, userID string) error {
	const q = `DELETE FROM eval_sets WHERE id = $1 AND user_id = $2`

	res, err := r.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return fmt.Errorf("deleting eval_set: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("eval_set not found or unauthorized")
	}
	return nil
}

// ─── EvalCase ────────────────────────────────────────────────────────────────

func (r *postgresEvalRepo) CreateEvalCase(ctx context.Context, ec EvalCase) error {
	const q = `
INSERT INTO eval_cases (
    id, eval_set_id, name, input, expected_trajectory, expected_response,
    rubrics, user_messages, sort_order, created_at, updated_at
) VALUES (
    :id, :eval_set_id, :name, :input, :expected_trajectory, :expected_response,
    :rubrics, :user_messages, :sort_order, :created_at, :updated_at
)`

	if _, err := r.db.NamedExecContext(ctx, q, ec); err != nil {
		return fmt.Errorf("inserting eval_case: %w", err)
	}
	return nil
}

func (r *postgresEvalRepo) GetEvalCase(ctx context.Context, id uuid.UUID) (EvalCase, error) {
	const q = `
SELECT id, eval_set_id, name, input, expected_trajectory, expected_response,
       rubrics, user_messages, sort_order, created_at, updated_at
FROM eval_cases WHERE id = $1`

	var ec EvalCase
	if err := r.db.GetContext(ctx, &ec, q, id); err != nil {
		if err == sql.ErrNoRows {
			return EvalCase{}, fmt.Errorf("eval_case not found")
		}
		return EvalCase{}, fmt.Errorf("selecting eval_case: %w", err)
	}
	return ec, nil
}

func (r *postgresEvalRepo) ListEvalCases(ctx context.Context, evalSetID uuid.UUID) ([]EvalCase, error) {
	const q = `
SELECT id, eval_set_id, name, input, expected_trajectory, expected_response,
       rubrics, user_messages, sort_order, created_at, updated_at
FROM eval_cases
WHERE eval_set_id = $1
ORDER BY sort_order ASC, created_at ASC`

	var cases []EvalCase
	if err := r.db.SelectContext(ctx, &cases, q, evalSetID); err != nil {
		return nil, fmt.Errorf("listing eval_cases: %w", err)
	}
	return cases, nil
}

func (r *postgresEvalRepo) UpdateEvalCase(ctx context.Context, id uuid.UUID, updates EvalCaseUpdate) error {
	q := `UPDATE eval_cases SET updated_at = now()`
	args := []any{}
	argIdx := 1

	if updates.Name != nil {
		q += fmt.Sprintf(", name = $%d", argIdx)
		args = append(args, *updates.Name)
		argIdx++
	}
	if updates.Input != nil {
		q += fmt.Sprintf(", input = $%d", argIdx)
		args = append(args, *updates.Input)
		argIdx++
	}
	if updates.ExpectedTrajectory != nil {
		q += fmt.Sprintf(", expected_trajectory = $%d", argIdx)
		args = append(args, updates.ExpectedTrajectory)
		argIdx++
	}
	if updates.ExpectedResponse != nil {
		q += fmt.Sprintf(", expected_response = $%d", argIdx)
		args = append(args, *updates.ExpectedResponse)
		argIdx++
	}
	if updates.Rubrics != nil {
		q += fmt.Sprintf(", rubrics = $%d", argIdx)
		args = append(args, updates.Rubrics)
		argIdx++
	}
	if updates.UserMessages != nil {
		q += fmt.Sprintf(", user_messages = $%d", argIdx)
		args = append(args, updates.UserMessages)
		argIdx++
	}

	q += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("updating eval_case: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("eval_case not found")
	}
	return nil
}

func (r *postgresEvalRepo) DeleteEvalCase(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM eval_cases WHERE id = $1`

	res, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("deleting eval_case: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("eval_case not found")
	}
	return nil
}

func (r *postgresEvalRepo) ReorderEvalCases(ctx context.Context, evalSetID uuid.UUID, caseIDs []uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const q = `UPDATE eval_cases SET sort_order = $1 WHERE id = $2 AND eval_set_id = $3`

	for i, caseID := range caseIDs {
		res, err := tx.ExecContext(ctx, q, i, caseID, evalSetID)
		if err != nil {
			return fmt.Errorf("reordering eval_case %d: %w", i, err)
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("checking rows affected: %w", err)
		}
		if rows == 0 {
			return fmt.Errorf("eval_case %s not found in eval_set %s", caseID, evalSetID)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing reorder transaction: %w", err)
	}
	return nil
}

// CreateEvalSetWithCases atomically inserts an EvalSet and all its EvalCases
// in a single transaction. If any insert fails, the entire operation rolls back.
func (r *postgresEvalRepo) CreateEvalSetWithCases(ctx context.Context, es EvalSet, cases []EvalCase) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Insert EvalSet
	const qSet = `
INSERT INTO eval_sets (id, user_id, name, description, created_at, updated_at)
VALUES (:id, :user_id, :name, :description, :created_at, :updated_at)`
	if _, err := sqlx.NamedExecContext(ctx, tx, qSet, es); err != nil {
		return fmt.Errorf("inserting eval_set: %w", err)
	}

	// Insert each EvalCase
	const qCase = `
INSERT INTO eval_cases (
    id, eval_set_id, name, input, expected_trajectory, expected_response,
    rubrics, user_messages, sort_order, created_at, updated_at
) VALUES (
    :id, :eval_set_id, :name, :input, :expected_trajectory, :expected_response,
    :rubrics, :user_messages, :sort_order, :created_at, :updated_at
)`
	for i, ec := range cases {
		if _, err := sqlx.NamedExecContext(ctx, tx, qCase, ec); err != nil {
			return fmt.Errorf("inserting eval_case %d: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

// ListEvalCasesWithSet returns an EvalSet populated with all its EvalCases.
func (r *postgresEvalRepo) ListEvalCasesWithSet(ctx context.Context, evalSetID uuid.UUID) (EvalSet, []EvalCase, error) {
	const qSet = `
SELECT id, user_id, name, description, created_at, updated_at
FROM eval_sets WHERE id = $1`
	var es EvalSet
	if err := r.db.GetContext(ctx, &es, qSet, evalSetID); err != nil {
		if err == sql.ErrNoRows {
			return EvalSet{}, nil, fmt.Errorf("eval_set not found")
		}
		return EvalSet{}, nil, fmt.Errorf("selecting eval_set: %w", err)
	}

	cases, err := r.ListEvalCases(ctx, evalSetID)
	if err != nil {
		return EvalSet{}, nil, err
	}
	return es, cases, nil
}

// LogEvalAudit inserts an audit log entry for an eval set action.
func (r *postgresEvalRepo) LogEvalAudit(ctx context.Context, evalSetID uuid.UUID, action, actor string, changes json.RawMessage) error {
	const q = `
INSERT INTO eval_audit_log (eval_set_id, action, actor, changes, created_at)
VALUES ($1, $2, $3, $4, now())`
	if _, err := r.db.ExecContext(ctx, q, evalSetID, action, actor, changes); err != nil {
		return fmt.Errorf("inserting audit log: %w", err)
	}
	return nil
}

// ─── Metric Profile ──────────────────────────────────────────────────────────

func (r *postgresEvalRepo) CreateMetricProfile(ctx context.Context, p MetricProfile) error {
	const q = `
INSERT INTO eval_metric_profiles (id, user_id, name, description, metrics, is_default, created_at, updated_at)
VALUES (:id, :user_id, :name, :description, :metrics, :is_default, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, q, p); err != nil {
		return fmt.Errorf("inserting metric profile: %w", err)
	}
	return nil
}

func (r *postgresEvalRepo) GetMetricProfile(ctx context.Context, id uuid.UUID) (MetricProfile, error) {
	const q = `
SELECT id, user_id, name, description, metrics, is_default, created_at, updated_at
FROM eval_metric_profiles WHERE id = $1`
	var p MetricProfile
	if err := r.db.GetContext(ctx, &p, q, id); err != nil {
		if err == sql.ErrNoRows {
			return MetricProfile{}, fmt.Errorf("metric profile not found")
		}
		return MetricProfile{}, fmt.Errorf("selecting metric profile: %w", err)
	}
	return p, nil
}

func (r *postgresEvalRepo) ListMetricProfiles(ctx context.Context, userID string) ([]MetricProfile, error) {
	const q = `
SELECT id, user_id, name, description, metrics, is_default, created_at, updated_at
FROM eval_metric_profiles WHERE user_id = $1 ORDER BY is_default DESC, updated_at DESC`
	var profiles []MetricProfile
	if err := r.db.SelectContext(ctx, &profiles, q, userID); err != nil {
		return nil, fmt.Errorf("listing metric profiles: %w", err)
	}
	return profiles, nil
}

func (r *postgresEvalRepo) UpdateMetricProfile(ctx context.Context, id uuid.UUID, updates MetricProfileUpdate) error {
	q := `UPDATE eval_metric_profiles SET updated_at = now()`
	args := []any{}
	argIdx := 1

	if updates.Name != nil {
		q += fmt.Sprintf(", name = $%d", argIdx)
		args = append(args, *updates.Name)
		argIdx++
	}
	if updates.Description != nil {
		q += fmt.Sprintf(", description = $%d", argIdx)
		args = append(args, *updates.Description)
		argIdx++
	}
	if updates.Metrics != nil {
		q += fmt.Sprintf(", metrics = $%d", argIdx)
		args = append(args, updates.Metrics)
		argIdx++
	}
	if updates.IsDefault != nil {
		q += fmt.Sprintf(", is_default = $%d", argIdx)
		args = append(args, *updates.IsDefault)
		argIdx++
	}

	q += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("updating metric profile: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("metric profile not found")
	}
	return nil
}

func (r *postgresEvalRepo) DeleteMetricProfile(ctx context.Context, id uuid.UUID, userID string) error {
	const q = `DELETE FROM eval_metric_profiles WHERE id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return fmt.Errorf("deleting metric profile: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("metric profile not found or unauthorized")
	}
	return nil
}

func (r *postgresEvalRepo) SetDefaultProfile(ctx context.Context, id uuid.UUID, userID string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Unset all defaults for this user
	const qUnset = `UPDATE eval_metric_profiles SET is_default = false WHERE user_id = $1`
	if _, err := tx.ExecContext(ctx, qUnset, userID); err != nil {
		return fmt.Errorf("unsetting default profiles: %w", err)
	}

	// Set the new default
	const qSet = `UPDATE eval_metric_profiles SET is_default = true, updated_at = now() WHERE id = $1 AND user_id = $2`
	res, err := tx.ExecContext(ctx, qSet, id, userID)
	if err != nil {
		return fmt.Errorf("setting default profile: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("metric profile not found or unauthorized")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}
