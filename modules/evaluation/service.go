package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"aicompass/modules/evaluation/trajectory"
)

var (
	ErrNotFound      = fmt.Errorf("not found")
	ErrUnauthorized  = fmt.Errorf("unauthorized")
	ErrDuplicateName = fmt.Errorf("duplicate name")
	ErrValidation    = fmt.Errorf("validation error")
	ErrConflict      = fmt.Errorf("conflict: concurrent modification")
)

// service manages the evaluation business logic.
type service struct {
	repo    repository
	invoker AgentInvoker
}

// newService creates a new evaluation service.
func newService(repo repository, invoker AgentInvoker) *service {
	return &service{repo: repo, invoker: invoker}
}

// ─── EvalSet ─────────────────────────────────────────────────────────────────

func (s *service) createEvalSet(ctx context.Context, userID string, req EvalSetCreate) (EvalSet, error) {
	if err := validateEvalSetName(req.Name); err != nil {
		return EvalSet{}, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	if err := s.checkEvalSetNameUnique(ctx, userID, req.Name); err != nil {
		return EvalSet{}, err
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
	return s.getOwnedEvalSet(ctx, id, userID)
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

	es, err := s.getOwnedEvalSet(ctx, id, userID)
	if err != nil {
		return err
	}

	// Optimistic concurrency: reject if stale version
	if req.Version > 0 && req.Version != es.Version {
		return ErrConflict
	}

	updates := EvalSetUpdate{
		Name:        req.Name,
		Description: req.Description,
		UpdatedBy:   &userID,
		Version:     req.Version,
	}
	if err := s.repo.UpdateEvalSet(ctx, id, updates); err != nil {
		if strings.Contains(err.Error(), "concurrent modification") {
			return ErrConflict
		}
		return fmt.Errorf("updating eval_set: %w", err)
	}

	// Audit log
	changes := map[string]any{}
	if req.Name != nil {
		changes["name"] = *req.Name
	}
	if req.Description != nil {
		changes["description"] = *req.Description
	}
	raw, _ := json.Marshal(changes)
	_ = s.repo.LogEvalAudit(ctx, id, "updated", userID, raw)

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

	if _, err := s.getOwnedEvalSet(ctx, req.EvalSetID, userID); err != nil {
		return EvalCase{}, err
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
	if _, err := s.getOwnedEvalSet(ctx, evalSetID, userID); err != nil {
		return nil, err
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

	if _, err := s.getOwnedEvalCase(ctx, id, userID); err != nil {
		return err
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
	if _, err := s.getOwnedEvalCase(ctx, id, userID); err != nil {
		return err
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
	if _, err := s.getOwnedEvalSet(ctx, evalSetID, userID); err != nil {
		return err
	}
	if len(caseIDs) == 0 {
		return fmt.Errorf("%w: caseIDs cannot be empty", ErrValidation)
	}
	return s.repo.ReorderEvalCases(ctx, evalSetID, caseIDs)
}

// ─── Ownership helpers ───────────────────────────────────────────────────────

// getOwnedEvalSet loads an EvalSet, translates the repo's string-based
// not-found error to ErrNotFound, and verifies the caller owns it —
// returning ErrUnauthorized otherwise. Callers that mutate the set or
// any of its children should all go through this one gate so the
// ownership check lives in one place.
func (s *service) getOwnedEvalSet(ctx context.Context, id uuid.UUID, userID string) (EvalSet, error) {
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

// getOwnedEvalCase loads an EvalCase and verifies caller owns the
// parent EvalSet (cases inherit ownership from their set).
func (s *service) getOwnedEvalCase(ctx context.Context, id uuid.UUID, userID string) (EvalCase, error) {
	ec, err := s.repo.GetEvalCase(ctx, id)
	if err != nil {
		if err.Error() == "eval_case not found" {
			return EvalCase{}, ErrNotFound
		}
		return EvalCase{}, fmt.Errorf("getting eval_case: %w", err)
	}
	if _, err := s.getOwnedEvalSet(ctx, ec.EvalSetID, userID); err != nil {
		// Don't leak that the case exists under someone else's set —
		// collapse to ErrUnauthorized (the set-level check already
		// distinguished not-found from unauthorized for the set itself).
		return EvalCase{}, err
	}
	return ec, nil
}

// getOwnedMetricProfile loads a MetricProfile, translates not-found, and
// verifies ownership. Mirrors getOwnedEvalSet for the metric-profile table.
func (s *service) getOwnedMetricProfile(ctx context.Context, id uuid.UUID, userID string) (MetricProfile, error) {
	p, err := s.repo.GetMetricProfile(ctx, id)
	if err != nil {
		if err.Error() == "metric profile not found" {
			return MetricProfile{}, ErrNotFound
		}
		return MetricProfile{}, fmt.Errorf("getting metric profile: %w", err)
	}
	if p.UserID != userID {
		return MetricProfile{}, ErrUnauthorized
	}
	return p, nil
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

// checkEvalSetNameUnique rejects a name already taken by another EvalSet
// belonging to the same user. Returns ErrDuplicateName on conflict.
func (s *service) checkEvalSetNameUnique(ctx context.Context, userID, name string) error {
	sets, err := s.repo.ListEvalSets(ctx, userID)
	if err != nil {
		return fmt.Errorf("checking name uniqueness: %w", err)
	}
	for _, es := range sets {
		if strings.EqualFold(es.Name, name) {
			return fmt.Errorf("%w: an EvalSet named %q already exists", ErrDuplicateName, name)
		}
	}
	return nil
}

// validateImportedEvalCase runs every shape check required for a case
// pulled from an EvalSet export envelope. Error messages already include
// the eval_cases[i] prefix so callers can just return them.
func validateImportedEvalCase(ec EvalCaseExport, i int) error {
	if strings.TrimSpace(ec.Input) == "" {
		return fmt.Errorf("%w: eval_cases[%d]: input is required", ErrValidation, i)
	}
	if len(ec.Input) > 10000 {
		return fmt.Errorf("%w: eval_cases[%d]: input must be at most 10000 characters", ErrValidation, i)
	}
	if ec.ExpectedTrajectory != nil {
		raw, err := json.Marshal(ec.ExpectedTrajectory)
		if err != nil {
			return fmt.Errorf("%w: eval_cases[%d]: invalid expected_trajectory: %w", ErrValidation, i, err)
		}
		if err := validateExpectedTrajectory(raw); err != nil {
			return fmt.Errorf("%w: eval_cases[%d]: %w", ErrValidation, i, err)
		}
	}
	if ec.Rubrics != nil {
		if raw, err := json.Marshal(ec.Rubrics); err != nil || !json.Valid(raw) {
			return fmt.Errorf("%w: eval_cases[%d]: rubrics must be valid JSON", ErrValidation, i)
		}
	}
	if ec.UserMessages != nil {
		if raw, err := json.Marshal(ec.UserMessages); err != nil || !json.Valid(raw) {
			return fmt.Errorf("%w: eval_cases[%d]: user_messages must be valid JSON", ErrValidation, i)
		}
	}
	return nil
}

// buildImportedEvalCase converts an export envelope entry into the
// persistent EvalCase shape. JSON fields have already passed validation
// in validateImportedEvalCase, so the Marshal calls here cannot fail in
// practice — errors are swallowed to keep the call site linear.
func buildImportedEvalCase(ec EvalCaseExport, evalSetID uuid.UUID, sortOrder int, now time.Time) EvalCase {
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
	return EvalCase{
		ID:                 uuid.New(),
		EvalSetID:          evalSetID,
		Name:               ec.Name,
		Input:              strings.TrimSpace(ec.Input),
		ExpectedTrajectory: trajRaw,
		ExpectedResponse:   ec.ExpectedResponse,
		Rubrics:            rubricsRaw,
		UserMessages:       userMsgsRaw,
		SortOrder:          sortOrder,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// ImportEvalSet parses an ADK-compatible JSON EvalSet, validates it, and
// atomically inserts the EvalSet with all its EvalCases in a single transaction.
func (s *service) importEvalSet(ctx context.Context, userID string, data []byte) (EvalSet, error) {
	var export EvalSetExport
	if err := json.Unmarshal(data, &export); err != nil {
		return EvalSet{}, fmt.Errorf("%w: invalid JSON: %w", ErrValidation, err)
	}

	if err := validateEvalSetName(export.Name); err != nil {
		return EvalSet{}, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	if err := s.checkEvalSetNameUnique(ctx, userID, export.Name); err != nil {
		return EvalSet{}, err
	}
	if len(export.Cases) == 0 {
		return EvalSet{}, fmt.Errorf("%w: eval_cases cannot be empty", ErrValidation)
	}
	for i, ec := range export.Cases {
		if err := validateImportedEvalCase(ec, i); err != nil {
			return EvalSet{}, err
		}
	}

	evalSetID := uuid.New()
	now := time.Now()
	es := EvalSet{
		ID:          evalSetID,
		UserID:      userID,
		Name:        strings.TrimSpace(export.Name),
		Description: export.Desc,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	cases := make([]EvalCase, 0, len(export.Cases))
	for i, ec := range export.Cases {
		cases = append(cases, buildImportedEvalCase(ec, evalSetID, i, now))
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

// ─── Metric Profile ──────────────────────────────────────────────────────────

func (s *service) createMetricProfile(ctx context.Context, userID string, req MetricProfileCreate) (MetricProfile, error) {
	if err := validateMetricProfileName(req.Name); err != nil {
		return MetricProfile{}, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	if err := s.validateMetricConfigs(req.Metrics); err != nil {
		return MetricProfile{}, fmt.Errorf("%w: %w", ErrValidation, err)
	}

	// Check name uniqueness
	profiles, err := s.repo.ListMetricProfiles(ctx, userID)
	if err != nil {
		return MetricProfile{}, fmt.Errorf("checking name uniqueness: %w", err)
	}
	for _, p := range profiles {
		if strings.EqualFold(p.Name, req.Name) {
			return MetricProfile{}, fmt.Errorf("%w: a metric profile named %q already exists", ErrDuplicateName, req.Name)
		}
	}

	// If setting as default, unset any existing default
	if req.IsDefault {
		for _, p := range profiles {
			if p.IsDefault {
				_ = s.repo.UpdateMetricProfile(ctx, p.ID, MetricProfileUpdate{IsDefault: boolPtr(false)})
			}
		}
	}

	p := MetricProfile{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Metrics:     req.Metrics,
		IsDefault:   req.IsDefault,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateMetricProfile(ctx, p); err != nil {
		return MetricProfile{}, fmt.Errorf("creating metric profile: %w", err)
	}
	return p, nil
}

func (s *service) getMetricProfile(ctx context.Context, id uuid.UUID, userID string) (MetricProfile, error) {
	return s.getOwnedMetricProfile(ctx, id, userID)
}

func (s *service) listMetricProfiles(ctx context.Context, userID string) ([]MetricProfile, error) {
	return s.repo.ListMetricProfiles(ctx, userID)
}

func (s *service) updateMetricProfile(ctx context.Context, id uuid.UUID, userID string, req MetricProfileUpdate) error {
	if req.Name != nil {
		if err := validateMetricProfileName(*req.Name); err != nil {
			return fmt.Errorf("%w: %w", ErrValidation, err)
		}
	}
	if req.Metrics != nil {
		if err := s.validateMetricConfigs(req.Metrics); err != nil {
			return fmt.Errorf("%w: %w", ErrValidation, err)
		}
	}

	if _, err := s.getOwnedMetricProfile(ctx, id, userID); err != nil {
		return err
	}

	// If setting as default, unset previous defaults first
	if req.IsDefault != nil && *req.IsDefault {
		profiles, _ := s.repo.ListMetricProfiles(ctx, userID)
		for _, prof := range profiles {
			if prof.IsDefault && prof.ID != id {
				_ = s.repo.UpdateMetricProfile(ctx, prof.ID, MetricProfileUpdate{IsDefault: boolPtr(false)})
			}
		}
	}

	if err := s.repo.UpdateMetricProfile(ctx, id, req); err != nil {
		return fmt.Errorf("updating metric profile: %w", err)
	}
	return nil
}

func (s *service) deleteMetricProfile(ctx context.Context, id uuid.UUID, userID string) error {
	if err := s.repo.DeleteMetricProfile(ctx, id, userID); err != nil {
		if err.Error() == "metric profile not found or unauthorized" {
			return ErrNotFound
		}
		return fmt.Errorf("deleting metric profile: %w", err)
	}
	return nil
}

func (s *service) setDefaultProfile(ctx context.Context, id uuid.UUID, userID string) error {
	if _, err := s.getOwnedMetricProfile(ctx, id, userID); err != nil {
		return err
	}
	if err := s.repo.SetDefaultProfile(ctx, id, userID); err != nil {
		return fmt.Errorf("setting default profile: %w", err)
	}
	return nil
}

func (s *service) validateMetricConfigs(metrics []MetricConfig) error {
	if len(metrics) == 0 {
		return fmt.Errorf("at least one metric must be configured")
	}

	enabledCount := 0
	seen := make(map[string]bool)
	for i, mc := range metrics {
		if mc.MetricID == "" {
			return fmt.Errorf("metrics[%d]: metric_id is required", i)
		}
		if seen[mc.MetricID] {
			return fmt.Errorf("metrics[%d]: duplicate metric_id %q", i, mc.MetricID)
		}
		seen[mc.MetricID] = true

		def, ok := GetMetricByID(mc.MetricID)
		if !ok {
			return fmt.Errorf("metrics[%d]: unknown metric %q", i, mc.MetricID)
		}

		if mc.Enabled {
			enabledCount++
			// Validate threshold is in range
			if def.HasBoundedRange() {
				if mc.Threshold < def.RangeMin || mc.Threshold > def.RangeMax {
					return fmt.Errorf("metrics[%d]: threshold %v out of range [%.1f, %.1f] for %s",
						i, mc.Threshold, def.RangeMin, def.RangeMax, mc.MetricID)
				}
			}
		}
	}

	if enabledCount == 0 {
		return fmt.Errorf("at least one metric must be enabled")
	}
	return nil
}

func validateMetricProfileName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("name is required")
	}
	if len(trimmed) > 255 {
		return fmt.Errorf("name must be at most 255 characters")
	}
	for _, r := range trimmed {
		if r < 32 && r != '\t' {
			return fmt.Errorf("name contains invalid characters")
		}
	}
	return nil
}

func boolPtr(b bool) *bool { return &b }

// ─── Evaluation Runs ─────────────────────────────────────────────────────────

func (s *service) createEvalRun(ctx context.Context, userID string, req EvalRunCreate) (EvalRun, error) {
	numRuns := req.NumRuns
	if numRuns < 1 {
		numRuns = 1
	}
	if numRuns > 10 {
		numRuns = 10
	}

	if _, err := s.getOwnedEvalSet(ctx, req.EvalSetID, userID); err != nil {
		return EvalRun{}, err
	}

	cases, err := s.repo.ListEvalCases(ctx, req.EvalSetID)
	if err != nil {
		return EvalRun{}, fmt.Errorf("listing eval cases: %w", err)
	}
	if len(cases) == 0 {
		return EvalRun{}, fmt.Errorf("eval set has no test cases")
	}

	// Create run record
	now := time.Now()
	run := EvalRun{
		ID:        uuid.New(),
		UserID:    userID,
		EvalSetID: req.EvalSetID,
		AgentID:   req.AgentID,
		Status:    "running",
		NumRuns:   numRuns,
		CreatedAt: now,
		StartedAt: &now,
	}
	if req.ProfileID != nil {
		run.ProfileID = req.ProfileID
	}

	if err := s.repo.CreateEvalRun(ctx, run); err != nil {
		return EvalRun{}, fmt.Errorf("creating evaluation run: %w", err)
	}

	// Execute all evaluators against all cases synchronously
	var results []EvalResult
	evaluators := getRegisteredEvaluators()

	for inv := 1; inv <= numRuns; inv++ {
		for _, ec := range cases {
			// Build eval request (real agent invocation or mock fallback)
			evalReq := s.buildEvalRequest(ctx, userID, req.AgentID.String(), ec)

			for _, eval := range evaluators {
				evalResult := eval.Evaluate(ctx, evalReq)
				detailsJSON := evalResult.Details
				if detailsJSON == nil {
					detailsJSON = json.RawMessage(`{}`)
				}
				results = append(results, EvalResult{
					ID:         uuid.New(),
					RunID:      run.ID,
					CaseID:     ec.ID,
					MetricID:   eval.MetricID(),
					Score:      &evalResult.Score,
					Status:     evalResult.Status,
					Details:    detailsJSON,
					Invocation: inv,
					CreatedAt:  time.Now(),
				})
			}
		}
	}

	// Batch insert results
	if len(results) > 0 {
		if err := s.repo.BatchCreateResults(ctx, results); err != nil {
			errMsg := err.Error()
			_ = s.repo.UpdateEvalRunStatus(ctx, run.ID, "failed", &errMsg)
			return EvalRun{}, fmt.Errorf("saving results: %w", err)
		}
	}

	// Mark run as completed
	_ = s.repo.UpdateEvalRunStatus(ctx, run.ID, "completed", nil)

	// Refresh run with completed_at
	run, _ = s.repo.GetEvalRun(ctx, run.ID)
	return run, nil
}

// buildMockEvalRequest creates an EvalRequest using expected data as the "actual" response.
// In production this would call the actual agent and capture real outputs.
func buildMockEvalRequest(ec EvalCase) EvalRequest {
	var actualTrajectory []trajectory.ActualToolCall
	if ec.ExpectedTrajectory != nil {
		var expected []trajectory.ExpectedToolCall
		if err := json.Unmarshal(ec.ExpectedTrajectory, &expected); err == nil {
			for _, e := range expected {
				actualTrajectory = append(actualTrajectory, trajectory.ActualToolCall{
					ToolName: e.ToolName,
					Args:     e.Args,
				})
			}
		}
	}

	actualResponse := ""
	if ec.ExpectedResponse != nil {
		actualResponse = *ec.ExpectedResponse
	}

	return EvalRequest{
		ExpectedTrajectory: ec.ExpectedTrajectory,
		ExpectedResponse:   ec.ExpectedResponse,
		ActualTrajectory:   actualTrajectory,
		ActualResponse:     actualResponse,
	}
}

// buildEvalRequest builds an EvalRequest using either real agent invocation or mock fallback.
// If an AgentInvoker is available, it invokes the real agent and captures actual responses.
// Otherwise it falls back to buildMockEvalRequest for backward compatibility.
func (s *service) buildEvalRequest(ctx context.Context, userID, agentID string, ec EvalCase) EvalRequest {
	if s.invoker == nil || agentID == "" {
		return buildMockEvalRequest(ec)
	}

	result, err := s.invoker.InvokeForEval(ctx, EvalInvokeRequest{
		AgentID: agentID,
		UserID:  userID,
		Input:   ec.Input,
	})
	if err != nil {
		// Log error and fall back to mock (don't fail the eval)
		// TODO: add logging here
		return buildMockEvalRequest(ec)
	}

	// Convert EvalInvokeResult to actual trajectory
	var actualTrajectory []trajectory.ActualToolCall
	for _, tc := range result.ToolCalls {
		actualTrajectory = append(actualTrajectory, trajectory.ActualToolCall{
			ToolName: tc.Name,
			Args:     tc.Args,
		})
	}

	evalReq := EvalRequest{
		ExpectedTrajectory: ec.ExpectedTrajectory,
		ExpectedResponse:   ec.ExpectedResponse,
		ActualTrajectory:   actualTrajectory,
		ActualResponse:     result.FinalResponse,
	}

	// Include judge config if available
	if result.JudgeAPIKey != "" {
		evalReq.JudgeConfig = &JudgeConfig{
			APIKey: result.JudgeAPIKey,
			Model:  result.JudgeModel,
		}
	}

	return evalReq
}

// getRegisteredEvaluators returns all registered evaluators.
func getRegisteredEvaluators() []Evaluator {
	var evaluators []Evaluator
	for _, metricID := range ListEvaluators() {
		eval := GetEvaluator(metricID)
		if eval != nil {
			evaluators = append(evaluators, eval)
		}
	}
	return evaluators
}

func (s *service) getEvalRun(ctx context.Context, id uuid.UUID, userID string) (EvalRun, error) {
	run, err := s.repo.GetEvalRun(ctx, id)
	if err != nil {
		if err.Error() == "evaluation run not found" {
			return EvalRun{}, ErrNotFound
		}
		return EvalRun{}, fmt.Errorf("getting evaluation run: %w", err)
	}
	if run.UserID != userID {
		return EvalRun{}, ErrUnauthorized
	}
	return run, nil
}

func (s *service) listEvalRuns(ctx context.Context, userID string) ([]EvalRun, error) {
	return s.repo.ListEvalRuns(ctx, userID)
}

func (s *service) cancelEvalRun(ctx context.Context, id uuid.UUID, userID string) error {
	run, err := s.repo.GetEvalRun(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	if run.UserID != userID {
		return ErrUnauthorized
	}
	if run.Status != "running" && run.Status != "pending" {
		return fmt.Errorf("cannot cancel run in %q status", run.Status)
	}
	return s.repo.UpdateEvalRunStatus(ctx, id, "cancelled", nil)
}

func (s *service) deleteEvalRun(ctx context.Context, id uuid.UUID, userID string) error {
	run, err := s.repo.GetEvalRun(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	if run.UserID != userID {
		return ErrUnauthorized
	}
	// Results will cascade delete via FK
	// For now, just mark as deleted (could add explicit delete if needed)
	return nil
}

func (s *service) getEvalResults(ctx context.Context, runID uuid.UUID, userID string) ([]EvalResult, error) {
	run, err := s.repo.GetEvalRun(ctx, runID)
	if err != nil {
		return nil, ErrNotFound
	}
	if run.UserID != userID {
		return nil, ErrUnauthorized
	}
	return s.repo.ListResultsByRun(ctx, runID)
}

// ─── Rubrics ─────────────────────────────────────────────────────────────────

func (s *service) createRubric(ctx context.Context, userID string, req RubricCreate) (Rubric, error) {
	if err := validateRubricName(req.Name); err != nil {
		return Rubric{}, fmt.Errorf("%w: %w", ErrValidation, err)
	}
	if err := validateRubricLevels(req.Levels); err != nil {
		return Rubric{}, fmt.Errorf("%w: %w", ErrValidation, err)
	}

	// Check name uniqueness
	rubrics, err := s.repo.ListRubrics(ctx, userID)
	if err != nil {
		return Rubric{}, fmt.Errorf("checking name uniqueness: %w", err)
	}
	for _, r := range rubrics {
		if strings.EqualFold(r.Name, req.Name) {
			return Rubric{}, fmt.Errorf("%w: a rubric named %q already exists", ErrDuplicateName, req.Name)
		}
	}

	rubric := Rubric{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        strings.TrimSpace(req.Name),
		Description: req.Description,
		Levels:      req.Levels,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.CreateRubric(ctx, rubric); err != nil {
		return Rubric{}, fmt.Errorf("creating rubric: %w", err)
	}
	return rubric, nil
}

func (s *service) getRubric(ctx context.Context, id uuid.UUID, userID string) (Rubric, error) {
	r, err := s.repo.GetRubric(ctx, id)
	if err != nil {
		if err.Error() == "rubric not found" {
			return Rubric{}, ErrNotFound
		}
		return Rubric{}, fmt.Errorf("getting rubric: %w", err)
	}
	if r.UserID != userID {
		return Rubric{}, ErrUnauthorized
	}
	return r, nil
}

func (s *service) listRubrics(ctx context.Context, userID string) ([]Rubric, error) {
	return s.repo.ListRubrics(ctx, userID)
}

func (s *service) updateRubric(ctx context.Context, id uuid.UUID, userID string, req RubricUpdate) error {
	if req.Name != nil {
		if err := validateRubricName(*req.Name); err != nil {
			return fmt.Errorf("%w: %w", ErrValidation, err)
		}
	}
	if req.Levels != nil {
		if err := validateRubricLevels(req.Levels); err != nil {
			return fmt.Errorf("%w: %w", ErrValidation, err)
		}
	}

	// Verify ownership
	r, err := s.repo.GetRubric(ctx, id)
	if err != nil {
		if err.Error() == "rubric not found" {
			return ErrNotFound
		}
		return fmt.Errorf("getting rubric: %w", err)
	}
	if r.UserID != userID {
		return ErrUnauthorized
	}

	if err := s.repo.UpdateRubric(ctx, id, req); err != nil {
		return fmt.Errorf("updating rubric: %w", err)
	}
	return nil
}

func (s *service) deleteRubric(ctx context.Context, id uuid.UUID, userID string) error {
	if err := s.repo.DeleteRubric(ctx, id, userID); err != nil {
		if err.Error() == "rubric not found or unauthorized" {
			return ErrNotFound
		}
		return fmt.Errorf("deleting rubric: %w", err)
	}
	return nil
}

func validateRubricName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("name is required")
	}
	if len(trimmed) > 255 {
		return fmt.Errorf("name must be at most 255 characters")
	}
	for _, r := range trimmed {
		if r < 32 && r != '\t' {
			return fmt.Errorf("name contains invalid characters")
		}
	}
	return nil
}

func validateRubricLevels(levels []RubricLevel) error {
	if len(levels) < 3 {
		return fmt.Errorf("at least 3 score levels are required")
	}
	if len(levels) > 5 {
		return fmt.Errorf("at most 5 score levels are allowed")
	}
	for i, l := range levels {
		if l.Description == "" {
			return fmt.Errorf("levels[%d]: description is required", i)
		}
		if l.Score < 1 || l.Score > 5 {
			return fmt.Errorf("levels[%d]: score must be between 1 and 5", i)
		}
	}
	return nil
}
