//go:build integration

package userrequirements

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
)

// debugRequest mirrors debughandler.DebugRequest. Keeping a local copy
// keeps the test suite free of a module-code import — the tests treat
// the running service as a black box.
type debugRequest struct {
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	APIKey      string          `json:"api_key"`
	Temperature *float64        `json:"temperature,omitempty"`
	UserID      string          `json:"user_id,omitempty"`
	ThreadID    string          `json:"thread_id,omitempty"`
	Prompt      string          `json:"prompt"`
	Agent       json.RawMessage `json:"agent,omitempty"`
	AgentYAML   string          `json:"agent_yaml,omitempty"`
}

// debugResponse mirrors debughandler.DebugResponse. Event payloads are kept
// as raw JSON so each test can reach into whichever fields it cares about
// without bloating a shared type.
type debugResponse struct {
	ThreadID        string      `json:"thread_id"`
	RunID           string      `json:"run_id"`
	AgentID         string      `json:"agent_id"`
	Response        string      `json:"response"`
	EventsEmitted   []wireEvent `json:"events_emitted"`
	EventsPersisted []wireEvent `json:"events_persisted"`
	EventsTransient []wireEvent `json:"events_transient"`
	Errors          []string    `json:"errors"`
}

type wireEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// postDebugRun drives POST /debug/agent:run and decodes the JSON response.
// It skips (not fails) when the endpoint is unreachable so the whole suite
// no-ops gracefully when the cluster is down.
func postDebugRun(t *testing.T, req debugRequest) debugResponse {
	t.Helper()

	raw, err := json.Marshal(req)
	require.NoError(t, err)

	httpReq, err := http.NewRequest(http.MethodPost, endpointURL(), bytes.NewReader(raw))
	require.NoError(t, err)
	httpReq.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		t.Skipf("debug endpoint %s not reachable: %v (bring up the cluster with `task up:cluster`)", endpointURL(), err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	require.NoError(t, err)

	if httpResp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s returned %d: %s", endpointURL(), httpResp.StatusCode, string(body))
	}

	var out debugResponse
	require.NoError(t, json.Unmarshal(body, &out),
		"malformed debug response: %s", string(body))
	return out
}

// endpointURL returns the debug endpoint URL, defaulting to the loopback
// port wired in docker-compose.cluster.yml → migrate.ports.
func endpointURL() string {
	if u := os.Getenv("DEBUG_AGENT_RUN_URL"); u != "" {
		return u
	}
	return "http://127.0.0.1:4015/debug/agent:run"
}

// requireOpenAIKey returns a real OpenAI key or skips the test. Use from
// tests that hit OpenAI directly so the skip reason is clear.
func requireOpenAIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	return key
}

// validateAgainstSchema fails the test when the given instance violates the
// JSON Schema. The schema is parsed via the vendored
// github.com/google/jsonschema-go library, the same validator the product
// uses internally for tool-argument checking.
func validateAgainstSchema(t *testing.T, schemaJSON string, instance any) {
	t.Helper()

	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal([]byte(schemaJSON), &schema),
		"test schema literal is malformed")

	resolved, err := schema.Resolve(nil)
	require.NoError(t, err, "failed to resolve schema")

	require.NoError(t, resolved.Validate(instance),
		"instance violates the declared schema: %+v", instance)
}

// findEventsByType returns every event in the list whose Type equals the
// given string. Useful for filtering the emitted trace to a specific event
// kind without walking the whole slice inline.
func findEventsByType(events []wireEvent, eventType string) []wireEvent {
	out := make([]wireEvent, 0)
	for _, e := range events {
		if e.Type == eventType {
			out = append(out, e)
		}
	}
	return out
}
