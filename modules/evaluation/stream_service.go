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
		numRuns := req.NumRuns
		if numRuns < 1 {
			numRuns = 1
		}
		if numRuns > 10 {
			numRuns = 10
		}

		// Verify ownership of eval set
		es, err := s.repo.GetEvalSet(ctx, req.EvalSetID)
		if err != nil {
			if err.Error() == "eval_set not found" {
				yield(nil, ErrNotFound)
			} else {
				yield(nil, fmt.Errorf("getting eval set: %w", err))
			}
			return
		}
		if es.UserID != userID {
			yield(nil, ErrUnauthorized)
			return
		}

		// Get eval cases
		cases, err := s.repo.ListEvalCases(ctx, req.EvalSetID)
		if err != nil {
			yield(nil, fmt.Errorf("listing eval cases: %w", err))
			return
		}
		if len(cases) == 0 {
			yield(nil, fmt.Errorf("eval set has no test cases"))
			return
		}

		// Create run record
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
			yield(nil, fmt.Errorf("creating evaluation run: %w", err))
			return
		}

		// Emit RUN_STARTED event
		if !yield(map[string]any{
			"type":      "RUN_STARTED",
			"run_id":    runID.String(),
			"eval_set":  es.Name,
			"num_cases": len(cases),
			"num_runs":  numRuns,
		}, nil) {
			return
		}

		// Execute all evaluators against all cases
		evaluators := getRegisteredEvaluators()
		var results []EvalResult
		totalCases := len(cases) * numRuns
		completed := 0

		for inv := 1; inv <= numRuns; inv++ {
			for caseIdx, ec := range cases {
				// Check for cancellation
				if ctx.Err() != nil {
					yield(nil, fmt.Errorf("context cancelled"))
					return
				}

				// Add realistic delay before starting case (simulate agent call)
				select {
				case <-time.After(500 * time.Millisecond):
					// Continue
				case <-ctx.Done():
					yield(nil, fmt.Errorf("context cancelled"))
					return
				}

				caseName := ""
				if ec.Name != nil {
					caseName = *ec.Name
				} else {
					caseName = fmt.Sprintf("Case %d", caseIdx+1)
				}

				// Emit CASE_STARTED event
				if !yield(map[string]any{
					"type":         "CASE_STARTED",
					"run_id":       runID.String(),
					"case_id":      ec.ID.String(),
					"case_name":    caseName,
					"invocation":   inv,
					"case_index":   caseIdx,
					"total_cases":  totalCases,
					"completed":    completed,
				}, nil) {
					return
				}

				// Build eval request (using expected data as mock actual response)
				evalReq := buildMockEvalRequest(ec)

				var caseMetrics []map[string]any
				var avgScore float64

				for _, eval := range evaluators {
					// Check for cancellation
					if ctx.Err() != nil {
						yield(nil, fmt.Errorf("context cancelled"))
						return
					}

					evalResult := eval.Evaluate(ctx, evalReq)
					detailsJSON := evalResult.Details
					if detailsJSON == nil {
						detailsJSON = json.RawMessage(`{}`)
					}

					status := "passed"
					if evalResult.Status == "failed" {
						status = "failed"
					}

					// Emit METRIC_DONE event
					if !yield(map[string]any{
						"type":       "METRIC_DONE",
						"run_id":     runID.String(),
						"case_id":    ec.ID.String(),
						"case_name":  caseName,
						"metric":     eval.MetricID(),
						"score":      evalResult.Score,
						"status":     status,
						"details":    detailsJSON,
					}, nil) {
						return

					// Small delay between metrics
					select {
					case <-time.After(100 * time.Millisecond):
					case <-ctx.Done():
						yield(nil, fmt.Errorf("context cancelled"))
						return
					}
					}

					avgScore += evalResult.Score
					caseMetrics = append(caseMetrics, map[string]any{
						"metric_id": eval.MetricID(),
						"score":     evalResult.Score,
						"status":    status,
						"details":   detailsJSON,
					})

					// Store result
					results = append(results, EvalResult{
						ID:         uuid.New(),
						RunID:      runID,
						CaseID:     ec.ID,
						MetricID:   eval.MetricID(),
						Score:      &evalResult.Score,
						Status:     status,
						Details:    detailsJSON,
						Invocation: inv,
						CreatedAt:  time.Now(),
					})
				}

				// Calculate average score
				if len(evaluators) > 0 {
					avgScore /= float64(len(evaluators))
				}

				// Determine overall case status
				caseStatus := "passed"
				for _, cm := range caseMetrics {
					if cm["status"] == "failed" {
						caseStatus = "failed"
						break
					}
				}

				completed++

				// Emit CASE_COMPLETE event
				if !yield(map[string]any{
					"type":         "CASE_COMPLETE",
					"run_id":       runID.String(),
					"case_id":      ec.ID.String(),
					"case_name":    caseName,
					"status":       caseStatus,
					"score":        avgScore,
					"metrics":      caseMetrics,
					"invocation":   inv,
					"total_cases":  totalCases,
					"completed":    completed,
					"progress_pct": float64(completed) / float64(totalCases) * 100,
				}, nil) {
					return

				// Delay between cases
				select {
				case <-time.After(300 * time.Millisecond):
				case <-ctx.Done():
					yield(nil, fmt.Errorf("context cancelled"))
					return
				}
				}
			}
		}

		// Batch insert results
		if len(results) > 0 {
			if err := s.repo.BatchCreateResults(ctx, results); err != nil {
				errMsg := err.Error()
				_ = s.repo.UpdateEvalRunStatus(ctx, runID, "failed", &errMsg)
				
				// Emit RUN_ERROR event
				yield(map[string]any{
					"type":    "RUN_ERROR",
					"run_id":  runID.String(),
					"message": errMsg,
				}, nil)
				return
			}
		}

		// Mark run as completed
		_ = s.repo.UpdateEvalRunStatus(ctx, runID, "completed", nil)

		// Refresh run with completed_at
		run, _ = s.repo.GetEvalRun(ctx, runID)
		durationMs := time.Since(*run.StartedAt).Milliseconds()

		// Emit RUN_COMPLETE event
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
}

// buildMockEvalRequest creates an EvalRequest using expected data as the "actual" response.
