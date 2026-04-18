// Package debughandler serves the internal, unauthenticated agent-debug HTTP
// endpoint. It is mounted on the debug mux (cfg.Web.DebugHost, default :4010),
// the same port as pprof/expvar, and is intended for developer integration
// testing without the web UI.
//
// A single request carries everything the runner needs — provider, model,
// API key, agent definition (JSON or YAML catalog), and user prompt — so
// nothing has to be pre-seeded in the database. The response bundles both
// the events emitted during the run and the events persisted to the
// PostgreSQL event store under the synthetic thread so callers can assert
// against the full trace.
package debughandler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"aicompass/adapters/adkgo"
	fagent "aicompass/foundation/agent"
	es "aicompass/foundation/eventsourcing"
	"aicompass/foundation/logger"
	"aicompass/modules/agentdefinition"
	"aicompass/modules/chat/thread"
	"aicompass/modules/chat/threadmgmt"

	"google.golang.org/adk/memory"
)

// Handler serves POST /debug/agent:run.
type Handler struct {
	log       *logger.Logger
	threadMgr *threadmgmt.Manager
	store     es.EventStore

	// memoryService is held once per Handler (process lifetime), not per
	// request, so sessions added after one /debug/agent:run can be recalled
	// from a later /debug/agent:run with the same UserID. Without this,
	// every debug run would get a fresh InMemoryService and cross-session
	// recall would be impossible to test end-to-end. Matches how production
	// wires a single memory.Service for the whole process.
	memoryService memory.Service
}

// New constructs a debug handler. The caller is expected to mount the returned
// handler on the debug mux, not the public app router.
func New(log *logger.Logger, threadMgr *threadmgmt.Manager, store es.EventStore) *Handler {
	return &Handler{
		log:           log,
		threadMgr:     threadMgr,
		store:         store,
		memoryService: memory.InMemoryService(),
	}
}

// DebugRequest is the JSON body accepted by POST /debug/agent:run.
//
// Provider/Model/APIKey go straight to adkgo.NewModel for this run — no
// lookup against user_ai_settings — so no user needs to exist in the DB.
// Exactly one of Agent (JSON) or AgentYAML (catalog YAML string) must be set.
type DebugRequest struct {
	Provider    string   `json:"provider"`
	Model       string   `json:"model"`
	APIKey      string   `json:"api_key"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`

	UserID   string `json:"user_id,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
	Prompt   string `json:"prompt"`

	Agent     json.RawMessage `json:"agent,omitempty"`
	AgentYAML string          `json:"agent_yaml,omitempty"`
}

// DebugResponse is the JSON body returned from POST /debug/agent:run.
//
// EventsEmitted is the full run trace (everything yielded by RunnerAdapter).
// EventsPersisted is what was written to the event store under the synthetic
// thread. EventsTransient is the multiset difference (emitted-minus-persisted),
// i.e. events that flowed during the run but never hit the DB — streaming
// assistant chunks, tool-start pings, agent-transfer markers, etc.
// Request echoes the decoded input (api_key redacted) so integration tests
// can log the exact body they received without re-marshalling.
type DebugResponse struct {
	Request         *DebugInputEcho `json:"request"`
	ThreadID        string          `json:"thread_id"`
	RunID           string          `json:"run_id"`
	AgentID         string          `json:"agent_id"`
	Response        string          `json:"response"`
	EventsEmitted   []wireEvent     `json:"events_emitted"`
	EventsPersisted []wireEvent     `json:"events_persisted"`
	EventsTransient []wireEvent     `json:"events_transient"`
	TokenUsage      *wireTokenUsage `json:"token_usage,omitempty"`
	DurationMs      int64           `json:"duration_ms"`
	Errors          []string        `json:"errors,omitempty"`
}

// DebugInputEcho mirrors DebugRequest but redacts the API key. It is sent
// back verbatim in the response so callers can correlate a reply with the
// body they posted without extra bookkeeping.
type DebugInputEcho struct {
	Provider    string          `json:"provider"`
	Model       string          `json:"model"`
	APIKey      string          `json:"api_key"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	UserID      string          `json:"user_id,omitempty"`
	ThreadID    string          `json:"thread_id,omitempty"`
	Prompt      string          `json:"prompt"`
	Agent       json.RawMessage `json:"agent,omitempty"`
	AgentYAML   string          `json:"agent_yaml,omitempty"`
}

type wireEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type wireTokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// HandleRun executes the agent end-to-end and writes the JSON response.
// It is intentionally not streaming — integration tests are easier to write
// against a single JSON body than against an SSE stream.
func (h *Handler) HandleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DebugRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON body: %v", err))
		return
	}
	if req.Provider == "" || req.Model == "" || req.APIKey == "" {
		writeErr(w, http.StatusBadRequest, "provider, model, and api_key are required")
		return
	}
	if req.Prompt == "" {
		writeErr(w, http.StatusBadRequest, "prompt is required")
		return
	}

	rootID, defs, err := parseAgents(&req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	userID := req.UserID
	if userID == "" {
		userID = "debug-user"
	}
	threadID := req.ThreadID
	if threadID == "" {
		threadID = "debug-" + fagent.GenerateID()
	}

	repo := newMemRepo(defs)
	provider := agentdefinition.NewUserAwareProvider(repo)

	modelCfg := &fagent.ModelConfig{
		Provider:    req.Provider,
		Model:       req.Model,
		APIKey:      req.APIKey,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	adk, err := adkgo.NewRunnerAdapter(adkgo.RunnerAdapterConfig{
		Log:            h.log,
		ThreadMgr:      h.threadMgr,
		AgentProvider:  provider,
		ConfigResolver: staticModelConfigResolver{cfg: modelCfg},
		APIKeyResolver: staticAPIKeyResolver{apiKey: req.APIKey},
		MemoryService:  h.memoryService,
		// All remaining ports (ToolReqs, MCPResolver, A2AResolver,
		// A2ASubAgentProvider, NewToolSetFactory, BlockEnforcer,
		// ConfirmChecker, PluginSettingsResolver, ProfileProvider,
		// ArtifactService, ExtraConfigFn, PromptInjectors) are nil on
		// purpose — adapters/adkgo/runner.go nil-guards each, which
		// reduces this path to an LLM-only turn with no tools.
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("creating runner: %v", err))
		return
	}

	ctx := r.Context()
	start := time.Now()

	invocationID := fagent.GenerateID()
	messageID := fagent.GenerateID()

	runReq := &fagent.RunRequest{
		AgentName:    rootID,
		SessionID:    threadID,
		UserID:       userID,
		Message:      fagent.NewUserMessage(messageID, req.Prompt),
		InvocationID: invocationID,
	}

	resp := DebugResponse{
		Request:  echoRequest(&req),
		ThreadID: threadID,
		RunID:    invocationID,
		AgentID:  rootID,
	}

	var (
		streamedText strings.Builder
		finalText    string
	)

	for evt, runErr := range adk.Run(ctx, runReq) {
		if runErr != nil {
			resp.Errors = append(resp.Errors, runErr.Error())
			continue
		}
		if evt == nil {
			continue
		}
		raw, merr := json.Marshal(evt)
		if merr != nil {
			resp.Errors = append(resp.Errors, fmt.Sprintf("marshal event %s: %v", evt.EventType(), merr))
			continue
		}
		resp.EventsEmitted = append(resp.EventsEmitted, wireEvent{Type: evt.EventType(), Data: raw})

		switch e := evt.(type) {
		case *thread.AssistantContentStreamed:
			streamedText.WriteString(e.Delta)
		case *thread.AssistantMessageAdded:
			if e.Content != "" {
				finalText = e.Content
			}
		case *thread.AssistantMessageCompleted:
			if finalText == "" && e.Content != "" {
				finalText = e.Content
			}
		case *thread.TokenUsageReported:
			if resp.TokenUsage == nil {
				resp.TokenUsage = &wireTokenUsage{}
			}
			resp.TokenUsage.PromptTokens += e.PromptTokens
			resp.TokenUsage.CompletionTokens += e.CompletionTokens
			resp.TokenUsage.TotalTokens += e.TotalTokens
		}
	}

	if finalText != "" {
		resp.Response = finalText
	} else {
		resp.Response = streamedText.String()
	}

	persisted, readErr := h.store.ReadStream(ctx, "Thread-"+threadID, 0)
	if readErr != nil {
		resp.Errors = append(resp.Errors, fmt.Sprintf("read persisted stream: %v", readErr))
	} else {
		for _, e := range persisted {
			raw, merr := json.Marshal(e)
			if merr != nil {
				resp.Errors = append(resp.Errors, fmt.Sprintf("marshal persisted event %s: %v", e.EventType(), merr))
				continue
			}
			resp.EventsPersisted = append(resp.EventsPersisted, wireEvent{Type: e.EventType(), Data: raw})
		}
	}

	resp.EventsTransient = diffEventsByType(resp.EventsEmitted, resp.EventsPersisted)

	resp.DurationMs = time.Since(start).Milliseconds()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// echoRequest copies the decoded request for the response body, redacting the
// API key so the reply is safe to log or paste into an issue.
func echoRequest(req *DebugRequest) *DebugInputEcho {
	return &DebugInputEcho{
		Provider:    req.Provider,
		Model:       req.Model,
		APIKey:      redactAPIKey(req.APIKey),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		UserID:      req.UserID,
		ThreadID:    req.ThreadID,
		Prompt:      req.Prompt,
		Agent:       req.Agent,
		AgentYAML:   req.AgentYAML,
	}
}

// redactAPIKey returns a fixed prefix plus the last 4 characters, so the
// caller can still tell keys apart in logs without leaking the secret.
func redactAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "***"
	}
	return "***" + key[len(key)-4:]
}

// diffEventsByType returns the multiset difference emitted − persisted keyed
// on event type. Each persisted event of a given type cancels one emitted
// event of that type; what's left is the set of transient events that never
// made it to the event store (e.g. AssistantContentStreamed chunks,
// AgentTransferStarted, ToolStart, WorkflowNodeCompleted).
func diffEventsByType(emitted, persisted []wireEvent) []wireEvent {
	remaining := make(map[string]int, len(persisted))
	for _, e := range persisted {
		remaining[e.Type]++
	}
	out := make([]wireEvent, 0, len(emitted))
	for _, e := range emitted {
		if remaining[e.Type] > 0 {
			remaining[e.Type]--
			continue
		}
		out = append(out, e)
	}
	return out
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
