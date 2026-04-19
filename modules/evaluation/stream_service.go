package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"time"

	"github.com/google/uuid"

)

// executeEvalStream returns an iterator that yields SSE events during evaluation.
// This follows the same pattern as the AGUI ExecuteRun method.
func (s *service) executeEvalStream(ctx context.Context, userID string, req EvalRunCreate, runID uuid.UUID) iter.Seq2[map[string]any, error] {
	return func(yield func(map[string]any, error) bool) {
		numRuns := clampNumRuns(req.NumRuns)

		es, cases, err := s.loadEvalRunInputs(ctx, userID, req.EvalSetID)
		if err != nil {
			yield(nil, err)
			return
		}
		if err := s.createStreamingRun(ctx, userID, req, runID, numRuns); err != nil {
			yield(nil, err)
			return
		}

		if !yield(runStartedEvent(runID, es.Name, len(cases), numRuns), nil) {
			return
		}

		evaluators := getRegisteredEvaluators()
		totalCases := len(cases) * numRuns
		var results []EvalResult
		completed := 0

		for inv := 1; inv <= numRuns; inv++ {
			for caseIdx, ec := range cases {
				caseResults, ok := s.streamCase(ctx, streamCaseArgs{
					runID:      runID,
					userID:     userID,
					agentID:    req.AgentID.String(),
					ec:         ec,
					caseIdx:    caseIdx,
					inv:        inv,
					totalCases: totalCases,
					completed:  &completed,
					evaluators: evaluators,
				}, yield)
				if !ok {
					return
				}
				results = append(results, caseResults...)
			}
		}

		s.finalizeStreamingRun(ctx, runID, results, totalCases, completed, yield)
	}
}

// clampNumRuns enforces the [1, 10] bounds on per-case invocation count.
// Zero/negative map to 1; anything above 10 caps at 10 to keep an
// accidental request from running hundreds of inference calls.
func clampNumRuns(n int) int {
	if n < 1 {
		return 1
	}
	if n > 10 {
		return 10
	}
	return n
}

// loadEvalRunInputs verifies the caller owns the target EvalSet, then
// loads its cases. Refuses to start a run against an empty set.
func (s *service) loadEvalRunInputs(ctx context.Context, userID string, evalSetID uuid.UUID) (EvalSet, []EvalCase, error) {
	es, err := s.getOwnedEvalSet(ctx, evalSetID, userID)
	if err != nil {
		return EvalSet{}, nil, err
	}
	cases, err := s.repo.ListEvalCases(ctx, evalSetID)
	if err != nil {
		return EvalSet{}, nil, fmt.Errorf("listing eval cases: %w", err)
	}
	if len(cases) == 0 {
		return EvalSet{}, nil, fmt.Errorf("eval set has no test cases")
	}
	return es, cases, nil
}

// createStreamingRun persists the EvalRun in "running" status so the UI
// and watchers have something to track before the first case completes.
func (s *service) createStreamingRun(ctx context.Context, userID string, req EvalRunCreate, runID uuid.UUID, numRuns int) error {
	now := time.Now()
	run := EvalRun{
		ID:        runID,
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
		return fmt.Errorf("creating evaluation run: %w", err)
	}
	return nil
}

// streamCaseArgs bundles the per-case iteration state to keep streamCase's
// signature readable.
type streamCaseArgs struct {
	runID      uuid.UUID
	userID     string
	agentID    string
	ec         EvalCase
	caseIdx    int
	inv        int
	totalCases int
	completed  *int
	evaluators []Evaluator
}

// streamCase runs every evaluator against one case, yielding
// CASE_STARTED / METRIC_DONE / CASE_COMPLETE events as it goes. Returns
// (resultsToPersist, continue) — continue=false means either ctx was
// cancelled or the consumer returned false from yield.
func (s *service) streamCase(ctx context.Context, a streamCaseArgs, yield func(map[string]any, error) bool) ([]EvalResult, bool) {
	if ctx.Err() != nil {
		yield(nil, fmt.Errorf("context cancelled"))
		return nil, false
	}

	caseName := a.ec.Name
	caseNameStr := fmt.Sprintf("Case %d", a.caseIdx+1)
	if caseName != nil {
		caseNameStr = *caseName
	}

	if !yield(caseStartedEvent(a, caseNameStr), nil) {
		return nil, false
	}

	evalReq := s.buildEvalRequest(ctx, a.userID, a.agentID, a.ec)

	var results []EvalResult
	var caseMetrics []map[string]any
	var avgScore float64

	for _, eval := range a.evaluators {
		if ctx.Err() != nil {
			yield(nil, fmt.Errorf("context cancelled"))
			return nil, false
		}
		evalResult := eval.Evaluate(ctx, evalReq)
		details := evalResult.Details
		if details == nil {
			details = json.RawMessage(`{}`)
		}
		status := "passed"
		if evalResult.Status == "failed" {
			status = "failed"
		}
		if !yield(metricDoneEvent(a.runID, a.ec.ID, caseNameStr, eval.MetricID(), evalResult.Score, status, details), nil) {
			return nil, false
		}
		avgScore += evalResult.Score
		caseMetrics = append(caseMetrics, map[string]any{
			"metric_id": eval.MetricID(),
			"score":     evalResult.Score,
			"status":    status,
			"details":   details,
		})
		results = append(results, EvalResult{
			ID:         uuid.New(),
			RunID:      a.runID,
			CaseID:     a.ec.ID,
			MetricID:   eval.MetricID(),
			Score:      &evalResult.Score,
			Status:     status,
			Details:    details,
			Invocation: a.inv,
			CreatedAt:  time.Now(),
		})
	}

	if len(a.evaluators) > 0 {
		avgScore /= float64(len(a.evaluators))
	}
	caseStatus := overallCaseStatus(caseMetrics)
	*a.completed++

	if !yield(caseCompleteEvent(a, caseNameStr, caseStatus, avgScore, caseMetrics), nil) {
		return nil, false
	}
	return results, true
}

// overallCaseStatus collapses per-metric statuses into a single
// case-level verdict: any failed metric fails the case.
func overallCaseStatus(metrics []map[string]any) string {
	for _, cm := range metrics {
		if cm["status"] == "failed" {
			return "failed"
		}
	}
	return "passed"
}

// finalizeStreamingRun batch-inserts accumulated results, flips the run
// record to "completed" (or "failed" on batch-insert error), and emits
// the terminal RUN_COMPLETE / RUN_ERROR event.
func (s *service) finalizeStreamingRun(ctx context.Context, runID uuid.UUID, results []EvalResult, totalCases, completed int, yield func(map[string]any, error) bool) {
	if len(results) > 0 {
		if err := s.repo.BatchCreateResults(ctx, results); err != nil {
			errMsg := err.Error()
			_ = s.repo.UpdateEvalRunStatus(ctx, runID, "failed", &errMsg)
			yield(map[string]any{
				"type":    "RUN_ERROR",
				"run_id":  runID.String(),
				"message": errMsg,
			}, nil)
			return
		}
	}
	_ = s.repo.UpdateEvalRunStatus(ctx, runID, "completed", nil)

	run, _ := s.repo.GetEvalRun(ctx, runID)
	durationMs := time.Since(*run.StartedAt).Milliseconds()
	yield(map[string]any{
		"type":         "RUN_COMPLETE",
		"run_id":       runID.String(),
		"status":       "completed",
		"total_cases":  totalCases,
		"completed":    completed,
		"duration_ms":  durationMs,
		"completed_at": run.CompletedAt,
	}, nil)
}

// ── SSE event payload builders ───────────────────────────────────────────────

func runStartedEvent(runID uuid.UUID, evalSetName string, numCases, numRuns int) map[string]any {
	return map[string]any{
		"type":      "RUN_STARTED",
		"run_id":    runID.String(),
		"eval_set":  evalSetName,
		"num_cases": numCases,
		"num_runs":  numRuns,
	}
}

func caseStartedEvent(a streamCaseArgs, caseName string) map[string]any {
	return map[string]any{
		"type":        "CASE_STARTED",
		"run_id":      a.runID.String(),
		"case_id":     a.ec.ID.String(),
		"case_name":   caseName,
		"invocation":  a.inv,
		"case_index":  a.caseIdx,
		"total_cases": a.totalCases,
		"completed":   *a.completed,
	}
}

func metricDoneEvent(runID, caseID uuid.UUID, caseName, metric string, score float64, status string, details json.RawMessage) map[string]any {
	return map[string]any{
		"type":      "METRIC_DONE",
		"run_id":    runID.String(),
		"case_id":   caseID.String(),
		"case_name": caseName,
		"metric":    metric,
		"score":     score,
		"status":    status,
		"details":   details,
	}
}

func caseCompleteEvent(a streamCaseArgs, caseName, status string, avgScore float64, metrics []map[string]any) map[string]any {
	return map[string]any{
		"type":         "CASE_COMPLETE",
		"run_id":       a.runID.String(),
		"case_id":      a.ec.ID.String(),
		"case_name":    caseName,
		"status":       status,
		"score":        avgScore,
		"metrics":      metrics,
		"invocation":   a.inv,
		"total_cases":  a.totalCases,
		"completed":    *a.completed,
		"progress_pct": float64(*a.completed) / float64(a.totalCases) * 100,
	}
}

// buildMockEvalRequest creates an EvalRequest using expected data as the "actual" response.
