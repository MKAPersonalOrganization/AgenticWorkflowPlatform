package billing

import (
	"errors"
	"fmt"
	"time"

	es "aicompass/foundation/eventsourcing"
)

// ==========================================================================
// PaymentAccountAggregate
//
// Event-sourced aggregate modelling a user's payment account as an immutable
// ledger. Every mutation produces an event; state is rebuilt by replaying them.
//
// Stripe SDK alignment (v84):
//   - Typed status enums match Stripe's PaymentIntentStatus, RefundStatus, etc.
//   - Stripe object IDs (pi_, ch_, re_, dp_) are tracked for full traceability
//   - Dispute handling follows Stripe's charge.dispute lifecycle
//   - Card info uses Stripe's PCI-safe fields (brand + last4 only)
//   - All amounts in smallest currency unit (cents) — matches stripe.Amount
//
// Banking concepts applied:
//   - Account lifecycle (open → active)
//   - Ledger entries (each payment attempt is a line item)
//   - Idempotency (duplicate Stripe webhooks are safe)
//   - Full audit trail (every action is an event)
//   - Balance tracking (total debited, total refunded, total disputed)
//   - Dispute holds (funds frozen during active disputes)
// ==========================================================================

// LedgerEntry is an in-memory projection of a single payment attempt,
// rebuilt from events during replay.
type LedgerEntry struct {
	PaymentID       string
	StripePaymentID string            // Stripe PaymentIntent ID (pi_xxx)
	StripeChargeID  string            // Stripe Charge ID (ch_xxx) — set on success
	AmountCents     int64             // Amount in smallest currency unit
	Currency        string            // ISO 4217 lowercase (matches stripe.Currency)
	Status          LedgerEntryStatus // Typed status matching Stripe lifecycle
	Description     string
	FailureReason   string
	ErrorCode       string // Stripe ErrorCode for programmatic failure handling
	DeclineCode     string // Stripe DeclineCode for card-specific failures
	RefundedCents   int64  // Running total of refunds against this entry
	DisputedCents   int64  // Amount currently under dispute
	CardBrand       string // e.g. "visa", "mastercard" (PCI-safe, from Charge)
	CardLast4       string // e.g. "4242" (PCI-safe, from Charge)
	ReceiptURL      string // Stripe receipt URL
	InitiatedAt     time.Time
	ResolvedAt      *time.Time
}

// NetAmount returns the effective amount after refunds and lost disputes.
func (e *LedgerEntry) NetAmount() int64 {
	return e.AmountCents - e.RefundedCents
}

// IsRefundable returns true if this entry can accept more refunds.
func (e *LedgerEntry) IsRefundable() bool {
	return e.Status == LedgerEntryStatusSucceeded && e.RefundedCents < e.AmountCents
}

// MaxRefundable returns the maximum amount that can still be refunded.
func (e *LedgerEntry) MaxRefundable() int64 {
	return e.AmountCents - e.RefundedCents
}

// PaymentAccountAggregate is the event-sourced aggregate for billing.
// The aggregate ID is the user's ID — one account per user.
type PaymentAccountAggregate struct {
	es.BaseAggregate

	// Account metadata
	Email    string
	OpenedAt time.Time
	IsOpen   bool

	// Stripe identity — maps to Stripe Customer object (cus_xxx)
	StripeCustomerID string

	// Ledger — ordered list of payment attempts (the "bank statement")
	Ledger []LedgerEntry

	// Computed balances (rebuilt from events)
	TotalDebitedCents  int64 // Sum of all succeeded payments
	TotalRefundedCents int64 // Sum of all refunds
	TotalDisputedCents int64 // Sum of funds held/lost in disputes

	// Dispute tracking
	ActiveDisputes   int // Count of unresolved disputes
	HasActiveDispute bool

	// Subscription lifecycle state (mirrors Stripe Subscription fields)
	CancelledAt *time.Time // When the user requested cancellation (maps to Subscription.CanceledAt)
	CancelsOn   *time.Time // When access actually ends (maps to Subscription.CancelAt)
	Expired     bool       // True after SubscriptionExpired — subscription ended

	// Invoice failure tracking (for subscription billing health)
	ConsecutiveInvoiceFailures int // Resets on successful payment

	// Payment method info (from latest succeeded payment — PCI-safe: last4 + brand only)
	// Maps to Charge.PaymentMethodDetails.Card.Brand and .Last4
	CardBrand  string // e.g. "visa", "mastercard"
	CardLast4  string // e.g. "4242"
	ReceiptURL string // Latest Stripe receipt URL

	// Webhook event dedup — tracks processed Stripe event IDs to prevent
	// double-processing. In production, the event store's optimistic
	// concurrency handles this, but tracking provides an additional safety net.
	ProcessedStripeEvents map[string]struct{}
}

// NewPaymentAccountAggregate creates a blank aggregate for rehydration.
func NewPaymentAccountAggregate(id string) *PaymentAccountAggregate {
	return &PaymentAccountAggregate{
		BaseAggregate: es.BaseAggregate{
			ID: id,
		},
		Ledger:                make([]LedgerEntry, 0),
		ProcessedStripeEvents: make(map[string]struct{}),
	}
}

// StreamName returns the event stream name for this aggregate.
func (a *PaymentAccountAggregate) StreamName() string {
	return es.StreamName("PaymentAccount", a.ID)
}

// isStripeEventProcessed checks if a Stripe webhook event has already been applied.
func (a *PaymentAccountAggregate) isStripeEventProcessed(stripeEventID string) bool {
	if stripeEventID == "" {
		return false // No event ID means we cannot deduplicate.
	}
	_, exists := a.ProcessedStripeEvents[stripeEventID]
	return exists
}

// -------------------------------------------------------------------------
// Commands — business logic that validates invariants then enqueues events
// -------------------------------------------------------------------------

// OpenAccount creates a new payment account. Static factory (like OpenThread).
func OpenPaymentAccount(userID, email string) (*PaymentAccountAggregate, error) {
	if userID == "" {
		return nil, errors.New("user ID is required")
	}
	if email == "" {
		return nil, errors.New("email is required")
	}

	acct := NewPaymentAccountAggregate(userID)
	acct.Enqueue(&PaymentAccountOpened{
		AccountID: userID,
		Email:     email,
		Timestamp: time.Now().UTC(),
	}, acct.Apply)

	return acct, nil
}

// InitiatePayment records a new payment attempt. Called after the Stripe
// PaymentIntent is created successfully.
func (a *PaymentAccountAggregate) InitiatePayment(
	paymentID, stripePaymentID, stripeCustomerID string,
	amountCents int64, currency, description, idempotencyKey string,
) error {
	if !a.IsOpen {
		return ErrAccountNotOpen
	}

	// Invariant: cannot initiate if already fully paid (subscription is active)
	if a.IsActivated() {
		return ErrAlreadyActivated
	}

	// Idempotency: if we already have a pending/succeeded entry for this Stripe PI, skip
	for _, entry := range a.Ledger {
		if entry.StripePaymentID == stripePaymentID {
			return nil // Already recorded — idempotent
		}
	}

	a.Enqueue(&PaymentInitiated{
		AccountID:        a.ID,
		PaymentID:        paymentID,
		StripePaymentID:  stripePaymentID,
		StripeCustomerID: stripeCustomerID,
		AmountCents:      amountCents,
		Currency:         currency,
		Description:      description,
		IdempotencyKey:   idempotencyKey,
		Timestamp:        time.Now().UTC(),
	}, a.Apply)

	return nil
}

// ConfirmPayment marks a payment as succeeded. Called by the Stripe webhook
// handler when payment_intent.succeeded fires.
//
// Stripe alignment: stripeChargeID and stripeEventID are extracted from the
// webhook payload for full traceability. receiptURL comes from Charge.ReceiptURL.
func (a *PaymentAccountAggregate) ConfirmPayment(stripePaymentID string, amountCents int64, currency, cardBrand, cardLast4 string) error {
	entry := a.findLedgerEntry(stripePaymentID)
	if entry == nil {
		return fmt.Errorf("no pending payment found for stripe PI %s", stripePaymentID)
	}
	if entry.Status == LedgerEntryStatusSucceeded {
		return nil // Idempotent — webhook delivered twice
	}
	if entry.Status != LedgerEntryStatusPending {
		return fmt.Errorf("cannot confirm payment in status %q", entry.Status)
	}

	a.Enqueue(&PaymentSucceeded{
		AccountID:       a.ID,
		PaymentID:       entry.PaymentID,
		StripePaymentID: stripePaymentID,
		AmountCents:     amountCents,
		Currency:        currency,
		CardBrand:       cardBrand,
		CardLast4:       cardLast4,
		Timestamp:       time.Now().UTC(),
	}, a.Apply)

	return nil
}

// ConfirmPaymentWithDetails marks a payment as succeeded with full Stripe
// metadata. This is the production-grade version that captures all traceability
// fields from the webhook payload.
func (a *PaymentAccountAggregate) ConfirmPaymentWithDetails(
	stripePaymentID, stripeChargeID, stripeEventID string,
	amountCents int64, currency, cardBrand, cardLast4, receiptURL string,
) error {
	// Stripe event dedup check
	if a.isStripeEventProcessed(stripeEventID) {
		return nil
	}

	entry := a.findLedgerEntry(stripePaymentID)
	if entry == nil {
		return fmt.Errorf("no pending payment found for stripe PI %s", stripePaymentID)
	}
	if entry.Status == LedgerEntryStatusSucceeded {
		return nil // Idempotent
	}
	if entry.Status != LedgerEntryStatusPending {
		return fmt.Errorf("cannot confirm payment in status %q", entry.Status)
	}

	a.Enqueue(&PaymentSucceeded{
		AccountID:       a.ID,
		PaymentID:       entry.PaymentID,
		StripePaymentID: stripePaymentID,
		StripeChargeID:  stripeChargeID,
		StripeEventID:   stripeEventID,
		AmountCents:     amountCents,
		Currency:        currency,
		CardBrand:       cardBrand,
		CardLast4:       cardLast4,
		ReceiptURL:      receiptURL,
		Timestamp:       time.Now().UTC(),
	}, a.Apply)

	return nil
}

// FailPayment marks a payment as failed. Called by the Stripe webhook
// handler when payment_intent.payment_failed fires.
func (a *PaymentAccountAggregate) FailPayment(stripePaymentID, reason string) error {
	entry := a.findLedgerEntry(stripePaymentID)
	if entry == nil {
		return fmt.Errorf("no pending payment found for stripe PI %s", stripePaymentID)
	}
	if entry.Status == LedgerEntryStatusFailed {
		return nil // Idempotent
	}
	if entry.Status != LedgerEntryStatusPending {
		return fmt.Errorf("cannot fail payment in status %q", entry.Status)
	}

	a.Enqueue(&PaymentFailed{
		AccountID:       a.ID,
		PaymentID:       entry.PaymentID,
		StripePaymentID: stripePaymentID,
		Reason:          reason,
		Timestamp:       time.Now().UTC(),
	}, a.Apply)

	return nil
}

// FailPaymentWithDetails marks a payment as failed with full Stripe error
// taxonomy. This is the production-grade version that preserves ErrorCode,
// DeclineCode, and ErrorType from the Stripe SDK's *stripe.Error.
func (a *PaymentAccountAggregate) FailPaymentWithDetails(
	stripePaymentID, stripeEventID, reason, errorCode, declineCode, errorType, failureMessage string,
) error {
	// Stripe event dedup check
	if a.isStripeEventProcessed(stripeEventID) {
		return nil
	}

	entry := a.findLedgerEntry(stripePaymentID)
	if entry == nil {
		return fmt.Errorf("no pending payment found for stripe PI %s", stripePaymentID)
	}
	if entry.Status == LedgerEntryStatusFailed {
		return nil // Idempotent
	}
	if entry.Status != LedgerEntryStatusPending {
		return fmt.Errorf("cannot fail payment in status %q", entry.Status)
	}

	a.Enqueue(&PaymentFailed{
		AccountID:       a.ID,
		PaymentID:       entry.PaymentID,
		StripePaymentID: stripePaymentID,
		StripeEventID:   stripeEventID,
		Reason:          reason,
		ErrorCode:       errorCode,
		DeclineCode:     declineCode,
		ErrorType:       errorType,
		FailureMessage:  failureMessage,
		Timestamp:       time.Now().UTC(),
	}, a.Apply)

	return nil
}

// RefundPayment records a full or partial refund against a succeeded payment.
//
// Stripe alignment: stripeRefundID is the Refund ID (re_xxx) from
// Charge.Refunds.Data[]. stripeChargeID is from the webhook Charge object.
func (a *PaymentAccountAggregate) RefundPayment(stripePaymentID, stripeRefundID string, refundCents int64, reason string) error {
	entry := a.findLedgerEntry(stripePaymentID)
	if entry == nil {
		return fmt.Errorf("no payment found for stripe PI %s", stripePaymentID)
	}
	if entry.Status != LedgerEntryStatusSucceeded {
		return fmt.Errorf("cannot refund payment in status %q", entry.Status)
	}
	if refundCents <= 0 || refundCents > (entry.AmountCents-entry.RefundedCents) {
		return fmt.Errorf("invalid refund amount: %d (max refundable: %d)", refundCents, entry.AmountCents-entry.RefundedCents)
	}

	a.Enqueue(&PaymentRefunded{
		AccountID:      a.ID,
		PaymentID:      entry.PaymentID,
		StripeRefundID: stripeRefundID,
		AmountCents:    refundCents,
		Reason:         reason,
		Timestamp:      time.Now().UTC(),
	}, a.Apply)

	return nil
}

// RefundPaymentWithDetails records a refund with full Stripe metadata.
func (a *PaymentAccountAggregate) RefundPaymentWithDetails(
	stripePaymentID, stripeRefundID, stripeChargeID, stripeEventID string,
	refundCents int64, currency, reason, refundStatus string,
) error {
	// Stripe event dedup check
	if a.isStripeEventProcessed(stripeEventID) {
		return nil
	}

	entry := a.findLedgerEntry(stripePaymentID)
	if entry == nil {
		return fmt.Errorf("no payment found for stripe PI %s", stripePaymentID)
	}
	if entry.Status != LedgerEntryStatusSucceeded {
		return fmt.Errorf("cannot refund payment in status %q", entry.Status)
	}
	if refundCents <= 0 || refundCents > entry.MaxRefundable() {
		return fmt.Errorf("invalid refund amount: %d (max refundable: %d)", refundCents, entry.MaxRefundable())
	}

	a.Enqueue(&PaymentRefunded{
		AccountID:      a.ID,
		PaymentID:      entry.PaymentID,
		StripeRefundID: stripeRefundID,
		StripeChargeID: stripeChargeID,
		StripeEventID:  stripeEventID,
		AmountCents:    refundCents,
		Currency:       currency,
		Reason:         reason,
		RefundStatus:   refundStatus,
		Timestamp:      time.Now().UTC(),
	}, a.Apply)

	return nil
}

// RecordDispute records a payment dispute (chargeback) opened against a payment.
// Disputes are critical for production — they can freeze funds and carry penalties.
//
// Stripe alignment: maps to charge.dispute.created webhook event.
func (a *PaymentAccountAggregate) RecordDispute(
	stripeDisputeID, stripeChargeID, stripeEventID string,
	amountCents int64, currency, reason, status string,
) error {
	if a.isStripeEventProcessed(stripeEventID) {
		return nil
	}

	// Find the ledger entry by charge ID or PI ID
	entry := a.findLedgerEntryByChargeID(stripeChargeID)
	if entry == nil {
		return fmt.Errorf("no payment found for stripe charge %s", stripeChargeID)
	}

	a.Enqueue(&PaymentDisputed{
		AccountID:       a.ID,
		PaymentID:       entry.PaymentID,
		StripeDisputeID: stripeDisputeID,
		StripeChargeID:  stripeChargeID,
		StripeEventID:   stripeEventID,
		AmountCents:     amountCents,
		Currency:        currency,
		Reason:          reason,
		Status:          status,
		Timestamp:       time.Now().UTC(),
	}, a.Apply)

	return nil
}

// ResolveDispute records the final outcome of a dispute (won or lost).
//
// Stripe alignment: maps to charge.dispute.closed webhook event.
func (a *PaymentAccountAggregate) ResolveDispute(
	stripeDisputeID, stripeEventID, status string, amountCents int64,
) error {
	if a.isStripeEventProcessed(stripeEventID) {
		return nil
	}

	a.Enqueue(&DisputeResolved{
		AccountID:       a.ID,
		StripeDisputeID: stripeDisputeID,
		StripeEventID:   stripeEventID,
		Status:          status,
		AmountCents:     amountCents,
		Timestamp:       time.Now().UTC(),
	}, a.Apply)

	return nil
}

// UpdatePaymentMethod records a payment method change (PCI-safe fields only).
//
// Stripe alignment: maps to payment_method.attached / payment_method.updated.
func (a *PaymentAccountAggregate) UpdatePaymentMethod(
	stripePaymentMethodID, stripeEventID, cardBrand, cardLast4 string,
	expMonth, expYear int64,
) error {
	if a.isStripeEventProcessed(stripeEventID) {
		return nil
	}

	a.Enqueue(&PaymentMethodUpdated{
		AccountID:             a.ID,
		StripePaymentMethodID: stripePaymentMethodID,
		StripeEventID:         stripeEventID,
		CardBrand:             cardBrand,
		CardLast4:             cardLast4,
		CardExpMonth:          expMonth,
		CardExpYear:           expYear,
		Timestamp:             time.Now().UTC(),
	}, a.Apply)

	return nil
}

// RecordInvoicePaymentFailure records a recurring invoice payment failure.
// This is critical for subscription billing — multiple failures may trigger
// dunning and eventual access revocation.
//
// Stripe alignment: maps to invoice.payment_failed webhook event.
func (a *PaymentAccountAggregate) RecordInvoicePaymentFailure(
	stripeInvoiceID, stripeEventID string,
	amountDueCents int64, currency string,
	attemptCount int64, nextAttemptAt *int64, errorCode string,
) error {
	if a.isStripeEventProcessed(stripeEventID) {
		return nil
	}

	a.Enqueue(&InvoicePaymentFailed{
		AccountID:       a.ID,
		StripeInvoiceID: stripeInvoiceID,
		StripeEventID:   stripeEventID,
		AmountDueCents:  amountDueCents,
		Currency:        currency,
		AttemptCount:    attemptCount,
		NextAttemptAt:   nextAttemptAt,
		ErrorCode:       errorCode,
		Timestamp:       time.Now().UTC(),
	}, a.Apply)

	return nil
}

// CancelSubscription records a cancellation request. Access continues until
// the end of the current billing period (cancelsOn).
func (a *PaymentAccountAggregate) CancelSubscription(cancelsOn time.Time, reason string) error {
	if !a.IsOpen {
		return ErrAccountNotOpen
	}
	if !a.IsActivated() {
		return fmt.Errorf("cannot cancel: account is not active")
	}
	if a.CancelledAt != nil {
		return nil // Idempotent — already cancelled
	}

	now := time.Now().UTC()
	a.Enqueue(&SubscriptionCancelled{
		AccountID:   a.ID,
		CancelledAt: now,
		CancelsOn:   cancelsOn,
		Reason:      reason,
		Timestamp:   now,
	}, a.Apply)

	return nil
}

// ExpireSubscription finalises a cancellation: revokes access after the
// billing period has elapsed. Called lazily when GetAccountStatus notices
// that time.Now() >= CancelsOn.
func (a *PaymentAccountAggregate) ExpireSubscription() error {
	if a.CancelledAt == nil {
		return fmt.Errorf("cannot expire: subscription was not cancelled")
	}
	if a.CancelsOn == nil {
		return fmt.Errorf("cannot expire: cancels_on not set")
	}
	// Idempotent — already expired
	if a.Expired {
		return nil
	}

	now := time.Now().UTC()
	a.Enqueue(&SubscriptionExpired{
		AccountID: a.ID,
		ExpiredAt: now,
		Timestamp: now,
	}, a.Apply)

	return nil
}

// ReactivateSubscription reverses a cancellation before the billing period
// ends. The subscription returns to full active status.
func (a *PaymentAccountAggregate) ReactivateSubscription() error {
	if !a.IsOpen {
		return ErrAccountNotOpen
	}
	if a.CancelledAt == nil {
		return ErrNotCancelled
	}
	// Guard: cannot reactivate if the billing period has already ended.
	// The caller should check this, but we enforce it here too.
	if a.CancelsOn != nil && !time.Now().UTC().Before(*a.CancelsOn) {
		return ErrBillingPeriodEnded
	}

	now := time.Now().UTC()
	a.Enqueue(&SubscriptionReactivated{
		AccountID:     a.ID,
		ReactivatedAt: now,
		Timestamp:     now,
	}, a.Apply)

	return nil
}

// -------------------------------------------------------------------------
// Queries on aggregate state
// -------------------------------------------------------------------------

// IsActivated returns true if the user has a net-positive balance and the
// subscription has not expired. The ledger balances remain truthful (never
// artificially zeroed) so the full financial history is always auditable.
func (a *PaymentAccountAggregate) IsActivated() bool {
	if a.Expired {
		return false
	}
	return a.TotalDebitedCents > a.TotalRefundedCents
}

// NetBalance returns the net amount the user has paid (debits − refunds).
func (a *PaymentAccountAggregate) NetBalance() int64 {
	return a.TotalDebitedCents - a.TotalRefundedCents
}

// EffectiveBalance returns the net amount considering disputes.
// This is the true financial position accounting for frozen funds.
func (a *PaymentAccountAggregate) EffectiveBalance() int64 {
	return a.TotalDebitedCents - a.TotalRefundedCents - a.TotalDisputedCents
}

// IsCancelled returns true if the subscription has been cancelled.
func (a *PaymentAccountAggregate) IsCancelled() bool {
	return a.CancelledAt != nil && a.CancelsOn != nil
}

// HasPendingPayment returns true if there's an unresolved payment attempt.
func (a *PaymentAccountAggregate) HasPendingPayment() bool {
	for _, e := range a.Ledger {
		if e.Status == LedgerEntryStatusPending {
			return true
		}
	}
	return false
}

// PendingStripePaymentID returns the Stripe PaymentIntent ID of the first
// pending ledger entry, or empty string if none.
func (a *PaymentAccountAggregate) PendingStripePaymentID() string {
	for _, e := range a.Ledger {
		if e.Status == LedgerEntryStatusPending {
			return e.StripePaymentID
		}
	}
	return ""
}

// IsInGoodStanding returns true if the account has no active disputes
// and no consecutive invoice failures. This is a production health check.
func (a *PaymentAccountAggregate) IsInGoodStanding() bool {
	return !a.HasActiveDispute && a.ConsecutiveInvoiceFailures == 0
}

// LastSucceededPayment returns the most recent succeeded ledger entry, or nil.
func (a *PaymentAccountAggregate) LastSucceededPayment() *LedgerEntry {
	for i := len(a.Ledger) - 1; i >= 0; i-- {
		if a.Ledger[i].Status == LedgerEntryStatusSucceeded {
			return &a.Ledger[i]
		}
	}
	return nil
}

// -------------------------------------------------------------------------
// Apply — the single state-mutation point, called during Enqueue & replay
// -------------------------------------------------------------------------

// Apply applies an event to rebuild aggregate state.
func (a *PaymentAccountAggregate) Apply(event es.Event) {
	switch e := event.(type) {
	case *PaymentAccountOpened:
		a.applyAccountOpened(e)
	case *PaymentInitiated:
		a.applyPaymentInitiated(e)
	case *PaymentSucceeded:
		a.applyPaymentSucceeded(e)
	case *PaymentFailed:
		a.applyPaymentFailed(e)
	case *PaymentRefunded:
		a.applyPaymentRefunded(e)
	case *PaymentDisputed:
		a.applyPaymentDisputed(e)
	case *DisputeResolved:
		a.applyDisputeResolved(e)
	case *PaymentMethodUpdated:
		a.applyPaymentMethodUpdated(e)
	case *InvoicePaymentFailed:
		a.applyInvoicePaymentFailed(e)
	case *SubscriptionCancelled:
		a.CancelledAt = &e.CancelledAt
		a.CancelsOn = &e.CancelsOn
	case *SubscriptionExpired:
		// Ledger balances are preserved intact so they remain a truthful
		// financial record even after the subscription expires.
		a.Expired = true
		a.CancelledAt = nil
		a.CancelsOn = nil
	case *SubscriptionReactivated:
		a.CancelledAt = nil
		a.CancelsOn = nil
		a.Expired = false
	}
}

// recordStripeEvent stores a Stripe event ID for idempotency so a
// replayed webhook doesn't apply the same state change twice. Blank IDs
// are skipped so the map doesn't accumulate empty keys from legacy
// events that predate event-ID tracking.
func (a *PaymentAccountAggregate) recordStripeEvent(id string) {
	if id != "" {
		a.ProcessedStripeEvents[id] = struct{}{}
	}
}

func (a *PaymentAccountAggregate) applyAccountOpened(e *PaymentAccountOpened) {
	a.Email = e.Email
	a.OpenedAt = e.Timestamp
	a.IsOpen = true
}

func (a *PaymentAccountAggregate) applyPaymentInitiated(e *PaymentInitiated) {
	a.StripeCustomerID = e.StripeCustomerID
	a.Ledger = append(a.Ledger, LedgerEntry{
		PaymentID:       e.PaymentID,
		StripePaymentID: e.StripePaymentID,
		AmountCents:     e.AmountCents,
		Currency:        e.Currency,
		Status:          LedgerEntryStatusPending,
		Description:     e.Description,
		InitiatedAt:     e.Timestamp,
	})
}

func (a *PaymentAccountAggregate) applyPaymentSucceeded(e *PaymentSucceeded) {
	if entry := a.findLedgerEntry(e.StripePaymentID); entry != nil {
		entry.Status = LedgerEntryStatusSucceeded
		entry.ResolvedAt = &e.Timestamp
		entry.CardBrand = e.CardBrand
		entry.CardLast4 = e.CardLast4
		entry.StripeChargeID = e.StripeChargeID
		entry.ReceiptURL = e.ReceiptURL
	}
	a.TotalDebitedCents += e.AmountCents
	// A new payment after expiration reactivates the account; reset the
	// invoice-failure counter since dunning only cares about the current
	// run of failures.
	a.Expired = false
	a.ConsecutiveInvoiceFailures = 0
	// Aggregate-level card info tracks the most recent successful charge.
	if e.CardBrand != "" {
		a.CardBrand = e.CardBrand
	}
	if e.CardLast4 != "" {
		a.CardLast4 = e.CardLast4
	}
	if e.ReceiptURL != "" {
		a.ReceiptURL = e.ReceiptURL
	}
	a.recordStripeEvent(e.StripeEventID)
}

func (a *PaymentAccountAggregate) applyPaymentFailed(e *PaymentFailed) {
	if entry := a.findLedgerEntry(e.StripePaymentID); entry != nil {
		entry.Status = LedgerEntryStatusFailed
		entry.FailureReason = e.Reason
		entry.ErrorCode = e.ErrorCode
		entry.DeclineCode = e.DeclineCode
		entry.ResolvedAt = &e.Timestamp
	}
	a.recordStripeEvent(e.StripeEventID)
}

func (a *PaymentAccountAggregate) applyPaymentRefunded(e *PaymentRefunded) {
	if entry := a.findLedgerEntryByID(e.PaymentID); entry != nil {
		entry.RefundedCents += e.AmountCents
		if entry.RefundedCents >= entry.AmountCents {
			entry.Status = LedgerEntryStatusRefunded
		}
	}
	a.TotalRefundedCents += e.AmountCents
	a.recordStripeEvent(e.StripeEventID)
}

func (a *PaymentAccountAggregate) applyPaymentDisputed(e *PaymentDisputed) {
	if entry := a.findLedgerEntryByID(e.PaymentID); entry != nil {
		entry.DisputedCents += e.AmountCents
		entry.Status = LedgerEntryStatusDisputed
	}
	a.TotalDisputedCents += e.AmountCents
	a.ActiveDisputes++
	a.HasActiveDispute = true
	a.recordStripeEvent(e.StripeEventID)
}

// applyDisputeResolved distinguishes won vs. lost — lost disputes move
// funds into refunded (the money's gone), won disputes release the
// disputed amount back to the debited balance.
func (a *PaymentAccountAggregate) applyDisputeResolved(e *DisputeResolved) {
	if e.Status == string(DisputeStatusLost) {
		a.TotalRefundedCents += e.AmountCents
	} else {
		a.TotalDisputedCents -= e.AmountCents
	}
	a.ActiveDisputes--
	if a.ActiveDisputes <= 0 {
		a.ActiveDisputes = 0
		a.HasActiveDispute = false
	}
	a.recordStripeEvent(e.StripeEventID)
}

func (a *PaymentAccountAggregate) applyPaymentMethodUpdated(e *PaymentMethodUpdated) {
	if e.CardBrand != "" {
		a.CardBrand = e.CardBrand
	}
	if e.CardLast4 != "" {
		a.CardLast4 = e.CardLast4
	}
	a.recordStripeEvent(e.StripeEventID)
}

func (a *PaymentAccountAggregate) applyInvoicePaymentFailed(e *InvoicePaymentFailed) {
	a.ConsecutiveInvoiceFailures++
	a.recordStripeEvent(e.StripeEventID)
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func (a *PaymentAccountAggregate) findLedgerEntry(stripePaymentID string) *LedgerEntry {
	for i := range a.Ledger {
		if a.Ledger[i].StripePaymentID == stripePaymentID {
			return &a.Ledger[i]
		}
	}
	return nil
}

func (a *PaymentAccountAggregate) findLedgerEntryByID(paymentID string) *LedgerEntry {
	for i := range a.Ledger {
		if a.Ledger[i].PaymentID == paymentID {
			return &a.Ledger[i]
		}
	}
	return nil
}

func (a *PaymentAccountAggregate) findLedgerEntryByChargeID(stripeChargeID string) *LedgerEntry {
	for i := range a.Ledger {
		if a.Ledger[i].StripeChargeID == stripeChargeID {
			return &a.Ledger[i]
		}
	}
	return nil
}

// -------------------------------------------------------------------------
// Sentinel errors
// -------------------------------------------------------------------------

// ErrAlreadyActivated is returned when a user tries to pay after activation.
var ErrAlreadyActivated = errors.New("account is already activated")

// ErrAccountNotOpen is returned when an operation requires an open account.
var ErrAccountNotOpen = errors.New("account is not open")

// ErrNotCancelled is returned when trying to reactivate a non-cancelled subscription.
var ErrNotCancelled = errors.New("subscription is not cancelled")

// ErrBillingPeriodEnded is returned when trying to reactivate after the billing period.
var ErrBillingPeriodEnded = errors.New("cannot reactivate: billing period has ended")

// ErrAccountNotFound is a billing-domain alias for es.ErrNotFound.
// The generic repository uses ErrNotFound for all aggregates;
// this alias makes payment-context code self-documenting.
var ErrAccountNotFound = es.ErrNotFound

// ErrDisputeActive is returned when an operation is blocked by an active dispute.
var ErrDisputeActive = errors.New("account has an active dispute")

// ErrInvoicePaymentOverdue is returned when the account has too many consecutive
// invoice failures and requires manual intervention.
var ErrInvoicePaymentOverdue = errors.New("account has overdue invoice payments")
