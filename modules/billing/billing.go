package billing

import (
	"context"

	"school-with-ai/foundation/event"
	es "school-with-ai/foundation/eventsourcing"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the billing bounded context. It owns the handler, service, and
// projection as internal collaborators, exposing only the wiring methods that
// main.go needs.
//
// This is the first module extracted as part of the modular-monolith migration
// (see arch/adr/012-modular-monolith-migration.md). It replaces three former
// packages: handlers/billing, services/billing, infra/projection/payment.
//
// Domain types (PaymentAccountAggregate, payment events, payment types) now
// live in this package. The shared eventsourcing kernel in foundation/
// provides the open Event interface, BaseAggregate, and Repository[T].
//
// Public API for main.go:
//
//	module := billing.New(log, db, eventStore, webhookSecret, publishableKey)
//	module.RegisterProjection(persistenceRouter)
//	module.RegisterRoutes(r, authMiddleware)
type Module struct {
	handler    *handler
	svc        *service
	projection *postgresPaymentProjection
}

// New creates the billing module. Call RegisterProjection and RegisterRoutes
// after construction to wire into the application.
func New(
	log *logger.Logger,
	db *sqlx.DB,
	eventStore es.EventStore,
	webhookSecret string,
	publishableKey string,
) *Module {
	projection := newPostgresPaymentProjection(log, db)
	repo := NewPaymentAccountRepository(eventStore)
	svc := newService(log, repo, projection)
	h := newHandler(log, svc, webhookSecret, publishableKey)

	return &Module{
		handler:    h,
		svc:        svc,
		projection: projection,
	}
}

// RegisterProjection wires the payment projection's event handlers into the
// persistence router so that committed events update the read model.
func (m *Module) RegisterProjection(router interface {
	Register(eventPrototype any, handler event.HandlerFunc)
},
) {
	m.projection.registerHandlers(router)
}

// RegisterRoutes mounts all billing HTTP routes onto the given chi.Router.
// Authenticated routes use the provided auth middleware; the Stripe webhook
// endpoint is public (signature-verified internally).
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Post("/api/billing/create-payment-intent", authMW.RequireAuthFunc(m.handler.createPaymentIntent))
	r.Get("/api/billing/status", authMW.RequireAuthFunc(m.handler.getPaymentStatus))
	r.Post("/api/billing/verify-payment", authMW.RequireAuthFunc(m.handler.verifyPayment))
	r.Post("/api/billing/cancel", authMW.RequireAuthFunc(m.handler.cancelSubscription))
	r.Post("/api/billing/reactivate", authMW.RequireAuthFunc(m.handler.reactivateSubscription))
	// Webhook is public — Stripe calls it with signature verification
	r.Post("/api/billing/webhook", m.handler.handleWebhook)
}

// HasPaid exposes the read-model query for cross-module consumption.
// Other modules that need to check payment status call this method
// rather than reaching into the projection directly.
func (m *Module) HasPaid(ctx context.Context, userID string) (bool, error) {
	return m.svc.hasPaid(ctx, userID)
}
