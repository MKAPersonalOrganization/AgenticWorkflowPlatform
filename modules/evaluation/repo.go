package evaluation

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

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

	// Evaluation run operations
	CreateEvalRun(ctx context.Context, run EvalRun) error
	GetEvalRun(ctx context.Context, id uuid.UUID) (EvalRun, error)
	ListEvalRuns(ctx context.Context, userID string) ([]EvalRun, error)
	UpdateEvalRunStatus(ctx context.Context, id uuid.UUID, status string, errMsg *string) error

	// Evaluation result operations
	BatchCreateResults(ctx context.Context, results []EvalResult) error
	ListResultsByRun(ctx context.Context, runID uuid.UUID) ([]EvalResult, error)

	// Rubric operations
	CreateRubric(ctx context.Context, r Rubric) error
	GetRubric(ctx context.Context, id uuid.UUID) (Rubric, error)
	ListRubrics(ctx context.Context, userID string) ([]Rubric, error)
	UpdateRubric(ctx context.Context, id uuid.UUID, updates RubricUpdate) error
	DeleteRubric(ctx context.Context, id uuid.UUID, userID string) error
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

// profileRow is the DB-facing representation with json.RawMessage for JSONB.
type profileRow struct {
	ID          uuid.UUID       `db:"id"`
	UserID      string          `db:"user_id"`
	Name        string          `db:"name"`
	Description *string         `db:"description"`
	Metrics     json.RawMessage `db:"metrics"`
	IsDefault   bool            `db:"is_default"`
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
}

func (r *postgresEvalRepo) CreateMetricProfile(ctx context.Context, p MetricProfile) error {
	metricsJSON, err := json.Marshal(p.Metrics)
	if err != nil {
		return fmt.Errorf("marshaling metrics: %w", err)
	}
	row := profileRow{
		ID:          p.ID,
		UserID:      p.UserID,
		Name:        p.Name,
		Description: p.Description,
		Metrics:     metricsJSON,
		IsDefault:   p.IsDefault,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
	const q = `
INSERT INTO eval_metric_profiles (id, user_id, name, description, metrics, is_default, created_at, updated_at)
VALUES (:id, :user_id, :name, :description, :metrics, :is_default, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, q, row); err != nil {
		return fmt.Errorf("inserting metric profile: %w", err)
	}
	return nil
}

func (r *postgresEvalRepo) GetMetricProfile(ctx context.Context, id uuid.UUID) (MetricProfile, error) {
	var row profileRow
	const q = `
SELECT id, user_id, name, description, metrics, is_default, created_at, updated_at
FROM eval_metric_profiles WHERE id = $1`
	if err := r.db.GetContext(ctx, &row, q, id); err != nil {
		if err == sql.ErrNoRows {
			return MetricProfile{}, fmt.Errorf("metric profile not found")
		}
		return MetricProfile{}, fmt.Errorf("selecting metric profile: %w", err)
	}
	p := MetricProfile{
		ID:          row.ID,
		UserID:      row.UserID,
		Name:        row.Name,
		Description: row.Description,
		IsDefault:   row.IsDefault,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	if row.Metrics != nil {
		if err := json.Unmarshal(row.Metrics, &p.Metrics); err != nil {
			return MetricProfile{}, fmt.Errorf("unmarshaling metrics: %w", err)
		}
	}
	return p, nil
}

func (r *postgresEvalRepo) ListMetricProfiles(ctx context.Context, userID string) ([]MetricProfile, error) {
	const q = `
SELECT id, user_id, name, description, metrics, is_default, created_at, updated_at
FROM eval_metric_profiles WHERE user_id = $1 ORDER BY is_default DESC, updated_at DESC`
	var rows []profileRow
	if err := r.db.SelectContext(ctx, &rows, q, userID); err != nil {
		return nil, fmt.Errorf("listing metric profiles: %w", err)
	}
	profiles := make([]MetricProfile, 0, len(rows))
	for _, row := range rows {
		p := MetricProfile{
			ID:          row.ID,
			UserID:      row.UserID,
			Name:        row.Name,
			Description: row.Description,
			IsDefault:   row.IsDefault,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}
		if row.Metrics != nil {
			if err := json.Unmarshal(row.Metrics, &p.Metrics); err != nil {
				return nil, fmt.Errorf("unmarshaling metrics for profile %s: %w", row.ID, err)
			}
		}
		profiles = append(profiles, p)
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
		metricsJSON, err := json.Marshal(updates.Metrics)
		if err != nil {
			return fmt.Errorf("marshaling metrics: %w", err)
		}
		q += fmt.Sprintf(", metrics = $%d", argIdx)
		args = append(args, metricsJSON)
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

// ─── Evaluation Runs ─────────────────────────────────────────────────────────

func (r *postgresEvalRepo) CreateEvalRun(ctx context.Context, run EvalRun) error {
	const q = `
INSERT INTO evaluation_runs (id, user_id, eval_set_id, agent_id, profile_id, status, num_runs, created_at)
VALUES (:id, :user_id, :eval_set_id, :agent_id, :profile_id, :status, :num_runs, :created_at)`
	if _, err := r.db.NamedExecContext(ctx, q, run); err != nil {
		return fmt.Errorf("inserting evaluation run: %w", err)
	}
	return nil
}

func (r *postgresEvalRepo) GetEvalRun(ctx context.Context, id uuid.UUID) (EvalRun, error) {
	const q = `
SELECT id, user_id, eval_set_id, agent_id, profile_id, status, num_runs,
       started_at, completed_at, error_msg, created_at
FROM evaluation_runs WHERE id = $1`
	var run EvalRun
	if err := r.db.GetContext(ctx, &run, q, id); err != nil {
		if err == sql.ErrNoRows {
			return EvalRun{}, fmt.Errorf("evaluation run not found")
		}
		return EvalRun{}, fmt.Errorf("selecting evaluation run: %w", err)
	}
	return run, nil
}

func (r *postgresEvalRepo) ListEvalRuns(ctx context.Context, userID string) ([]EvalRun, error) {
	const q = `
SELECT id, user_id, eval_set_id, agent_id, profile_id, status, num_runs,
       started_at, completed_at, error_msg, created_at
FROM evaluation_runs WHERE user_id = $1 ORDER BY created_at DESC`
	var runs []EvalRun
	if err := r.db.SelectContext(ctx, &runs, q, userID); err != nil {
		return nil, fmt.Errorf("listing evaluation runs: %w", err)
	}
	return runs, nil
}

func (r *postgresEvalRepo) UpdateEvalRunStatus(ctx context.Context, id uuid.UUID, status string, errMsg *string) error {
	q := `UPDATE evaluation_runs SET status = $1`
	args := []any{status}
	argIdx := 2

	if status == "running" {
		q += ", started_at = now()"
	}
	if status == "completed" || status == "failed" || status == "cancelled" {
		q += ", completed_at = now()"
	}
	if errMsg != nil {
		q += fmt.Sprintf(", error_msg = $%d", argIdx)
		args = append(args, *errMsg)
		argIdx++
	}

	q += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("updating evaluation run status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("evaluation run not found")
	}
	return nil
}

// ─── Evaluation Results ──────────────────────────────────────────────────────

func (r *postgresEvalRepo) BatchCreateResults(ctx context.Context, results []EvalResult) error {
	if len(results) == 0 {
		return nil
	}
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const q = `
INSERT INTO evaluation_results (id, run_id, case_id, metric_id, score, status, details, invocation, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())`

	for _, res := range results {
		if _, err := tx.ExecContext(ctx, q,
			res.ID, res.RunID, res.CaseID, res.MetricID,
			res.Score, res.Status, res.Details, res.Invocation); err != nil {
			return fmt.Errorf("inserting evaluation result: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing results: %w", err)
	}
	return nil
}

func (r *postgresEvalRepo) ListResultsByRun(ctx context.Context, runID uuid.UUID) ([]EvalResult, error) {
	const q = `
SELECT id, run_id, case_id, metric_id, score, status, details, invocation, created_at
FROM evaluation_results WHERE run_id = $1 ORDER BY case_id, metric_id, invocation`
	var results []EvalResult
	if err := r.db.SelectContext(ctx, &results, q, runID); err != nil {
		return nil, fmt.Errorf("listing evaluation results: %w", err)
	}
	return results, nil
}

// ─── Rubrics ─────────────────────────────────────────────────────────────────

type rubricRow struct {
	ID          uuid.UUID       `db:"id"`
	UserID      string          `db:"user_id"`
	Name        string          `db:"name"`
	Description *string         `db:"description"`
	Levels      json.RawMessage `db:"levels"`
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
}

func (r *postgresEvalRepo) CreateRubric(ctx context.Context, rubric Rubric) error {
	levelsJSON, err := json.Marshal(rubric.Levels)
	if err != nil {
		return fmt.Errorf("marshaling levels: %w", err)
	}
	row := rubricRow{
		ID:          rubric.ID,
		UserID:      rubric.UserID,
		Name:        rubric.Name,
		Description: rubric.Description,
		Levels:      levelsJSON,
		CreatedAt:   rubric.CreatedAt,
		UpdatedAt:   rubric.UpdatedAt,
	}
	const q = `
INSERT INTO rubrics (id, user_id, name, description, levels, created_at, updated_at)
VALUES (:id, :user_id, :name, :description, :levels, :created_at, :updated_at)`
	if _, err := r.db.NamedExecContext(ctx, q, row); err != nil {
		return fmt.Errorf("inserting rubric: %w", err)
	}
	return nil
}

func (r *postgresEvalRepo) GetRubric(ctx context.Context, id uuid.UUID) (Rubric, error) {
	var row rubricRow
	const q = `
SELECT id, user_id, name, description, levels, created_at, updated_at
FROM rubrics WHERE id = $1`
	if err := r.db.GetContext(ctx, &row, q, id); err != nil {
		if err == sql.ErrNoRows {
			return Rubric{}, fmt.Errorf("rubric not found")
		}
		return Rubric{}, fmt.Errorf("selecting rubric: %w", err)
	}
	rubric := Rubric{
		ID:          row.ID,
		UserID:      row.UserID,
		Name:        row.Name,
		Description: row.Description,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	if row.Levels != nil {
		if err := json.Unmarshal(row.Levels, &rubric.Levels); err != nil {
			return Rubric{}, fmt.Errorf("unmarshaling levels: %w", err)
		}
	}
	return rubric, nil
}

func (r *postgresEvalRepo) ListRubrics(ctx context.Context, userID string) ([]Rubric, error) {
	const q = `
SELECT id, user_id, name, description, levels, created_at, updated_at
FROM rubrics WHERE user_id = $1 ORDER BY updated_at DESC`
	var rows []rubricRow
	if err := r.db.SelectContext(ctx, &rows, q, userID); err != nil {
		return nil, fmt.Errorf("listing rubrics: %w", err)
	}
	rubrics := make([]Rubric, 0, len(rows))
	for _, row := range rows {
		rubric := Rubric{
			ID:          row.ID,
			UserID:      row.UserID,
			Name:        row.Name,
			Description: row.Description,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}
		if row.Levels != nil {
			if err := json.Unmarshal(row.Levels, &rubric.Levels); err != nil {
				return nil, fmt.Errorf("unmarshaling levels for rubric %s: %w", row.ID, err)
			}
		}
		rubrics = append(rubrics, rubric)
	}
	return rubrics, nil
}

func (r *postgresEvalRepo) UpdateRubric(ctx context.Context, id uuid.UUID, updates RubricUpdate) error {
	q := `UPDATE rubrics SET updated_at = now()`
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
	if updates.Levels != nil {
		levelsJSON, err := json.Marshal(updates.Levels)
		if err != nil {
			return fmt.Errorf("marshaling levels: %w", err)
		}
		q += fmt.Sprintf(", levels = $%d", argIdx)
		args = append(args, levelsJSON)
		argIdx++
	}

	q += fmt.Sprintf(" WHERE id = $%d", argIdx)
	args = append(args, id)

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("updating rubric: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("rubric not found")
	}
	return nil
}

func (r *postgresEvalRepo) DeleteRubric(ctx context.Context, id uuid.UUID, userID string) error {
	const q = `DELETE FROM rubrics WHERE id = $1 AND user_id = $2`
	res, err := r.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return fmt.Errorf("deleting rubric: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("rubric not found or unauthorized")
	}
	return nil
}
