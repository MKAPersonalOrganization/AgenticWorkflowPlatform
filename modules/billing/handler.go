package billing

// This file was extracted from handlers/billing/handler.go as part of the
// modular-monolith migration. All types and methods are now unexported
// (module-internal); the Module facade in billing.go exposes route registration.

import (
	"aicompass/foundation/logger"
	"aicompass/shared"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/webhook"
)

// handler holds HTTP handlers for billing endpoints.
type handler struct {
	log            *logger.Logger
	billing        *service
	webhookSecret  string
	publishableKey string
}

// newHandler creates a billing handler.
func newHandler(log *logger.Logger, svc *service, webhookSecret, publishableKey string) *handler {
	return &handler{
		log:            log,
		billing:        svc,
		webhookSecret:  webhookSecret,
		publishableKey: publishableKey,
	}
}

// --------------------------------------------------------------------------
// Shared helpers
// --------------------------------------------------------------------------

// billingError is a structured error response for all billing endpoints.
type billingError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"` // Machine-readable error code
}

// writeError writes a structured JSON error response.
func writeError(w http.ResponseWriter, status int, msg, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(billingError{Error: msg, Code: code})
}

// requireJSON validates that the request Content-Type is application/json
// for POST endpoints. Returns false if validation fails (response already sent).
func requireJSON(w http.ResponseWriter, r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json", "invalid_content_type")
		return false
	}
	return true
}

// --------------------------------------------------------------------------
// POST /api/billing/create-payment-intent
// --------------------------------------------------------------------------

type createPaymentIntentResponse struct {
	ClientSecret   string `json:"clientSecret"`
	PublishableKey string `json:"publishableKey"`
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
}

// createPaymentIntent creates a Stripe PaymentIntent and returns the client
// secret so the frontend can confirm payment via Stripe Elements.
func (h *handler) createPaymentIntent(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	// Limit request body to prevent abuse
	r.Body = http.MaxBytesReader(w, r.Body, 1024)

	clientSecret, err := h.billing.createPaymentIntent(r.Context(), user.ID, user.Email)
	if err != nil {
		h.log.Error(r.Context(), "billing: create payment intent failed", "error", err, "user_id", user.ID)

		// Map domain and Stripe errors to appropriate HTTP responses.
		// Use errors.As to extract the SDK's *stripe.Error for typed handling.
		switch {
		case errors.Is(err, ErrAlreadyActivated):
			writeError(w, http.StatusConflict, "Account is already activated.", "already_activated")
		default:
			var stripeErr *stripe.Error
			if errors.As(err, &stripeErr) {
				h.log.Error(r.Context(), "billing: stripe API error",
					"stripe_type", string(stripeErr.Type),
					"stripe_code", string(stripeErr.Code),
					"stripe_msg", stripeErr.Msg,
					"stripe_request_id", stripeErr.RequestID,
					"user_id", user.ID)
			}
			writeError(w, http.StatusInternalServerError, "Failed to create payment. Please try again.", "payment_init_failed")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(createPaymentIntentResponse{
		ClientSecret:   clientSecret,
		PublishableKey: h.publishableKey,
		Amount:         AmountCents,
		Currency:       DefaultCurrency,
	})
}

// --------------------------------------------------------------------------
// POST /api/billing/webhook  (public — Stripe calls this)
// --------------------------------------------------------------------------

// handleWebhook processes Stripe webhook events with cryptographic signature
// verification. Dispatches:
//
//   - payment_intent.succeeded  → confirmPaymentWithDetails
//   - payment_intent.payment_failed → failPaymentWithDetails
//   - charge.refunded           → handleRefund
//   - charge.dispute.created    → handleDispute
//   - charge.dispute.closed     → handleDisputeClosed
//   - invoice.payment_failed    → handleInvoicePaymentFailed
//
// All others are acknowledged with 200 OK.
//
// Idempotency is guaranteed at two levels:
//  1. Aggregate-level: duplicate commands are no-ops
//  2. Stripe event dedup: ProcessedStripeEvents map in the aggregate
func (h *handler) handleWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBodyBytes = 65536
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.log.Error(r.Context(), "billing: webhook read body failed", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Verify Stripe webhook signature to prevent forgery
	sigHeader := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(payload, sigHeader, h.webhookSecret)
	if err != nil {
		h.log.Error(r.Context(), "billing: webhook signature verification failed", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.log.Info(r.Context(), "billing: webhook event received",
		"event_id", event.ID, "type", string(event.Type))

	switch event.Type {
	case stripe.EventTypePaymentIntentSucceeded:
		h.handlePaymentSucceeded(w, r, event)
		return

	case stripe.EventTypePaymentIntentPaymentFailed:
		h.handlePaymentFailed(w, r, event)
		return

	case stripe.EventTypeChargeRefunded:
		h.handleChargeRefunded(w, r, event)
		return

	case stripe.EventTypeChargeDisputeCreated:
		h.handleDisputeCreated(w, r, event)
		return

	case stripe.EventTypeChargeDisputeClosed:
		h.handleDisputeClosed(w, r, event)
		return

	case stripe.EventTypeInvoicePaymentFailed:
		h.handleInvoicePaymentFailed(w, r, event)
		return

	default:
		h.log.Info(r.Context(), "billing: unhandled webhook event",
			"type", string(event.Type), "event_id", event.ID)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"received":true}`)
}

// handlePaymentSucceeded processes a payment_intent.succeeded webhook event.
// Extracts full charge details (chargeID, card brand, last4, receipt URL)
// and passes them to confirmPaymentWithDetails for complete traceability.
func (h *handler) handlePaymentSucceeded(w http.ResponseWriter, r *http.Request, event stripe.Event) {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		h.log.Error(r.Context(), "billing: webhook unmarshal succeeded", "error", err, "event_id", event.ID)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"received":true,"error":"unmarshal_failed"}`)
		return
	}

	// Extract PCI-safe charge details from the expanded LatestCharge
	var chargeID, cardBrand, cardLast4, receiptURL string
	if pi.LatestCharge != nil {
		chargeID = pi.LatestCharge.ID
		receiptURL = pi.LatestCharge.ReceiptURL
		if pi.LatestCharge.PaymentMethodDetails != nil &&
			pi.LatestCharge.PaymentMethodDetails.Card != nil {
			card := pi.LatestCharge.PaymentMethodDetails.Card
			cardBrand = string(card.Brand)
			cardLast4 = card.Last4
		}
	}

	if err := h.billing.confirmPaymentWithDetails(
		r.Context(),
		pi.ID, chargeID, event.ID,
		pi.Amount, string(pi.Currency), cardBrand, cardLast4, receiptURL,
		pi.Metadata,
	); err != nil {
		h.log.Error(r.Context(), "billing: confirm payment failed",
			"error", err, "stripe_pi", pi.ID, "stripe_charge", chargeID,
			"event_id", event.ID, "metadata", pi.Metadata)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"received":true}`)
}

// handlePaymentFailed processes a payment_intent.payment_failed webhook event.
// Extracts the full Stripe error taxonomy (ErrorType, ErrorCode, DeclineCode,
// FailureMessage) and passes them to failPaymentWithDetails.
func (h *handler) handlePaymentFailed(w http.ResponseWriter, r *http.Request, event stripe.Event) {
	var pi stripe.PaymentIntent
	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		h.log.Error(r.Context(), "billing: webhook unmarshal failed", "error", err, "event_id", event.ID)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"received":true,"error":"unmarshal_failed"}`)
		return
	}

	if err := h.billing.failPaymentWithDetails(r.Context(), pi.ID, event.ID, &pi, pi.Metadata); err != nil {
		h.log.Error(r.Context(), "billing: mark payment failed",
			"error", err, "stripe_pi", pi.ID, "event_id", event.ID, "metadata", pi.Metadata)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"received":true}`)
}

// handleChargeRefunded processes a charge.refunded webhook event.
// This handles refunds initiated from the Stripe Dashboard or API.
func (h *handler) handleChargeRefunded(w http.ResponseWriter, r *http.Request, event stripe.Event) {
	var charge stripe.Charge
	if err := json.Unmarshal(event.Data.Raw, &charge); err != nil {
		h.log.Error(r.Context(), "billing: webhook unmarshal charge.refunded", "error", err, "event_id", event.ID)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"received":true,"error":"unmarshal_failed"}`)
		return
	}

	if err := h.billing.handleRefund(r.Context(), &charge); err != nil {
		h.log.Error(r.Context(), "billing: handle refund failed",
			"error", err, "charge_id", charge.ID, "event_id", event.ID)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"received":true}`)
}

// handleDisputeCreated processes a charge.dispute.created webhook event.
// Disputes (chargebacks) are business-critical: they freeze funds, carry
// financial penalties, and may require evidence submission within 7 days.
func (h *handler) handleDisputeCreated(w http.ResponseWriter, r *http.Request, event stripe.Event) {
	var dispute stripe.Dispute
	if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
		h.log.Error(r.Context(), "billing: webhook unmarshal charge.dispute.created",
			"error", err, "event_id", event.ID)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"received":true,"error":"unmarshal_failed"}`)
		return
	}

	if err := h.billing.handleDispute(r.Context(), &dispute, event.ID); err != nil {
		h.log.Error(r.Context(), "billing: handle dispute failed",
			"error", err, "dispute_id", dispute.ID, "event_id", event.ID)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"received":true}`)
}

// handleDisputeClosed processes a charge.dispute.closed webhook event.
// Determines if the dispute was won (reversed) or lost (funds withdrawn).
func (h *handler) handleDisputeClosed(w http.ResponseWriter, r *http.Request, event stripe.Event) {
	var dispute stripe.Dispute
	if err := json.Unmarshal(event.Data.Raw, &dispute); err != nil {
		h.log.Error(r.Context(), "billing: webhook unmarshal charge.dispute.closed",
			"error", err, "event_id", event.ID)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"received":true,"error":"unmarshal_failed"}`)
		return
	}

	if err := h.billing.handleDisputeClosed(r.Context(), &dispute, event.ID); err != nil {
		h.log.Error(r.Context(), "billing: handle dispute closed failed",
			"error", err, "dispute_id", dispute.ID, "event_id", event.ID)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"received":true}`)
}

// handleInvoicePaymentFailed processes an invoice.payment_failed webhook event.
// Tracks recurring payment failures for subscription health monitoring and
// dunning management. After MaxConsecutiveInvoiceFailures, the account is
// flagged as overdue.
func (h *handler) handleInvoicePaymentFailed(w http.ResponseWriter, r *http.Request, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		h.log.Error(r.Context(), "billing: webhook unmarshal invoice.payment_failed",
			"error", err, "event_id", event.ID)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"received":true,"error":"unmarshal_failed"}`)
		return
	}

	if err := h.billing.handleInvoicePaymentFailed(r.Context(), &invoice, event.ID); err != nil {
		h.log.Error(r.Context(), "billing: handle invoice payment failed",
			"error", err, "invoice_id", invoice.ID, "event_id", event.ID)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"received":true}`)
}

// --------------------------------------------------------------------------
// GET /api/billing/status
// --------------------------------------------------------------------------

type paymentStatusResponse struct {
	Status        string `json:"status"`                  // "none", "active", "pending", "cancelled"
	PlanName      string `json:"planName,omitempty"`      // e.g. "Full Access"
	PlanAmount    string `json:"planAmount,omitempty"`    // e.g. "$5.00"
	PlanInterval  string `json:"planInterval,omitempty"`  // e.g. "month"
	PaidAt        string `json:"paidAt,omitempty"`        // ISO 8601
	NextBillingAt string `json:"nextBillingAt,omitempty"` // ISO 8601 — next charge date
	CardBrand     string `json:"cardBrand,omitempty"`     // e.g. "visa", "mastercard"
	CardLast4     string `json:"cardLast4,omitempty"`     // e.g. "4242"
	CancelledAt   string `json:"cancelledAt,omitempty"`   // ISO 8601
	CancelsOn     string `json:"cancelsOn,omitempty"`     // ISO 8601 — access-until date
	DaysRemaining int    `json:"daysRemaining,omitempty"` // days until cancellation takes effect
}

// getPaymentStatus returns the current payment status for the authenticated user.
func (h *handler) getPaymentStatus(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	status, err := h.billing.getAccountStatus(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "billing: get status failed", "error", err, "user_id", user.ID)
		writeError(w, http.StatusInternalServerError, "Failed to get payment status.", "status_error")
		return
	}

	resp := paymentStatusResponse{Status: "none"}
	if status != nil {
		resp.Status = status.Status

		// Plan metadata — shown for any non-"none" status
		if status.Status != "none" {
			resp.PlanName = "Full Access"
			resp.PlanAmount = "$5.00"
			resp.PlanInterval = "month"
		}

		if status.PaidAt != nil {
			resp.PaidAt = status.PaidAt.Format("2006-01-02T15:04:05Z")

			// Next billing date = paid_at + 1 month (for active subscriptions)
			if status.Status == "active" {
				nextBilling := status.PaidAt.AddDate(0, 1, 0)
				resp.NextBillingAt = nextBilling.Format("2006-01-02T15:04:05Z")
			}
		}
		if status.CardBrand != nil {
			resp.CardBrand = *status.CardBrand
		}
		if status.CardLast4 != nil {
			resp.CardLast4 = *status.CardLast4
		}
		if status.CancelledAt != nil {
			resp.CancelledAt = status.CancelledAt.Format("2006-01-02T15:04:05Z")
		}
		if status.CancelsOn != nil {
			resp.CancelsOn = status.CancelsOn.Format("2006-01-02T15:04:05Z")

			// Calculate days remaining until access expires
			daysLeft := int(status.CancelsOn.Sub(time.Now().UTC()).Hours() / 24)
			if daysLeft < 0 {
				daysLeft = 0
			}
			resp.DaysRemaining = daysLeft
		}
	}

	// Short cache to prevent excessive polling
	w.Header().Set("Cache-Control", "private, max-age=5")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// --------------------------------------------------------------------------
// POST /api/billing/verify-payment
// --------------------------------------------------------------------------

type verifyPaymentRequest struct {
	PaymentIntent string `json:"paymentIntent"`
}

// verifyPayment is the redirect-return verification handler. After Stripe
// redirects the user back with redirect_status=succeeded, the frontend
// calls this endpoint with the payment_intent ID. We retrieve the PI from
// Stripe, verify it succeeded and belongs to this user, then confirm the
// aggregate. This is the fallback for when webhooks are unavailable.
func (h *handler) verifyPayment(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	if !requireJSON(w, r) {
		return
	}

	// Limit request body
	r.Body = http.MaxBytesReader(w, r.Body, 1024)

	var req verifyPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PaymentIntent == "" {
		writeError(w, http.StatusBadRequest, "paymentIntent is required", "missing_param")
		return
	}

	// Validate PaymentIntent ID format (must start with "pi_")
	if !strings.HasPrefix(req.PaymentIntent, "pi_") {
		writeError(w, http.StatusBadRequest, "Invalid payment intent ID format.", "invalid_pi_format")
		return
	}

	if err := h.billing.verifyPayment(r.Context(), user.ID, req.PaymentIntent); err != nil {
		h.log.Error(r.Context(), "billing: verify payment failed",
			"error", err, "user_id", user.ID, "stripe_pi", req.PaymentIntent)
		writeError(w, http.StatusUnprocessableEntity, "Payment could not be verified. Please try again or contact support.", "verification_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "confirmed"})
}

// --------------------------------------------------------------------------
// POST /api/billing/cancel
// --------------------------------------------------------------------------

type cancelSubscriptionRequest struct {
	Reason string `json:"reason,omitempty"`
}

type cancelSubscriptionResponse struct {
	Status    string `json:"status"`
	CancelsOn string `json:"cancelsOn"`
}

// cancelSubscription handles user-initiated subscription cancellation.
// Access continues until the end of the current billing period.
func (h *handler) cancelSubscription(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	// Limit request body
	r.Body = http.MaxBytesReader(w, r.Body, 2048)

	var req cancelSubscriptionRequest
	// Body is optional — reason is not required
	json.NewDecoder(r.Body).Decode(&req)

	// Sanitize reason length to prevent abuse
	if len(req.Reason) > 500 {
		req.Reason = req.Reason[:500]
	}

	cancelsOn, err := h.billing.cancelSubscription(r.Context(), user.ID, req.Reason)
	if err != nil {
		h.log.Error(r.Context(), "billing: cancel subscription failed",
			"error", err, "user_id", user.ID)
		writeError(w, http.StatusUnprocessableEntity, "Could not cancel subscription. Please try again or contact support.", "cancel_failed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cancelSubscriptionResponse{
		Status:    "cancelled",
		CancelsOn: cancelsOn.Format("2006-01-02T15:04:05Z"),
	})
}

// --------------------------------------------------------------------------
// POST /api/billing/reactivate
// --------------------------------------------------------------------------

// reactivateSubscription handles undoing a cancellation before the billing
// period ends. The subscription returns to active.
func (h *handler) reactivateSubscription(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	if err := h.billing.reactivateSubscription(r.Context(), user.ID); err != nil {
		h.log.Error(r.Context(), "billing: reactivate subscription failed",
			"error", err, "user_id", user.ID)

		// Map specific errors to user-friendly messages
		msg := "Could not reactivate subscription. Please try again or contact support."
		code := "reactivate_failed"
		if errors.Is(err, ErrBillingPeriodEnded) {
			msg = "Your billing period has ended. Please subscribe again."
			code = "billing_period_ended"
		} else if errors.Is(err, ErrNotCancelled) {
			msg = "Subscription is not cancelled."
			code = "not_cancelled"
		}

		writeError(w, http.StatusUnprocessableEntity, msg, code)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "active"})
}
