package billing

import (
es "school-with-ai/foundation/eventsourcing"
)

// PaymentAccountRepository manages persistence of PaymentAccountAggregate.
// It wraps the generic es.Repository[*PaymentAccountAggregate] and follows the
// same pattern as domain.ThreadRepository.
type PaymentAccountRepository struct {
*es.Repository[*PaymentAccountAggregate]
}

// NewPaymentAccountRepository creates a new payment account repository.
func NewPaymentAccountRepository(eventStore es.EventStore) *PaymentAccountRepository {
genericRepo := es.NewRepository(
eventStore,
NewPaymentAccountAggregate,
func(a *PaymentAccountAggregate) string { return a.StreamName() },
)

return &PaymentAccountRepository{
Repository: genericRepo,
}
}
