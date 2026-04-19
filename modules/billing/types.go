package billing

// ==========================================================================
// Stripe-Aligned Payment Types
//
// These types mirror the Stripe Go SDK (v84) type system for domain-level
// consistency. By using typed string enums matching Stripe's own constants,
// we eliminate strconv bugs, enable exhaustive switch checks, and keep the
// domain language aligned with Stripe's API vocabulary.
//
// Reference: github.com/stripe/stripe-go/v84
// ==========================================================================

// ---------------------------------------------------------------------------
// PaymentIntent Status — mirrors stripe.PaymentIntentStatus
// ---------------------------------------------------------------------------

// PaymentIntentStatus represents the lifecycle state of a PaymentIntent.
// These values match Stripe's API exactly, enabling direct comparison
// with webhook payloads without string conversion.
type PaymentIntentStatus string

const (
	PaymentIntentStatusRequiresPaymentMethod PaymentIntentStatus = "requires_payment_method"
	PaymentIntentStatusRequiresConfirmation  PaymentIntentStatus = "requires_confirmation"
	PaymentIntentStatusRequiresAction        PaymentIntentStatus = "requires_action"
	PaymentIntentStatusProcessing            PaymentIntentStatus = "processing"
	PaymentIntentStatusRequiresCapture       PaymentIntentStatus = "requires_capture"
	PaymentIntentStatusSucceeded             PaymentIntentStatus = "succeeded"
	PaymentIntentStatusCanceled              PaymentIntentStatus = "canceled"
)

// ---------------------------------------------------------------------------
// Charge Status — mirrors stripe.ChargeStatus
// ---------------------------------------------------------------------------

// ChargeStatus represents the state of a Stripe Charge.
type ChargeStatus string

const (
	ChargeStatusPending   ChargeStatus = "pending"
	ChargeStatusSucceeded ChargeStatus = "succeeded"
	ChargeStatusFailed    ChargeStatus = "failed"
)

// ---------------------------------------------------------------------------
// Refund Status & Reason — mirrors stripe.RefundStatus, stripe.RefundReason
// ---------------------------------------------------------------------------

// RefundStatus represents the lifecycle state of a refund.
type RefundStatus string

const (
	RefundStatusPending        RefundStatus = "pending"
	RefundStatusSucceeded      RefundStatus = "succeeded"
	RefundStatusFailed         RefundStatus = "failed"
	RefundStatusCanceled       RefundStatus = "canceled"
	RefundStatusRequiresAction RefundStatus = "requires_action"
)

// RefundReason represents why a refund was issued, matching Stripe's enum.
type RefundReason string

const (
	RefundReasonDuplicate               RefundReason = "duplicate"
	RefundReasonFraudulent              RefundReason = "fraudulent"
	RefundReasonRequestedByCustomer     RefundReason = "requested_by_customer"
	RefundReasonExpiredUncapturedCharge RefundReason = "expired_uncaptured_charge"
)

// ---------------------------------------------------------------------------
// Subscription Status — mirrors stripe.SubscriptionStatus
// ---------------------------------------------------------------------------

// SubscriptionStatus represents the lifecycle state of a Stripe Subscription.
type SubscriptionStatus string

const (
	SubscriptionStatusActive            SubscriptionStatus = "active"
	SubscriptionStatusCanceled          SubscriptionStatus = "canceled"
	SubscriptionStatusIncomplete        SubscriptionStatus = "incomplete"
	SubscriptionStatusIncompleteExpired SubscriptionStatus = "incomplete_expired"
	SubscriptionStatusPastDue           SubscriptionStatus = "past_due"
	SubscriptionStatusPaused            SubscriptionStatus = "paused"
	SubscriptionStatusTrialing          SubscriptionStatus = "trialing"
	SubscriptionStatusUnpaid            SubscriptionStatus = "unpaid"
)

// ---------------------------------------------------------------------------
// Checkout Session — mirrors stripe.CheckoutSessionStatus
// ---------------------------------------------------------------------------

// CheckoutSessionStatus represents the state of a Stripe Checkout Session.
type CheckoutSessionStatus string

const (
	CheckoutSessionStatusOpen     CheckoutSessionStatus = "open"
	CheckoutSessionStatusComplete CheckoutSessionStatus = "complete"
	CheckoutSessionStatusExpired  CheckoutSessionStatus = "expired"
)

// CheckoutSessionPaymentStatus mirrors stripe.CheckoutSessionPaymentStatus.
type CheckoutSessionPaymentStatus string

const (
	CheckoutSessionPaymentStatusUnpaid            CheckoutSessionPaymentStatus = "unpaid"
	CheckoutSessionPaymentStatusPaid              CheckoutSessionPaymentStatus = "paid"
	CheckoutSessionPaymentStatusNoPaymentRequired CheckoutSessionPaymentStatus = "no_payment_required"
)

// ---------------------------------------------------------------------------
// Dispute Status — mirrors stripe.DisputeStatus
// ---------------------------------------------------------------------------

// DisputeStatus represents the lifecycle of a payment dispute (chargeback).
type DisputeStatus string

const (
	DisputeStatusNeedsResponse    DisputeStatus = "needs_response"
	DisputeStatusUnderReview      DisputeStatus = "under_review"
	DisputeStatusWon              DisputeStatus = "won"
	DisputeStatusLost             DisputeStatus = "lost"
	DisputeStatusChargeRefunded   DisputeStatus = "charge_refunded"
	DisputeStatusWarningNeedsResp DisputeStatus = "warning_needs_response"
	DisputeStatusWarningUnderRev  DisputeStatus = "warning_under_review"
	DisputeStatusWarningClosed    DisputeStatus = "warning_closed"
)

// DisputeReason mirrors stripe.DisputeReason.
type DisputeReason string

const (
	DisputeReasonCreditNotProcessed   DisputeReason = "credit_not_processed"
	DisputeReasonDuplicate            DisputeReason = "duplicate"
	DisputeReasonFraudulent           DisputeReason = "fraudulent"
	DisputeReasonGeneral              DisputeReason = "general"
	DisputeReasonProductNotReceived   DisputeReason = "product_not_received"
	DisputeReasonProductUnacceptable  DisputeReason = "product_unacceptable"
	DisputeReasonSubscriptionCanceled DisputeReason = "subscription_canceled"
	DisputeReasonUnrecognized         DisputeReason = "unrecognized"
)

// ---------------------------------------------------------------------------
// Payment Error Classification — mirrors stripe.ErrorCode, stripe.DeclineCode
// ---------------------------------------------------------------------------

// PaymentErrorCode classifies payment failures using Stripe's error taxonomy.
// This enables domain-level error routing without coupling to the SDK.
type PaymentErrorCode string

const (
	PaymentErrorCardDeclined           PaymentErrorCode = "card_declined"
	PaymentErrorExpiredCard            PaymentErrorCode = "expired_card"
	PaymentErrorIncorrectCVC           PaymentErrorCode = "incorrect_cvc"
	PaymentErrorIncorrectNumber        PaymentErrorCode = "incorrect_number"
	PaymentErrorInsufficientFunds      PaymentErrorCode = "insufficient_funds"
	PaymentErrorProcessingError        PaymentErrorCode = "processing_error"
	PaymentErrorAuthenticationRequired PaymentErrorCode = "authentication_required"
	PaymentErrorRateLimit              PaymentErrorCode = "rate_limit"
	PaymentErrorAmountTooLarge         PaymentErrorCode = "amount_too_large"
	PaymentErrorAmountTooSmall         PaymentErrorCode = "amount_too_small"
)

// DeclineCode represents card-specific decline reasons from the issuing bank.
// Mirrors stripe.DeclineCode for the most common decline scenarios.
type DeclineCode string

const (
	DeclineCodeGenericDecline         DeclineCode = "generic_decline"
	DeclineCodeInsufficientFunds      DeclineCode = "insufficient_funds"
	DeclineCodeLostCard               DeclineCode = "lost_card"
	DeclineCodeStolenCard             DeclineCode = "stolen_card"
	DeclineCodeExpiredCard            DeclineCode = "expired_card"
	DeclineCodeIncorrectCVC           DeclineCode = "incorrect_cvc"
	DeclineCodeIncorrectNumber        DeclineCode = "incorrect_number"
	DeclineCodeCardNotSupported       DeclineCode = "card_not_supported"
	DeclineCodeDoNotHonor             DeclineCode = "do_not_honor"
	DeclineCodeFraudulent             DeclineCode = "fraudulent"
	DeclineCodeAuthenticationRequired DeclineCode = "authentication_required"
)

// ---------------------------------------------------------------------------
// Stripe Webhook Event Types — mirrors stripe.EventType for events we handle
// ---------------------------------------------------------------------------

// StripeEventType represents a Stripe webhook event type string.
// Using typed constants ensures webhook switch statements are exhaustive
// and typos are caught at compile time.
type StripeEventType string

const (
	// PaymentIntent lifecycle
	StripeEventPaymentIntentCreated        StripeEventType = "payment_intent.created"
	StripeEventPaymentIntentSucceeded      StripeEventType = "payment_intent.succeeded"
	StripeEventPaymentIntentFailed         StripeEventType = "payment_intent.payment_failed"
	StripeEventPaymentIntentCanceled       StripeEventType = "payment_intent.canceled"
	StripeEventPaymentIntentProcessing     StripeEventType = "payment_intent.processing"
	StripeEventPaymentIntentRequiresAction StripeEventType = "payment_intent.requires_action"

	// Charge lifecycle
	StripeEventChargeSucceeded StripeEventType = "charge.succeeded"
	StripeEventChargeFailed    StripeEventType = "charge.failed"
	StripeEventChargeRefunded  StripeEventType = "charge.refunded"
	StripeEventChargeCaptured  StripeEventType = "charge.captured"
	StripeEventChargeExpired   StripeEventType = "charge.expired"
	StripeEventChargeUpdated   StripeEventType = "charge.updated"

	// Dispute (chargeback) lifecycle
	StripeEventDisputeCreated StripeEventType = "charge.dispute.created"
	StripeEventDisputeUpdated StripeEventType = "charge.dispute.updated"
	StripeEventDisputeClosed  StripeEventType = "charge.dispute.closed"

	// Customer subscription lifecycle
	StripeEventSubscriptionCreated StripeEventType = "customer.subscription.created"
	StripeEventSubscriptionUpdated StripeEventType = "customer.subscription.updated"
	StripeEventSubscriptionDeleted StripeEventType = "customer.subscription.deleted"
	StripeEventSubscriptionPaused  StripeEventType = "customer.subscription.paused"
	StripeEventSubscriptionResumed StripeEventType = "customer.subscription.resumed"

	// Invoice lifecycle (subscription billing)
	StripeEventInvoicePaid          StripeEventType = "invoice.paid"
	StripeEventInvoicePaymentFailed StripeEventType = "invoice.payment_failed"
	StripeEventInvoiceCreated       StripeEventType = "invoice.created"
	StripeEventInvoiceFinalized     StripeEventType = "invoice.finalized"
	StripeEventInvoiceUpcoming      StripeEventType = "invoice.upcoming"

	// Checkout session lifecycle
	StripeEventCheckoutSessionCompleted StripeEventType = "checkout.session.completed"
	StripeEventCheckoutSessionExpired   StripeEventType = "checkout.session.expired"

	// Refund lifecycle
	StripeEventRefundCreated StripeEventType = "refund.created"
	StripeEventRefundUpdated StripeEventType = "refund.updated"
	StripeEventRefundFailed  StripeEventType = "refund.failed"

	// Payment method lifecycle
	StripeEventPaymentMethodAttached StripeEventType = "payment_method.attached"
	StripeEventPaymentMethodDetached StripeEventType = "payment_method.detached"
	StripeEventPaymentMethodUpdated  StripeEventType = "payment_method.updated"

	// Customer lifecycle
	StripeEventCustomerCreated StripeEventType = "customer.created"
	StripeEventCustomerUpdated StripeEventType = "customer.updated"
	StripeEventCustomerDeleted StripeEventType = "customer.deleted"
)

// ---------------------------------------------------------------------------
// Ledger Entry Status — typed enum replacing raw strings in the aggregate
// ---------------------------------------------------------------------------

// LedgerEntryStatus represents the lifecycle state of a payment attempt
// within the event-sourced ledger.
type LedgerEntryStatus string

const (
	LedgerEntryStatusPending   LedgerEntryStatus = "pending"
	LedgerEntryStatusSucceeded LedgerEntryStatus = "succeeded"
	LedgerEntryStatusFailed    LedgerEntryStatus = "failed"
	LedgerEntryStatusRefunded  LedgerEntryStatus = "refunded"
	LedgerEntryStatusDisputed  LedgerEntryStatus = "disputed"
	LedgerEntryStatusCanceled  LedgerEntryStatus = "canceled"
)

// IsResolved returns true if the entry is in a terminal state.
func (s LedgerEntryStatus) IsResolved() bool {
	switch s {
	case LedgerEntryStatusSucceeded, LedgerEntryStatusFailed,
		LedgerEntryStatusRefunded, LedgerEntryStatusCanceled:
		return true
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// Account Status — typed enum for the payment account projection
// ---------------------------------------------------------------------------

// AccountStatusValue represents the projected status of a payment account.
type AccountStatusValue string

const (
	AccountStatusNone      AccountStatusValue = "none"
	AccountStatusPending   AccountStatusValue = "pending"
	AccountStatusActive    AccountStatusValue = "active"
	AccountStatusCancelled AccountStatusValue = "cancelled"
	AccountStatusPastDue   AccountStatusValue = "past_due"
	AccountStatusDisputed  AccountStatusValue = "disputed"
)

// ---------------------------------------------------------------------------
// Currency — typed ISO 4217 currency codes matching Stripe's Currency type
// ---------------------------------------------------------------------------

// Currency represents an ISO 4217 currency code in lowercase (Stripe convention).
type Currency string

const (
	CurrencyUSD Currency = "usd"
	CurrencyEUR Currency = "eur"
	CurrencyGBP Currency = "gbp"
	CurrencyCAD Currency = "cad"
	CurrencyAUD Currency = "aud"
	CurrencyJPY Currency = "jpy"
)

// ---------------------------------------------------------------------------
// Card Brand — typed enum matching Stripe's card brand values
// ---------------------------------------------------------------------------

// CardBrand represents the brand of a payment card, matching Stripe's values.
type CardBrand string

const (
	CardBrandVisa       CardBrand = "visa"
	CardBrandMastercard CardBrand = "mastercard"
	CardBrandAmex       CardBrand = "amex"
	CardBrandDiscover   CardBrand = "discover"
	CardBrandDiners     CardBrand = "diners"
	CardBrandJCB        CardBrand = "jcb"
	CardBrandUnionPay   CardBrand = "unionpay"
	CardBrandUnknown    CardBrand = "unknown"
)

// ---------------------------------------------------------------------------
// Stripe Metadata Keys — standardized metadata key names
// ---------------------------------------------------------------------------

// Stripe metadata keys used across PaymentIntent, Customer, and Charge objects.
// Centralizing these prevents key name drift across the codebase.
const (
	MetadataKeyUserID    = "user_id"
	MetadataKeyEmail     = "email"
	MetadataKeyPaymentID = "payment_id"
	MetadataKeyPurpose   = "purpose"
)

// MetadataPurpose values for the "purpose" metadata key.
const (
	MetadataPurposeMonthlySubscription = "monthly_subscription"
)
