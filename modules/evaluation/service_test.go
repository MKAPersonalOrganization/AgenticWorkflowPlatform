package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"
)

// ─── Mock repository ────────────────────────────────────────────────────────

type mockRepo struct {
	sets      map[uuid.UUID]EvalSet
	cases     map[uuid.UUID]EvalCase
	evalCases []EvalCase // for ListEvalCases
	profiles  map[uuid.UUID]MetricProfile
	runs      map[uuid.UUID]EvalRun
	results   map[uuid.UUID]EvalResult
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		sets:     make(map[uuid.UUID]EvalSet),
		cases:    make(map[uuid.UUID]EvalCase),
		profiles: make(map[uuid.UUID]MetricProfile),
		runs:     make(map[uuid.UUID]EvalRun),
		results:  make(map[uuid.UUID]EvalResult),
	}
}

func (m *mockRepo) CreateEvalSet(_ context.Context, es EvalSet) error {
	es.Version = 1 // match DB default
	m.sets[es.ID] = es
	return nil
}

func (m *mockRepo) GetEvalSet(_ context.Context, id uuid.UUID) (EvalSet, error) {
	es, ok := m.sets[id]
	if !ok {
		return EvalSet{}, fmt.Errorf("eval_set not found")
	}
	return es, nil
}

func (m *mockRepo) ListEvalSets(_ context.Context, userID string) ([]EvalSet, error) {
	var result []EvalSet
	for _, es := range m.sets {
		if es.UserID == userID {
			result = append(result, es)
		}
	}
	return result, nil
}

func (m *mockRepo) UpdateEvalSet(_ context.Context, id uuid.UUID, updates EvalSetUpdate) error {
	es, ok := m.sets[id]
	if !ok {
		return fmt.Errorf("eval_set not found")
	}
	// Optimistic concurrency check
	if updates.Version > 0 && updates.Version != es.Version {
		return fmt.Errorf("eval_set not found or concurrent modification")
	}
	if updates.Name != nil {
		es.Name = *updates.Name
	}
	if updates.Description != nil {
		es.Description = updates.Description
	}
	if updates.UpdatedBy != nil {
		es.UpdatedBy = updates.UpdatedBy
	}
	es.Version++
	m.sets[id] = es
	return nil
}

func (m *mockRepo) DeleteEvalSet(_ context.Context, id uuid.UUID, userID string) error {
	es, ok := m.sets[id]
	if !ok || es.UserID != userID {
		return fmt.Errorf("eval_set not found or unauthorized")
	}
	delete(m.sets, id)
	// Cascade delete cases
	for cid, ec := range m.cases {
		if ec.EvalSetID == id {
			delete(m.cases, cid)
		}
	}
	return nil
}

func (m *mockRepo) CreateEvalCase(_ context.Context, ec EvalCase) error {
	m.cases[ec.ID] = ec
	m.evalCases = append(m.evalCases, ec)
	return nil
}

func (m *mockRepo) GetEvalCase(_ context.Context, id uuid.UUID) (EvalCase, error) {
	ec, ok := m.cases[id]
	if !ok {
		return EvalCase{}, fmt.Errorf("eval_case not found")
	}
	return ec, nil
}

func (m *mockRepo) ListEvalCases(_ context.Context, evalSetID uuid.UUID) ([]EvalCase, error) {
	var result []EvalCase
	for _, ec := range m.evalCases {
		if ec.EvalSetID == evalSetID {
			result = append(result, ec)
		}
	}
	return result, nil
}

func (m *mockRepo) UpdateEvalCase(_ context.Context, id uuid.UUID, updates EvalCaseUpdate) error {
	ec, ok := m.cases[id]
	if !ok {
		return fmt.Errorf("eval_case not found")
	}
	if updates.Name != nil {
		ec.Name = updates.Name
	}
	if updates.Input != nil {
		ec.Input = *updates.Input
	}
	if updates.ExpectedTrajectory != nil {
		ec.ExpectedTrajectory = updates.ExpectedTrajectory
	}
	if updates.ExpectedResponse != nil {
		ec.ExpectedResponse = updates.ExpectedResponse
	}
	if updates.Rubrics != nil {
		ec.Rubrics = updates.Rubrics
	}
	if updates.UserMessages != nil {
		ec.UserMessages = updates.UserMessages
	}
	m.cases[id] = ec
	return nil
}

func (m *mockRepo) DeleteEvalCase(_ context.Context, id uuid.UUID) error {
	_, ok := m.cases[id]
	if !ok {
		return fmt.Errorf("eval_case not found")
	}
	delete(m.cases, id)
	return nil
}

func (m *mockRepo) ReorderEvalCases(_ context.Context, evalSetID uuid.UUID, caseIDs []uuid.UUID) error {
	// Update sort order in mock
	for i, caseID := range caseIDs {
		ec, ok := m.cases[caseID]
		if !ok || ec.EvalSetID != evalSetID {
			return fmt.Errorf("eval_case %s not found in eval_set %s", caseID, evalSetID)
		}
		ec.SortOrder = i
		m.cases[caseID] = ec
	}
	return nil
}

func (m *mockRepo) CreateEvalSetWithCases(_ context.Context, es EvalSet, cases []EvalCase) error {
	m.sets[es.ID] = es
	for _, ec := range cases {
		m.cases[ec.ID] = ec
		m.evalCases = append(m.evalCases, ec)
	}
	return nil
}

func (m *mockRepo) ListEvalCasesWithSet(_ context.Context, evalSetID uuid.UUID) (EvalSet, []EvalCase, error) {
	es, ok := m.sets[evalSetID]
	if !ok {
		return EvalSet{}, nil, fmt.Errorf("eval_set not found")
	}
	var result []EvalCase
	for _, ec := range m.evalCases {
		if ec.EvalSetID == evalSetID {
			result = append(result, ec)
		}
	}
	return es, result, nil
}

func (m *mockRepo) LogEvalAudit(_ context.Context, _ uuid.UUID, _, _ string, _ json.RawMessage) error {
	return nil
}

func (m *mockRepo) CreateMetricProfile(_ context.Context, p MetricProfile) error {
	m.profiles[p.ID] = p
	return nil
}

func (m *mockRepo) GetMetricProfile(_ context.Context, id uuid.UUID) (MetricProfile, error) {
	p, ok := m.profiles[id]
	if !ok {
		return MetricProfile{}, fmt.Errorf("metric profile not found")
	}
	return p, nil
}

func (m *mockRepo) ListMetricProfiles(_ context.Context, userID string) ([]MetricProfile, error) {
	var result []MetricProfile
	for _, p := range m.profiles {
		if p.UserID == userID {
			result = append(result, p)
		}
	}
	// Sort: default first, then by name
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDefault != result[j].IsDefault {
			return result[i].IsDefault
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func (m *mockRepo) UpdateMetricProfile(_ context.Context, id uuid.UUID, updates MetricProfileUpdate) error {
	p, ok := m.profiles[id]
	if !ok {
		return fmt.Errorf("metric profile not found")
	}
	if updates.Name != nil {
		p.Name = *updates.Name
	}
	if updates.Description != nil {
		p.Description = updates.Description
	}
	if updates.Metrics != nil {
		p.Metrics = updates.Metrics
	}
	if updates.IsDefault != nil {
		p.IsDefault = *updates.IsDefault
	}
	m.profiles[id] = p
	return nil
}

func (m *mockRepo) DeleteMetricProfile(_ context.Context, id uuid.UUID, userID string) error {
	p, ok := m.profiles[id]
	if !ok || p.UserID != userID {
		return fmt.Errorf("metric profile not found or unauthorized")
	}
	delete(m.profiles, id)
	return nil
}

func (m *mockRepo) SetDefaultProfile(_ context.Context, id uuid.UUID, userID string) error {
	p, ok := m.profiles[id]
	if !ok || p.UserID != userID {
		return fmt.Errorf("metric profile not found or unauthorized")
	}
	// Unset other defaults for this user
	for pid, prof := range m.profiles {
		if prof.UserID == userID && prof.IsDefault && pid != id {
			prof.IsDefault = false
			m.profiles[pid] = prof
		}
	}
	p.IsDefault = true
	m.profiles[id] = p
	return nil
}

func (m *mockRepo) CreateEvalRun(_ context.Context, run EvalRun) error {
	m.runs[run.ID] = run
	return nil
}

func (m *mockRepo) GetEvalRun(_ context.Context, id uuid.UUID) (EvalRun, error) {
	run, ok := m.runs[id]
	if !ok {
		return EvalRun{}, fmt.Errorf("evaluation run not found")
	}
	return run, nil
}

func (m *mockRepo) ListEvalRuns(_ context.Context, userID string) ([]EvalRun, error) {
	var result []EvalRun
	for _, r := range m.runs {
		if r.UserID == userID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockRepo) UpdateEvalRunStatus(_ context.Context, id uuid.UUID, status string, _ *string) error {
	run, ok := m.runs[id]
	if !ok {
		return fmt.Errorf("evaluation run not found")
	}
	run.Status = status
	m.runs[id] = run
	return nil
}

func (m *mockRepo) BatchCreateResults(_ context.Context, results []EvalResult) error {
	for _, r := range results {
		m.results[r.ID] = r
	}
	return nil
}

func (m *mockRepo) ListResultsByRun(_ context.Context, runID uuid.UUID) ([]EvalResult, error) {
	var result []EvalResult
	for _, r := range m.results {
		if r.RunID == runID {
			result = append(result, r)
		}
	}
	return result, nil
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestCreateEvalSet_ValidName(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, err := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID:      "user1",
		Name:        "My Test Suite",
		Description: ptr("A test eval set"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if es.Name != "My Test Suite" {
		t.Errorf("expected name %q, got %q", "My Test Suite", es.Name)
	}
	if es.UserID != "user1" {
		t.Errorf("expected user_id %q, got %q", "user1", es.UserID)
	}
}

func TestCreateEvalSet_EmptyName(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "",
	})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateEvalSet_WhitespaceName(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "   ",
	})
	if err == nil {
		t.Fatal("expected error for whitespace-only name")
	}
}

func TestCreateEvalSet_NameTooLong(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   string(make([]byte, 256)),
	})
	if err == nil {
		t.Fatal("expected error for name > 255 chars")
	}
}

func TestCreateEvalSet_DuplicateName(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, _ = svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Duplicate",
	})

	_, err := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Duplicate",
	})
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestListEvalSets_UserScoped(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, _ = svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "User1 Set",
	})
	_, _ = svc.createEvalSet(context.Background(), "user2", EvalSetCreate{
		UserID: "user2",
		Name:   "User2 Set",
	})

	sets, err := svc.listEvalSets(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sets) != 1 {
		t.Fatalf("expected 1 set, got %d", len(sets))
	}
	if sets[0].Name != "User1 Set" {
		t.Errorf("expected User1 Set, got %q", sets[0].Name)
	}
}

func TestCreateEvalCase_EmptyInput(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Suite",
	})

	_, err := svc.createEvalCase(context.Background(), "user1", EvalCaseCreate{
		EvalSetID: es.ID,
		Input:     "",
	})
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestCreateEvalCase_InputTooLong(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Suite",
	})

	_, err := svc.createEvalCase(context.Background(), "user1", EvalCaseCreate{
		EvalSetID: es.ID,
		Input:     string(make([]byte, 10001)),
	})
	if err == nil {
		t.Fatal("expected error for input > 10000 chars")
	}
}

func TestCreateEvalCase_InvalidTrajectory(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Suite",
	})

	// Invalid JSON
	_, err := svc.createEvalCase(context.Background(), "user1", EvalCaseCreate{
		EvalSetID:          es.ID,
		Input:              "test input",
		ExpectedTrajectory: json.RawMessage(`not json`),
	})
	if err == nil {
		t.Fatal("expected error for invalid trajectory JSON")
	}

	// Missing tool_name
	_, err = svc.createEvalCase(context.Background(), "user1", EvalCaseCreate{
		EvalSetID:          es.ID,
		Input:              "test input",
		ExpectedTrajectory: json.RawMessage(`[{}]`),
	})
	if err == nil {
		t.Fatal("expected error for trajectory with missing tool_name")
	}
}

func TestCreateEvalCase_InvalidRubrics(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Suite",
	})

	_, err := svc.createEvalCase(context.Background(), "user1", EvalCaseCreate{
		EvalSetID: es.ID,
		Input:     "test input",
		Rubrics:   json.RawMessage(`not json`),
	})
	if err == nil {
		t.Fatal("expected error for invalid rubrics JSON")
	}
}

func TestCreateEvalCase_InvalidUserMessages(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Suite",
	})

	_, err := svc.createEvalCase(context.Background(), "user1", EvalCaseCreate{
		EvalSetID:    es.ID,
		Input:        "test input",
		UserMessages: json.RawMessage(`not json`),
	})
	if err == nil {
		t.Fatal("expected error for invalid user_messages JSON")
	}
}

func TestDeleteEvalSet_CascadeDeletesCases(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Suite",
	})

	_, _ = svc.createEvalCase(context.Background(), "user1", EvalCaseCreate{
		EvalSetID: es.ID,
		Input:     "test case 1",
	})
	_, _ = svc.createEvalCase(context.Background(), "user1", EvalCaseCreate{
		EvalSetID: es.ID,
		Input:     "test case 2",
	})

	// Verify cases exist
	cases, _ := svc.listEvalCases(context.Background(), es.ID, "user1")
	if len(cases) != 2 {
		t.Fatalf("expected 2 cases before delete, got %d", len(cases))
	}

	// Delete eval set
	if err := svc.deleteEvalSet(context.Background(), es.ID, "user1"); err != nil {
		t.Fatalf("unexpected error deleting eval_set: %v", err)
	}

	// Verify cascade: cases should be deleted by mock
	remaining, _ := svc.listEvalCases(context.Background(), es.ID, "user1")
	if len(remaining) != 0 {
		t.Errorf("expected 0 cases after cascade delete, got %d", len(remaining))
	}
}

func TestEvalCase_UnauthorizedAccess(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "User1 Suite",
	})

	// user2 should not be able to create cases in user1's set
	_, err := svc.createEvalCase(context.Background(), "user2", EvalCaseCreate{
		EvalSetID: es.ID,
		Input:     "malicious input",
	})
	if err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestJSONFields_SerializeDeserialize(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "JSON Test Suite",
	})

	trajectory := json.RawMessage(`[{"tool_name":"search","args":{"query":"hello"},"match_type":"EXACT"}]`)
	rubrics := json.RawMessage(`[{"name":"quality","levels":[{"score":5,"description":"excellent"}]}]`)
	userMsgs := json.RawMessage(`["follow up question","another turn"]`)

	ec, err := svc.createEvalCase(context.Background(), "user1", EvalCaseCreate{
		EvalSetID:          es.ID,
		Input:              "test input",
		ExpectedTrajectory: trajectory,
		ExpectedResponse:   ptr("expected response"),
		Rubrics:            rubrics,
		UserMessages:       userMsgs,
	})
	if err != nil {
		t.Fatalf("unexpected error creating case: %v", err)
	}

	// Verify JSON fields round-trip
	if string(ec.ExpectedTrajectory) != string(trajectory) {
		t.Errorf("trajectory mismatch:\ngot  %s\nwant %s", ec.ExpectedTrajectory, trajectory)
	}
	if string(ec.Rubrics) != string(rubrics) {
		t.Errorf("rubrics mismatch:\ngot  %s\nwant %s", ec.Rubrics, rubrics)
	}
	if string(ec.UserMessages) != string(userMsgs) {
		t.Errorf("user_messages mismatch:\ngot  %s\nwant %s", ec.UserMessages, userMsgs)
	}
	if ec.ExpectedResponse == nil || *ec.ExpectedResponse != "expected response" {
		t.Errorf("expected_response mismatch")
	}

	// Verify the JSON fields can be parsed back
	var tools []ExpectedToolCall
	if err := json.Unmarshal(ec.ExpectedTrajectory, &tools); err != nil {
		t.Fatalf("failed to unmarshal trajectory: %v", err)
	}
	if len(tools) != 1 || tools[0].ToolName != "search" {
		t.Errorf("parsed trajectory incorrectly")
	}
}

func ptr(s string) *string {
	return &s
}

// ─── Import / Export Tests ───────────────────────────────────────────────────

func TestImportEvalSet_ValidJSON(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	data := []byte(`{
		"name": "Weather Tests",
		"description": "Weather agent evaluation",
		"eval_cases": [
			{
				"name": "basic_query",
				"input": "What's the weather in Tokyo?",
				"expected_trajectory": [
					{"tool_name": "weather_api", "args": {"city": "Tokyo"}}
				],
				"expected_response": "The weather is sunny"
			}
		]
	}`)

	es, err := svc.importEvalSet(context.Background(), "user1", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if es.Name != "Weather Tests" {
		t.Errorf("expected name %q, got %q", "Weather Tests", es.Name)
	}
	if es.CaseCount != 1 {
		t.Errorf("expected 1 case, got %d", es.CaseCount)
	}
}

func TestImportEvalSet_InvalidJSON(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.importEvalSet(context.Background(), "user1", []byte(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestImportEvalSet_EmptyCases(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	data := []byte(`{"name": "Empty", "eval_cases": []}`)
	_, err := svc.importEvalSet(context.Background(), "user1", data)
	if err == nil {
		t.Fatal("expected error for empty eval_cases")
	}
}

func TestImportEvalSet_MissingInput(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	data := []byte(`{"name": "Bad", "eval_cases": [{"name": "no_input"}]}`)
	_, err := svc.importEvalSet(context.Background(), "user1", data)
	if err == nil {
		t.Fatal("expected error for missing input")
	}
}

func TestImportEvalSet_InvalidTrajectory(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	data := []byte(`{
		"name": "Bad Trajectory",
		"eval_cases": [{
			"input": "test",
			"expected_trajectory": "not an array"
		}]
	}`)
	_, err := svc.importEvalSet(context.Background(), "user1", data)
	if err == nil {
		t.Fatal("expected error for invalid trajectory")
	}
}

func TestImportEvalSet_DuplicateName(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	data := []byte(`{"name": "Dup", "eval_cases": [{"input": "test"}]}`)
	_, _ = svc.importEvalSet(context.Background(), "user1", data)

	_, err := svc.importEvalSet(context.Background(), "user1", data)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestExportEvalSet_RoundTrip(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	// Import a set
	importData := []byte(`{
		"name": "Round Trip",
		"description": "Test round trip",
		"eval_cases": [
			{
				"name": "case1",
				"input": "Hello world",
				"expected_trajectory": [
					{"tool_name": "search", "args": {"q": "hello"}, "match_type": "EXACT"}
				],
				"expected_response": "Hi there",
				"rubrics": [{"name": "quality", "levels": [{"score": 5, "description": "great"}]}],
				"user_messages": ["follow up"]
			},
			{
				"input": "Second case"
			}
		]
	}`)

	es, err := svc.importEvalSet(context.Background(), "user1", importData)
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	// Export it
	export, err := svc.exportEvalSet(context.Background(), es.ID, "user1")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Verify export matches import
	if export.Name != "Round Trip" {
		t.Errorf("expected name %q, got %q", "Round Trip", export.Name)
	}
	if export.Desc == nil || *export.Desc != "Test round trip" {
		t.Errorf("description mismatch")
	}
	if len(export.Cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(export.Cases))
	}

	// Verify first case
	c1 := export.Cases[0]
	if c1.Name == nil || *c1.Name != "case1" {
		t.Errorf("case1 name mismatch")
	}
	if c1.Input != "Hello world" {
		t.Errorf("case1 input mismatch")
	}
	if c1.ExpectedResponse == nil || *c1.ExpectedResponse != "Hi there" {
		t.Errorf("case1 expected_response mismatch")
	}

	// Verify trajectory round-trip
	traj, ok := c1.ExpectedTrajectory.([]any)
	if !ok || len(traj) != 1 {
		t.Fatalf("expected trajectory array with 1 element")
	}
	tool, ok := traj[0].(map[string]any)
	if !ok || tool["tool_name"] != "search" {
		t.Errorf("trajectory tool_name mismatch")
	}

	// Re-import the export and verify it works
	// Must use different name (uniqueness)
	export.Name = "Round Trip Reimport"
	reimportData, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("failed to marshal export: %v", err)
	}

	es2, err := svc.importEvalSet(context.Background(), "user1", reimportData)
	if err != nil {
		t.Fatalf("re-import failed: %v", err)
	}
	if es2.CaseCount != 2 {
		t.Errorf("expected 2 cases after re-import, got %d", es2.CaseCount)
	}
}

func TestExportEvalSet_NotFound(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.exportEvalSet(context.Background(), uuid.New(), "user1")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestExportEvalSet_Unauthorized(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	// Create as user1
	es, _ := svc.importEvalSet(context.Background(), "user1",
		[]byte(`{"name": "Secret", "eval_cases": [{"input": "test"}]}`))

	// Try to export as user2
	_, err := svc.exportEvalSet(context.Background(), es.ID, "user2")
	if err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestUpdateEvalSet_OptimisticConcurrency(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Concurrency Test",
	})

	// First update with correct version
	err := svc.updateEvalSet(context.Background(), es.ID, "user1", EvalSetUpdate{
		Name:    ptr("Updated Name"),
		Version: es.Version,
	})
	if err != nil {
		t.Fatalf("unexpected error with correct version: %v", err)
	}

	// Second update with stale version should fail
	err = svc.updateEvalSet(context.Background(), es.ID, "user1", EvalSetUpdate{
		Name:    ptr("Stale Update"),
		Version: 1, // stale version
	})
	if err != ErrConflict {
		t.Errorf("expected ErrConflict for stale version, got %v", err)
	}
}

// ─── Metric Profile Tests ────────────────────────────────────────────────────

func TestCreateMetricProfile_Valid(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	p, err := svc.createMetricProfile(context.Background(), "user1", MetricProfileCreate{
		UserID: "user1",
		Name:   "Standard Quality",
		Metrics: []MetricConfig{
			{MetricID: "tool_trajectory_avg_score", Enabled: true, Threshold: 1.0},
			{MetricID: "response_match_score", Enabled: true, Threshold: 0.8},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "Standard Quality" {
		t.Errorf("expected name %q, got %q", "Standard Quality", p.Name)
	}
	if len(p.Metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(p.Metrics))
	}
}

func TestCreateMetricProfile_EmptyName(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.createMetricProfile(context.Background(), "user1", MetricProfileCreate{
		UserID:  "user1",
		Name:    "",
		Metrics: []MetricConfig{{MetricID: "tool_trajectory_avg_score", Enabled: true, Threshold: 1.0}},
	})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateMetricProfile_DuplicateName(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	req := MetricProfileCreate{
		UserID:  "user1",
		Name:    "Dup Profile",
		Metrics: []MetricConfig{{MetricID: "tool_trajectory_avg_score", Enabled: true, Threshold: 1.0}},
	}
	_, _ = svc.createMetricProfile(context.Background(), "user1", req)
	_, err := svc.createMetricProfile(context.Background(), "user1", req)
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestCreateMetricProfile_UnknownMetric(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.createMetricProfile(context.Background(), "user1", MetricProfileCreate{
		UserID:  "user1",
		Name:    "Bad Metrics",
		Metrics: []MetricConfig{{MetricID: "nonexistent_metric", Enabled: true, Threshold: 0.5}},
	})
	if err == nil {
		t.Fatal("expected error for unknown metric")
	}
}

func TestCreateMetricProfile_ThresholdOutOfRange(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	// tool_trajectory_avg_score has range [0, 1]
	_, err := svc.createMetricProfile(context.Background(), "user1", MetricProfileCreate{
		UserID:  "user1",
		Name:    "Bad Threshold",
		Metrics: []MetricConfig{{MetricID: "tool_trajectory_avg_score", Enabled: true, Threshold: 2.0}},
	})
	if err == nil {
		t.Fatal("expected error for out-of-range threshold")
	}
}

func TestCreateMetricProfile_NoneEnabled(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.createMetricProfile(context.Background(), "user1", MetricProfileCreate{
		UserID:  "user1",
		Name:    "None Enabled",
		Metrics: []MetricConfig{{MetricID: "tool_trajectory_avg_score", Enabled: false, Threshold: 1.0}},
	})
	if err == nil {
		t.Fatal("expected error when no metrics are enabled")
	}
}

func TestCreateMetricProfile_EmptyMetrics(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.createMetricProfile(context.Background(), "user1", MetricProfileCreate{
		UserID:  "user1",
		Name:    "Empty",
		Metrics: []MetricConfig{},
	})
	if err == nil {
		t.Fatal("expected error for empty metrics")
	}
}

func TestListMetricProfiles_UserScoped(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	req := MetricProfileCreate{
		UserID:  "user1",
		Name:    "User1 Profile",
		Metrics: []MetricConfig{{MetricID: "tool_trajectory_avg_score", Enabled: true, Threshold: 1.0}},
	}
	_, _ = svc.createMetricProfile(context.Background(), "user1", req)

	req.UserID = "user2"
	req.Name = "User2 Profile"
	_, _ = svc.createMetricProfile(context.Background(), "user2", req)

	profiles, err := svc.listMetricProfiles(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	if profiles[0].Name != "User1 Profile" {
		t.Errorf("expected User1 Profile, got %q", profiles[0].Name)
	}
}

// ─── Evaluation Run Tests ────────────────────────────────────────────────────

func TestCreateEvalRun_Valid(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	// Create an eval set first
	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Set",
	})

	run, err := svc.createEvalRun(context.Background(), "user1", EvalRunCreate{
		UserID:    "user1",
		EvalSetID: es.ID,
		AgentID:   uuid.New(),
		NumRuns:   3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != "pending" {
		t.Errorf("expected status pending, got %s", run.Status)
	}
	if run.NumRuns != 3 {
		t.Errorf("expected num_runs 3, got %d", run.NumRuns)
	}
}

func TestCreateEvalRun_NumRunsClamped(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Set",
	})

	// Test below minimum
	run, err := svc.createEvalRun(context.Background(), "user1", EvalRunCreate{
		UserID:    "user1",
		EvalSetID: es.ID,
		AgentID:   uuid.New(),
		NumRuns:   0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.NumRuns != 1 {
		t.Errorf("expected num_runs clamped to 1, got %d", run.NumRuns)
	}

	// Test above maximum
	run, err = svc.createEvalRun(context.Background(), "user1", EvalRunCreate{
		UserID:    "user1",
		EvalSetID: es.ID,
		AgentID:   uuid.New(),
		NumRuns:   50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.NumRuns != 10 {
		t.Errorf("expected num_runs clamped to 10, got %d", run.NumRuns)
	}
}

func TestCreateEvalRun_UnauthorizedEvalSet(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "User1 Set",
	})

	_, err := svc.createEvalRun(context.Background(), "user2", EvalRunCreate{
		UserID:    "user2",
		EvalSetID: es.ID,
		AgentID:   uuid.New(),
	})
	if err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestCreateEvalRun_EvalSetNotFound(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	_, err := svc.createEvalRun(context.Background(), "user1", EvalRunCreate{
		UserID:    "user1",
		EvalSetID: uuid.New(),
		AgentID:   uuid.New(),
	})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListEvalRuns_UserScoped(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Set",
	})
	_, _ = svc.createEvalRun(context.Background(), "user1", EvalRunCreate{
		UserID:    "user1",
		EvalSetID: es.ID,
		AgentID:   uuid.New(),
	})

	es2, _ := svc.createEvalSet(context.Background(), "user2", EvalSetCreate{
		UserID: "user2",
		Name:   "User2 Set",
	})
	_, _ = svc.createEvalRun(context.Background(), "user2", EvalRunCreate{
		UserID:    "user2",
		EvalSetID: es2.ID,
		AgentID:   uuid.New(),
	})

	runs, err := svc.listEvalRuns(context.Background(), "user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run for user1, got %d", len(runs))
	}
}

func TestCancelEvalRun_OnlyRunningOrPending(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Set",
	})
	run, _ := svc.createEvalRun(context.Background(), "user1", EvalRunCreate{
		UserID:    "user1",
		EvalSetID: es.ID,
		AgentID:   uuid.New(),
	})

	// Cancel pending run — should work
	err := svc.cancelEvalRun(context.Background(), run.ID, "user1")
	if err != nil {
		t.Fatalf("expected cancel to succeed for pending run, got %v", err)
	}

	// Try to cancel again — should fail (now cancelled)
	err = svc.cancelEvalRun(context.Background(), run.ID, "user1")
	if err == nil {
		t.Fatal("expected error when cancelling already-cancelled run")
	}
}

func TestGetEvalResults_Unauthorized(t *testing.T) {
	t.Parallel()
	repo := newMockRepo()
	svc := newService(repo)

	es, _ := svc.createEvalSet(context.Background(), "user1", EvalSetCreate{
		UserID: "user1",
		Name:   "Test Set",
	})
	run, _ := svc.createEvalRun(context.Background(), "user1", EvalRunCreate{
		UserID:    "user1",
		EvalSetID: es.ID,
		AgentID:   uuid.New(),
	})

	_, err := svc.getEvalResults(context.Background(), run.ID, "user2")
	if err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}
