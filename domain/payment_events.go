package domain

import "time"

// ==========================================================================
// Payment Account Events
//
// These events model a payment account as an event-sourced ledger, following
// banking practices: every state change is an immutable, auditable fact.
// The aggregate ID is the user ID — each user has exactly one payment account.
//
// Stripe SDK alignment (v84):
//   - Events carry Stripe object IDs for full traceability
//   - StripeEventID enables webhook deduplication at the event store level
//   - Error fields use Stripe's typed error/decline code taxonomy
//   - Amounts are always int64 in smallest currency unit (cents)
//   - Timestamps are time.Time (domain) — Stripe uses int64 Unix seconds
// ==========================================================================

// PaymentAccountOpened is emitted once when a user's payment account is created.
// This is analogous to opening a bank account.
type PaymentAccountOpened struct {
	AccountID string    `json:"accountId"` // == UserID
	Email     string    `json:"email"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *PaymentAccountOpened) EventType() string        { return "PaymentAccountOpened" }
func (e *PaymentAccountOpened) EventTimestamp() time.Time { return e.Timestamp }
func (e *PaymentAccountOpened) AggregateID() string      { return e.AccountID }
func (e *PaymentAccountOpened) sealed()                  {}

// PaymentInitiated is emitted when a Stripe PaymentIntent is created and the
// user begins the checkout flow. This is a "hold" — funds are not yet captured.
//
// Stripe alignment: maps to payment_intent.created webhook event.
// The StripePaymentID is the PaymentIntent ID (pi_xxx) from the Stripe SDK.
type PaymentInitiated struct {
	AccountID        string    `json:"accountId"`
	PaymentID        string    `json:"paymentId"`        // Internal unique ID for this payment attempt
	StripePaymentID  string    `json:"stripePaymentId"`  // Stripe PaymentIntent ID (pi_xxx)
	StripeCustomerID string    `json:"stripeCustomerId"` // Stripe Customer ID (cus_xxx)
	AmountCents      int64     `json:"amountCents"`
	Currency         string    `json:"currency"` // ISO 4217 lowercase (matches stripe.Currency)
	Description      string    `json:"description"`
	IdempotencyKey   string    `json:"idempotencyKey"`
	Timestamp        time.Time `json:"timestamp"`
}

func (e *PaymentInitiated) EventType() string        { return "PaymentInitiated" }
func (e *PaymentInitiated) EventTimestamp() time.Time { return e.Timestamp }
func (e *PaymentInitiated) AggregateID() string      { return e.AccountID }
func (e *PaymentInitiated) sealed()                  {}

// PaymentSucceeded is emitted when Stripe confirms the payment was captured.
// This is the authoritative "debit posted" event.
//
// Stripe alignment: maps to payment_intent.succeeded webhook event.
// CardBrand/CardLast4 come from Charge.PaymentMethodDetails.Card (PCI-safe).
type PaymentSucceeded struct {
	AccountID       string    `json:"accountId"`
	PaymentID       string    `json:"paymentId"`
	StripePaymentID string    `json:"stripePaymentId"`
	StripeChargeID  string    `json:"stripeChargeId,omitempty"` // Stripe Charge ID (ch_xxx)
	StripeEventID   string    `json:"stripeEventId,omitempty"`  // Webhook event ID for dedup (evt_xxx)
	AmountCents     int64     `json:"amountCents"`
	Currency        string    `json:"currency"`
	CardBrand       string    `json:"cardBrand,omitempty"` // e.g. "visa", "mastercard" (from ChargePaymentMethodDetails)
	CardLast4       string    `json:"cardLast4,omitempty"` // e.g. "4242"
	ReceiptURL      string    `json:"receiptUrl,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}

func (e *PaymentSucceeded) EventType() string        { return "PaymentSucceeded" }
func (e *PaymentSucceeded) EventTimestamp() time.Time { return e.Timestamp }
func (e *PaymentSucceeded) AggregateID() string      { return e.AccountID }
func (e *PaymentSucceeded) sealed()                  {}

// PaymentFailed is emitted when Stripe reports a payment failure (card
// declined, insufficient funds, etc.).
//
// Stripe alignment: maps to payment_intent.payment_failed webhook event.
// ErrorCode and DeclineCode map directly to stripe.ErrorCode and stripe.DeclineCode.
type PaymentFailed struct {
	AccountID       string    `json:"accountId"`
	PaymentID       string    `json:"paymentId"`
	StripePaymentID string    `json:"stripePaymentId"`
	StripeEventID   string    `json:"stripeEventId,omitempty"`  // Webhook event ID for dedup (evt_xxx)
	Reason          string    `json:"reason"`                   // Human-readable failure reason
	ErrorCode       string    `json:"errorCode,omitempty"`      // Stripe ErrorCode (e.g. "card_declined")
	DeclineCode     string    `json:"declineCode,omitempty"`    // Stripe DeclineCode (e.g. "insufficient_funds")
	ErrorType       string    `json:"errorType,omitempty"`      // Stripe ErrorType (e.g. "card_error")
	FailureMessage  string    `json:"failureMessage,omitempty"` // Stripe error message for logging
	Timestamp       time.Time `json:"timestamp"`
}

func (e *PaymentFailed) EventType() string        { return "PaymentFailed" }
func (e *PaymentFailed) EventTimestamp() time.Time { return e.Timestamp }
func (e *PaymentFailed) AggregateID() string      { return e.AccountID }
func (e *PaymentFailed) sealed()                  {}

// PaymentRefunded is emitted when a completed payment is refunded (full or
// partial). Follows the banking "credit" pattern.
//
// Stripe alignment: maps to charge.refunded webhook event.
// The StripeRefundID is from the Refund object (re_xxx).
type PaymentRefunded struct {
	AccountID      string    `json:"accountId"`
	PaymentID      string    `json:"paymentId"`
	StripeRefundID string    `json:"stripeRefundId"`           // Stripe Refund ID (re_xxx)
	StripeChargeID string    `json:"stripeChargeId,omitempty"` // Stripe Charge ID (ch_xxx)
	StripeEventID  string    `json:"stripeEventId,omitempty"`  // Webhook event ID for dedup (evt_xxx)
	AmountCents    int64     `json:"amountCents"`              // Amount refunded (may be partial)
	Currency       string    `json:"currency,omitempty"`       // ISO 4217 lowercase
	Reason         string    `json:"reason"`                   // RefundReason or custom
	RefundStatus   string    `json:"refundStatus,omitempty"`   // Stripe RefundStatus
	Timestamp      time.Time `json:"timestamp"`
}

func (e *PaymentRefunded) EventType() string        { return "PaymentRefunded" }
func (e *PaymentRefunded) EventTimestamp() time.Time { return e.Timestamp }
func (e *PaymentRefunded) AggregateID() string      { return e.AccountID }
func (e *PaymentRefunded) sealed()                  {}

// PaymentDisputed is emitted when a chargeback/dispute is opened against a
// payment. This is critical for production: disputes can reverse funds and
// carry financial penalties.
//
// Stripe alignment: maps to charge.dispute.created webhook event.
type PaymentDisputed struct {
	AccountID       string    `json:"accountId"`
	PaymentID       string    `json:"paymentId"`
	StripeDisputeID string    `json:"stripeDisputeId"`         // Stripe Dispute ID (dp_xxx)
	StripeChargeID  string    `json:"stripeChargeId"`          // Stripe Charge ID (ch_xxx)
	StripeEventID   string    `json:"stripeEventId,omitempty"` // Webhook event ID for dedup (evt_xxx)
	AmountCents     int64     `json:"amountCents"`             // Disputed amount
	Currency        string    `json:"currency,omitempty"`
	Reason          string    `json:"reason"` // DisputeReason
	Status          string    `json:"status"` // DisputeStatus
	Timestamp       time.Time `json:"timestamp"`
}

func (e *PaymentDisputed) EventType() string        { return "PaymentDisputed" }
func (e *PaymentDisputed) EventTimestamp() time.Time { return e.Timestamp }
func (e *PaymentDisputed) AggregateID() string      { return e.AccountID }
func (e *PaymentDisputed) sealed()                  {}

// DisputeResolved is emitted when a dispute reaches a terminal state
// (won or lost). If lost, the funds are permanently reversed.
//
// Stripe alignment: maps to charge.dispute.closed webhook event.
type DisputeResolved struct {
	AccountID       string    `json:"accountId"`
	StripeDisputeID string    `json:"stripeDisputeId"`
	StripeEventID   string    `json:"stripeEventId,omitempty"`
	Status          string    `json:"status"` // "won" or "lost"
	AmountCents     int64     `json:"amountCents"`
	Timestamp       time.Time `json:"timestamp"`
}

func (e *DisputeResolved) EventType() string        { return "DisputeResolved" }
func (e *DisputeResolved) EventTimestamp() time.Time { return e.Timestamp }
func (e *DisputeResolved) AggregateID() string      { return e.AccountID }
func (e *DisputeResolved) sealed()                  {}

// PaymentMethodUpdated is emitted when the user's payment method changes
// (new card attached, card updated by card network). Tracks PCI-safe info only.
//
// Stripe alignment: maps to payment_method.attached / payment_method.updated.
type PaymentMethodUpdated struct {
	AccountID             string    `json:"accountId"`
	StripePaymentMethodID string    `json:"stripePaymentMethodId"` // Stripe PaymentMethod ID (pm_xxx)
	StripeEventID         string    `json:"stripeEventId,omitempty"`
	CardBrand             string    `json:"cardBrand,omitempty"` // e.g. "visa"
	CardLast4             string    `json:"cardLast4,omitempty"` // e.g. "4242"
	CardExpMonth          int64     `json:"cardExpMonth,omitempty"`
	CardExpYear           int64     `json:"cardExpYear,omitempty"`
	Timestamp             time.Time `json:"timestamp"`
}

func (e *PaymentMethodUpdated) EventType() string        { return "PaymentMethodUpdated" }
func (e *PaymentMethodUpdated) EventTimestamp() time.Time { return e.Timestamp }
func (e *PaymentMethodUpdated) AggregateID() string      { return e.AccountID }
func (e *PaymentMethodUpdated) sealed()                  {}

// InvoicePaymentFailed is emitted when a recurring invoice payment fails.
// This is distinct from PaymentFailed (which covers initial payment attempts)
// and is critical for subscription billing health monitoring.
//
// Stripe alignment: maps to invoice.payment_failed webhook event.
type InvoicePaymentFailed struct {
	AccountID       string    `json:"accountId"`
	StripeInvoiceID string    `json:"stripeInvoiceId"`         // Stripe Invoice ID (in_xxx)
	StripeEventID   string    `json:"stripeEventId,omitempty"` // Webhook event ID for dedup
	AmountDueCents  int64     `json:"amountDueCents"`
	Currency        string    `json:"currency,omitempty"`
	AttemptCount    int64     `json:"attemptCount"`            // Number of payment attempts
	NextAttemptAt   *int64    `json:"nextAttemptAt,omitempty"` // Unix timestamp of next retry
	ErrorCode       string    `json:"errorCode,omitempty"`
	Timestamp       time.Time `json:"timestamp"`
}

func (e *InvoicePaymentFailed) EventType() string        { return "InvoicePaymentFailed" }
func (e *InvoicePaymentFailed) EventTimestamp() time.Time { return e.Timestamp }
func (e *InvoicePaymentFailed) AggregateID() string      { return e.AccountID }
func (e *InvoicePaymentFailed) sealed()                  {}

// SubscriptionCancelled is emitted when a user requests cancellation.
// The subscription remains active until CancelsOn (end of billing period).
//
// Stripe alignment: cancel_at_period_end=true on the Stripe Subscription.
// CancelsOn maps to Subscription.CancelAt (Unix timestamp in Stripe).
type SubscriptionCancelled struct {
	AccountID            string    `json:"accountId"`
	StripeSubscriptionID string    `json:"stripeSubscriptionId,omitempty"` // Stripe Subscription ID (sub_xxx)
	CancelledAt          time.Time `json:"cancelledAt"`                    // When the user requested cancellation
	CancelsOn            time.Time `json:"cancelsOn"`                      // When access actually ends
	Reason               string    `json:"reason,omitempty"`
	Timestamp            time.Time `json:"timestamp"`
}

func (e *SubscriptionCancelled) EventType() string        { return "SubscriptionCancelled" }
func (e *SubscriptionCancelled) EventTimestamp() time.Time { return e.Timestamp }
func (e *SubscriptionCancelled) AggregateID() string      { return e.AccountID }
func (e *SubscriptionCancelled) sealed()                  {}

// SubscriptionExpired is emitted when a cancelled subscription reaches its
// cancels_on date and access is revoked. The account returns to free tier.
type SubscriptionExpired struct {
	AccountID string    `json:"accountId"`
	ExpiredAt time.Time `json:"expiredAt"` // When access was actually revoked
	Timestamp time.Time `json:"timestamp"`
}

func (e *SubscriptionExpired) EventType() string        { return "SubscriptionExpired" }
func (e *SubscriptionExpired) EventTimestamp() time.Time { return e.Timestamp }
func (e *SubscriptionExpired) AggregateID() string      { return e.AccountID }
func (e *SubscriptionExpired) sealed()                  {}

// SubscriptionReactivated is emitted when a user reverses a cancellation
// before the billing period ends. The subscription returns to active.
type SubscriptionReactivated struct {
	AccountID     string    `json:"accountId"`
	ReactivatedAt time.Time `json:"reactivatedAt"`
	Timestamp     time.Time `json:"timestamp"`
}

func (e *SubscriptionReactivated) EventType() string        { return "SubscriptionReactivated" }
func (e *SubscriptionReactivated) EventTimestamp() time.Time { return e.Timestamp }
func (e *SubscriptionReactivated) AggregateID() string      { return e.AccountID }
func (e *SubscriptionReactivated) sealed()                  {}
