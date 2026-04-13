package evaluation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Catalog Tests ───────────────────────────────────────────────────────────

func TestGetMetricCatalog_Count(t *testing.T) {
	t.Parallel()
	catalog := GetMetricCatalog()
	if len(catalog) != 12 {
		t.Errorf("expected 12 metrics, got %d", len(catalog))
	}
}

func TestGetMetricCatalog_Ordering(t *testing.T) {
	t.Parallel()
	catalog := GetMetricCatalog()

	// Core metrics should come first
	foundAdvanced := false
	for _, m := range catalog {
		if m.Category == "advanced" {
			foundAdvanced = true
		}
		if foundAdvanced && m.Category == "core" {
			t.Error("core metric appeared after advanced metric")
		}
	}
}

func TestGetMetricByID_AllExist(t *testing.T) {
	t.Parallel()
	ids := []string{
		"tool_trajectory_avg_score",
		"response_evaluation_score",
		"response_match_score",
		"safety_v1",
		"final_response_match_v2",
		"rubric_based_final_response_quality_v1",
		"hallucinations_v1",
		"rubric_based_tool_use_quality_v1",
		"per_turn_user_simulator_quality_v1",
		"multi_turn_task_success_v1",
		"multi_turn_trajectory_quality_v1",
		"multi_turn_tool_use_quality_v1",
	}

	for _, id := range ids {
		m, ok := GetMetricByID(id)
		if !ok {
			t.Errorf("metric %q not found", id)
			continue
		}
		if m.ID != id {
			t.Errorf("expected ID %q, got %q", id, m.ID)
		}
		if m.Name == "" {
			t.Errorf("metric %q has empty name", id)
		}
		if m.Description == "" {
			t.Errorf("metric %q has empty description", id)
		}
	}
}

func TestGetMetricByID_Unknown(t *testing.T) {
	t.Parallel()
	_, ok := GetMetricByID("nonexistent_metric")
	if ok {
		t.Error("expected false for unknown metric ID")
	}
}

func TestGetMetricsByType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		metricType MetricType
		wantCount  int
	}{
		{MetricTrajectory, 1},
		{MetricResponse, 3},
		{MetricSafety, 1},
		{MetricHallucination, 1},
		{MetricRubric, 2},
		{MetricSimulation, 1},
		{MetricMultiTurn, 3},
	}

	for _, tt := range tests {
		t.Run(string(tt.metricType), func(t *testing.T) {
			t.Parallel()
			result := GetMetricsByType(tt.metricType)
			if len(result) != tt.wantCount {
				t.Errorf("expected %d metrics of type %s, got %d", tt.wantCount, tt.metricType, len(result))
			}
			for _, m := range result {
				if m.Type != tt.metricType {
					t.Errorf("expected type %s, got %s", tt.metricType, m.Type)
				}
			}
		})
	}
}

func TestMetricDefinition_HasBoundedRange(t *testing.T) {
	t.Parallel()
	m, _ := GetMetricByID("tool_trajectory_avg_score")
	if !m.HasBoundedRange() {
		t.Error("expected bounded range for trajectory score")
	}

	// All metrics should have bounded ranges except those with [0,0]
	for _, cat := range GetMetricCatalog() {
		if cat.RangeMax > cat.RangeMin && !cat.HasBoundedRange() {
			t.Errorf("metric %q has range but HasBoundedRange returned false", cat.ID)
		}
	}
}

func TestMetricDefaults_AreInRange(t *testing.T) {
	t.Parallel()
	for _, m := range GetMetricCatalog() {
		if m.HasBoundedRange() {
			if m.DefaultThreshold < m.RangeMin || m.DefaultThreshold > m.RangeMax {
				t.Errorf("metric %q default threshold %v out of range [%v, %v]",
					m.ID, m.DefaultThreshold, m.RangeMin, m.RangeMax)
			}
		}
	}
}

// ─── Handler Tests ───────────────────────────────────────────────────────────

func TestListMetrics_ReturnsAll(t *testing.T) {
	t.Parallel()
	h := newHandler(newService(newMockRepo()))

	req := httptest.NewRequest(http.MethodGet, "/api/evaluation/metrics", nil)
	rec := httptest.NewRecorder()

	h.listMetrics(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var metrics []MetricDefinition
	if err := json.Unmarshal(rec.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(metrics) != 12 {
		t.Errorf("expected 12 metrics, got %d", len(metrics))
	}
}

func TestGetMetric_Found(t *testing.T) {
	t.Parallel()
	h := newHandler(newService(newMockRepo()))

	req := httptest.NewRequest(http.MethodGet, "/api/evaluation/metrics/tool_trajectory_avg_score", nil)
	rec := httptest.NewRecorder()
	req.SetPathValue("id", "tool_trajectory_avg_score")

	h.getMetric(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var m MetricDefinition
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if m.ID != "tool_trajectory_avg_score" {
		t.Errorf("expected tool_trajectory_avg_score, got %s", m.ID)
	}
	if m.Type != MetricTrajectory {
		t.Errorf("expected trajectory type, got %s", m.Type)
	}
}

func TestGetMetric_NotFound(t *testing.T) {
	t.Parallel()
	h := newHandler(newService(newMockRepo()))

	req := httptest.NewRequest(http.MethodGet, "/api/evaluation/metrics/nonexistent", nil)
	rec := httptest.NewRecorder()
	req.SetPathValue("id", "nonexistent")

	h.getMetric(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
