package tts

import (
	"context"
	"encoding/json"
	"testing"

	"school-with-ai/foundation/agent"
)

func TestSpeakTool_Name(t *testing.T) {
	tool := NewSpeakTool(nil, "")
	if got := tool.Name(); got != "speak" {
		t.Errorf("Name() = %q, want %q", got, "speak")
	}
}

func TestSpeakTool_Schema(t *testing.T) {
	tool := NewSpeakTool(nil, "")
	schema := tool.Schema()
	if schema == nil {
		t.Fatal("Schema() returned nil")
	}
	if schema.Type != agent.TypeObject {
		t.Errorf("Schema type = %q, want %q", schema.Type, agent.TypeObject)
	}
	if _, ok := schema.Properties["text"]; !ok {
		t.Error("Schema missing 'text' property")
	}
	if _, ok := schema.Properties["voice"]; !ok {
		t.Error("Schema missing 'voice' property")
	}
	if _, ok := schema.Properties["instructions"]; !ok {
		t.Error("Schema missing 'instructions' property")
	}
}

func TestSpeakTool_Execute_EmptyText(t *testing.T) {
	tool := NewSpeakTool(nil, "test-key")
	args := agent.Arguments{
		Raw: json.RawMessage(`{"text": ""}`),
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Error == nil {
		t.Error("Expected error for empty text, got nil")
	}
}

func TestSpeakTool_Execute_TextTooLong(t *testing.T) {
	tool := NewSpeakTool(nil, "test-key")
	longText := make([]byte, 4097)
	for i := range longText {
		longText[i] = 'a'
	}
	rawJSON, _ := json.Marshal(map[string]string{"text": string(longText)})
	args := agent.Arguments{
		Raw: json.RawMessage(rawJSON),
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Error == nil {
		t.Error("Expected error for text too long, got nil")
	}
}

func TestSpeakTool_Execute_MissingAPIKey(t *testing.T) {
	tool := NewSpeakTool(nil, "")
	args := agent.Arguments{
		Raw: json.RawMessage(`{"text": "Hello world"}`),
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Error == nil {
		t.Error("Expected error for missing API key, got nil")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"coral", "coral"},
		{"ECHO", "echo"},
		{"my voice!", "myvoice"},
		{"", "audio"},
		{"marin-2", "marin-2"},
	}
	for _, tt := range tests {
		if got := sanitizeFilename(tt.input); got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWordTimestamp_JSONRoundtrip(t *testing.T) {
	original := []WordTimestamp{
		{Word: "Hello", Start: 0.0, End: 0.48},
		{Word: "world", Start: 0.52, End: 1.1},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded []WordTimestamp
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(decoded) != 2 {
		t.Fatalf("got %d timestamps, want 2", len(decoded))
	}
	if decoded[0].Word != "Hello" || decoded[0].Start != 0.0 || decoded[0].End != 0.48 {
		t.Errorf("decoded[0] = %+v, want {Hello 0 0.48}", decoded[0])
	}
	if decoded[1].Word != "world" || decoded[1].Start != 0.52 || decoded[1].End != 1.1 {
		t.Errorf("decoded[1] = %+v, want {world 0.52 1.1}", decoded[1])
	}
}

func TestWhisperResponse_Decode(t *testing.T) {
	raw := `{
		"text": "Hello world",
		"words": [
			{"word": "Hello", "start": 0.0, "end": 0.48},
			{"word": "world", "start": 0.52, "end": 1.1}
		]
	}`
	var resp whisperResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.Text != "Hello world" {
		t.Errorf("Text = %q, want %q", resp.Text, "Hello world")
	}
	if len(resp.Words) != 2 {
		t.Fatalf("Words len = %d, want 2", len(resp.Words))
	}
	if resp.Words[0].Word != "Hello" {
		t.Errorf("Words[0].Word = %q, want %q", resp.Words[0].Word, "Hello")
	}
}
