package evaluation

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EvalSet is a named collection of evaluation test cases.
type EvalSet struct {
	ID          uuid.UUID `json:"id" db:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description,omitempty" db:"description"`
	Version     int       `json:"version" db:"version"`
	CreatedBy   *string   `json:"created_by,omitempty" db:"created_by"`
	UpdatedBy   *string   `json:"updated_by,omitempty" db:"updated_by"`
	CaseCount   int       `json:"case_count,omitempty" db:"case_count"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// EvalSetCreate holds the fields needed to create a new EvalSet.
type EvalSetCreate struct {
	UserID      string
	Name        string
	Description *string
}

// EvalSetUpdate holds the mutable fields for an EvalSet.
type EvalSetUpdate struct {
	Name        *string
	Description *string
	UpdatedBy   *string
	Version     int
}

// EvalCase is an individual test case within an EvalSet.
type EvalCase struct {
	ID                 uuid.UUID       `json:"id" db:"id"`
	EvalSetID          uuid.UUID       `json:"eval_set_id" db:"eval_set_id"`
	Name               *string         `json:"name,omitempty" db:"name"`
	Input              string          `json:"input" db:"input"`
	ExpectedTrajectory json.RawMessage `json:"expected_trajectory,omitempty" db:"expected_trajectory"`
	ExpectedResponse   *string         `json:"expected_response,omitempty" db:"expected_response"`
	Rubrics            json.RawMessage `json:"rubrics,omitempty" db:"rubrics"`
	UserMessages       json.RawMessage `json:"user_messages,omitempty" db:"user_messages"`
	SortOrder          int             `json:"sort_order" db:"sort_order"`
	CreatedAt          time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at" db:"updated_at"`
}

// EvalCaseCreate holds the fields needed to create a new EvalCase.
type EvalCaseCreate struct {
	EvalSetID          uuid.UUID
	Name               *string
	Input              string
	ExpectedTrajectory json.RawMessage
	ExpectedResponse   *string
	Rubrics            json.RawMessage
	UserMessages       json.RawMessage
	SortOrder          int
}

// EvalCaseUpdate holds the mutable fields for an EvalCase.
type EvalCaseUpdate struct {
	Name               *string
	Input              *string
	ExpectedTrajectory json.RawMessage
	ExpectedResponse   *string
	Rubrics            json.RawMessage
	UserMessages       json.RawMessage
}

// ExpectedToolCall represents a single expected tool invocation in a trajectory.
type ExpectedToolCall struct {
	ToolName  string         `json:"tool_name"`
	Args      map[string]any `json:"args,omitempty"`
	MatchType string         `json:"match_type,omitempty"` // EXACT, IN_ORDER, ANY_ORDER
}

// ─── Import / Export types (ADK-compatible JSON format) ──────────────────────

// EvalSetExport represents the JSON format used for import/export,
// matching the ADK Python eval_set.py schema.
type EvalSetExport struct {
	EvalSetID string           `json:"eval_set_id,omitempty"`
	Name      string           `json:"name"`
	Desc      *string          `json:"description,omitempty"`
	Cases     []EvalCaseExport `json:"eval_cases"`
}

// EvalCaseExport represents a single test case in the import/export JSON format.
type EvalCaseExport struct {
	Name               *string `json:"name,omitempty"`
	Input              string  `json:"input"`
	ExpectedTrajectory any     `json:"expected_trajectory,omitempty"`
	ExpectedResponse   *string `json:"expected_response,omitempty"`
	Rubrics            any     `json:"rubrics,omitempty"`
	UserMessages       any     `json:"user_messages,omitempty"`
}

// ─── Metric Profile types ────────────────────────────────────────────────────

// MetricProfile is a named configuration of evaluation metrics with thresholds.
type MetricProfile struct {
	ID          uuid.UUID      `json:"id" db:"id"`
	UserID      string         `json:"user_id" db:"user_id"`
	Name        string         `json:"name" db:"name"`
	Description *string        `json:"description,omitempty" db:"description"`
	Metrics     []MetricConfig `json:"metrics" db:"metrics"`
	IsDefault   bool           `json:"is_default" db:"is_default"`
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" db:"updated_at"`
}

// MetricConfig configures a single metric within a profile.
type MetricConfig struct {
	MetricID  string  `json:"metric_id"`
	Enabled   bool    `json:"enabled"`
	Threshold float64 `json:"threshold"`
}

// MetricProfileCreate holds fields for creating a new metric profile.
type MetricProfileCreate struct {
	UserID      string
	Name        string
	Description *string
	Metrics     []MetricConfig
	IsDefault   bool
}

// MetricProfileUpdate holds mutable fields for a metric profile.
type MetricProfileUpdate struct {
	Name        *string
	Description *string
	Metrics     []MetricConfig
	IsDefault   *bool
}

// ─── Evaluation Run types ────────────────────────────────────────────────────

// EvalRun represents a single evaluation execution.
type EvalRun struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	UserID      string     `json:"user_id" db:"user_id"`
	EvalSetID   uuid.UUID  `json:"eval_set_id" db:"eval_set_id"`
	AgentID     uuid.UUID  `json:"agent_id" db:"agent_id"`
	ProfileID   *uuid.UUID `json:"profile_id,omitempty" db:"profile_id"`
	Status      string     `json:"status" db:"status"`
	NumRuns     int        `json:"num_runs" db:"num_runs"`
	StartedAt   *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	ErrorMsg    *string    `json:"error_msg,omitempty" db:"error_msg"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// EvalRunCreate holds fields for starting a new evaluation run.
type EvalRunCreate struct {
	UserID    string     `json:"-"`
	EvalSetID uuid.UUID  `json:"eval_set_id"`
	AgentID   uuid.UUID  `json:"agent_id"`
	ProfileID *uuid.UUID `json:"profile_id,omitempty"`
	NumRuns   int        `json:"num_runs"`
}

// EvalResult is a single metric score for one case/invocation.
type EvalResult struct {
	ID         uuid.UUID       `json:"id" db:"id"`
	RunID      uuid.UUID       `json:"run_id" db:"run_id"`
	CaseID     uuid.UUID       `json:"case_id" db:"case_id"`
	MetricID   string          `json:"metric_id" db:"metric_id"`
	Score      *float64        `json:"score,omitempty" db:"score"`
	Status     string          `json:"status" db:"status"`
	Details    json.RawMessage `json:"details,omitempty" db:"details"`
	Invocation int             `json:"invocation" db:"invocation"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
}

// ─── Rubric types ────────────────────────────────────────────────────────────

// RubricLevel defines a single score level in a rubric.
type RubricLevel struct {
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

// Rubric is a named scoring guide with levels.
type Rubric struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	UserID      string          `json:"user_id" db:"user_id"`
	Name        string          `json:"name" db:"name"`
	Description *string         `json:"description,omitempty" db:"description"`
	Levels      []RubricLevel   `json:"levels" db:"-"`
	LevelsJSON  json.RawMessage `json:"-" db:"levels"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// RubricCreate holds fields for creating a new rubric.
type RubricCreate struct {
	UserID      string
	Name        string
	Description *string
	Levels      []RubricLevel
}

// RubricUpdate holds mutable fields for a rubric.
type RubricUpdate struct {
	Name        *string
	Description *string
	Levels      []RubricLevel
}
