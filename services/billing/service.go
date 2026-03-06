// Package billing provides the application service for the payment bounded
// context. It orchestrates the event-sourced PaymentAccountAggregate and
// Stripe API calls following banking practices:
//
//   - Every state change is an immutable event in the event store
//   - Full audit trail — the ledger can be reconstructed at any point
//   - Idempotent operations — safe against duplicate webhook delivery
//   - PCI SAQ-A compliant — raw card data never touches our servers
//   - Optimistic concurrency via event stream versioning
//   - Stripe SDK v84 alignment — typed statuses, error taxonomy, event dedup
package billing

import (
	"context"
	"errors"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"time"

	projPayment "school-with-ai/infra/projection/payment"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/customer"
	"github.com/stripe/stripe-go/v84/paymentintent"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// AmountCents is the monthly subscription fee in the smallest currency unit.
// Matches Stripe's int64 amount convention (stripe.PaymentIntent.Amount).
const AmountCents int64 = 500 // $5.00 USD

// Currency is the ISO 4217 currency code (lowercase, matches stripe.Currency).
const Currency = string(stripe.CurrencyUSD)

// MaxConsecutiveInvoiceFailures is the threshold before the account is flagged
// as overdue and requires manual intervention. Aligns with Stripe's dunning
// configuration (Dashboard → Billing → Smart Retries).
const MaxConsecutiveInvoiceFailures = 3

// ---------------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------------

// Service is the application-layer orchestrator for the payment bounded
// context. It loads/saves the aggregate through the event store and
// coordinates with the Stripe API.
type Service struct {
	log        *logger.Logger
	repo       *domain.PaymentAccountRepository
	projection *projPayment.PostgresPaymentProjection
}

// New creates a new billing service.
// stripe.Key must be set before calling any method that talks to Stripe.
func New(log *logger.Logger, repo *domain.PaymentAccountRepository, projection *projPayment.PostgresPaymentProjection) *Service {
	return &Service{
		log:        log,
		repo:       repo,
		projection: projection,
	}
}

// ---------------------------------------------------------------------------
// Queries (use the read-model projection for speed)
// ---------------------------------------------------------------------------

// HasPaid checks whether the user has an active (paid) account via the
// projection (read model). This is O(1), no event replay needed.
func (s *Service) HasPaid(ctx context.Context, userID string) (bool, error) {
	return s.projection.HasPaid(ctx, userID)
}

// GetAccountStatus returns the projected payment status. If the projection
// shows a non-active status, it performs lazy verification: loads the
// aggregate to check for pending Stripe PaymentIntents and auto-confirms
// any that have already succeeded on Stripe's side. This self-healing
// mechanism handles webhook failures, missed redirects, and race conditions.
func (s *Service) GetAccountStatus(ctx context.Context, userID string) (*projPayment.AccountStatus, error) {
	status, err := s.projection.GetAccountStatus(ctx, userID)
	if err != nil {
		return nil, err
	}

	// If already active (or no account at all), return immediately
	if status == nil || status.Status == "active" {
		return status, nil
	}

	// Lazy expiration: if the subscription is cancelled and the billing
	// period has elapsed, emit SubscriptionExpired to finalise it.
	if status.Status == "cancelled" && status.CancelsOn != nil && !time.Now().Before(*status.CancelsOn) {
		acct, loadErr := s.repo.Load(ctx, userID)
		if loadErr != nil {
			s.log.Info(ctx, "billing: lazy-expire skipped, aggregate load failed",
				"user_id", userID, "error", loadErr)
			return status, nil
		}
		if err := acct.ExpireSubscription(); err != nil {
			s.log.Error(ctx, "billing: lazy-expire command failed",
				"user_id", userID, "error", err)
			return status, nil
		}
		if err := s.repo.Save(ctx, acct); err != nil {
			s.log.Error(ctx, "billing: lazy-expire save failed",
				"user_id", userID, "error", err)
			return status, nil
		}
		s.log.Info(ctx, "billing: lazy-expire — subscription expired",
			"user_id", userID, "cancels_on", status.CancelsOn)

		// Return the definitive answer now; projection will catch up async
		status.Status = "none"
		status.PaidAt = nil
		status.CancelledAt = nil
		status.CancelsOn = nil
		return status, nil
	}

	// If cancelled but period hasn't ended yet, return as-is
	if status.Status == "cancelled" {
		return status, nil
	}

	// Lazy verification: check aggregate for pending PIs
	acct, loadErr := s.repo.Load(ctx, userID)
	if loadErr != nil {
		// Can't load aggregate — return stale projection, don't block the user
		s.log.Info(ctx, "billing: lazy-verify skipped, aggregate load failed",
			"user_id", userID, "error", loadErr)
		return status, nil
	}

	if acct.IsActivated() {
		// Aggregate says active but projection is stale — return active status.
		// The projection updater will catch up shortly.
		status.Status = "active"
		return status, nil
	}

	pendingPI := acct.PendingStripePaymentID()
	if pendingPI == "" {
		return status, nil
	}

	// Try to verify the pending PI with Stripe
	pi, getErr := paymentintent.Get(pendingPI, piGetParams(ctx))
	if getErr != nil {
		s.log.Info(ctx, "billing: lazy-verify stripe get failed",
			"user_id", userID, "stripe_pi", pendingPI, "error", getErr)
		return status, nil
	}

	if pi.Status != stripe.PaymentIntentStatusSucceeded {
		return status, nil
	}

	// PI succeeded on Stripe — confirm the aggregate with full details
	chargeID, cardBrand, cardLast4, receiptURL := extractChargeDetails(pi)
	if err := acct.ConfirmPaymentWithDetails(
		pi.ID, chargeID, "", // No event ID for lazy verification
		pi.Amount, string(pi.Currency), cardBrand, cardLast4, receiptURL,
	); err != nil {
		s.log.Error(ctx, "billing: lazy-verify confirm failed",
			"user_id", userID, "stripe_pi", pi.ID, "error", err)
		return status, nil
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		s.log.Error(ctx, "billing: lazy-verify save failed",
			"user_id", userID, "stripe_pi", pi.ID, "error", err)
		return status, nil
	}

	s.log.Info(ctx, "billing: lazy-verify confirmed payment",
		"user_id", userID, "stripe_pi", pi.ID)

	// Re-fetch the projection (the projection updater runs async, so
	// give the caller the definitive answer now)
	status.Status = "active"
	return status, nil
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

// CreatePaymentIntent opens a payment account (if needed), creates a Stripe
// PaymentIntent, records a PaymentInitiated event, and returns the client
// secret for Stripe Elements.
//
// If a pending PaymentIntent already exists (e.g. the user reopened the
// checkout dialog), we retrieve it from Stripe instead of creating a new one.
//
// Stripe SDK alignment: uses stripe.PaymentIntentParams with
// AutomaticPaymentMethods (the recommended approach per Stripe docs).
func (s *Service) CreatePaymentIntent(ctx context.Context, userID, email string) (clientSecret string, err error) {
	// Load or create the payment account aggregate
	acct, err := s.loadOrOpenAccount(ctx, userID, email)
	if err != nil {
		return "", err
	}

	// Guard: already activated
	if acct.IsActivated() {
		return "", domain.ErrAlreadyActivated
	}

	// If we already have a pending PaymentIntent, retrieve it from Stripe
	// and return the existing client secret. This avoids idempotency-key
	// collisions and duplicate charges.
	if pendingPI := acct.PendingStripePaymentID(); pendingPI != "" {
		pi, getErr := paymentintent.Get(pendingPI, piGetParams(ctx))
		if getErr != nil {
			s.log.Info(ctx, "billing: stale pending PI, will create new",
				"user_id", userID, "stripe_pi", pendingPI, "error", getErr)
		} else if pi.Status == stripe.PaymentIntentStatusRequiresPaymentMethod ||
			pi.Status == stripe.PaymentIntentStatusRequiresConfirmation ||
			pi.Status == stripe.PaymentIntentStatusRequiresAction {
			s.log.Info(ctx, "billing: reusing pending payment intent",
				"user_id", userID, "stripe_pi", pi.ID,
				"status", string(pi.Status))
			return pi.ClientSecret, nil
		}
		// If the PI is in a terminal/unexpected state, fall through and
		// create a fresh one.
	}

	// Ensure a Stripe Customer exists (maps to stripe.Customer)
	customerID, err := s.ensureStripeCustomer(ctx, userID, email, acct.StripeCustomerID)
	if err != nil {
		return "", err
	}

	// Generate internal payment ID — used as the idempotency key so each
	// distinct attempt gets its own Stripe PI without collisions.
	paymentID := uuid.New().String()

	// Create Stripe PaymentIntent using SDK v84 params.
	// AutomaticPaymentMethods dynamically shows payment methods enabled
	// in the Stripe Dashboard — preferred over hardcoded PaymentMethodTypes.
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(AmountCents),
		Currency: stripe.String(Currency),
		Customer: stripe.String(customerID),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Description:  stripe.String("SchoolWithAI \u2014 Monthly subscription"),
		ReceiptEmail: stripe.String(email),
		Metadata: map[string]string{
			domain.MetadataKeyUserID:    userID,
			domain.MetadataKeyEmail:     email,
			domain.MetadataKeyPaymentID: paymentID,
			domain.MetadataKeyPurpose:   domain.MetadataPurposeMonthlySubscription,
		},
	}
	params.SetIdempotencyKey(paymentID)
	params.Context = ctx

	pi, err := paymentintent.New(params)
	if err != nil {
		s.logStripeError(ctx, "billing: stripe create payment intent", err, "user_id", userID)
		return "", fmt.Errorf("billing: stripe create payment intent: %w", err)
	}

	// Record the initiation event on the aggregate
	if err := acct.InitiatePayment(paymentID, pi.ID, customerID, AmountCents, Currency, "Monthly subscription", paymentID); err != nil {
		// ErrAlreadyActivated or idempotent skip — both are safe
		if errors.Is(err, domain.ErrAlreadyActivated) {
			return "", err
		}
		return "", fmt.Errorf("billing: initiate payment: %w", err)
	}

	// Persist events to the event store (optimistic concurrency)
	if err := s.repo.Save(ctx, acct); err != nil {
		return "", fmt.Errorf("billing: save aggregate: %w", err)
	}

	s.log.Info(ctx, "billing: payment initiated",
		"user_id", userID,
		"payment_id", paymentID,
		"stripe_pi", pi.ID,
		"stripe_customer", customerID,
		"amount_cents", AmountCents,
		"currency", Currency,
	)

	return pi.ClientSecret, nil
}

// ConfirmPayment is called by the webhook handler when payment_intent.succeeded
// fires. Loads the aggregate, applies the succeeded event, and saves.
//
// Stripe SDK alignment: accepts stripeEventID for dedup, stripeChargeID and
// receiptURL from the expanded PaymentIntent.LatestCharge.
func (s *Service) ConfirmPayment(ctx context.Context, stripePaymentID string, amountCents int64, currency, cardBrand, cardLast4 string, metadata map[string]string) error {
	userID := metadata[domain.MetadataKeyUserID]
	if userID == "" {
		return fmt.Errorf("billing: webhook missing user_id in metadata")
	}

	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for confirm: %w", err)
	}

	if err := acct.ConfirmPayment(stripePaymentID, amountCents, currency, cardBrand, cardLast4); err != nil {
		return fmt.Errorf("billing: confirm payment: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after confirm: %w", err)
	}

	s.log.Info(ctx, "billing: payment confirmed", "user_id", userID, "stripe_pi", stripePaymentID, "card", cardBrand+" ••••"+cardLast4)
	return nil
}

// ConfirmPaymentWithDetails is the production-grade version that captures all
// Stripe traceability fields from the webhook payload.
func (s *Service) ConfirmPaymentWithDetails(
	ctx context.Context,
	stripePaymentID, stripeChargeID, stripeEventID string,
	amountCents int64, currency, cardBrand, cardLast4, receiptURL string,
	metadata map[string]string,
) error {
	userID := metadata[domain.MetadataKeyUserID]
	if userID == "" {
		return fmt.Errorf("billing: webhook missing user_id in metadata")
	}

	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for confirm: %w", err)
	}

	if err := acct.ConfirmPaymentWithDetails(
		stripePaymentID, stripeChargeID, stripeEventID,
		amountCents, currency, cardBrand, cardLast4, receiptURL,
	); err != nil {
		return fmt.Errorf("billing: confirm payment: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after confirm: %w", err)
	}

	s.log.Info(ctx, "billing: payment confirmed",
		"user_id", userID,
		"stripe_pi", stripePaymentID,
		"stripe_charge", stripeChargeID,
		"stripe_event", stripeEventID,
		"card", cardBrand+" ••••"+cardLast4,
		"receipt_url", receiptURL,
	)
	return nil
}

// FailPayment is called by the webhook when payment_intent.payment_failed fires.
func (s *Service) FailPayment(ctx context.Context, stripePaymentID, reason string, metadata map[string]string) error {
	userID := metadata[domain.MetadataKeyUserID]
	if userID == "" {
		return fmt.Errorf("billing: webhook missing user_id in metadata")
	}

	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for fail: %w", err)
	}

	if err := acct.FailPayment(stripePaymentID, reason); err != nil {
		return fmt.Errorf("billing: fail payment: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after fail: %w", err)
	}

	s.log.Info(ctx, "billing: payment failed", "user_id", userID, "stripe_pi", stripePaymentID, "reason", reason)
	return nil
}

// FailPaymentWithDetails is the production-grade version that captures the full
// Stripe error taxonomy (ErrorCode, DeclineCode, ErrorType) from the SDK.
func (s *Service) FailPaymentWithDetails(
	ctx context.Context,
	stripePaymentID, stripeEventID string,
	pi *stripe.PaymentIntent,
	metadata map[string]string,
) error {
	userID := metadata[domain.MetadataKeyUserID]
	if userID == "" {
		return fmt.Errorf("billing: webhook missing user_id in metadata")
	}

	// Extract Stripe error taxonomy from LastPaymentError (stripe.Error)
	reason, errorCode, declineCode, errorType, failureMessage := extractPaymentError(pi)

	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for fail: %w", err)
	}

	if err := acct.FailPaymentWithDetails(
		stripePaymentID, stripeEventID,
		reason, errorCode, declineCode, errorType, failureMessage,
	); err != nil {
		return fmt.Errorf("billing: fail payment: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after fail: %w", err)
	}

	s.log.Info(ctx, "billing: payment failed",
		"user_id", userID,
		"stripe_pi", stripePaymentID,
		"stripe_event", stripeEventID,
		"reason", reason,
		"error_code", errorCode,
		"decline_code", declineCode,
		"error_type", errorType,
	)
	return nil
}

// VerifyPayment is called by the redirect-return handler when the user lands
// back on the app after a successful Stripe payment. It retrieves the
// PaymentIntent from Stripe and, if succeeded, confirms the aggregate.
// This is the fallback path when webhooks are unavailable or delayed.
func (s *Service) VerifyPayment(ctx context.Context, userID, stripePaymentIntentID string) error {
	// Retrieve the PaymentIntent from Stripe to check its status
	pi, err := paymentintent.Get(stripePaymentIntentID, piGetParams(ctx))
	if err != nil {
		return fmt.Errorf("billing: stripe get payment intent: %w", err)
	}

	// Verify the PI belongs to this user (security check)
	if pi.Metadata[domain.MetadataKeyUserID] != userID {
		return fmt.Errorf("billing: payment intent does not belong to user")
	}

	if pi.Status != stripe.PaymentIntentStatusSucceeded {
		s.log.Info(ctx, "billing: verify-payment PI not succeeded",
			"user_id", userID, "stripe_pi", stripePaymentIntentID,
			"status", string(pi.Status))
		return fmt.Errorf("billing: payment intent status is %s, not succeeded", pi.Status)
	}

	// Load the aggregate
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for verify: %w", err)
	}

	// Already confirmed — idempotent
	if acct.IsActivated() {
		s.log.Info(ctx, "billing: verify-payment already activated", "user_id", userID)
		return nil
	}

	// Confirm via the aggregate with full Stripe details
	chargeID, cardBrand, cardLast4, receiptURL := extractChargeDetails(pi)
	if err := acct.ConfirmPaymentWithDetails(
		pi.ID, chargeID, "", // No event ID for redirect verification
		pi.Amount, string(pi.Currency), cardBrand, cardLast4, receiptURL,
	); err != nil {
		return fmt.Errorf("billing: confirm payment during verify: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after verify: %w", err)
	}

	s.log.Info(ctx, "billing: payment verified and confirmed via redirect",
		"user_id", userID, "stripe_pi", pi.ID, "stripe_charge", chargeID)
	return nil
}

// CancelSubscription records the user's cancellation request. Access continues
// until the end of the current billing period (one month from the activation
// date, or one month from today if no paid_at is available).
func (s *Service) CancelSubscription(ctx context.Context, userID, reason string) (cancelsOn time.Time, err error) {
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return time.Time{}, fmt.Errorf("billing: load account for cancel: %w", err)
	}

	// Calculate the end of the current billing period.
	// Billing period = the day-of-month they paid, each month.
	// If they paid Jan 15, the current period ends on the next upcoming 15th.
	now := time.Now().UTC()
	cancelsOn = calculateBillingPeriodEnd(now, acct.Ledger)

	if err := acct.CancelSubscription(cancelsOn, reason); err != nil {
		return time.Time{}, fmt.Errorf("billing: cancel subscription: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return time.Time{}, fmt.Errorf("billing: save after cancel: %w", err)
	}

	s.log.Info(ctx, "billing: subscription cancelled",
		"user_id", userID, "cancels_on", cancelsOn.Format("2006-01-02"))
	return cancelsOn, nil
}

// ReactivateSubscription reverses a cancellation before the billing period
// ends. The subscription returns to active.
func (s *Service) ReactivateSubscription(ctx context.Context, userID string) error {
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for reactivate: %w", err)
	}

	if err := acct.ReactivateSubscription(); err != nil {
		return fmt.Errorf("billing: reactivate subscription: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after reactivate: %w", err)
	}

	s.log.Info(ctx, "billing: subscription reactivated", "user_id", userID)
	return nil
}

// HandleRefund processes a charge.refunded webhook event. It finds the
// corresponding PaymentIntent, loads the user's aggregate, and records
// the refund event. This handles refunds initiated from the Stripe Dashboard.
//
// Stripe SDK alignment: extracts Refund.ID (re_xxx), Refund.Amount,
// Refund.Status, and Refund.Reason from the Charge.Refunds list.
// The charge.PaymentIntent object in webhooks is typically NOT expanded —
// it may only contain the ID. We check metadata first, then fetch the PI.
func (s *Service) HandleRefund(ctx context.Context, charge *stripe.Charge) error {
	if charge == nil {
		return fmt.Errorf("billing: charge.refunded received nil charge")
	}

	// Resolve the PaymentIntent ID. In webhook payloads the PI may be:
	// (a) a fully expanded object, (b) an object with just the ID, or
	// (c) nil (edge case for charges without a PI).
	var piID string
	if charge.PaymentIntent != nil {
		piID = charge.PaymentIntent.ID
	}
	if piID == "" {
		return fmt.Errorf("billing: charge.refunded missing payment intent for charge %s", charge.ID)
	}

	// Resolve user_id. Try charge metadata first (fastest), then PI metadata
	// from the webhook payload, then fetch the PI from Stripe as a fallback.
	userID := charge.Metadata[domain.MetadataKeyUserID]
	if userID == "" && charge.PaymentIntent != nil && charge.PaymentIntent.Metadata != nil {
		userID = charge.PaymentIntent.Metadata[domain.MetadataKeyUserID]
	}
	if userID == "" {
		// PaymentIntent wasn't expanded in the webhook — fetch it from Stripe
		pi, err := paymentintent.Get(piID, piGetParams(ctx))
		if err != nil {
			return fmt.Errorf("billing: charge.refunded failed to fetch PI %s: %w", piID, err)
		}
		userID = pi.Metadata[domain.MetadataKeyUserID]
	}
	if userID == "" {
		return fmt.Errorf("billing: charge.refunded missing user_id in metadata for charge %s (pi: %s)", charge.ID, piID)
	}

	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for refund: %w", err)
	}

	// Extract refund details from the Charge.Refunds list (Stripe SDK v84).
	// The most recent refund is first in the list.
	var refundAmount int64
	var refundID string
	var refundReason string
	var refundStatus string
	if charge.Refunds != nil && len(charge.Refunds.Data) > 0 {
		latestRefund := charge.Refunds.Data[0]
		refundAmount = latestRefund.Amount
		refundID = latestRefund.ID
		refundReason = string(latestRefund.Reason)
		refundStatus = string(latestRefund.Status)
	} else {
		refundAmount = charge.AmountRefunded
		refundID = fmt.Sprintf("re_auto_%s", charge.ID)
		refundReason = "stripe_dashboard_refund"
	}

	if refundAmount <= 0 {
		s.log.Info(ctx, "billing: charge.refunded with zero amount, skipping",
			"user_id", userID, "charge_id", charge.ID)
		return nil
	}

	if err := acct.RefundPaymentWithDetails(
		piID, refundID, charge.ID, "", // stripeEventID set by handler
		refundAmount, string(charge.Currency), refundReason, refundStatus,
	); err != nil {
		return fmt.Errorf("billing: refund payment: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after refund: %w", err)
	}

	s.log.Info(ctx, "billing: refund processed from Stripe webhook",
		"user_id", userID,
		"charge_id", charge.ID,
		"refund_id", refundID,
		"amount_cents", refundAmount,
		"refund_reason", refundReason,
		"refund_status", refundStatus,
	)
	return nil
}

// HandleDispute processes a charge.dispute.created webhook event.
// Disputes (chargebacks) are critical for production — they freeze funds,
// carry financial penalties, and may require evidence submission.
//
// Stripe SDK alignment: extracts from stripe.Dispute object.
func (s *Service) HandleDispute(ctx context.Context, dispute *stripe.Dispute, stripeEventID string) error {
	if dispute == nil {
		return fmt.Errorf("billing: dispute.created received nil dispute")
	}

	// Resolve the charge and payment intent
	var chargeID string
	if dispute.Charge != nil {
		chargeID = dispute.Charge.ID
	}
	if chargeID == "" {
		return fmt.Errorf("billing: dispute missing charge ID for dispute %s", dispute.ID)
	}

	// Resolve user_id from the charge or payment intent metadata
	userID := resolveUserIDFromDispute(dispute)
	if userID == "" {
		// Fallback: try fetching the PI from Stripe
		if dispute.PaymentIntent != nil && dispute.PaymentIntent.ID != "" {
			pi, err := paymentintent.Get(dispute.PaymentIntent.ID, piGetParams(ctx))
			if err != nil {
				return fmt.Errorf("billing: dispute failed to fetch PI: %w", err)
			}
			userID = pi.Metadata[domain.MetadataKeyUserID]
		}
	}
	if userID == "" {
		return fmt.Errorf("billing: dispute missing user_id for dispute %s (charge: %s)", dispute.ID, chargeID)
	}

	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for dispute: %w", err)
	}

	if err := acct.RecordDispute(
		dispute.ID,
		chargeID,
		stripeEventID,
		dispute.Amount,
		string(dispute.Currency),
		string(dispute.Reason),
		string(dispute.Status),
	); err != nil {
		return fmt.Errorf("billing: record dispute: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after dispute: %w", err)
	}

	s.log.Error(ctx, "billing: ⚠️  DISPUTE OPENED — requires attention",
		"user_id", userID,
		"dispute_id", dispute.ID,
		"charge_id", chargeID,
		"amount_cents", dispute.Amount,
		"reason", string(dispute.Reason),
		"status", string(dispute.Status),
		"stripe_event", stripeEventID,
	)
	return nil
}

// HandleDisputeClosed processes a charge.dispute.closed webhook event.
// Determines if the dispute was won or lost and updates the aggregate accordingly.
func (s *Service) HandleDisputeClosed(ctx context.Context, dispute *stripe.Dispute, stripeEventID string) error {
	if dispute == nil {
		return fmt.Errorf("billing: dispute.closed received nil dispute")
	}

	userID := resolveUserIDFromDispute(dispute)
	if userID == "" && dispute.PaymentIntent != nil && dispute.PaymentIntent.ID != "" {
		pi, err := paymentintent.Get(dispute.PaymentIntent.ID, piGetParams(ctx))
		if err != nil {
			return fmt.Errorf("billing: dispute.closed failed to fetch PI: %w", err)
		}
		userID = pi.Metadata[domain.MetadataKeyUserID]
	}
	if userID == "" {
		return fmt.Errorf("billing: dispute.closed missing user_id for dispute %s", dispute.ID)
	}

	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for dispute resolution: %w", err)
	}

	if err := acct.ResolveDispute(
		dispute.ID, stripeEventID,
		string(dispute.Status), dispute.Amount,
	); err != nil {
		return fmt.Errorf("billing: resolve dispute: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after dispute resolution: %w", err)
	}

	s.log.Info(ctx, "billing: dispute resolved",
		"user_id", userID,
		"dispute_id", dispute.ID,
		"status", string(dispute.Status),
		"amount_cents", dispute.Amount,
		"stripe_event", stripeEventID,
	)
	return nil
}

// HandleInvoicePaymentFailed processes an invoice.payment_failed webhook event.
// This tracks recurring payment failures for subscription health monitoring.
//
// Stripe SDK alignment: extracts from stripe.Invoice object.
func (s *Service) HandleInvoicePaymentFailed(ctx context.Context, invoice *stripe.Invoice, stripeEventID string) error {
	if invoice == nil {
		return fmt.Errorf("billing: invoice.payment_failed received nil invoice")
	}

	// Resolve user_id from customer metadata or invoice metadata
	var userID string
	if invoice.Customer != nil && invoice.Customer.Metadata != nil {
		userID = invoice.Customer.Metadata[domain.MetadataKeyUserID]
	}
	// In v84, subscription is accessed via Parent.SubscriptionDetails;
	// if customer metadata didn't work, try the parent subscription.
	if userID == "" && invoice.Parent != nil &&
		invoice.Parent.SubscriptionDetails != nil &&
		invoice.Parent.SubscriptionDetails.Subscription != nil &&
		invoice.Parent.SubscriptionDetails.Subscription.Metadata != nil {
		userID = invoice.Parent.SubscriptionDetails.Subscription.Metadata[domain.MetadataKeyUserID]
	}
	if userID == "" {
		s.log.Error(ctx, "billing: invoice.payment_failed missing user_id",
			"invoice_id", invoice.ID, "stripe_event", stripeEventID)
		return fmt.Errorf("billing: invoice.payment_failed missing user_id for invoice %s", invoice.ID)
	}

	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for invoice failure: %w", err)
	}

	var nextAttempt *int64
	if invoice.NextPaymentAttempt > 0 {
		nextAttempt = &invoice.NextPaymentAttempt
	}

	if err := acct.RecordInvoicePaymentFailure(
		invoice.ID, stripeEventID,
		invoice.AmountDue, string(invoice.Currency),
		invoice.AttemptCount, nextAttempt, "",
	); err != nil {
		return fmt.Errorf("billing: record invoice failure: %w", err)
	}

	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after invoice failure: %w", err)
	}

	s.log.Error(ctx, "billing: recurring invoice payment failed",
		"user_id", userID,
		"invoice_id", invoice.ID,
		"amount_due_cents", invoice.AmountDue,
		"attempt_count", invoice.AttemptCount,
		"consecutive_failures", acct.ConsecutiveInvoiceFailures,
		"stripe_event", stripeEventID,
	)
	return nil
}

// calculateBillingPeriodEnd determines when the current billing period ends.
// Uses the last succeeded payment date + 1 month. If no payment date is
// found, it defaults to 30 days from now.
func calculateBillingPeriodEnd(now time.Time, ledger []domain.LedgerEntry) time.Time {
	// Find the most recent succeeded payment date
	for i := len(ledger) - 1; i >= 0; i-- {
		if ledger[i].Status == domain.LedgerEntryStatusSucceeded && ledger[i].ResolvedAt != nil {
			return ledger[i].ResolvedAt.AddDate(0, 1, 0)
		}
	}
	// Fallback: 30 days from now
	return now.AddDate(0, 0, 30)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadOrOpenAccount loads the aggregate from the event store, or opens a new
// account if none exists (first-time user).
func (s *Service) loadOrOpenAccount(ctx context.Context, userID, email string) (*domain.PaymentAccountAggregate, error) {
	acct, err := s.repo.Load(ctx, userID)
	if err == nil {
		return acct, nil
	}

	// If not found, open a new account
	if errors.Is(err, domain.ErrAccountNotFound) {
		newAcct, openErr := domain.OpenPaymentAccount(userID, email)
		if openErr != nil {
			return nil, fmt.Errorf("billing: open account: %w", openErr)
		}
		if saveErr := s.repo.SaveNew(ctx, newAcct); saveErr != nil {
			return nil, fmt.Errorf("billing: save new account: %w", saveErr)
		}

		// Reload to get correct version after SaveNew
		return s.repo.Load(ctx, userID)
	}

	return nil, fmt.Errorf("billing: load account: %w", err)
}

// ensureStripeCustomer returns the existing Stripe customer ID or creates one.
// Uses stripe.CustomerParams with metadata for bidirectional traceability.
func (s *Service) ensureStripeCustomer(ctx context.Context, userID, email, existingCustomerID string) (string, error) {
	if existingCustomerID != "" {
		return existingCustomerID, nil
	}

	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			domain.MetadataKeyUserID: userID,
		},
	}
	params.Context = ctx
	c, err := customer.New(params)
	if err != nil {
		s.logStripeError(ctx, "billing: create stripe customer", err, "user_id", userID)
		return "", fmt.Errorf("billing: create stripe customer: %w", err)
	}
	return c.ID, nil
}

// extractChargeDetails safely pulls charge ID, card info, and receipt URL
// from a PaymentIntent's latest charge. Returns empty strings if not available.
//
// Stripe SDK alignment: reads from PaymentIntent.LatestCharge (stripe.Charge),
// which contains PaymentMethodDetails.Card (PCI-safe fields) and ReceiptURL.
func extractChargeDetails(pi *stripe.PaymentIntent) (chargeID, brand, last4, receiptURL string) {
	if pi.LatestCharge != nil {
		chargeID = pi.LatestCharge.ID
		receiptURL = pi.LatestCharge.ReceiptURL
		if pi.LatestCharge.PaymentMethodDetails != nil &&
			pi.LatestCharge.PaymentMethodDetails.Card != nil {
			card := pi.LatestCharge.PaymentMethodDetails.Card
			brand = string(card.Brand)
			last4 = card.Last4
		}
	}
	return chargeID, brand, last4, receiptURL
}

// extractCardInfo safely pulls card brand and last4 from a PaymentIntent's
// latest charge. Returns ("", "") if not available (e.g. non-card payment).
// These fields are PCI-safe (non-sensitive) and standard for display.
//
// Deprecated: Use extractChargeDetails for full charge metadata.
func extractCardInfo(pi *stripe.PaymentIntent) (brand, last4 string) {
	_, brand, last4, _ = extractChargeDetails(pi)
	return brand, last4
}

// extractPaymentError extracts the full Stripe error taxonomy from a
// PaymentIntent's LastPaymentError (stripe.Error). Returns empty strings
// if no error is present.
//
// Stripe SDK alignment: uses stripe.Error fields: Type, Code, DeclineCode, Msg.
func extractPaymentError(pi *stripe.PaymentIntent) (reason, errorCode, declineCode, errorType, failureMessage string) {
	reason = "unknown"
	if pi.LastPaymentError != nil {
		errorCode = string(pi.LastPaymentError.Code)
		declineCode = string(pi.LastPaymentError.DeclineCode)
		errorType = string(pi.LastPaymentError.Type)
		failureMessage = pi.LastPaymentError.Msg
		reason = errorCode
		if declineCode != "" {
			reason = fmt.Sprintf("%s:%s", errorCode, declineCode)
		}
	}
	return reason, errorCode, declineCode, errorType, failureMessage
}

// resolveUserIDFromDispute attempts to extract user_id from a dispute's
// associated charge or payment intent metadata.
func resolveUserIDFromDispute(dispute *stripe.Dispute) string {
	// Try charge metadata first
	if dispute.Charge != nil && dispute.Charge.Metadata != nil {
		if uid := dispute.Charge.Metadata[domain.MetadataKeyUserID]; uid != "" {
			return uid
		}
	}
	// Try payment intent metadata
	if dispute.PaymentIntent != nil && dispute.PaymentIntent.Metadata != nil {
		if uid := dispute.PaymentIntent.Metadata[domain.MetadataKeyUserID]; uid != "" {
			return uid
		}
	}
	return ""
}

// logStripeError extracts structured fields from a *stripe.Error (the SDK's
// typed API error) and logs them alongside any extra key-value pairs.
// This gives operators Stripe request IDs, error codes, and decline codes
// in structured logs without manual type-assertion at every call site.
//
// Stripe SDK alignment: uses stripe.Error typed fields from v84.
func (s *Service) logStripeError(ctx context.Context, msg string, err error, extra ...any) {
	fields := []any{"error", err}
	var stripeErr *stripe.Error
	if errors.As(err, &stripeErr) {
		fields = append(fields,
			"stripe_type", string(stripeErr.Type),
			"stripe_code", string(stripeErr.Code),
			"stripe_msg", stripeErr.Msg,
			"stripe_request_id", stripeErr.RequestID,
			"stripe_status", stripeErr.HTTPStatusCode,
		)
		if stripeErr.DeclineCode != "" {
			fields = append(fields, "stripe_decline_code", string(stripeErr.DeclineCode))
		}
	}
	fields = append(fields, extra...)
	s.log.Error(ctx, msg, fields...)
}

// ClassifyStripeError classifies a Stripe error into a domain error category.
// This enables domain-level error routing without tight coupling to the SDK.
//
// Stripe SDK alignment: uses stripe.ErrorType to determine if the error is
// retryable, requires user action, or is a permanent failure.
func ClassifyStripeError(err error) (retryable bool, userActionRequired bool) {
	var stripeErr *stripe.Error
	if !errors.As(err, &stripeErr) {
		return false, false
	}
	switch stripeErr.Type {
	case stripe.ErrorTypeCard:
		return false, stripeErr.Code == stripe.ErrorCodeAuthenticationRequired
	case stripe.ErrorTypeAPI:
		return true, false // API errors are typically transient
	case stripe.ErrorTypeIdempotency:
		return false, false // Idempotency errors mean we already processed this
	case stripe.ErrorTypeInvalidRequest:
		return false, false // Bad request — programming error, not retryable
	default:
		return false, false
	}
}

// piGetParams returns PaymentIntentParams with the latest_charge expanded
// so extractChargeDetails can read charge ID, card brand, last4, and receipt URL.
// Without this expansion the Stripe API returns the charge as an un-expanded ID string.
//
// Stripe SDK alignment: AddExpand follows Stripe's "object expansion" pattern.
func piGetParams(ctx context.Context) *stripe.PaymentIntentParams {
	p := &stripe.PaymentIntentParams{}
	p.AddExpand("latest_charge.payment_method_details")
	p.Context = ctx
	return p
}
