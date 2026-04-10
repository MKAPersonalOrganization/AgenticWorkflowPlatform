package agentdefinition

import (
	"encoding/json"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// AgentModelConfig — validation
// ──────────────────────────────────────────────────────────────────────────────

func TestAgentModelConfig_Validate(t *testing.T) {
	f64 := func(v float64) *float64 { return &v }
	f32 := func(v float32) *float32 { return &v }
	i32 := func(v int32) *int32 { return &v }

	t.Run("nil config is valid", func(t *testing.T) {
		var cfg *AgentModelConfig
		if err := cfg.Validate(); err != nil {
			t.Errorf("nil config should be valid, got: %v", err)
		}
	})

	t.Run("zero-value config is valid", func(t *testing.T) {
		cfg := &AgentModelConfig{}
		if err := cfg.Validate(); err != nil {
			t.Errorf("zero config should be valid, got: %v", err)
		}
	})

	t.Run("valid temperature boundaries", func(t *testing.T) {
		for _, temp := range []float64{0.0, 0.7, 1.0, 2.0} {
			cfg := &AgentModelConfig{Temperature: f64(temp)}
			if err := cfg.Validate(); err != nil {
				t.Errorf("temperature %g should be valid, got: %v", temp, err)
			}
		}
	})

	t.Run("temperature below 0 is rejected", func(t *testing.T) {
		cfg := &AgentModelConfig{Temperature: f64(-0.1)}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for temperature -0.1")
		}
	})

	t.Run("temperature above 2 is rejected", func(t *testing.T) {
		cfg := &AgentModelConfig{Temperature: f64(2.1)}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for temperature 2.1")
		}
	})

	t.Run("valid topP boundaries", func(t *testing.T) {
		for _, p := range []float64{0.0, 0.5, 1.0} {
			cfg := &AgentModelConfig{TopP: f64(p)}
			if err := cfg.Validate(); err != nil {
				t.Errorf("topP %g should be valid, got: %v", p, err)
			}
		}
	})

	t.Run("topP above 1 is rejected", func(t *testing.T) {
		cfg := &AgentModelConfig{TopP: f64(1.1)}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for topP 1.1")
		}
	})

	t.Run("topP below 0 is rejected", func(t *testing.T) {
		cfg := &AgentModelConfig{TopP: f64(-0.1)}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for topP -0.1")
		}
	})

	t.Run("topK zero is rejected", func(t *testing.T) {
		cfg := &AgentModelConfig{TopK: f32(0)}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for topK 0")
		}
	})

	t.Run("topK 1 is valid", func(t *testing.T) {
		cfg := &AgentModelConfig{TopK: f32(1)}
		if err := cfg.Validate(); err != nil {
			t.Errorf("topK 1 should be valid, got: %v", err)
		}
	})

	t.Run("maxOutputTokens zero is rejected", func(t *testing.T) {
		cfg := &AgentModelConfig{MaxOutputTokens: i32(0)}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for maxOutputTokens 0")
		}
	})

	t.Run("maxOutputTokens 1 is valid", func(t *testing.T) {
		cfg := &AgentModelConfig{MaxOutputTokens: i32(1)}
		if err := cfg.Validate(); err != nil {
			t.Errorf("maxOutputTokens 1 should be valid, got: %v", err)
		}
	})

	t.Run("maxOutputTokens 65536 is valid", func(t *testing.T) {
		cfg := &AgentModelConfig{MaxOutputTokens: i32(65536)}
		if err := cfg.Validate(); err != nil {
			t.Errorf("maxOutputTokens 65536 should be valid, got: %v", err)
		}
	})

	t.Run("maxOutputTokens above 65536 is rejected", func(t *testing.T) {
		cfg := &AgentModelConfig{MaxOutputTokens: i32(65537)}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for maxOutputTokens 65537")
		}
	})

	t.Run("all fields valid together", func(t *testing.T) {
		cfg := &AgentModelConfig{
			Temperature:     f64(0.7),
			TopP:            f64(0.9),
			TopK:            f32(40),
			MaxOutputTokens: i32(4096),
			StopSequences:   []string{"END", "STOP"},
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("fully populated config should be valid, got: %v", err)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// AgentDefinition — MarshalModelConfig / UnmarshalModelConfig round-trip
// ──────────────────────────────────────────────────────────────────────────────

func TestAgentDefinition_ModelConfigRoundTrip(t *testing.T) {
	f64 := func(v float64) *float64 { return &v }
	f32 := func(v float32) *float32 { return &v }
	i32 := func(v int32) *int32 { return &v }

	t.Run("nil config marshals to empty string", func(t *testing.T) {
		a := &AgentDefinition{}
		if err := a.MarshalModelConfig(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.ModelConfigJSON != "" {
			t.Errorf("expected empty JSON, got %q", a.ModelConfigJSON)
		}
	})

	t.Run("empty JSON unmarshals to nil", func(t *testing.T) {
		a := &AgentDefinition{ModelConfigJSON: ""}
		if err := a.UnmarshalModelConfig(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.ModelConfig != nil {
			t.Error("expected nil ModelConfig for empty JSON")
		}
	})

	t.Run("null JSON unmarshals to nil", func(t *testing.T) {
		a := &AgentDefinition{ModelConfigJSON: "null"}
		if err := a.UnmarshalModelConfig(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.ModelConfig != nil {
			t.Error("expected nil ModelConfig for 'null' JSON")
		}
	})

	t.Run("full config round-trips correctly", func(t *testing.T) {
		original := &AgentModelConfig{
			Temperature:     f64(0.7),
			TopP:            f64(0.9),
			TopK:            f32(40),
			MaxOutputTokens: i32(4096),
			StopSequences:   []string{"END", "DONE"},
		}
		a := &AgentDefinition{ModelConfig: original}

		if err := a.MarshalModelConfig(); err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if a.ModelConfigJSON == "" {
			t.Fatal("expected non-empty JSON after marshal")
		}

		// Parse into a new definition.
		b := &AgentDefinition{ModelConfigJSON: a.ModelConfigJSON}
		if err := b.UnmarshalModelConfig(); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if b.ModelConfig == nil {
			t.Fatal("expected non-nil ModelConfig after unmarshal")
		}

		if *b.ModelConfig.Temperature != *original.Temperature {
			t.Errorf("Temperature: got %g, want %g", *b.ModelConfig.Temperature, *original.Temperature)
		}
		if *b.ModelConfig.TopP != *original.TopP {
			t.Errorf("TopP: got %g, want %g", *b.ModelConfig.TopP, *original.TopP)
		}
		if *b.ModelConfig.TopK != *original.TopK {
			t.Errorf("TopK: got %g, want %g", *b.ModelConfig.TopK, *original.TopK)
		}
		if *b.ModelConfig.MaxOutputTokens != *original.MaxOutputTokens {
			t.Errorf("MaxOutputTokens: got %d, want %d", *b.ModelConfig.MaxOutputTokens, *original.MaxOutputTokens)
		}
		if len(b.ModelConfig.StopSequences) != len(original.StopSequences) {
			t.Errorf("StopSequences length: got %d, want %d", len(b.ModelConfig.StopSequences), len(original.StopSequences))
		}
	})

	t.Run("partial config marshals only set fields", func(t *testing.T) {
		a := &AgentDefinition{ModelConfig: &AgentModelConfig{Temperature: f64(1.2)}}
		if err := a.MarshalModelConfig(); err != nil {
			t.Fatalf("marshal error: %v", err)
		}

		var m map[string]any
		if err := json.Unmarshal([]byte(a.ModelConfigJSON), &m); err != nil {
			t.Fatalf("could not parse JSON: %v", err)
		}
		if _, ok := m["temperature"]; !ok {
			t.Error("expected 'temperature' key in JSON")
		}
		if _, ok := m["topP"]; ok {
			t.Error("topP should be omitted when nil")
		}
		if _, ok := m["topK"]; ok {
			t.Error("topK should be omitted when nil")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// AgentModelConfig — JSON field names (frontend contract)
// ──────────────────────────────────────────────────────────────────────────────

func TestAgentModelConfig_JSONFieldNames(t *testing.T) {
	f64 := func(v float64) *float64 { return &v }
	f32 := func(v float32) *float32 { return &v }
	i32 := func(v int32) *int32 { return &v }

	cfg := &AgentModelConfig{
		Temperature:     f64(0.8),
		TopP:            f64(0.95),
		TopK:            f32(50),
		MaxOutputTokens: i32(2048),
		StopSequences:   []string{"STOP"},
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	expectedKeys := []string{"temperature", "topP", "topK", "maxOutputTokens", "stopSequences"}
	for _, key := range expectedKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}
