package evaluation

import (
	"sort"
)

// MetricType categorizes evaluation metrics.
type MetricType string

const (
	MetricTrajectory    MetricType = "trajectory"
	MetricResponse      MetricType = "response"
	MetricSafety        MetricType = "safety"
	MetricHallucination MetricType = "hallucination"
	MetricRubric        MetricType = "rubric"
	MetricSimulation    MetricType = "simulation"
	MetricMultiTurn     MetricType = "multi_turn"
)

// MetricDefinition describes a single prebuilt evaluation metric.
type MetricDefinition struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	Description      string     `json:"description"`
	Type             MetricType `json:"type"`
	RangeMin         float64    `json:"range_min"`
	RangeMax         float64    `json:"range_max"`
	DefaultThreshold float64    `json:"default_threshold"`
	RequiresLLM      bool       `json:"requires_llm"`
	RequiresVertex   bool       `json:"requires_vertex"`
	Category         string     `json:"category"` // "core" or "advanced"
}

// HasBoundedRange returns true if the metric has a defined min/max range.
func (m MetricDefinition) HasBoundedRange() bool {
	return m.RangeMax > m.RangeMin
}

// metricCatalog holds all 12 ADK prebuilt metrics.
var metricCatalog = []MetricDefinition{
	// ─── Core metrics ─────────────────────────────────────────────────────
	{
		ID:               "tool_trajectory_avg_score",
		Name:             "Tool Trajectory Score",
		Description:      "Compares the agent's actual tool call sequence against the expected trajectory. Supports EXACT, IN_ORDER, and ANY_ORDER matching.",
		Type:             MetricTrajectory,
		RangeMin:         0,
		RangeMax:         1,
		DefaultThreshold: 1.0,
		RequiresLLM:      false,
		RequiresVertex:   false,
		Category:         "core",
	},
	{
		ID:               "response_match_score",
		Name:             "Response Match Score",
		Description:      "ROUGE-1 F1 score comparing the agent's response against a golden reference response. Range [0, 1].",
		Type:             MetricResponse,
		RangeMin:         0,
		RangeMax:         1,
		DefaultThreshold: 0.8,
		RequiresLLM:      false,
		RequiresVertex:   false,
		Category:         "core",
	},
	{
		ID:               "response_evaluation_score",
		Name:             "Response Coherence",
		Description:      "Evaluates response coherence and quality using Vertex AI. Range [1, 5].",
		Type:             MetricResponse,
		RangeMin:         1,
		RangeMax:         5,
		DefaultThreshold: 3.0,
		RequiresLLM:      true,
		RequiresVertex:   true,
		Category:         "core",
	},
	{
		ID:               "safety_v1",
		Name:             "Safety Score",
		Description:      "Scores the agent's responses for safety and harmlessness using Vertex AI's safety models. Range [0, 1] where higher is safer.",
		Type:             MetricSafety,
		RangeMin:         0,
		RangeMax:         1,
		DefaultThreshold: 0.8,
		RequiresLLM:      false,
		RequiresVertex:   true,
		Category:         "core",
	},
	{
		ID:               "hallucinations_v1",
		Name:             "Hallucination Detection",
		Description:      "Two-step process: segments response into claims, then classifies each as Supported, Unsupported, Contradictory, Disputed, or Not Applicable. Score reflects proportion of supported claims.",
		Type:             MetricHallucination,
		RangeMin:         0,
		RangeMax:         1,
		DefaultThreshold: 0.9,
		RequiresLLM:      true,
		RequiresVertex:   true,
		Category:         "core",
	},

	// ─── Advanced metrics ─────────────────────────────────────────────────
	{
		ID:               "final_response_match_v2",
		Name:             "Final Response Match (LLM Judge)",
		Description:      "LLM-as-judge evaluates semantic equivalence between agent response and golden reference. Not limited to lexical overlap like ROUGE.",
		Type:             MetricResponse,
		RangeMin:         0,
		RangeMax:         1,
		DefaultThreshold: 0.8,
		RequiresLLM:      true,
		RequiresVertex:   false,
		Category:         "advanced",
	},
	{
		ID:               "rubric_based_final_response_quality_v1",
		Name:             "Response Quality (Rubric)",
		Description:      "Evaluates final response quality against user-defined rubrics with score levels. LLM-as-judge with majority vote aggregation.",
		Type:             MetricRubric,
		RangeMin:         1,
		RangeMax:         5,
		DefaultThreshold: 3.0,
		RequiresLLM:      true,
		RequiresVertex:   false,
		Category:         "advanced",
	},
	{
		ID:               "rubric_based_tool_use_quality_v1",
		Name:             "Tool Use Quality (Rubric)",
		Description:      "Evaluates tool use quality against user-defined rubrics. LLM-as-judge with majority vote aggregation.",
		Type:             MetricRubric,
		RangeMin:         1,
		RangeMax:         5,
		DefaultThreshold: 3.0,
		RequiresLLM:      true,
		RequiresVertex:   false,
		Category:         "advanced",
	},
	{
		ID:               "per_turn_user_simulator_quality_v1",
		Name:             "User Simulator Quality",
		Description:      "Evaluates how well the LLM-backed user simulator generates realistic multi-turn conversations.",
		Type:             MetricSimulation,
		RangeMin:         1,
		RangeMax:         5,
		DefaultThreshold: 3.0,
		RequiresLLM:      true,
		RequiresVertex:   false,
		Category:         "advanced",
	},
	{
		ID:               "multi_turn_task_success_v1",
		Name:             "Multi-Turn Task Success",
		Description:      "Evaluates whether the agent accomplished the overall task across a multi-turn conversation.",
		Type:             MetricMultiTurn,
		RangeMin:         0,
		RangeMax:         1,
		DefaultThreshold: 0.8,
		RequiresLLM:      true,
		RequiresVertex:   false,
		Category:         "advanced",
	},
	{
		ID:               "multi_turn_trajectory_quality_v1",
		Name:             "Multi-Turn Trajectory Quality",
		Description:      "Evaluates the quality of tool call trajectories across all turns in a multi-turn conversation.",
		Type:             MetricMultiTurn,
		RangeMin:         0,
		RangeMax:         1,
		DefaultThreshold: 0.8,
		RequiresLLM:      false,
		RequiresVertex:   false,
		Category:         "advanced",
	},
	{
		ID:               "multi_turn_tool_use_quality_v1",
		Name:             "Multi-Turn Tool Use Quality",
		Description:      "Evaluates the quality of individual tool calls across all turns in a multi-turn conversation.",
		Type:             MetricMultiTurn,
		RangeMin:         0,
		RangeMax:         1,
		DefaultThreshold: 0.8,
		RequiresLLM:      false,
		RequiresVertex:   false,
		Category:         "advanced",
	},
}

// GetMetricCatalog returns all 12 prebuilt metrics sorted by category (core first) then ID.
func GetMetricCatalog() []MetricDefinition {
	result := make([]MetricDefinition, len(metricCatalog))
	copy(result, metricCatalog)

	sort.Slice(result, func(i, j int) bool {
		if result[i].Category != result[j].Category {
			return result[i].Category == "core"
		}
		return result[i].ID < result[j].ID
	})
	return result
}

// GetMetricByID returns a metric definition by ID, or false if not found.
func GetMetricByID(id string) (MetricDefinition, bool) {
	for _, m := range metricCatalog {
		if m.ID == id {
			return m, true
		}
	}
	return MetricDefinition{}, false
}

// GetMetricsByType returns all metrics of the given type.
func GetMetricsByType(t MetricType) []MetricDefinition {
	var result []MetricDefinition
	for _, m := range metricCatalog {
		if m.Type == t {
			result = append(result, m)
		}
	}
	return result
}
