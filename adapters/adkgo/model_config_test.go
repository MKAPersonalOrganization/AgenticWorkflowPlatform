package adkgo

import (
	"testing"

	fagent "aicompass/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// buildGenerateContentConfig — per-agent model config (Gap 1)
//
// Verifies the three-layer precedence:
//   Layer 3 (lowest)  — user global ModelConfig
//   Layer 2           — per-agent AgentModelConfig
//   Layer 1 (highest) — per-request RunConfig
// ──────────────────────────────────────────────────────────────────────────────

func f64(v float64) *float64 { return &v }
func f32(v float32) *float32 { return &v }
func i32(v int32) *int32     { return &v }

func TestBuildGenerateContentConfig_AgentModelConfig(t *testing.T) {
	t.Run("nil agent config falls back to user global temperature", func(t *testing.T) {
		userGlobal := &fagent.ModelConfig{Temperature: f64(0.5)}
		gc := buildGenerateContentConfig(userGlobal, nil, nil)
		if gc.Temperature == nil {
			t.Fatal("expected non-nil temperature from user global")
		}
		if *gc.Temperature != float32(0.5) {
			t.Errorf("expected 0.5, got %f", *gc.Temperature)
		}
	})

	t.Run("agent temperature overrides user global temperature", func(t *testing.T) {
		userGlobal := &fagent.ModelConfig{Temperature: f64(0.5)}
		agentCfg := &fagent.AgentModelConfig{Temperature: f64(1.2)}

		gc := buildGenerateContentConfig(userGlobal, agentCfg, nil)
		if gc.Temperature == nil {
			t.Fatal("expected non-nil temperature")
		}
		if *gc.Temperature != float32(1.2) {
			t.Errorf("agent config should override user global: expected 1.2, got %f", *gc.Temperature)
		}
	})

	t.Run("run config temperature overrides agent config temperature", func(t *testing.T) {
		userGlobal := &fagent.ModelConfig{Temperature: f64(0.5)}
		agentCfg := &fagent.AgentModelConfig{Temperature: f64(1.2)}
		runCfg := &fagent.RunConfig{Temperature: 1.8}

		gc := buildGenerateContentConfig(userGlobal, agentCfg, runCfg)
		if gc.Temperature == nil {
			t.Fatal("expected non-nil temperature")
		}
		if *gc.Temperature != float32(1.8) {
			t.Errorf("run config should override agent config: expected 1.8, got %f", *gc.Temperature)
		}
	})

	t.Run("agent config topP is applied", func(t *testing.T) {
		agentCfg := &fagent.AgentModelConfig{TopP: f64(0.9)}
		gc := buildGenerateContentConfig(&fagent.ModelConfig{}, agentCfg, nil)
		if gc.TopP == nil {
			t.Fatal("expected non-nil topP")
		}
		if *gc.TopP != float32(0.9) {
			t.Errorf("expected topP 0.9, got %f", *gc.TopP)
		}
	})

	t.Run("agent config topK is applied", func(t *testing.T) {
		agentCfg := &fagent.AgentModelConfig{TopK: f32(40)}
		gc := buildGenerateContentConfig(&fagent.ModelConfig{}, agentCfg, nil)
		if gc.TopK == nil {
			t.Fatal("expected non-nil topK")
		}
		if *gc.TopK != 40 {
			t.Errorf("expected topK 40, got %g", *gc.TopK)
		}
	})

	t.Run("agent config maxOutputTokens overrides user global maxTokens", func(t *testing.T) {
		userGlobal := &fagent.ModelConfig{MaxTokens: func() *int { v := 2048; return &v }()}
		agentCfg := &fagent.AgentModelConfig{MaxOutputTokens: i32(512)}

		gc := buildGenerateContentConfig(userGlobal, agentCfg, nil)
		if gc.MaxOutputTokens != 512 {
			t.Errorf("agent config should override user global: expected 512, got %d", gc.MaxOutputTokens)
		}
	})

	t.Run("run config maxTokens overrides agent config maxOutputTokens", func(t *testing.T) {
		agentCfg := &fagent.AgentModelConfig{MaxOutputTokens: i32(512)}
		runCfg := &fagent.RunConfig{MaxTokens: 1024}

		gc := buildGenerateContentConfig(&fagent.ModelConfig{}, agentCfg, runCfg)
		if gc.MaxOutputTokens != 1024 {
			t.Errorf("run config should override agent config: expected 1024, got %d", gc.MaxOutputTokens)
		}
	})

	t.Run("agent config stop sequences are applied", func(t *testing.T) {
		agentCfg := &fagent.AgentModelConfig{StopSequences: []string{"END", "DONE"}}
		gc := buildGenerateContentConfig(&fagent.ModelConfig{}, agentCfg, nil)
		if len(gc.StopSequences) != 2 {
			t.Fatalf("expected 2 stop sequences, got %d", len(gc.StopSequences))
		}
		if gc.StopSequences[0] != "END" || gc.StopSequences[1] != "DONE" {
			t.Errorf("unexpected stop sequences: %v", gc.StopSequences)
		}
	})

	t.Run("run config stop sequences override agent config stop sequences", func(t *testing.T) {
		agentCfg := &fagent.AgentModelConfig{StopSequences: []string{"AGENT_STOP"}}
		runCfg := &fagent.RunConfig{StopSequences: []string{"REQUEST_STOP"}}

		gc := buildGenerateContentConfig(&fagent.ModelConfig{}, agentCfg, runCfg)
		if len(gc.StopSequences) != 1 || gc.StopSequences[0] != "REQUEST_STOP" {
			t.Errorf("run config should override agent stop sequences, got: %v", gc.StopSequences)
		}
	})

	t.Run("user global fields not touched by agent config are preserved", func(t *testing.T) {
		// User global has maxTokens. Agent config only sets temperature.
		// MaxOutputTokens must come from user global.
		userGlobal := &fagent.ModelConfig{MaxTokens: func() *int { v := 8192; return &v }()}
		agentCfg := &fagent.AgentModelConfig{Temperature: f64(0.3)}

		gc := buildGenerateContentConfig(userGlobal, agentCfg, nil)
		if gc.MaxOutputTokens != 8192 {
			t.Errorf("user global maxTokens should be preserved: expected 8192, got %d", gc.MaxOutputTokens)
		}
		if gc.Temperature == nil || *gc.Temperature != float32(0.3) {
			t.Errorf("agent temperature should be applied: expected 0.3")
		}
	})

	t.Run("all layers applied together respect full precedence chain", func(t *testing.T) {
		userGlobal := &fagent.ModelConfig{Temperature: f64(0.3), MaxTokens: func() *int { v := 1000; return &v }()}
		agentCfg := &fagent.AgentModelConfig{
			Temperature:     f64(0.7), // overrides user global
			TopP:            f64(0.95),
			MaxOutputTokens: i32(2000), // overrides user global maxTokens
		}
		runCfg := &fagent.RunConfig{
			Temperature: 1.5, // overrides agent config
		}

		gc := buildGenerateContentConfig(userGlobal, agentCfg, runCfg)

		// RunConfig wins for temperature.
		if gc.Temperature == nil || *gc.Temperature != float32(1.5) {
			t.Errorf("temperature: expected 1.5 from run config, got %v", gc.Temperature)
		}
		// AgentConfig wins for topP (not set by run config).
		if gc.TopP == nil || *gc.TopP != float32(0.95) {
			t.Errorf("topP: expected 0.95 from agent config, got %v", gc.TopP)
		}
		// AgentConfig wins for maxOutputTokens (run config MaxTokens is 0).
		if gc.MaxOutputTokens != 2000 {
			t.Errorf("maxOutputTokens: expected 2000 from agent config, got %d", gc.MaxOutputTokens)
		}
	})

	t.Run("empty agent config does not overwrite user global settings", func(t *testing.T) {
		userGlobal := &fagent.ModelConfig{Temperature: f64(0.6), MaxTokens: func() *int { v := 4096; return &v }()}
		agentCfg := &fagent.AgentModelConfig{} // all nil — no overrides

		gc := buildGenerateContentConfig(userGlobal, agentCfg, nil)
		if gc.Temperature == nil || *gc.Temperature != float32(0.6) {
			t.Errorf("empty agent config should not overwrite user temperature: expected 0.6")
		}
		if gc.MaxOutputTokens != 4096 {
			t.Errorf("empty agent config should not overwrite user maxTokens: expected 4096, got %d", gc.MaxOutputTokens)
		}
	})
}
