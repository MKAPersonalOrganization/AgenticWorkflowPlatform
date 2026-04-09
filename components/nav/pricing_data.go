package nav

import (
	"aicompass/foundation/i18n"
	"context"
	"fmt"
)

// activationAlpineData returns the Alpine.js x-data JSON for the activation
// dialog. Single plan: $5/month subscription with full access.
func activationAlpineData(ctx context.Context) string {
	return fmt.Sprintf(`{
		open: false,
		alreadyActive: false,
		checking: false,
		async checkAndOpen() {
			this.open = true;
			this.checking = true;
			this.alreadyActive = false;
			try {
				const r = await fetch('/api/billing/status', { credentials: 'same-origin' });
				if (r.ok) {
					const d = await r.json();
					if (d.status === 'active' || d.status === 'cancelled') { this.alreadyActive = true; }
				}
			} catch(e) {}
			this.checking = false;
		},
		activate() {
			this.open = false;
			$dispatch('open-checkout-dialog');
		},
		label: %q,
		price: %q,
		period: %q,
		description: %q,
		features: [
			%q,
			%q,
			%q,
			%q,
			%q,
			%q,
			%q
		]
	}`,
		i18n.T(ctx, "Full Access"),
		i18n.T(ctx, "$5"),
		i18n.T(ctx, "/month"),
		i18n.T(ctx, "Subscribe and get unlimited access to every feature"),
		i18n.T(ctx, "Unlimited conversations"),
		i18n.T(ctx, "All AI models"),
		i18n.T(ctx, "Priority support"),
		i18n.T(ctx, "Export conversation history"),
		i18n.T(ctx, "Advanced analytics"),
		i18n.T(ctx, "Custom instructions"),
		i18n.T(ctx, "Cancel anytime"),
	)
}
