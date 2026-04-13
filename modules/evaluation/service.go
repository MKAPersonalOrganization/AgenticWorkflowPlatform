package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound      = fmt.Errorf("not found")
	ErrUnauthorized  = fmt.Errorf("unauthorized")
	ErrDuplicateName = fmt.Errorf("duplicate name")
	ErrValidation    = fmt.Errorf("validation error")
)

// service manages the evaluation business logic.
type service struct {
	repo repository
}

// newService creates a new evaluation service.
func newService(repo repository) *service {
	return &service{repo: repo}
}

// ─── EvalSet ─────────────────────────────────────────────────────────────────

func (s *service) createEvalSet(ctx context.Context, userID string, req EvalSetCreate) (EvalSet, error) {
	if err := validateEvalSetName(req.Name); err != nil {
		return EvalSet{}, fmt.Errorf("%w: %w", ErrValidation, err)
	}

	// Check uniqueness
	sets, err := s.repo.ListEvalSets(ctx, userID)
	if err != nil {
		return EvalSet{}, fmt.Errorf("checking name uniqueness: %w", err)
	}
	for _, es := range sets {
		if strings.EqualFold(es.Name, req.Name) {
			return EvalSet{}, fmt.Errorf("%w: an EvalSet named %q already exists", ErrDuplicateName, req.Name)
		}
	}

	es := EvalSet{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateEvalSet(ctx, es); err != nil {
		return EvalSet{}, fmt.Errorf("creating eval_set: %w", err)
	}
	return es, nil
}

func (s *service) getEvalSet(ctx context.Context, id uuid.UUID, userID string) (EvalSet, error) {
	es, err := s.repo.GetEvalSet(ctx, id)
	if err != nil {
		if err.Error() == "eval_set not found" {
			return EvalSet{}, ErrNotFound
		}
		return EvalSet{}, fmt.Errorf("getting eval_set: %w", err)
	}
	if es.UserID != userID {
		return EvalSet{}, ErrUnauthorized
	}
	return es, nil
}

func (s *service) listEvalSets(ctx context.Context, userID string) ([]EvalSet, error) {
	return s.repo.ListEvalSets(ctx, userID)
}

func (s *service) updateEvalSet(ctx context.Context, id uuid.UUID, userID string, req EvalSetUpdate) error {
	if req.Name != nil {
		if err := validateEvalSetName(*req.Name); err != nil {
			return fmt.Errorf("%w: %w", ErrValidation, err)
		}
	}

	// Verify ownership before update
	es, err := s.repo.GetEvalSet(ctx, id)
	if err != nil {
		if err.Error() == "eval_set not found" {
			return ErrNotFound
		}
		return fmt.Errorf("getting eval_set: %w", err)
	}
	if es.UserID != userID {
		return ErrUnauthorized
	}

	if err := s.repo.UpdateEvalSet(ctx, id, req); err != nil {
		return fmt.Errorf("updating eval_set: %w", err)
	}
	return nil
}

func (s *service) deleteEvalSet(ctx context.Context, id uuid.UUID, userID string) error {
	if err := s.repo.DeleteEvalSet(ctx, id, userID); err != nil {
		if err.Error() == "eval_set not found or unauthorized" {
			return ErrNotFound
		}
		return fmt.Errorf("deleting eval_set: %w", err)
	}
	return nil
}

// ─── EvalCase ────────────────────────────────────────────────────────────────

func (s *service) createEvalCase(ctx context.Context, userID string, req EvalCaseCreate) (EvalCase, error) {
	if strings.TrimSpace(req.Input) == "" {
		return EvalCase{}, fmt.Errorf("%w: input is required", ErrValidation)
	}
	if len(req.Input) > 10000 {
		return EvalCase{}, fmt.Errorf("%w: input must be at most 10000 characters", ErrValidation)
	}

	// Verify ownership of parent EvalSet
	es, err := s.repo.GetEvalSet(ctx, req.EvalSetID)
	if err != nil {
		if err.Error() == "eval_set not found" {
			return EvalCase{}, ErrNotFound
		}
		return EvalCase{}, fmt.Errorf("getting eval_set: %w", err)
	}
	if es.UserID != userID {
		return EvalCase{}, ErrUnauthorized
	}

	// Validate JSON fields if provided
	if req.ExpectedTrajectory != nil {
		if err := validateExpectedTrajectory(req.ExpectedTrajectory); err != nil {
			return EvalCase{}, fmt.Errorf("%w: %w", ErrValidation, err)
		}
	}
	if req.Rubrics != nil {
		if !json.Valid(req.Rubrics) {
			return EvalCase{}, fmt.Errorf("%w: rubrics must be a valid JSON array", ErrValidation)
		}
	}
	if req.UserMessages != nil {
		if !json.Valid(req.UserMessages) {
			return EvalCase{}, fmt.Errorf("%w: user_messages must be a valid JSON array", ErrValidation)
		}
	}

	ec := EvalCase{
		ID:                 uuid.New(),
		EvalSetID:          req.EvalSetID,
		Name:               req.Name,
		Input:              strings.TrimSpace(req.Input),
		ExpectedTrajectory: req.ExpectedTrajectory,
		ExpectedResponse:   req.ExpectedResponse,
		Rubrics:            req.Rubrics,
		UserMessages:       req.UserMessages,
		SortOrder:          req.SortOrder,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := s.repo.CreateEvalCase(ctx, ec); err != nil {
		return EvalCase{}, fmt.Errorf("creating eval_case: %w", err)
	}
	return ec, nil
}

func (s *service) listEvalCases(ctx context.Context, evalSetID uuid.UUID, userID string) ([]EvalCase, error) {
	// Verify ownership
	es, err := s.repo.GetEvalSet(ctx, evalSetID)
	if err != nil {
		if err.Error() == "eval_set not found" {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("getting eval_set: %w", err)
	}
	if es.UserID != userID {
		return nil, ErrUnauthorized
	}

	return s.repo.ListEvalCases(ctx, evalSetID)
}

func (s *service) updateEvalCase(ctx context.Context, id uuid.UUID, userID string, req EvalCaseUpdate) error {
	if req.Input != nil {
		if strings.TrimSpace(*req.Input) == "" {
			return fmt.Errorf("%w: input cannot be empty", ErrValidation)
		}
		if len(*req.Input) > 10000 {
			return fmt.Errorf("%w: input must be at most 10000 characters", ErrValidation)
		}
	}

	// Verify ownership via parent EvalSet
	existing, err := s.repo.GetEvalCase(ctx, id)
	if err != nil {
		if err.Error() == "eval_case not found" {
			return ErrNotFound
		}
		return fmt.Errorf("getting eval_case: %w", err)
	}
	es, err := s.repo.GetEvalSet(ctx, existing.EvalSetID)
	if err != nil {
		return fmt.Errorf("getting parent eval_set: %w", err)
	}
	if es.UserID != userID {
		return ErrUnauthorized
	}

	// Validate JSON fields if being updated
	if req.ExpectedTrajectory != nil {
		if err := validateExpectedTrajectory(req.ExpectedTrajectory); err != nil {
			return fmt.Errorf("%w: %w", ErrValidation, err)
		}
	}
	if req.Rubrics != nil {
		if !json.Valid(req.Rubrics) {
			return fmt.Errorf("%w: rubrics must be a valid JSON array", ErrValidation)
		}
	}
	if req.UserMessages != nil {
		if !json.Valid(req.UserMessages) {
			return fmt.Errorf("%w: user_messages must be a valid JSON array", ErrValidation)
		}
	}

	if err := s.repo.UpdateEvalCase(ctx, id, req); err != nil {
		return fmt.Errorf("updating eval_case: %w", err)
	}
	return nil
}

func (s *service) deleteEvalCase(ctx context.Context, id uuid.UUID, userID string) error {
	existing, err := s.repo.GetEvalCase(ctx, id)
	if err != nil {
		if err.Error() == "eval_case not found" {
			return ErrNotFound
		}
		return fmt.Errorf("getting eval_case: %w", err)
	}

	// Verify ownership via parent EvalSet
	es, err := s.repo.GetEvalSet(ctx, existing.EvalSetID)
	if err != nil {
		return fmt.Errorf("getting parent eval_set: %w", err)
	}
	if es.UserID != userID {
		return ErrUnauthorized
	}

	if err := s.repo.DeleteEvalCase(ctx, id); err != nil {
		if err.Error() == "eval_case not found" {
			return ErrNotFound
		}
		return fmt.Errorf("deleting eval_case: %w", err)
	}
	return nil
}

func (s *service) reorderEvalCases(ctx context.Context, evalSetID uuid.UUID, userID string, caseIDs []uuid.UUID) error {
	// Verify ownership
	es, err := s.repo.GetEvalSet(ctx, evalSetID)
	if err != nil {
		if err.Error() == "eval_set not found" {
			return ErrNotFound
		}
		return fmt.Errorf("getting eval_set: %w", err)
	}
	if es.UserID != userID {
		return ErrUnauthorized
	}

	if len(caseIDs) == 0 {
		return fmt.Errorf("%w: caseIDs cannot be empty", ErrValidation)
	}

	return s.repo.ReorderEvalCases(ctx, evalSetID, caseIDs)
}

// ─── Validation helpers ──────────────────────────────────────────────────────

func validateEvalSetName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("name is required")
	}
	if len(trimmed) > 255 {
		return fmt.Errorf("name must be at most 255 characters")
	}
	// Reject control characters
	for _, r := range trimmed {
		if r < 32 && r != '\t' {
			return fmt.Errorf("name contains invalid characters")
		}
	}
	return nil
}

func validateExpectedTrajectory(data json.RawMessage) error {
	if !json.Valid(data) {
		return fmt.Errorf("expected_trajectory must be valid JSON")
	}
	var calls []ExpectedToolCall
	if err := json.Unmarshal(data, &calls); err != nil {
		return fmt.Errorf("expected_trajectory must be a JSON array of objects with 'tool_name': %w", err)
	}
	for i, c := range calls {
		if strings.TrimSpace(c.ToolName) == "" {
			return fmt.Errorf("expected_trajectory[%d]: tool_name is required", i)
		}
	}
	return nil
}

// ─── Import / Export ─────────────────────────────────────────────────────────

// ImportEvalSet parses an ADK-compatible JSON EvalSet, validates it, and
// atomically inserts the EvalSet with all its EvalCases in a single transaction.
func (s *service) importEvalSet(ctx context.Context, userID string, data []byte) (EvalSet, error) {
	var export EvalSetExport
	if err := json.Unmarshal(data, &export); err != nil {
		return EvalSet{}, fmt.Errorf("%w: invalid JSON: %w", ErrValidation, err)
	}

	// Validate EvalSet
	if err := validateEvalSetName(export.Name); err != nil {
		return EvalSet{}, fmt.Errorf("%w: %w", ErrValidation, err)
	}

	// Check name uniqueness
	sets, err := s.repo.ListEvalSets(ctx, userID)
	if err != nil {
		return EvalSet{}, fmt.Errorf("checking name uniqueness: %w", err)
	}
	for _, es := range sets {
		if strings.EqualFold(es.Name, export.Name) {
			return EvalSet{}, fmt.Errorf("%w: an EvalSet named %q already exists", ErrDuplicateName, export.Name)
		}
	}

	if len(export.Cases) == 0 {
		return EvalSet{}, fmt.Errorf("%w: eval_cases cannot be empty", ErrValidation)
	}

	// Validate each EvalCase
	for i, ec := range export.Cases {
		if strings.TrimSpace(ec.Input) == "" {
			return EvalSet{}, fmt.Errorf("%w: eval_cases[%d]: input is required", ErrValidation, i)
		}
		if len(ec.Input) > 10000 {
			return EvalSet{}, fmt.Errorf("%w: eval_cases[%d]: input must be at most 10000 characters", ErrValidation, i)
		}
		// Validate trajectory if provided
		if ec.ExpectedTrajectory != nil {
			raw, err := json.Marshal(ec.ExpectedTrajectory)
			if err != nil {
				return EvalSet{}, fmt.Errorf("%w: eval_cases[%d]: invalid expected_trajectory: %w", ErrValidation, i, err)
			}
			if err := validateExpectedTrajectory(raw); err != nil {
				return EvalSet{}, fmt.Errorf("%w: eval_cases[%d]: %w", ErrValidation, i, err)
			}
		}
		// Validate rubrics if provided (must be valid JSON)
		if ec.Rubrics != nil {
			raw, err := json.Marshal(ec.Rubrics)
			if err != nil || !json.Valid(raw) {
				return EvalSet{}, fmt.Errorf("%w: eval_cases[%d]: rubrics must be valid JSON", ErrValidation, i)
			}
		}
		// Validate user_messages if provided
		if ec.UserMessages != nil {
			raw, err := json.Marshal(ec.UserMessages)
			if err != nil || !json.Valid(raw) {
				return EvalSet{}, fmt.Errorf("%w: eval_cases[%d]: user_messages must be valid JSON", ErrValidation, i)
			}
		}
	}

	// Build EvalSet and EvalCases for atomic insert
	evalSetID := uuid.New()
	es := EvalSet{
		ID:          evalSetID,
		UserID:      userID,
		Name:        strings.TrimSpace(export.Name),
		Description: export.Desc,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	cases := make([]EvalCase, 0, len(export.Cases))
	for i, ec := range export.Cases {
		var trajRaw, rubricsRaw, userMsgsRaw json.RawMessage
		if ec.ExpectedTrajectory != nil {
			trajRaw, _ = json.Marshal(ec.ExpectedTrajectory)
		}
		if ec.Rubrics != nil {
			rubricsRaw, _ = json.Marshal(ec.Rubrics)
		}
		if ec.UserMessages != nil {
			userMsgsRaw, _ = json.Marshal(ec.UserMessages)
		}

		cases = append(cases, EvalCase{
			ID:                 uuid.New(),
			EvalSetID:          evalSetID,
			Name:               ec.Name,
			Input:              strings.TrimSpace(ec.Input),
			ExpectedTrajectory: trajRaw,
			ExpectedResponse:   ec.ExpectedResponse,
			Rubrics:            rubricsRaw,
			UserMessages:       userMsgsRaw,
			SortOrder:          i,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		})
	}

	if err := s.repo.CreateEvalSetWithCases(ctx, es, cases); err != nil {
		return EvalSet{}, fmt.Errorf("importing eval_set: %w", err)
	}

	// Return the created EvalSet with case_count
	es.CaseCount = len(cases)
	return es, nil
}

// ExportEvalSet returns an EvalSet with all its EvalCases in the
// ADK-compatible JSON export format.
func (s *service) exportEvalSet(ctx context.Context, id uuid.UUID, userID string) (EvalSetExport, error) {
	es, cases, err := s.repo.ListEvalCasesWithSet(ctx, id)
	if err != nil {
		if err.Error() == "eval_set not found" {
			return EvalSetExport{}, ErrNotFound
		}
		return EvalSetExport{}, fmt.Errorf("getting eval_set: %w", err)
	}
	if es.UserID != userID {
		return EvalSetExport{}, ErrUnauthorized
	}

	export := EvalSetExport{
		Name:  es.Name,
		Desc:  es.Description,
		Cases: make([]EvalCaseExport, 0, len(cases)),
	}

	for _, ec := range cases {
		var traj, rubrics, msgs any
		if ec.ExpectedTrajectory != nil {
			_ = json.Unmarshal(ec.ExpectedTrajectory, &traj)
		}
		if ec.Rubrics != nil {
			_ = json.Unmarshal(ec.Rubrics, &rubrics)
		}
		if ec.UserMessages != nil {
			_ = json.Unmarshal(ec.UserMessages, &msgs)
		}

		export.Cases = append(export.Cases, EvalCaseExport{
			Name:               ec.Name,
			Input:              ec.Input,
			ExpectedTrajectory: traj,
			ExpectedResponse:   ec.ExpectedResponse,
			Rubrics:            rubrics,
			UserMessages:       msgs,
		})
	}

	return export, nil
}
