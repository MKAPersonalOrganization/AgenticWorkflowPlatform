package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"aicompass/foundation/logger"

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

	// Lazy expiration
	if status.Status == "cancelled" && status.CancelsOn != nil && !time.Now().Before(*status.CancelsOn) {
		acct, loadErr := s.repo.Load(ctx, userID)
		if loadErr != nil {
			s.log.Info(ctx, "billing: lazy-expire skipped, aggregate load failed", "user_id", userID, "error", loadErr)
			return status, nil
		}
		if err := acct.ExpireSubscription(); err != nil {
			s.log.Error(ctx, "billing: lazy-expire command failed", "user_id", userID, "error", err)
			return status, nil
		}
		if err := s.repo.Save(ctx, acct); err != nil {
			s.log.Error(ctx, "billing: lazy-expire save failed", "user_id", userID, "error", err)
			return status, nil
		}
		s.log.Info(ctx, "billing: lazy-expire completed", "user_id", userID, "cancels_on", status.CancelsOn)
		status.Status = "none"
		status.PaidAt = nil
		status.CancelledAt = nil
		status.CancelsOn = nil
		return status, nil
	}

	if status.Status == "cancelled" {
		return status, nil
	}

	// Lazy verification
	acct, loadErr := s.repo.Load(ctx, userID)
	if loadErr != nil {
		s.log.Info(ctx, "billing: lazy-verify skipped, aggregate load failed", "user_id", userID, "error", loadErr)
		return status, nil
	}
	if acct.IsActivated() {
		status.Status = "active"
		return status, nil
	}

	pendingPI := acct.PendingStripePaymentID()
	if pendingPI == "" {
		return status, nil
	}

	pi, getErr := paymentintent.Get(pendingPI, piGetParams(ctx))
	if getErr != nil {
		s.log.Info(ctx, "billing: lazy-verify stripe get failed", "user_id", userID, "stripe_pi", pendingPI, "error", getErr)
		return status, nil
	}
	if pi.Status != stripe.PaymentIntentStatusSucceeded {
		return status, nil
	}

	chargeID, cardBrand, cardLast4, receiptURL := extractChargeDetails(pi)
	if err := acct.ConfirmPaymentWithDetails(pi.ID, chargeID, "", pi.Amount, string(pi.Currency), cardBrand, cardLast4, receiptURL); err != nil {
		s.log.Error(ctx, "billing: lazy-verify confirm failed", "user_id", userID, "stripe_pi", pi.ID, "error", err)
		return status, nil
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		s.log.Error(ctx, "billing: lazy-verify save failed", "user_id", userID, "stripe_pi", pi.ID, "error", err)
		return status, nil
	}
	s.log.Info(ctx, "billing: lazy-verify confirmed payment", "user_id", userID, "stripe_pi", pi.ID)
	status.Status = "active"
	return status, nil
}

func (s *service) createPaymentIntent(ctx context.Context, userID, email string) (clientSecret string, err error) {
	acct, err := s.loadOrOpenAccount(ctx, userID, email)
	if err != nil {
		return "", err
	}
	if acct.IsActivated() {
		return "", ErrAlreadyActivated
	}

	if pendingPI := acct.PendingStripePaymentID(); pendingPI != "" {
		pi, getErr := paymentintent.Get(pendingPI, piGetParams(ctx))
		if getErr != nil {
			s.log.Info(ctx, "billing: stale pending PI, will create new", "user_id", userID, "stripe_pi", pendingPI, "error", getErr)
		} else if pi.Status == stripe.PaymentIntentStatusRequiresPaymentMethod ||
			pi.Status == stripe.PaymentIntentStatusRequiresConfirmation ||
			pi.Status == stripe.PaymentIntentStatusRequiresAction {
			s.log.Info(ctx, "billing: reusing pending payment intent", "user_id", userID, "stripe_pi", pi.ID, "status", string(pi.Status))
			return pi.ClientSecret, nil
		}
	}

	customerID, err := s.ensureStripeCustomer(ctx, userID, email, acct.StripeCustomerID)
	if err != nil {
		return "", err
	}

	paymentID := uuid.New().String()

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

	pi, err := paymentintent.New(params)
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

func (s *service) confirmPayment(ctx context.Context, stripePaymentID string, amountCents int64, currency, cardBrand, cardLast4 string, metadata map[string]string) error {
	userID := metadata[MetadataKeyUserID]
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

func (s *service) failPayment(ctx context.Context, stripePaymentID, reason string, metadata map[string]string) error {
	userID := metadata[MetadataKeyUserID]
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
	var piID string
	if charge.PaymentIntent != nil {
		piID = charge.PaymentIntent.ID
	}
	if piID == "" {
		return fmt.Errorf("billing: charge.refunded missing payment intent for charge %s", charge.ID)
	}
	userID := charge.Metadata[MetadataKeyUserID]
	if userID == "" && charge.PaymentIntent != nil && charge.PaymentIntent.Metadata != nil {
		userID = charge.PaymentIntent.Metadata[MetadataKeyUserID]
	}
	if userID == "" {
		pi, err := paymentintent.Get(piID, piGetParams(ctx))
		if err != nil {
			return fmt.Errorf("billing: charge.refunded failed to fetch PI %s: %w", piID, err)
		}
		userID = pi.Metadata[MetadataKeyUserID]
	}
	if userID == "" {
		return fmt.Errorf("billing: charge.refunded missing user_id in metadata for charge %s (pi: %s)", charge.ID, piID)
	}
	acct, err := s.repo.Load(ctx, userID)
	if err != nil {
		return fmt.Errorf("billing: load account for refund: %w", err)
	}

	var refundAmount int64
	var refundID, refundReason, refundStatus string
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
		s.log.Info(ctx, "billing: charge.refunded with zero amount, skipping", "user_id", userID, "charge_id", charge.ID)
		return nil
	}
	if err := acct.RefundPaymentWithDetails(piID, refundID, charge.ID, "", refundAmount, string(charge.Currency), refundReason, refundStatus); err != nil {
		return fmt.Errorf("billing: refund payment: %w", err)
	}
	if err := s.repo.Save(ctx, acct); err != nil {
		return fmt.Errorf("billing: save after refund: %w", err)
	}
	s.log.Info(ctx, "billing: refund processed from Stripe webhook",
		"user_id", userID, "charge_id", charge.ID, "refund_id", refundID,
		"amount_cents", refundAmount, "refund_reason", refundReason, "refund_status", refundStatus)
	return nil
}

func (s *service) handleDispute(ctx context.Context, dispute *stripe.Dispute, stripeEventID string) error {
	if dispute == nil {
		return fmt.Errorf("billing: dispute.created received nil dispute")
	}
	var chargeID string
	if dispute.Charge != nil {
		chargeID = dispute.Charge.ID
	}
	if chargeID == "" {
		return fmt.Errorf("billing: dispute missing charge ID for dispute %s", dispute.ID)
	}
	userID := resolveUserIDFromDispute(dispute)
	if userID == "" {
		if dispute.PaymentIntent != nil && dispute.PaymentIntent.ID != "" {
			pi, err := paymentintent.Get(dispute.PaymentIntent.ID, piGetParams(ctx))
			if err != nil {
				return fmt.Errorf("billing: dispute failed to fetch PI: %w", err)
			}
			userID = pi.Metadata[MetadataKeyUserID]
		}
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
	userID := resolveUserIDFromDispute(dispute)
	if userID == "" && dispute.PaymentIntent != nil && dispute.PaymentIntent.ID != "" {
		pi, err := paymentintent.Get(dispute.PaymentIntent.ID, piGetParams(ctx))
		if err != nil {
			return fmt.Errorf("billing: dispute.closed failed to fetch PI: %w", err)
		}
		userID = pi.Metadata[MetadataKeyUserID]
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

func (s *service) handleInvoicePaymentFailed(ctx context.Context, invoice *stripe.Invoice, stripeEventID string) error {
	if invoice == nil {
		return fmt.Errorf("billing: invoice.payment_failed received nil invoice")
	}
	var userID string
	if invoice.Customer != nil && invoice.Customer.Metadata != nil {
		userID = invoice.Customer.Metadata[MetadataKeyUserID]
	}
	if userID == "" && invoice.Parent != nil &&
		invoice.Parent.SubscriptionDetails != nil &&
		invoice.Parent.SubscriptionDetails.Subscription != nil &&
		invoice.Parent.SubscriptionDetails.Subscription.Metadata != nil {
		userID = invoice.Parent.SubscriptionDetails.Subscription.Metadata[MetadataKeyUserID]
	}
	if userID == "" {
		s.log.Error(ctx, "billing: invoice.payment_failed missing user_id", "invoice_id", invoice.ID, "stripe_event", stripeEventID)
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

func classifyStripeError(err error) (retryable bool, userActionRequired bool) {
	var stripeErr *stripe.Error
	if !errors.As(err, &stripeErr) {
		return false, false
	}
	switch stripeErr.Type {
	case stripe.ErrorTypeCard:
		return false, stripeErr.Code == stripe.ErrorCodeAuthenticationRequired
	case stripe.ErrorTypeAPI:
		return true, false
	case stripe.ErrorTypeIdempotency:
		return false, false
	case stripe.ErrorTypeInvalidRequest:
		return false, false
	default:
		return false, false
	}
}

func piGetParams(ctx context.Context) *stripe.PaymentIntentParams {
	p := &stripe.PaymentIntentParams{}
	p.AddExpand("latest_charge.payment_method_details")
	p.Context = ctx
	return p
}
