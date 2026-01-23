package statistics

import (
	"net/http"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"school-with-ai/infra/projection/statistics"
	component "school-with-ai/components/analytics"
)

type Handler struct {
	log   *logger.Logger
	stats *statistics.PostgresProjection
}

func NewHandler(log *logger.Logger, stats *statistics.PostgresProjection) *Handler {
	return &Handler{
		log:   log,
		stats: stats,
	}
}

func (h *Handler) ServeDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := domain.GetUserFromContext(ctx)

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

	component.Dashboard(component.DashboardProps{
		Stats:        stats,
		User:         user,
		Agents:       agents,
		CurrentAgent: agentFilter,
	}).Render(ctx, w)
}
