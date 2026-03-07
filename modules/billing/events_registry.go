package billing

import (
	es "school-with-ai/foundation/eventsourcing"
)

// RegisterEvents registers all billing/payment domain events with the given
// deserializer registry. Call this at application startup before any events
// are read from the store.
func RegisterEvents(r *es.DeserializerRegistry) {
	r.Register("PaymentAccountOpened", func() es.Event { return &PaymentAccountOpened{} })
	r.Register("PaymentInitiated", func() es.Event { return &PaymentInitiated{} })
	r.Register("PaymentSucceeded", func() es.Event { return &PaymentSucceeded{} })
	r.Register("PaymentFailed", func() es.Event { return &PaymentFailed{} })
	r.Register("PaymentRefunded", func() es.Event { return &PaymentRefunded{} })
	r.Register("PaymentDisputed", func() es.Event { return &PaymentDisputed{} })
	r.Register("DisputeResolved", func() es.Event { return &DisputeResolved{} })
	r.Register("PaymentMethodUpdated", func() es.Event { return &PaymentMethodUpdated{} })
	r.Register("InvoicePaymentFailed", func() es.Event { return &InvoicePaymentFailed{} })
	r.Register("SubscriptionCancelled", func() es.Event { return &SubscriptionCancelled{} })
	r.Register("SubscriptionExpired", func() es.Event { return &SubscriptionExpired{} })
	r.Register("SubscriptionReactivated", func() es.Event { return &SubscriptionReactivated{} })
}
