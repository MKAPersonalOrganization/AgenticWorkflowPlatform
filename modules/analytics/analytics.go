package analytics

import (
	"aicompass/foundation/logger"
	"aicompass/modules/analytics/statistics"
	"aicompass/shared"
	"crypto/subtle"
	"encoding/json"
	"net/http"

	component "aicompass/frontend/ui/analytics"
)

// Module is the analytics bounded context. It wraps the statistics projection
// handler and exposes an HTTP dashboard with basic-auth protection.
type Module struct {
	handler *handler
}

// New creates the analytics module.
func New(log *logger.Logger, stats *statistics.PostgresProjection, adminUser, adminPass string) *Module {
	return &Module{
		handler: newHandler(log, stats, adminUser, adminPass),
	}
}

// RegisterRoutes mounts all analytics HTTP routes onto the given router.
func (m *Module) RegisterRoutes(r interface {
	Get(pattern string, handlerFn http.HandlerFunc)
},
) {
	r.Get("/analytics", m.handler.basicAuth(m.handler.serveDashboard))
	r.Get("/analytics/api/timeseries", m.handler.basicAuth(m.handler.serveToolTimeSeries))
	r.Get("/analytics/api/tools/{tool}", m.handler.basicAuth(m.handler.serveToolDetail))
}

// handler contains the HTTP handler logic for the analytics module.
type handler struct {
	log       *logger.Logger
	stats     *statistics.PostgresProjection
	adminUser string
	adminPass string
}

// newHandler creates a new analytics handler.
func newHandler(log *logger.Logger, stats *statistics.PostgresProjection, adminUser, adminPass string) *handler {
	return &handler{
		log:       log,
		stats:     stats,
		adminUser: adminUser,
		adminPass: adminPass,
	}
}

// basicAuth wraps an http.HandlerFunc with HTTP Basic Authentication.
func (h *handler) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(h.adminUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(h.adminPass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Analytics Dashboard"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// serveDashboard renders the analytics dashboard page.
func (h *handler) serveDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentFilter := r.URL.Query().Get("agent")

	stats, err := h.stats.GetStats(ctx, agentFilter)
	if err != nil {
		h.log.Error(ctx, "analytics: failed to get stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	agents, err := h.stats.GetAgents(ctx)
	if err != nil {
		h.log.Error(ctx, "analytics: failed to get agents", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	timeSeries, err := h.stats.GetToolTimeSeries(ctx, agentFilter, "")
	if err != nil {
		h.log.Error(ctx, "analytics: failed to get time series", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	toolList, err := h.stats.GetToolList(ctx, agentFilter)
	if err != nil {
		h.log.Error(ctx, "analytics: failed to get tool list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	user := shared.GetUserFromContext(ctx)

	// Map domain types to component view models to keep the presentation
	// layer decoupled from the analytics module's internal types.
	viewStats := make([]component.StatRow, len(stats))
	for i, s := range stats {
		viewStats[i] = component.StatRow{
			AgentName: s.AgentName, Category: s.Category, Name: s.Name,
			TotalCount: s.TotalCount, Successes: s.Successes, Failures: s.Failures,
		}
	}
	viewTS := make([]component.TimeSeriesPoint, len(timeSeries))
	for i, p := range timeSeries {
		viewTS[i] = component.TimeSeriesPoint{
			Bucket: p.Bucket, TotalCalls: p.TotalCalls, Errors: p.Errors,
			Successes: p.Successes, ErrorRate: p.ErrorRate,
		}
	}
	viewTools := make([]component.ToolSummary, len(toolList))
	for i, t := range toolList {
		viewTools[i] = component.ToolSummary{
			ToolName: t.ToolName, TotalCalls: t.TotalCalls, Errors: t.Errors,
			Successes: t.Successes, SuccessRate: t.SuccessRate,
		}
	}

	props := component.DashboardProps{
		Stats:        viewStats,
		Agents:       agents,
		CurrentAgent: agentFilter,
		User:         user,
		TimeSeries:   viewTS,
		ToolList:     viewTools,
	}

	if err := component.Dashboard(props).Render(ctx, w); err != nil {
		h.log.Error(ctx, "analytics: failed to render dashboard", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// serveToolTimeSeries returns tool time-series data as JSON.
func (h *handler) serveToolTimeSeries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentFilter := r.URL.Query().Get("agent")
	toolFilter := r.URL.Query().Get("tool")

	points, err := h.stats.GetToolTimeSeries(ctx, agentFilter, toolFilter)
	if err != nil {
		h.log.Error(ctx, "analytics: failed to get tool time series", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(points); err != nil {
		h.log.Error(ctx, "analytics: failed to encode time series response", "error", err)
	}
}

// serveToolDetail returns detail data for a specific tool as JSON.
func (h *handler) serveToolDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	toolName := r.PathValue("tool")
	agentFilter := r.URL.Query().Get("agent")

	points, err := h.stats.GetToolDetail(ctx, toolName, agentFilter)
	if err != nil {
		h.log.Error(ctx, "analytics: failed to get tool detail", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(points); err != nil {
		h.log.Error(ctx, "analytics: failed to encode tool detail response", "error", err)
	}
}
