package analytics

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	component "school-with-ai/components/analytics"
	"school-with-ai/foundation/logger"
	"school-with-ai/infra/projection/statistics"
	"school-with-ai/shared"
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

	props := component.DashboardProps{
		Stats:        stats,
		Agents:       agents,
		CurrentAgent: agentFilter,
		User:         user,
		TimeSeries:   timeSeries,
		ToolList:     toolList,
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
