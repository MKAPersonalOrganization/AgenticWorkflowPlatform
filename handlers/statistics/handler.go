package statistics

import (
	"net/http"

	component "school-with-ai/components/analytics"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/infra/projection/statistics"
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

// BasicAuth provides simple authentication for the analytics dashboard
func (h *Handler) BasicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != h.adminUser || pass != h.adminPass {
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

	// Get user from context
	user := domain.GetUserFromContext(ctx)

	component.Dashboard(component.DashboardProps{
		Stats:        stats,
		Agents:       agents,
		CurrentAgent: agentFilter,
		User:         user,
	}).Render(ctx, w)
}
