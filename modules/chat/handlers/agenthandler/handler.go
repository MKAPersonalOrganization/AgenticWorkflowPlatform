package agenthandler

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// Handler handles agent-related HTTP endpoints.
type Handler struct {
	log           *logger.Logger
	agentProvider agent.AgentProvider
}

// New creates a new agent handler.
func New(
	log *logger.Logger,
	agentProvider agent.AgentProvider,
) *Handler {
	return &Handler{
		log:           log,
		agentProvider: agentProvider,
	}
}

// defaultAgentIconSVG is the fallback compass icon for agents without a valid SVG icon.
var defaultAgentIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10" /><path d="m16.24 7.76-1.804 5.411a2 2 0 0 1-1.265 1.265L7.76 16.24l1.804-5.411a2 2 0 0 1 1.265-1.265z" /></svg>`

// resolveAgentIcon extracts the icon from an agent (via optional Icon() method)
// and falls back to the default compass SVG if the agent doesn't provide one.
func resolveAgentIcon(a agent.Agent) string {
	if ip, ok := a.(interface{ Icon() string }); ok {
		if icon := ip.Icon(); strings.HasPrefix(icon, "<svg") {
			return icon
		}
	}
	return defaultAgentIconSVG
}

// ListAgents returns a list of available agents.
func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	// Try to list user-specific agents if provider supports it
	var agents []agent.Agent

	user := auth.GetUserFromContext(r.Context())

	type userAwareProvider interface {
		ListForUser(ctx context.Context, userID string) ([]agent.Agent, error)
	}

	if uap, ok := h.agentProvider.(userAwareProvider); ok && user != nil {
		var err error
		agents, err = uap.ListForUser(r.Context(), user.ID)
		if err != nil {
			agents = h.agentProvider.List() // Fallback to default
		}
	} else {
		agents = h.agentProvider.List()
	}

	response := agent.AgentListResponse{
		Agents: make([]agent.AgentMetadata, 0, len(agents)),
	}

	for _, a := range agents {
		meta := agent.AgentMetadata{
			ID:          a.ID(),
			Name:        a.Name(),
			Description: a.Description(),
			Icon:        resolveAgentIcon(a),
		}
		if wa, ok := a.(agent.WorkflowAgent); ok {
			g := wa.WorkflowGraph()
			meta.Graph = &g
		}
		response.Agents = append(response.Agents, meta)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error(r.Context(), "failed to encode agents list", "error", err)
		http.Error(w, "Failed to encode agents list", http.StatusInternalServerError)
	}
}

// GetAgent returns a specific agent's details by ID.
func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("agentId")
	if agentID == "" {
		http.Error(w, "agentId is required", http.StatusBadRequest)
		return
	}

	user := auth.GetUserFromContext(r.Context())

	type userAwareProvider interface {
		GetForUser(ctx context.Context, name, userID string) (agent.Agent, error)
	}

	var a agent.Agent
	var err error

	if uap, ok := h.agentProvider.(userAwareProvider); ok && user != nil {
		a, err = uap.GetForUser(r.Context(), agentID, user.ID)
	} else {
		a, err = h.agentProvider.Get(agentID)
	}

	if err != nil {
		h.log.Error(r.Context(), "failed to get agent", "agentID", agentID, "error", err)
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	meta := agent.AgentMetadata{
		ID:          a.ID(),
		Name:        a.Name(),
		Description: a.Description(),
		Icon:        resolveAgentIcon(a),
	}
	if wa, ok := a.(agent.WorkflowAgent); ok {
		g := wa.WorkflowGraph()
		meta.Graph = &g
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(meta); err != nil {
		h.log.Error(r.Context(), "failed to encode agent response", "error", err)
		http.Error(w, "Failed to encode agent response", http.StatusInternalServerError)
	}
}
