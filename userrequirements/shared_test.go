//go:build integration

// shared_test.go is the REST-only test harness.
//
// The rules are deliberately strict:
//
//   1. Tests MUST drive the app through its public HTTP surface. No direct
//      DB access, no internal-package imports that touch storage, no
//      backdoor endpoints.
//   2. Auth goes through POST /auth/dev-login — the only test-only
//      affordance the server exposes. It is gated by
//      APP_AUTH_DEV_LOGIN_TOKEN and returns a real signed school_session
//      cookie for a fixed debug user (APP_AUTH_DEV_LOGIN_USER_ID).
//   3. Everything after login is identical to what the browser UI does:
//      upsert AI settings, create an agent, send a message (SSE),
//      rehydrate the thread via the AG-UI read API.
//   4. Each test owns its own agents and threads and deletes them in
//      t.Cleanup via the same public endpoints the UI uses.
package userrequirements

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/require"
)

// baseURL is the public app URL. Tests hit it exactly like the UI does.
// Self-signed TLS in local clusters is accepted below via InsecureSkipVerify.
func baseURL() string {
	if u := os.Getenv("APP_TEST_BASE_URL"); u != "" {
		return u
	}
	return "https://localhost:8090"
}

// devLoginToken is the value the server expects in X-Dev-Login-Token. Tests
// read it from the same env var the server reads (APP_AUTH_DEV_LOGIN_TOKEN)
// so cluster and test always agree. An empty value means the endpoint is
// not mounted — tests skip rather than fail.
func devLoginToken(t *testing.T) string {
	t.Helper()
	tok := os.Getenv("APP_AUTH_DEV_LOGIN_TOKEN")
	if tok == "" {
		t.Skip("APP_AUTH_DEV_LOGIN_TOKEN unset — dev-login endpoint not available; bring up cluster with `task up:cluster`")
	}
	return tok
}

// authedClient returns an *http.Client that already carries a valid
// school_session cookie for the debug user. The cookie jar persists across
// every call the test makes, so subsequent requests (agent create, message
// send, thread read) flow through the normal auth middleware.
//
// If the cluster is unreachable or dev-login is disabled, the test is
// skipped — the CI contract is "require the cluster to be up" rather than
// spinning one up here.
func authedClient(t *testing.T) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)

	client := &http.Client{
		Jar: jar,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec — local self-signed cluster
		},
		Timeout: 120 * time.Second,
	}

	token := devLoginToken(t)

	req, err := http.NewRequest(http.MethodPost, baseURL()+"/auth/dev-login", nil)
	require.NoError(t, err)
	req.Header.Set("X-Dev-Login-Token", token)

	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("dev-login unreachable at %s: %v (bring up cluster with `task up:cluster`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /auth/dev-login returned %d: %s", resp.StatusCode, string(body))
	}

	return client
}

// upsertAISettings writes the provider + api_key for the debug user. The
// runner picks the user's "default" provider row (is_default=true), and
// the save endpoint only auto-sets default when there are ZERO prior rows
// — which won't hold for the shared debug user across test runs. To keep
// behaviour simple and deterministic, this helper first deletes every
// existing AI-settings row for the debug user (via the same public API
// the Settings pane calls), then creates the one this test needs. The
// fresh insert auto-defaults.
func upsertAISettings(t *testing.T, client *http.Client, provider, apiKey string) {
	t.Helper()

	deleteAllAISettings(t, client)

	body, err := json.Marshal(map[string]any{
		"provider": provider,
		"api_key":  apiKey,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPut, baseURL()+"/api/ai-settings", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("PUT /api/ai-settings returned %d: %s", resp.StatusCode, string(b))
	}
}

// deleteAllAISettings wipes the current user's AI-settings rows through
// the public API. Used by upsertAISettings to reset state so the fresh
// insert auto-defaults.
func deleteAllAISettings(t *testing.T, client *http.Client) {
	t.Helper()

	getReq, err := http.NewRequest(http.MethodGet, baseURL()+"/api/ai-settings", nil)
	require.NoError(t, err)
	getResp, err := client.Do(getReq)
	require.NoError(t, err)
	defer getResp.Body.Close()

	if getResp.StatusCode >= 300 {
		b, _ := io.ReadAll(getResp.Body)
		t.Fatalf("GET /api/ai-settings returned %d: %s", getResp.StatusCode, string(b))
	}

	var existing []struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&existing))

	for _, e := range existing {
		if e.ID == "" {
			continue
		}
		delReq, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/ai-settings/%s", baseURL(), url.PathEscape(e.ID)), nil)
		require.NoError(t, err)
		delResp, err := client.Do(delReq)
		require.NoError(t, err)
		_ = delResp.Body.Close()
	}
}

// createAgent POSTs to /v1/agent-definitions and returns the new agent's ID.
// The body is whatever the UI would send — the test composes a map so each
// test can express only the fields it cares about (systemPrompt,
// modelConfig, codeExecutionPolicy, etc.).
//
// A cleanup is registered: the agent is DELETEd at the end of the test via
// the same public endpoint.
func createAgent(t *testing.T, client *http.Client, body map[string]any) string {
	t.Helper()

	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL()+"/v1/agent-definitions", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /v1/agent-definitions returned %d: %s", resp.StatusCode, string(b))
	}

	var decoded struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
	require.NotEmpty(t, decoded.ID, "POST /v1/agent-definitions did not return an id")

	t.Cleanup(func() {
		deleteAgent(t, client, decoded.ID)
	})

	return decoded.ID
}

// deleteAgent hits DELETE /v1/agent-definitions/{id}. Always best-effort —
// the test's assertion has already completed; we just want state gone.
func deleteAgent(t *testing.T, client *http.Client, agentID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, baseURL()+"/v1/agent-definitions/"+url.PathEscape(agentID), nil)
	if err != nil {
		t.Logf("deleteAgent: building request failed: %v", err)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("deleteAgent: request failed: %v", err)
		return
	}
	_ = resp.Body.Close()
}

// sseEvent is one parsed Server-Sent Event as emitted by /v1/message:send.
// Event is the `event:` line ("" for data-only events — the chat stream
// uses the default `message` type for every frame). Data is the decoded
// `data:` line as JSON; callers typically unmarshal it further.
type sseEvent struct {
	Event string
	Data  json.RawMessage
}

// Type returns the raw AG-UI `type` field ("RUN_STARTED",
// "TEXT_MESSAGE_CONTENT", "CUSTOM", etc.). Note that domain events like
// CODE_EXECUTION_RAN are transported as CUSTOM frames — use Name() to
// route by the logical event name in that case.
func (e sseEvent) Type() string {
	if len(e.Data) == 0 {
		return ""
	}
	var envelope struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(e.Data, &envelope)
	return envelope.Type
}

// Name returns the routable identifier for this frame — the `name` field
// on CUSTOM events (e.g. "CODE_EXECUTION_RAN", "TOKEN_USAGE_REPORTED"),
// otherwise the `type` field. This matches how the frontend routes SSE
// frames: CUSTOM frames are demultiplexed by `name`.
func (e sseEvent) Name() string {
	if len(e.Data) == 0 {
		return ""
	}
	var envelope struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	_ = json.Unmarshal(e.Data, &envelope)
	if envelope.Type == "CUSTOM" && envelope.Name != "" {
		return envelope.Name
	}
	return envelope.Type
}

// Value returns the `value` field of a CUSTOM event decoded into the
// provided destination. For non-custom events it's a no-op (leaves dst
// untouched). Intended for quick extraction in tests:
//
//	var payload struct { Code string `json:"code"` }
//	evt.Value(&payload)
func (e sseEvent) Value(dst any) error {
	var envelope struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(e.Data, &envelope); err != nil {
		return err
	}
	if len(envelope.Value) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Value, dst)
}

// sendMessage drives POST /v1/message:send with the given contextID (empty
// → server generates one), agentName, and message text. It blocks until
// the server closes the SSE stream, collects every frame, and returns
// (threadID, frames).
//
// threadID comes from the first SESSION_START frame the server emits
// before running the agent — the same frame the UI reads to populate the
// URL after a new chat starts. The test uses it to correlate cleanup and
// to call /api/agui/threads/{id}/messages for refresh-survival
// assertions.
func sendMessage(t *testing.T, client *http.Client, contextID, agentName, message string) (string, []sseEvent) {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"contextId": contextID,
		"message":   message,
		"agentName": agentName,
	})
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, baseURL()+"/v1/message:send", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /v1/message:send returned %d: %s", resp.StatusCode, string(b))
	}

	events, threadID := parseSSE(t, resp.Body)
	if threadID != "" {
		t.Cleanup(func() { deleteThread(t, client, threadID) })
	}
	return threadID, events
}

// parseSSE walks the text/event-stream response, yielding one sseEvent per
// `event: / data:` pair. The chat stream delimits frames with a blank line.
// The first frame is { "type": "SESSION_START", "contextId": "..." } —
// parseSSE extracts the context ID and returns it alongside the frames,
// since callers universally need it.
func parseSSE(t *testing.T, body io.Reader) ([]sseEvent, string) {
	t.Helper()

	var (
		events   []sseEvent
		threadID string
	)

	scanner := bufio.NewScanner(body)
	// SSE frames can carry larger JSON than the default 64KB buffer — bump it.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	var current sseEvent
	var dataBuf strings.Builder

	flush := func() {
		if dataBuf.Len() == 0 && current.Event == "" {
			return
		}
		if dataBuf.Len() > 0 {
			current.Data = json.RawMessage(dataBuf.String())
		}
		// Pull out threadID from the SESSION_START frame.
		if threadID == "" {
			var envelope struct {
				Type      string `json:"type"`
				ContextID string `json:"contextId"`
			}
			if err := json.Unmarshal(current.Data, &envelope); err == nil && envelope.Type == "SESSION_START" {
				threadID = envelope.ContextID
			}
		}
		events = append(events, current)
		current = sseEvent{}
		dataBuf.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			flush()
		case strings.HasPrefix(line, "event:"):
			current.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	// Handle trailing frame without a blank line.
	flush()

	if err := scanner.Err(); err != nil {
		t.Fatalf("SSE scan: %v", err)
	}
	return events, threadID
}

// readThreadMessages hits GET /api/agui/threads/{id}/messages — the same
// endpoint the UI calls on a page refresh. The response carries messages
// AND the `codeExecutions` sidecar that drives the inline code card. Tests
// use this to assert persistence (i.e. "does the card survive a refresh?")
// without touching the DB.
func readThreadMessages(t *testing.T, client *http.Client, threadID string) map[string]any {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/agui/threads/%s/messages", baseURL(), url.PathEscape(threadID)), nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /api/agui/threads/%s/messages returned %d: %s", threadID, resp.StatusCode, string(b))
	}

	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

// deleteThread hits DELETE /api/agui/threads/{id}. Best-effort cleanup.
func deleteThread(t *testing.T, client *http.Client, threadID string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/agui/threads/%s", baseURL(), url.PathEscape(threadID)), nil)
	if err != nil {
		t.Logf("deleteThread: building request failed: %v", err)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Logf("deleteThread: request failed: %v", err)
		return
	}
	_ = resp.Body.Close()
}

// requireOpenAIKey returns a real OpenAI key or skips the test.
func requireOpenAIKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	return key
}

// requireGeminiKey returns a real Gemini API key or skips the test.
// Accepts either GEMINI_API_KEY or GOOGLE_API_KEY.
func requireGeminiKey(t *testing.T) string {
	t.Helper()
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		return key
	}
	t.Skip("GEMINI_API_KEY / GOOGLE_API_KEY not set")
	return ""
}

// agentBody returns a starter body for POST /v1/agent-definitions with the
// bits every test needs (temperature=0 for determinism, provider+model
// pinned so the run doesn't depend on whatever the debug user's global
// AI-settings default happens to be). Tests merge-in spec-specific fields
// on top — systemPrompt, outputSchema, memoryPolicy, graph, etc.
func agentBody(name, systemPrompt, provider, model string) map[string]any {
	return map[string]any{
		"name":         name,
		"systemPrompt": systemPrompt,
		"icon":         "flag",
		"modelConfig":  map[string]any{"temperature": 0.0},
		"modelOverride": map[string]any{
			"provider":  provider,
			"modelName": model,
		},
	}
}

// assistantFinalText concatenates every TEXT_MESSAGE_CONTENT.delta in order
// into the assistant's final reply. This is what the UI would render after
// the stream closes, so assertions over text output use it as the ground
// truth. (AssistantMessageAdded does not cross the SSE wire — see
// modules/chat/handlers/common/converter.go.)
func assistantFinalText(events []sseEvent) string {
	var b strings.Builder
	for _, e := range events {
		if e.Name() != "TEXT_MESSAGE_CONTENT" {
			continue
		}
		var p struct {
			Delta string `json:"delta"`
		}
		_ = json.Unmarshal(e.Data, &p)
		b.WriteString(p.Delta)
	}
	return b.String()
}

// findToolStarts decodes every TOOL_START frame's `value` payload. Keys
// the caller usually cares about: toolName, toolCallId, arguments (as a
// JSON string), agentName.
func findToolStarts(events []sseEvent) []map[string]any {
	var out []map[string]any
	for _, e := range events {
		if e.Name() != "TOOL_START" {
			continue
		}
		var m map[string]any
		if err := e.Value(&m); err == nil && m != nil {
			out = append(out, m)
		}
	}
	return out
}

// runErrorMessage returns the first RUN_ERROR frame's message, or "" if
// the run finished cleanly. Tests that expect a run to fail inspect this
// to confirm the failure came from the right place (template-variable
// expansion, tool refusal, etc.).
func runErrorMessage(events []sseEvent) string {
	for _, e := range events {
		if e.Name() != "RUN_ERROR" {
			continue
		}
		var p struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(e.Data, &p)
		return p.Message
	}
	return ""
}

// validateAgainstSchema fails the test when the given instance violates
// the JSON Schema. Kept from the previous harness — specs exercising the
// agent's output_schema still need it.
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
