package analytics

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"aicompass/modules/analytics/statistics"
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"

	component "aicompass/frontend/ui/analytics"
)

// Module is the analytics bounded context. It wraps the statistics projection
// handler and exposes an HTTP dashboard with basic-auth protection.
type Module struct {
	guard   *authGuard
	handler *handler
}

// New creates the analytics module.
func New(log *logger.Logger, stats *statistics.PostgresProjection, adminUser, adminPass string) *Module {
	return &Module{
		guard:   &authGuard{user: adminUser, pass: adminPass},
		handler: &handler{log: log, stats: stats},
	}
}

// RegisterRoutes mounts all analytics HTTP routes onto the given router.
func (m *Module) RegisterRoutes(r interface {
	Get(pattern string, handlerFn http.HandlerFunc)
},
) {
	r.Get("/analytics", m.guard.wrap(m.handler.serveDashboard))
	r.Get("/analytics/api/timeseries", m.guard.wrap(m.handler.serveToolTimeSeries))
	r.Get("/analytics/api/tools/{tool}", m.guard.wrap(m.handler.serveToolDetail))
}

// authGuard carries the HTTP Basic Auth credentials for the analytics
// dashboard. Split off the serve handler so the serve methods share a
// single state set (log + stats) — the previous shape tripped LCOM4=2
// because basicAuth only touched user/pass while the serves only touched
// log/stats.
type authGuard struct {
	user string
	pass string
}

// wrap requires HTTP Basic credentials matching the guard before the
// next handler runs.
func (g *authGuard) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(g.user)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(g.pass)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Analytics Dashboard"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// handler contains the HTTP handler logic for the analytics module.
type handler struct {
	log   *logger.Logger
	stats *statistics.PostgresProjection
}

// serveDashboard renders the analytics dashboard page.
func (h *handler) serveDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentFilter := r.URL.Query().Get("agent")

	props, err := h.loadDashboardProps(ctx, agentFilter)
	if err != nil {
		h.log.Error(ctx, "analytics: dashboard load failed", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := component.Dashboard(props).Render(ctx, w); err != nil {
		h.log.Error(ctx, "analytics: failed to render dashboard", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// loadDashboardProps gathers every dataset the dashboard needs (stats,
// agents, time series, tool list) and converts them to their view-model
// shapes. Returning errors up lets serveDashboard stay flat and log once.
func (h *handler) loadDashboardProps(ctx context.Context, agentFilter string) (component.DashboardProps, error) {
	stats, err := h.stats.GetStats(ctx, agentFilter)
	if err != nil {
		return component.DashboardProps{}, err
	}
	agents, err := h.stats.GetAgents(ctx)
	if err != nil {
		return component.DashboardProps{}, err
	}
	timeSeries, err := h.stats.GetToolTimeSeries(ctx, agentFilter, "")
	if err != nil {
		return component.DashboardProps{}, err
	}
	toolList, err := h.stats.GetToolList(ctx, agentFilter)
	if err != nil {
		return component.DashboardProps{}, err
	}
	return component.DashboardProps{
		Stats:        toStatRows(stats),
		Agents:       agents,
		CurrentAgent: agentFilter,
		User:         auth.GetUserFromContext(ctx),
		TimeSeries:   toTimeSeriesPoints(timeSeries),
		ToolList:     toToolSummaries(toolList),
	}, nil
}

// toStatRows maps statistics.StatRow to the view-model StatRow,
// keeping the presentation layer decoupled from the module's internals.
func toStatRows(src []statistics.StatRow) []component.StatRow {
	out := make([]component.StatRow, len(src))
	for i, s := range src {
		out[i] = component.StatRow{
			AgentName: s.AgentName, Category: s.Category, Name: s.Name,
			TotalCount: s.TotalCount, Successes: s.Successes, Failures: s.Failures,
		}
	}
	return out
}

// toTimeSeriesPoints maps statistics.TimeSeriesPoint to the view-model.
func toTimeSeriesPoints(src []statistics.TimeSeriesPoint) []component.TimeSeriesPoint {
	out := make([]component.TimeSeriesPoint, len(src))
	for i, p := range src {
		out[i] = component.TimeSeriesPoint{
			Bucket: p.Bucket, TotalCalls: p.TotalCalls, Errors: p.Errors,
			Successes: p.Successes, ErrorRate: p.ErrorRate,
		}
	}
	return out
}

// toToolSummaries maps statistics.ToolSummary to the view-model.
func toToolSummaries(src []statistics.ToolSummary) []component.ToolSummary {
	out := make([]component.ToolSummary, len(src))
	for i, t := range src {
		out[i] = component.ToolSummary{
			ToolName: t.ToolName, TotalCalls: t.TotalCalls, Errors: t.Errors,
			Successes: t.Successes, SuccessRate: t.SuccessRate,
		}
	}
	return out
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
