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
