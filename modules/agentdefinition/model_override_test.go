package agentdefinition

import (
	"encoding/json"
	"testing"
)

// ──────────────────────────────────────────────────────────────────────────────
// AgentModelOverride — validation
// ──────────────────────────────────────────────────────────────────────────────

func TestAgentModelOverride_Validate(t *testing.T) {
	t.Parallel()

	t.Run("nil override is valid", func(t *testing.T) {
		t.Parallel()
		var nilOverride *AgentModelOverride
		if err := nilOverride.Validate(); err != nil {
			t.Errorf("nil override should be valid, got: %v", err)
		}
	})

	t.Run("both fields set is valid", func(t *testing.T) {
		t.Parallel()
		fullyPopulatedOverride := &AgentModelOverride{Provider: "gemini", ModelName: "gemini-2.0-flash"}
		if err := fullyPopulatedOverride.Validate(); err != nil {
			t.Errorf("fully populated override should be valid, got: %v", err)
		}
	})

	t.Run("missing provider is rejected", func(t *testing.T) {
		t.Parallel()
		overrideWithNoProvider := &AgentModelOverride{ModelName: "gpt-4o"}
		if err := overrideWithNoProvider.Validate(); err == nil {
			t.Error("expected error for missing provider")
		}
	})

	t.Run("missing modelName is rejected", func(t *testing.T) {
		t.Parallel()
		overrideWithNoModelName := &AgentModelOverride{Provider: "openai"}
		if err := overrideWithNoModelName.Validate(); err == nil {
			t.Error("expected error for missing modelName")
		}
	})

	t.Run("both empty is rejected", func(t *testing.T) {
		t.Parallel()
		emptyOverride := &AgentModelOverride{}
		if err := emptyOverride.Validate(); err == nil {
			t.Error("expected error for both empty fields")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// AgentDefinition — MarshalModelOverride / UnmarshalModelOverride round-trip
// ──────────────────────────────────────────────────────────────────────────────

func TestAgentDefinition_ModelOverrideRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("nil override marshals to empty string", func(t *testing.T) {
		t.Parallel()
		agentWithNoOverride := &AgentDefinition{}
		if err := agentWithNoOverride.MarshalModelOverride(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agentWithNoOverride.ModelOverrideJSON != "" {
			t.Errorf("expected empty JSON, got %q", agentWithNoOverride.ModelOverrideJSON)
		}
	})

	t.Run("empty JSON unmarshals to nil", func(t *testing.T) {
		t.Parallel()
		agentWithEmptyJSON := &AgentDefinition{ModelOverrideJSON: ""}
		if err := agentWithEmptyJSON.UnmarshalModelOverride(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agentWithEmptyJSON.ModelOverride != nil {
			t.Error("expected nil ModelOverride for empty JSON")
		}
	})

	t.Run("null JSON unmarshals to nil", func(t *testing.T) {
		t.Parallel()
		agentWithNullJSON := &AgentDefinition{ModelOverrideJSON: "null"}
		if err := agentWithNullJSON.UnmarshalModelOverride(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if agentWithNullJSON.ModelOverride != nil {
			t.Error("expected nil ModelOverride for 'null' JSON")
		}
	})

	t.Run("full override round-trips correctly", func(t *testing.T) {
		t.Parallel()
		originalOverride := &AgentModelOverride{Provider: "anthropic", ModelName: "claude-3-opus-20240229"}
		agentBeforeMarshal := &AgentDefinition{ModelOverride: originalOverride}

		if err := agentBeforeMarshal.MarshalModelOverride(); err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if agentBeforeMarshal.ModelOverrideJSON == "" {
			t.Fatal("expected non-empty JSON after marshal")
		}

		agentAfterUnmarshal := &AgentDefinition{ModelOverrideJSON: agentBeforeMarshal.ModelOverrideJSON}
		if err := agentAfterUnmarshal.UnmarshalModelOverride(); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if agentAfterUnmarshal.ModelOverride == nil {
			t.Fatal("expected non-nil ModelOverride after unmarshal")
		}
		if agentAfterUnmarshal.ModelOverride.Provider != originalOverride.Provider {
			t.Errorf("Provider: got %q, want %q", agentAfterUnmarshal.ModelOverride.Provider, originalOverride.Provider)
		}
		if agentAfterUnmarshal.ModelOverride.ModelName != originalOverride.ModelName {
			t.Errorf("ModelName: got %q, want %q", agentAfterUnmarshal.ModelOverride.ModelName, originalOverride.ModelName)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// AgentModelOverride — JSON field names (frontend contract)
// ──────────────────────────────────────────────────────────────────────────────

func TestAgentModelOverride_JSONFieldNames(t *testing.T) {
	t.Parallel()

	geminiOverride := &AgentModelOverride{Provider: "gemini", ModelName: "gemini-2.0-flash"}

	serialized, err := json.Marshal(geminiOverride)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var fieldMap map[string]any
	if err := json.Unmarshal(serialized, &fieldMap); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	for _, expectedKey := range []string{"provider", "modelName"} {
		if _, ok := fieldMap[expectedKey]; !ok {
			t.Errorf("expected JSON key %q to be present", expectedKey)
		}
	}

	if v, ok := fieldMap["provider"]; !ok || v != "gemini" {
		t.Errorf("provider: got %v, want %q", v, "gemini")
	}
	if v, ok := fieldMap["modelName"]; !ok || v != "gemini-2.0-flash" {
		t.Errorf("modelName: got %v, want %q", v, "gemini-2.0-flash")
	}
}
