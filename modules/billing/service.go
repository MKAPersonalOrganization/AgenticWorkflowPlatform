package billing

import (
	"aicompass/foundation/logger"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/customer"
	"github.com/stripe/stripe-go/v84/paymentintent"
)

// AmountCents is the monthly subscription fee in the smallest currency unit.
const AmountCents int64 = 500

// DefaultCurrency is the ISO 4217 currency code used for new charges.
const DefaultCurrency = string(stripe.CurrencyUSD)

// MaxConsecutiveInvoiceFailures is the threshold before the account is flagged as overdue.
const MaxConsecutiveInvoiceFailures = 3

type service struct {
	log        *logger.Logger
	repo       *PaymentAccountRepository
	projection *postgresPaymentProjection
}

func newService(log *logger.Logger, repo *PaymentAccountRepository, projection *postgresPaymentProjection) *service {
	return &service{log: log, repo: repo, projection: projection}
}

func (s *service) hasPaid(ctx context.Context, userID string) (bool, error) {
	return s.projection.hasPaid(ctx, userID)
}

func (s *service) getAccountStatus(ctx context.Context, userID string) (*accountStatus, error) {
	status, err := s.projection.getAccountStatus(ctx, userID)
	if err != nil {
		return nil, err
	}
	if status == nil || status.Status == "active" {
		return status, nil
	}

	if status.Status == "cancelled" {
		if cancelExpired(status) {
			s.lazilyExpireCancelledSubscription(ctx, userID, status)
		}
		return status, nil
	}

	s.lazilyVerifyPendingPayment(ctx, userID, status)
	return status, nil
}

// cancelExpired reports whether a cancelled subscription has reached its
// CancelsOn date and should be converted to "none" on next read.
func cancelExpired(status *accountStatus) bool {
	return status.CancelsOn != nil && !time.Now().Before(*status.CancelsOn)
}

// lazilyExpireCancelledSubscription mutates status in place to reflect expiry
// when a "cancelled" account has passed its CancelsOn date. Any failure along
// the way (aggregate load / ExpireSubscription / save) is logged and the
// function returns silently — callers still get the pre-expiry projection.
func (s *service) lazilyExpireCancelledSubscription(ctx context.Context, userID string, status *accountStatus) {
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		s.log.Info(ctx, "billing: lazy-expire skipped, aggregate load failed", "user_id", userID, "error", err)
		return
	}
	if err := acct.ExpireSubscription(); err != nil {
		s.log.Error(ctx, "billing: lazy-expire command failed", "user_id", userID, "error", err)
		return
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		s.log.Error(ctx, "billing: lazy-expire save failed", "user_id", userID, "error", err)
		return
	}
	s.log.Info(ctx, "billing: lazy-expire completed", "user_id", userID, "cancels_on", status.CancelsOn)
	status.Status = "none"
	status.PaidAt = nil
	status.CancelledAt = nil
	status.CancelsOn = nil
}

// lazilyVerifyPendingPayment checks Stripe for any PI the aggregate is waiting
// on and confirms the payment locally if Stripe reports success. Mirrors the
// error policy of lazilyExpireCancelledSubscription: all failures are logged
// and swallowed; status is only promoted to "active" on a fully successful path.
func (s *service) lazilyVerifyPendingPayment(ctx context.Context, userID string, status *accountStatus) {
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		s.log.Info(ctx, "billing: lazy-verify skipped, aggregate load failed", "user_id", userID, "error", err)
		return
	}
	if acct.IsActivated() {
		status.Status = "active"
		return
	}

	pendingPI := acct.PendingStripePaymentID()
	if pendingPI == "" {
		return
	}

	pi, err := paymentintent.Get(pendingPI, piGetParams(ctx))
	if err != nil {
		s.log.Info(ctx, "billing: lazy-verify stripe get failed", "user_id", userID, "stripe_pi", pendingPI, "error", err)
		return
	}
	if pi.Status != stripe.PaymentIntentStatusSucceeded {
		return
	}

	chargeID, cardBrand, cardLast4, receiptURL := extractChargeDetails(pi)
	if err := acct.ConfirmPaymentWithDetails(pi.ID, chargeID, "", pi.Amount, string(pi.Currency), cardBrand, cardLast4, receiptURL); err != nil {
		s.log.Error(ctx, "billing: lazy-verify confirm failed", "user_id", userID, "stripe_pi", pi.ID, "error", err)
		return
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		s.log.Error(ctx, "billing: lazy-verify save failed", "user_id", userID, "stripe_pi", pi.ID, "error", err)
		return
	}
	s.log.Info(ctx, "billing: lazy-verify confirmed payment", "user_id", userID, "stripe_pi", pi.ID)
	status.Status = "active"
}

func (s *service) createPaymentIntent(ctx context.Context, userID, email string) (clientSecret string, err error) {
	acct, err := s.loadOrOpenAccount(ctx, userID, email)
	if err != nil {
		return "", err
	}
	if acct.IsActivated() {
		return "", ErrAlreadyActivated
	}
	if reused, ok := s.reusePendingPaymentIntent(ctx, userID, acct.PendingStripePaymentID()); ok {
		return reused, nil
	}

	customerID, err := s.ensureStripeCustomer(ctx, userID, email, acct.StripeCustomerID)
	if err != nil {
		return "", err
	}
	paymentID := uuid.New().String()

	pi, err := paymentintent.New(newSubscriptionPIParams(ctx, customerID, email, paymentID, userID))
	if err != nil {
		s.logStripeError(ctx, "billing: stripe create payment intent", err, "user_id", userID)
		return "", fmt.Errorf("billing: stripe create payment intent: %w", err)
	}

	if err := acct.InitiatePayment(paymentID, pi.ID, customerID, AmountCents, DefaultCurrency, "Monthly subscription", paymentID); err != nil {
		if errors.Is(err, ErrAlreadyActivated) {
			return "", err
		}
		return "", fmt.Errorf("billing: initiate payment: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return "", fmt.Errorf("billing: save aggregate: %w", err)
	}

	s.log.Info(ctx, "billing: payment initiated",
		"user_id", userID, "payment_id", paymentID, "stripe_pi", pi.ID,
		"stripe_customer", customerID, "amount_cents", AmountCents, "currency", DefaultCurrency)
	return pi.ClientSecret, nil
}

// reusePendingPaymentIntent checks whether an earlier payment-intent is
// still in a reusable state on Stripe (RequiresPaymentMethod/Confirmation/
// Action). Returns the existing client_secret + ok when it is; ok=false
// means the caller should create a new PI.
func (s *service) reusePendingPaymentIntent(ctx context.Context, userID, pendingPI string) (string, bool) {
	if pendingPI == "" {
		return "", false
	}
	pi, err := paymentintent.Get(pendingPI, piGetParams(ctx))
	if err != nil {
		s.log.Info(ctx, "billing: stale pending PI, will create new", "user_id", userID, "stripe_pi", pendingPI, "error", err)
		return "", false
	}
	if !isReusablePIStatus(pi.Status) {
		return "", false
	}
	s.log.Info(ctx, "billing: reusing pending payment intent", "user_id", userID, "stripe_pi", pi.ID, "status", string(pi.Status))
	return pi.ClientSecret, true
}

// isReusablePIStatus returns true for the Stripe statuses where the
// existing PaymentIntent can still be confirmed by the frontend.
func isReusablePIStatus(status stripe.PaymentIntentStatus) bool {
	switch status {
	case stripe.PaymentIntentStatusRequiresPaymentMethod,
		stripe.PaymentIntentStatusRequiresConfirmation,
		stripe.PaymentIntentStatusRequiresAction:
		return true
	}
	return false
}

// newSubscriptionPIParams builds the Stripe PaymentIntent params for
// the monthly subscription charge, including idempotency + the metadata
// the webhook flow relies on to resolve the user.
func newSubscriptionPIParams(ctx context.Context, customerID, email, paymentID, userID string) *stripe.PaymentIntentParams {
	params := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(AmountCents),
		Currency: stripe.String(DefaultCurrency),
		Customer: stripe.String(customerID),
		AutomaticPaymentMethods: &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled: stripe.Bool(true),
		},
		Description:  stripe.String("AI Platform — Monthly subscription"),
		ReceiptEmail: stripe.String(email),
		Metadata: map[string]string{
			MetadataKeyUserID:    userID,
			MetadataKeyEmail:     email,
			MetadataKeyPaymentID: paymentID,
			MetadataKeyPurpose:   MetadataPurposeMonthlySubscription,
		},
	}
	params.SetIdempotencyKey(paymentID)
	params.Context = ctx
	return params
}

func (s *service) confirmPaymentWithDetails(
	ctx context.Context,
	stripePaymentID, stripeChargeID, stripeEventID string,
	amountCents int64, currency, cardBrand, cardLast4, receiptURL string,
	metadata map[string]string,
) error {
	userID := metadata[MetadataKeyUserID]
	if userID == "" {
		return fmt.Errorf("billing: webhook missing user_id in metadata")
	}
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for confirm: %w", err)
	}
	if err := acct.ConfirmPaymentWithDetails(stripePaymentID, stripeChargeID, stripeEventID, amountCents, currency, cardBrand, cardLast4, receiptURL); err != nil {
		return fmt.Errorf("billing: confirm payment: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after confirm: %w", err)
	}
	s.log.Info(ctx, "billing: payment confirmed",
		"user_id", userID, "stripe_pi", stripePaymentID, "stripe_charge", stripeChargeID,
		"stripe_event", stripeEventID, "card", cardBrand+" ••••"+cardLast4, "receipt_url", receiptURL)
	return nil
}

func (s *service) failPaymentWithDetails(
	ctx context.Context,
	stripePaymentID, stripeEventID string,
	pi *stripe.PaymentIntent,
	metadata map[string]string,
) error {
	userID := metadata[MetadataKeyUserID]
	if userID == "" {
		return fmt.Errorf("billing: webhook missing user_id in metadata")
	}
	reason, errorCode, declineCode, errorType, failureMessage := extractPaymentError(pi)
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for fail: %w", err)
	}
	if err := acct.FailPaymentWithDetails(stripePaymentID, stripeEventID, reason, errorCode, declineCode, errorType, failureMessage); err != nil {
		return fmt.Errorf("billing: fail payment: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after fail: %w", err)
	}
	s.log.Info(ctx, "billing: payment failed",
		"user_id", userID, "stripe_pi", stripePaymentID, "stripe_event", stripeEventID,
		"reason", reason, "error_code", errorCode, "decline_code", declineCode, "error_type", errorType)
	return nil
}

func (s *service) verifyPayment(ctx context.Context, userID, stripePaymentIntentID string) error {
	pi, err := paymentintent.Get(stripePaymentIntentID, piGetParams(ctx))
	if err != nil {
		return fmt.Errorf("billing: stripe get payment intent: %w", err)
	}
	if pi.Metadata[MetadataKeyUserID] != userID {
		return fmt.Errorf("billing: payment intent does not belong to user")
	}
	if pi.Status != stripe.PaymentIntentStatusSucceeded {
		s.log.Info(ctx, "billing: verify-payment PI not succeeded",
			"user_id", userID, "stripe_pi", stripePaymentIntentID, "status", string(pi.Status))
		return fmt.Errorf("billing: payment intent status is %s, not succeeded", pi.Status)
	}
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for verify: %w", err)
	}
	if acct.IsActivated() {
		s.log.Info(ctx, "billing: verify-payment already activated", "user_id", userID)
		return nil
	}
	chargeID, cardBrand, cardLast4, receiptURL := extractChargeDetails(pi)
	if err := acct.ConfirmPaymentWithDetails(pi.ID, chargeID, "", pi.Amount, string(pi.Currency), cardBrand, cardLast4, receiptURL); err != nil {
		return fmt.Errorf("billing: confirm payment during verify: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after verify: %w", err)
	}
	s.log.Info(ctx, "billing: payment verified and confirmed via redirect",
		"user_id", userID, "stripe_pi", pi.ID, "stripe_charge", chargeID)
	return nil
}

func (s *service) cancelSubscription(ctx context.Context, userID, reason string) (cancelsOn time.Time, err error) {
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return time.Time{}, fmt.Errorf("billing: load account for cancel: %w", err)
	}
	now := time.Now().UTC()
	cancelsOn = calculateBillingPeriodEnd(now, acct.Ledger)
	if err := acct.CancelSubscription(cancelsOn, reason); err != nil {
		return time.Time{}, fmt.Errorf("billing: cancel subscription: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return time.Time{}, fmt.Errorf("billing: save after cancel: %w", err)
	}
	s.log.Info(ctx, "billing: subscription cancelled", "user_id", userID, "cancels_on", cancelsOn.Format("2006-01-02"))
	return cancelsOn, nil
}

func (s *service) reactivateSubscription(ctx context.Context, userID string) error {
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

func (s *service) handleRefund(ctx context.Context, charge *stripe.Charge) error {
	if charge == nil {
		return fmt.Errorf("billing: charge.refunded received nil charge")
	}
	piID := chargePaymentIntentID(charge)
	if piID == "" {
		return fmt.Errorf("billing: charge.refunded missing payment intent for charge %s", charge.ID)
	}
	userID, err := s.resolveUserIDFromCharge(ctx, charge, piID)
	if err != nil {
		return err
	}
	if userID == "" {
		return fmt.Errorf("billing: charge.refunded missing user_id in metadata for charge %s (pi: %s)", charge.ID, piID)
	}
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for refund: %w", err)
	}

	refund := extractRefundDetails(charge)
	if refund.amount <= 0 {
		s.log.Info(ctx, "billing: charge.refunded with zero amount, skipping", "user_id", userID, "charge_id", charge.ID)
		return nil
	}
	if err := acct.RefundPaymentWithDetails(piID, refund.id, charge.ID, "", refund.amount, string(charge.Currency), refund.reason, refund.status); err != nil {
		return fmt.Errorf("billing: refund payment: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after refund: %w", err)
	}
	s.log.Info(ctx, "billing: refund processed from Stripe webhook",
		"user_id", userID, "charge_id", charge.ID, "refund_id", refund.id,
		"amount_cents", refund.amount, "refund_reason", refund.reason, "refund_status", refund.status)
	return nil
}

// chargePaymentIntentID extracts the PaymentIntent ID from a Stripe
// Charge, returning "" when the charge has no attached intent.
func chargePaymentIntentID(charge *stripe.Charge) string {
	if charge.PaymentIntent == nil {
		return ""
	}
	return charge.PaymentIntent.ID
}

// resolveUserIDFromCharge walks the three places the user_id can live
// on a Charge webhook: the charge metadata, the embedded PaymentIntent
// metadata, and (as a fallback) a fresh PaymentIntent.Get lookup. Only
// the Stripe fetch path returns an error; empty userID is a caller
// concern.
func (s *service) resolveUserIDFromCharge(ctx context.Context, charge *stripe.Charge, piID string) (string, error) {
	if id := charge.Metadata[MetadataKeyUserID]; id != "" {
		return id, nil
	}
	if charge.PaymentIntent != nil && charge.PaymentIntent.Metadata != nil {
		if id := charge.PaymentIntent.Metadata[MetadataKeyUserID]; id != "" {
			return id, nil
		}
	}
	pi, err := paymentintent.Get(piID, piGetParams(ctx))
	if err != nil {
		return "", fmt.Errorf("billing: charge.refunded failed to fetch PI %s: %w", piID, err)
	}
	return pi.Metadata[MetadataKeyUserID], nil
}

// refundDetails captures the four fields handleRefund needs off either
// the first Refund in the charge's refund list or the charge-level
// dashboard-refund fallback.
type refundDetails struct {
	amount int64
	id     string
	reason string
	status string
}

// extractRefundDetails reads the latest refund off charge.Refunds.Data
// if present, falling back to charge.AmountRefunded + a synthetic ID for
// dashboard-driven refunds that don't attach a Refund object.
func extractRefundDetails(charge *stripe.Charge) refundDetails {
	if charge.Refunds != nil && len(charge.Refunds.Data) > 0 {
		latest := charge.Refunds.Data[0]
		return refundDetails{
			amount: latest.Amount,
			id:     latest.ID,
			reason: string(latest.Reason),
			status: string(latest.Status),
		}
	}
	return refundDetails{
		amount: charge.AmountRefunded,
		id:     fmt.Sprintf("re_auto_%s", charge.ID),
		reason: "stripe_dashboard_refund",
	}
}

func (s *service) handleDispute(ctx context.Context, dispute *stripe.Dispute, stripeEventID string) error {
	if dispute == nil {
		return fmt.Errorf("billing: dispute.created received nil dispute")
	}
	chargeID := disputeChargeID(dispute)
	if chargeID == "" {
		return fmt.Errorf("billing: dispute missing charge ID for dispute %s", dispute.ID)
	}
	userID, err := s.resolveDisputeUserID(ctx, dispute, "billing: dispute failed to fetch PI")
	if err != nil {
		return err
	}
	if userID == "" {
		return fmt.Errorf("billing: dispute missing user_id for dispute %s (charge: %s)", dispute.ID, chargeID)
	}
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for dispute: %w", err)
	}
	if err := acct.RecordDispute(dispute.ID, chargeID, stripeEventID, dispute.Amount, string(dispute.Currency), string(dispute.Reason), string(dispute.Status)); err != nil {
		return fmt.Errorf("billing: record dispute: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after dispute: %w", err)
	}
	s.log.Error(ctx, "billing: DISPUTE OPENED — requires attention",
		"user_id", userID, "dispute_id", dispute.ID, "charge_id", chargeID,
		"amount_cents", dispute.Amount, "reason", string(dispute.Reason),
		"status", string(dispute.Status), "stripe_event", stripeEventID)
	return nil
}

func (s *service) handleDisputeClosed(ctx context.Context, dispute *stripe.Dispute, stripeEventID string) error {
	if dispute == nil {
		return fmt.Errorf("billing: dispute.closed received nil dispute")
	}
	userID, err := s.resolveDisputeUserID(ctx, dispute, "billing: dispute.closed failed to fetch PI")
	if err != nil {
		return err
	}
	if userID == "" {
		return fmt.Errorf("billing: dispute.closed missing user_id for dispute %s", dispute.ID)
	}
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for dispute resolution: %w", err)
	}
	if err := acct.ResolveDispute(dispute.ID, stripeEventID, string(dispute.Status), dispute.Amount); err != nil {
		return fmt.Errorf("billing: resolve dispute: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after dispute resolution: %w", err)
	}
	s.log.Info(ctx, "billing: dispute resolved",
		"user_id", userID, "dispute_id", dispute.ID, "status", string(dispute.Status),
		"amount_cents", dispute.Amount, "stripe_event", stripeEventID)
	return nil
}

// disputeChargeID extracts the Charge ID from a Dispute webhook,
// returning "" when Stripe sent the dispute without the charge pointer.
func disputeChargeID(dispute *stripe.Dispute) string {
	if dispute.Charge == nil {
		return ""
	}
	return dispute.Charge.ID
}

// resolveDisputeUserID returns the user_id for a dispute, preferring the
// metadata already on the dispute and falling back to a PaymentIntent
// lookup. fetchErrPrefix is used to prefix any Stripe fetch error so the
// two dispute handlers surface distinct log strings.
func (s *service) resolveDisputeUserID(ctx context.Context, dispute *stripe.Dispute, fetchErrPrefix string) (string, error) {
	if userID := resolveUserIDFromDispute(dispute); userID != "" {
		return userID, nil
	}
	if dispute.PaymentIntent == nil || dispute.PaymentIntent.ID == "" {
		return "", nil
	}
	pi, err := paymentintent.Get(dispute.PaymentIntent.ID, piGetParams(ctx))
	if err != nil {
		return "", fmt.Errorf("%s: %w", fetchErrPrefix, err)
	}
	return pi.Metadata[MetadataKeyUserID], nil
}

func (s *service) handleInvoicePaymentFailed(ctx context.Context, invoice *stripe.Invoice, stripeEventID string) error {
	if invoice == nil {
		return fmt.Errorf("billing: invoice.payment_failed received nil invoice")
	}
	userID := resolveUserIDFromInvoice(invoice)
	if userID == "" {
		s.log.Error(ctx, "billing: invoice.payment_failed missing user_id", "invoice_id", invoice.ID, "stripe_event", stripeEventID)
		return fmt.Errorf("billing: invoice.payment_failed missing user_id for invoice %s", invoice.ID)
	}
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for invoice failure: %w", err)
	}
	nextAttempt := optionalNextAttempt(invoice.NextPaymentAttempt)
	if err := acct.RecordInvoicePaymentFailure(invoice.ID, stripeEventID, invoice.AmountDue, string(invoice.Currency), invoice.AttemptCount, nextAttempt, ""); err != nil {
		return fmt.Errorf("billing: record invoice failure: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after invoice failure: %w", err)
	}
	s.log.Error(ctx, "billing: recurring invoice payment failed",
		"user_id", userID, "invoice_id", invoice.ID, "amount_due_cents", invoice.AmountDue,
		"attempt_count", invoice.AttemptCount, "consecutive_failures", acct.ConsecutiveInvoiceFailures,
		"stripe_event", stripeEventID)
	return nil
}

// resolveUserIDFromInvoice walks both metadata locations on an invoice:
// the customer metadata first, then the parent subscription's metadata
// (populated by Stripe Billing subscriptions).
func resolveUserIDFromInvoice(invoice *stripe.Invoice) string {
	if invoice.Customer != nil && invoice.Customer.Metadata != nil {
		if id := invoice.Customer.Metadata[MetadataKeyUserID]; id != "" {
			return id
		}
	}
	sub := invoiceParentSubscription(invoice)
	if sub != nil && sub.Metadata != nil {
		return sub.Metadata[MetadataKeyUserID]
	}
	return ""
}

// invoiceParentSubscription peels the nullable pointer chain off
// invoice.Parent.SubscriptionDetails.Subscription so callers don't have
// to re-check four levels of nil.
func invoiceParentSubscription(invoice *stripe.Invoice) *stripe.Subscription {
	if invoice.Parent == nil {
		return nil
	}
	if invoice.Parent.SubscriptionDetails == nil {
		return nil
	}
	return invoice.Parent.SubscriptionDetails.Subscription
}

// optionalNextAttempt converts Stripe's 0-means-absent Unix timestamp
// into a nullable pointer the domain layer can persist as NULL.
func optionalNextAttempt(nextAttempt int64) *int64 {
	if nextAttempt <= 0 {
		return nil
	}
	return &nextAttempt
}

func calculateBillingPeriodEnd(now time.Time, ledger []LedgerEntry) time.Time {
	for i := len(ledger) - 1; i >= 0; i-- {
		if ledger[i].Status == LedgerEntryStatusSucceeded && ledger[i].ResolvedAt != nil {
			return ledger[i].ResolvedAt.AddDate(0, 1, 0)
		}
	}
	return now.AddDate(0, 0, 30)
}

func (s *service) loadOrOpenAccount(ctx context.Context, userID, email string) (*PaymentAccountAggregate, error) {
	acct, err := s.repo.Load(ctx, userID)
	if err == nil {
		return acct, nil
	}
	if errors.Is(err, ErrAccountNotFound) {
		newAcct, openErr := OpenPaymentAccount(userID, email)
		if openErr != nil {
			return nil, fmt.Errorf("billing: open account: %w", openErr)
		}
		if saveErr := s.repo.SaveNew(ctx, newAcct); saveErr != nil {
			return nil, fmt.Errorf("billing: save new account: %w", saveErr)
		}
		return s.repo.Load(ctx, userID)
	}
	return nil, fmt.Errorf("billing: load account: %w", err)
}

func (s *service) ensureStripeCustomer(ctx context.Context, userID, email, existingCustomerID string) (string, error) {
	if existingCustomerID != "" {
		return existingCustomerID, nil
	}
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			MetadataKeyUserID: userID,
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

func extractChargeDetails(pi *stripe.PaymentIntent) (chargeID, brand, last4, receiptURL string) {
	if pi.LatestCharge != nil {
		chargeID = pi.LatestCharge.ID
		receiptURL = pi.LatestCharge.ReceiptURL
		if pi.LatestCharge.PaymentMethodDetails != nil && pi.LatestCharge.PaymentMethodDetails.Card != nil {
			card := pi.LatestCharge.PaymentMethodDetails.Card
			brand = string(card.Brand)
			last4 = card.Last4
		}
	}
	return chargeID, brand, last4, receiptURL
}

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

func resolveUserIDFromDispute(dispute *stripe.Dispute) string {
	if dispute.Charge != nil && dispute.Charge.Metadata != nil {
		if uid := dispute.Charge.Metadata[MetadataKeyUserID]; uid != "" {
			return uid
		}
	}
	if dispute.PaymentIntent != nil && dispute.PaymentIntent.Metadata != nil {
		if uid := dispute.PaymentIntent.Metadata[MetadataKeyUserID]; uid != "" {
			return uid
		}
	}
	return ""
}

func (s *service) logStripeError(ctx context.Context, msg string, err error, extra ...any) {
	fields := []any{"error", err}
	var stripeErr *stripe.Error
	if errors.As(err, &stripeErr) {
		fields = append(fields,
			"stripe_type", string(stripeErr.Type),
			"stripe_code", string(stripeErr.Code),
			"stripe_msg", stripeErr.Msg,
			"stripe_request_id", stripeErr.RequestID,
			"stripe_status", stripeErr.HTTPStatusCode)
		if stripeErr.DeclineCode != "" {
			fields = append(fields, "stripe_decline_code", string(stripeErr.DeclineCode))
		}
	}
	fields = append(fields, extra...)
	s.log.Error(ctx, msg, fields...)
}

func piGetParams(ctx context.Context) *stripe.PaymentIntentParams {
	p := &stripe.PaymentIntentParams{}
	p.AddExpand("latest_charge.payment_method_details")
	p.Context = ctx
	return p
}
