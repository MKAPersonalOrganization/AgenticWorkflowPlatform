package domain

// PaymentAccountRepository manages persistence of PaymentAccountAggregate.
// It wraps the generic Repository[*PaymentAccountAggregate] and follows the
// same pattern as ThreadRepository.
type PaymentAccountRepository struct {
	*Repository[*PaymentAccountAggregate]
}

// NewPaymentAccountRepository creates a new payment account repository.
func NewPaymentAccountRepository(eventStore EventStore) *PaymentAccountRepository {
	genericRepo := NewRepository(
		eventStore,
		NewPaymentAccountAggregate,
		func(a *PaymentAccountAggregate) string { return a.StreamName() },
	)

	return &PaymentAccountRepository{
		Repository: genericRepo,
	}
}
