package billing

// This file was extracted from infra/projection/payment/postgres_projection.go
// as part of the modular-monolith migration. All types and methods are now
// unexported (module-internal); the Module facade in billing.go exposes
// projection registration.

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/event"
	"school-with-ai/foundation/logger"

	"github.com/jmoiron/sqlx"
)

// accountStatus is the projected read model for a user's payment account.
type accountStatus struct {
	UserID      string     `db:"user_id"      json:"userId"`
	Status      string     `db:"status"       json:"status"` // "none", "active", "cancelled"
	PaidAt      *time.Time `db:"paid_at"      json:"paidAt,omitempty"`
	AmountCents int64      `db:"amount_cents"  json:"amountCents"`
	Currency    string     `db:"currency"      json:"currency"`
	CardBrand   *string    `db:"card_brand"    json:"cardBrand,omitempty"`
	CardLast4   *string    `db:"card_last4"    json:"cardLast4,omitempty"`
	CancelledAt *time.Time `db:"cancelled_at"  json:"cancelledAt,omitempty"`
	CancelsOn   *time.Time `db:"cancels_on"    json:"cancelsOn,omitempty"`
	UpdatedAt   time.Time  `db:"updated_at"    json:"updatedAt"`
}

// postgresPaymentProjection builds a denormalized read model from payment events.
type postgresPaymentProjection struct {
	log *logger.Logger
	db  *sqlx.DB
}

// newPostgresPaymentProjection creates a new projection.
func newPostgresPaymentProjection(log *logger.Logger, db *sqlx.DB) *postgresPaymentProjection {
	return &postgresPaymentProjection{log: log, db: db}
}

// registerHandlers wires event handlers into the persistence router.
func (p *postgresPaymentProjection) registerHandlers(router interface {
	Register(eventPrototype any, handler event.HandlerFunc)
},
) {
	router.Register(&domain.PaymentAccountOpened{}, func(ctx context.Context, e domain.Event) error {
		return p.onAccountOpened(ctx, e.(*domain.PaymentAccountOpened))
	})
	router.Register(&domain.PaymentInitiated{}, func(ctx context.Context, e domain.Event) error {
		return p.onPaymentInitiated(ctx, e.(*domain.PaymentInitiated))
	})
	router.Register(&domain.PaymentSucceeded{}, func(ctx context.Context, e domain.Event) error {
		return p.onPaymentSucceeded(ctx, e.(*domain.PaymentSucceeded))
	})
	router.Register(&domain.PaymentFailed{}, func(ctx context.Context, e domain.Event) error {
		return p.onPaymentFailed(ctx, e.(*domain.PaymentFailed))
	})
	router.Register(&domain.PaymentRefunded{}, func(ctx context.Context, e domain.Event) error {
		return p.onPaymentRefunded(ctx, e.(*domain.PaymentRefunded))
	})
	router.Register(&domain.SubscriptionCancelled{}, func(ctx context.Context, e domain.Event) error {
		return p.onSubscriptionCancelled(ctx, e.(*domain.SubscriptionCancelled))
	})
	router.Register(&domain.SubscriptionExpired{}, func(ctx context.Context, e domain.Event) error {
		return p.onSubscriptionExpired(ctx, e.(*domain.SubscriptionExpired))
	})
	router.Register(&domain.SubscriptionReactivated{}, func(ctx context.Context, e domain.Event) error {
		return p.onSubscriptionReactivated(ctx, e.(*domain.SubscriptionReactivated))
	})

	// --- New Stripe SDK v84 aligned events ---
	router.Register(&domain.PaymentDisputed{}, func(ctx context.Context, e domain.Event) error {
		return p.onPaymentDisputed(ctx, e.(*domain.PaymentDisputed))
	})
	router.Register(&domain.DisputeResolved{}, func(ctx context.Context, e domain.Event) error {
		return p.onDisputeResolved(ctx, e.(*domain.DisputeResolved))
	})
	router.Register(&domain.PaymentMethodUpdated{}, func(ctx context.Context, e domain.Event) error {
		return p.onPaymentMethodUpdated(ctx, e.(*domain.PaymentMethodUpdated))
	})
	router.Register(&domain.InvoicePaymentFailed{}, func(ctx context.Context, e domain.Event) error {
		return p.onInvoicePaymentFailed(ctx, e.(*domain.InvoicePaymentFailed))
	})
}

// getAccountStatus returns the projected payment status for a user.
// Returns nil, nil if no account exists.
func (p *postgresPaymentProjection) getAccountStatus(ctx context.Context, userID string) (*accountStatus, error) {
	var status accountStatus
	err := p.db.GetContext(ctx, &status,
		`SELECT user_id, status, paid_at, amount_cents, currency, card_brand, card_last4, cancelled_at, cancels_on, updated_at
		   FROM payment_accounts WHERE user_id = $1`, userID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("payment projection: get status: %w", err)
	}
	return &status, nil
}

// hasPaid checks whether a user has an active (paid) account.
// A user in "cancelled" state still has access until the billing period ends.
func (p *postgresPaymentProjection) hasPaid(ctx context.Context, userID string) (bool, error) {
	status, err := p.getAccountStatus(ctx, userID)
	if err != nil {
		return false, err
	}
	return status != nil && (status.Status == "active" || status.Status == "cancelled"), nil
}

// -------------------------------------------------------------------------
// Event handlers — project events into the read model
// -------------------------------------------------------------------------

func (p *postgresPaymentProjection) onAccountOpened(ctx context.Context, e *domain.PaymentAccountOpened) error {
	_, err := p.db.ExecContext(ctx,
		`INSERT INTO payment_accounts (user_id, status, amount_cents, currency, updated_at)
		 VALUES ($1, 'none', 0, 'usd', $2)
		 ON CONFLICT (user_id) DO NOTHING`,
		e.AccountID, e.Timestamp)
	if err != nil {
		return fmt.Errorf("payment projection: account opened: %w", err)
	}
	return nil
}

func (p *postgresPaymentProjection) onPaymentInitiated(ctx context.Context, e *domain.PaymentInitiated) error {
	// Track the pending payment in the read model. Only transition to
	// 'pending' if the account is currently in 'none' state (not already
	// active or cancelled) to avoid regressing the status.
	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET status = CASE WHEN status = 'none' THEN 'pending' ELSE status END,
		        updated_at = $1
		  WHERE user_id = $2`,
		e.Timestamp, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: payment initiated: %w", err)
	}
	return nil
}

func (p *postgresPaymentProjection) onPaymentSucceeded(ctx context.Context, e *domain.PaymentSucceeded) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET status = 'active',
		        paid_at = $1,
		        amount_cents = amount_cents + $2,
		        currency = $3,
		        card_brand = COALESCE(NULLIF($4, ''), card_brand),
		        card_last4 = COALESCE(NULLIF($5, ''), card_last4),
		        updated_at = $1
		  WHERE user_id = $6`,
		e.Timestamp, e.AmountCents, e.Currency, e.CardBrand, e.CardLast4, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: payment succeeded: %w", err)
	}
	return nil
}

func (p *postgresPaymentProjection) onPaymentFailed(ctx context.Context, e *domain.PaymentFailed) error {
	// Revert status from 'pending' to 'none' so the user can retry.
	// Only revert if the current status is 'pending' — don't touch active/cancelled.
	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET status = CASE WHEN status = 'pending' THEN 'none' ELSE status END,
		        updated_at = $1
		  WHERE user_id = $2`,
		e.Timestamp, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: payment failed: %w", err)
	}
	p.log.Info(context.Background(), "payment projection: payment failed — reverted to none if pending",
		"user_id", e.AccountID, "reason", e.Reason)
	return nil
}

func (p *postgresPaymentProjection) onPaymentRefunded(ctx context.Context, e *domain.PaymentRefunded) error {
	// Check if net balance goes to zero → deactivate
	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET amount_cents = GREATEST(amount_cents - $1, 0),
		        status = CASE WHEN (amount_cents - $1) <= 0 THEN 'none' ELSE status END,
		        paid_at = CASE WHEN (amount_cents - $1) <= 0 THEN NULL ELSE paid_at END,
		        updated_at = $2
		  WHERE user_id = $3`,
		e.AmountCents, e.Timestamp, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: refunded: %w", err)
	}
	return nil
}

func (p *postgresPaymentProjection) onSubscriptionCancelled(ctx context.Context, e *domain.SubscriptionCancelled) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET status = 'cancelled',
		        cancelled_at = $1,
		        cancels_on = $2,
		        updated_at = $3
		  WHERE user_id = $4`,
		e.CancelledAt, e.CancelsOn, e.Timestamp, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: subscription cancelled: %w", err)
	}
	return nil
}

func (p *postgresPaymentProjection) onSubscriptionExpired(ctx context.Context, e *domain.SubscriptionExpired) error {
	// Transition to 'none' (free tier) but preserve paid_at and amount_cents
	// as historical record. The event store has the full audit trail, but
	// keeping these in the projection enables "previously paid" queries.
	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET status = 'none',
		        cancelled_at = NULL,
		        cancels_on = NULL,
		        updated_at = $1
		  WHERE user_id = $2`,
		e.Timestamp, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: subscription expired: %w", err)
	}
	return nil
}

func (p *postgresPaymentProjection) onSubscriptionReactivated(ctx context.Context, e *domain.SubscriptionReactivated) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET status = 'active',
		        cancelled_at = NULL,
		        cancels_on = NULL,
		        updated_at = $1
		  WHERE user_id = $2`,
		e.Timestamp, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: subscription reactivated: %w", err)
	}
	return nil
}

// -------------------------------------------------------------------------
// New event handlers — Stripe SDK v84 aligned
// -------------------------------------------------------------------------

func (p *postgresPaymentProjection) onPaymentDisputed(ctx context.Context, e *domain.PaymentDisputed) error {
	// Mark the account as disputed. The status stays "active" since the
	// user retains access during the dispute period, but we record the
	// dispute metadata. Operators can query disputes via SQL.
	//
	// We use a soft approach: log + update updated_at. A future migration
	// could add dispute_status, disputed_amount columns to the projection.
	p.log.Error(context.Background(), "payment projection: ⚠️  DISPUTE opened",
		"user_id", e.AccountID,
		"dispute_id", e.StripeDisputeID,
		"charge_id", e.StripeChargeID,
		"amount_cents", e.AmountCents,
		"reason", e.Reason,
	)

	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET updated_at = $1
		  WHERE user_id = $2`,
		e.Timestamp, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: dispute: %w", err)
	}
	return nil
}

func (p *postgresPaymentProjection) onDisputeResolved(ctx context.Context, e *domain.DisputeResolved) error {
	p.log.Info(context.Background(), "payment projection: dispute resolved",
		"user_id", e.AccountID,
		"dispute_id", e.StripeDisputeID,
		"resolution", e.Status,
		"amount_cents", e.AmountCents,
	)

	// If the dispute was lost, the funds are withdrawn. Deduct from the
	// projected balance (same as a refund).
	if e.Status == "lost" {
		_, err := p.db.ExecContext(ctx,
			`UPDATE payment_accounts
			    SET amount_cents = GREATEST(amount_cents - $1, 0),
			        status = CASE WHEN (amount_cents - $1) <= 0 THEN 'none' ELSE status END,
			        paid_at = CASE WHEN (amount_cents - $1) <= 0 THEN NULL ELSE paid_at END,
			        updated_at = $2
			  WHERE user_id = $3`,
			e.AmountCents, e.Timestamp, e.AccountID)
		if err != nil {
			return fmt.Errorf("payment projection: dispute lost: %w", err)
		}
	} else {
		// Won — just update the timestamp
		_, err := p.db.ExecContext(ctx,
			`UPDATE payment_accounts
			    SET updated_at = $1
			  WHERE user_id = $2`,
			e.Timestamp, e.AccountID)
		if err != nil {
			return fmt.Errorf("payment projection: dispute won: %w", err)
		}
	}
	return nil
}

func (p *postgresPaymentProjection) onPaymentMethodUpdated(ctx context.Context, e *domain.PaymentMethodUpdated) error {
	// Update the displayed card brand and last4 when the user updates
	// their payment method (e.g. card expires, they add a new one).
	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET card_brand = COALESCE(NULLIF($1, ''), card_brand),
		        card_last4 = COALESCE(NULLIF($2, ''), card_last4),
		        updated_at = $3
		  WHERE user_id = $4`,
		e.CardBrand, e.CardLast4, e.Timestamp, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: payment method updated: %w", err)
	}
	return nil
}

func (p *postgresPaymentProjection) onInvoicePaymentFailed(ctx context.Context, e *domain.InvoicePaymentFailed) error {
	// Log the recurring payment failure. The projection doesn't change
	// the account status yet — the aggregate tracks ConsecutiveInvoiceFailures
	// and can flag the account when the threshold is reached.
	p.log.Error(context.Background(), "payment projection: invoice payment failed",
		"user_id", e.AccountID,
		"invoice_id", e.StripeInvoiceID,
		"amount_due_cents", e.AmountDueCents,
		"attempt_count", e.AttemptCount,
	)

	_, err := p.db.ExecContext(ctx,
		`UPDATE payment_accounts
		    SET updated_at = $1
		  WHERE user_id = $2`,
		e.Timestamp, e.AccountID)
	if err != nil {
		return fmt.Errorf("payment projection: invoice payment failed: %w", err)
	}
	return nil
}
