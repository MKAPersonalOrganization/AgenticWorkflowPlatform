package statistics

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"school-with-ai/shared"
	"school-with-ai/foundation/logger"
	"school-with-ai/infra/projection/statistics"

	component "school-with-ai/components/analytics"
)

type Handler struct {
	log       *logger.Logger
	stats     *statistics.PostgresProjection
	adminUser string
	adminPass string
}

func NewHandler(log *logger.Logger, stats *statistics.PostgresProjection, adminUser, adminPass string) *Handler {
	return &Handler{
		log:       log,
		stats:     stats,
		adminUser: adminUser,
		adminPass: adminPass,
	}
}

// BasicAuth provides simple authentication for the analytics dashboard.
// Uses constant-time comparison to prevent timing attacks.
func (h *Handler) BasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(h.adminUser))
		passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(h.adminPass))
		if !ok || userMatch != 1 || passMatch != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Analytics Dashboard"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	agentFilter := r.URL.Query().Get("agent")

	stats, err := h.stats.GetStats(ctx, agentFilter)
	if err != nil {
		h.log.Error(ctx, "failed to get stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	agents, err := h.stats.GetAgents(ctx)
	if err != nil {
		h.log.Error(ctx, "failed to get agents", "error", err)
	}

	// Fetch time-series and tool list data
	timeSeries, err := h.stats.GetToolTimeSeries(ctx, agentFilter, "")
	if err != nil {
		h.log.Error(ctx, "failed to get tool time series", "error", err)
	}

	toolList, err := h.stats.GetToolList(ctx, agentFilter)
	if err != nil {
		h.log.Error(ctx, "failed to get tool list", "error", err)
	}

	// Get user from context
	user := shared.GetUserFromContext(ctx)

	component.Dashboard(component.DashboardProps{
		Stats:        stats,
		Agents:       agents,
		CurrentAgent: agentFilter,
		User:         user,
		TimeSeries:   timeSeries,
		ToolList:     toolList,
	}).Render(ctx, w)
}

// ServeToolTimeSeries returns time-series data as JSON for chart updates.
func (h *Handler) ServeToolTimeSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	agentFilter := r.URL.Query().Get("agent")
	toolFilter := r.URL.Query().Get("tool")

	points, err := h.stats.GetToolTimeSeries(ctx, agentFilter, toolFilter)
	if err != nil {
		h.log.Error(ctx, "failed to get tool time series", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}

// ServeToolDetail returns detailed time-series data for a specific tool as JSON.
func (h *Handler) ServeToolDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract tool name from URL path
	toolName := r.PathValue("tool")
	if toolName == "" {
		http.Error(w, "tool name required", http.StatusBadRequest)
		return
	}

	agentFilter := r.URL.Query().Get("agent")

	points, err := h.stats.GetToolDetail(ctx, toolName, agentFilter)
	if err != nil {
		h.log.Error(ctx, "failed to get tool detail", "error", err, "tool", toolName)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}
