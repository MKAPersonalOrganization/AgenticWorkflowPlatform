package nav

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/foundation/i18n"
)

type pricingPlan struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Price       string   `json:"price"`
	Period      string   `json:"period"`
	Description string   `json:"description"`
	Features    []string `json:"features"`
	Popular     bool     `json:"popular"`
	ButtonText  string   `json:"buttonText"`
}

func pricingAlpineData(ctx context.Context) string {
	plans := []pricingPlan{
		{
			ID:          "free",
			Name:        i18n.T(ctx, "Free"),
			Price:       i18n.T(ctx, "$0"),
			Description: i18n.T(ctx, "Perfect for getting started"),
			Features: []string{
				i18n.T(ctx, "10 conversations per day"),
				i18n.T(ctx, "Basic AI model"),
				i18n.T(ctx, "Community support"),
				i18n.T(ctx, "Standard response time"),
			},
			ButtonText: i18n.T(ctx, "Current Plan"),
		},
		{
			ID:          "go",
			Name:        i18n.T(ctx, "Go"),
			Price:       i18n.T(ctx, "$9"),
			Period:      i18n.T(ctx, "/month"),
			Description: i18n.T(ctx, "For individuals who need more"),
			Features: []string{
				i18n.T(ctx, "100 conversations per day"),
				i18n.T(ctx, "Advanced AI model"),
				i18n.T(ctx, "Priority support"),
				i18n.T(ctx, "Faster response time"),
				i18n.T(ctx, "Export conversation history"),
			},
			ButtonText: i18n.T(ctx, "Upgrade"),
		},
		{
			ID:          "plus",
			Name:        i18n.T(ctx, "Plus"),
			Price:       i18n.T(ctx, "$19"),
			Period:      i18n.T(ctx, "/month"),
			Description: i18n.T(ctx, "Most popular for power users"),
			Features: []string{
				i18n.T(ctx, "Unlimited conversations"),
				i18n.T(ctx, "Premium AI models"),
				i18n.T(ctx, "Priority support"),
				i18n.T(ctx, "Fastest response time"),
				i18n.T(ctx, "Export conversation history"),
				i18n.T(ctx, "Advanced analytics"),
				i18n.T(ctx, "Custom instructions"),
			},
			Popular:    true,
			ButtonText: i18n.T(ctx, "Upgrade"),
		},
		{
			ID:          "pro",
			Name:        i18n.T(ctx, "Pro"),
			Price:       i18n.T(ctx, "$49"),
			Period:      i18n.T(ctx, "/month"),
			Description: i18n.T(ctx, "For professionals and teams"),
			Features: []string{
				i18n.T(ctx, "Everything in Plus"),
				i18n.T(ctx, "Dedicated support"),
				i18n.T(ctx, "API access"),
				i18n.T(ctx, "Team collaboration"),
				i18n.T(ctx, "Custom integrations"),
				i18n.T(ctx, "SLA guarantee"),
				i18n.T(ctx, "Advanced security"),
			},
			ButtonText: i18n.T(ctx, "Upgrade"),
		},
	}

	plansJSON, _ := json.Marshal(plans)

	return fmt.Sprintf(`{
		open: false,
		plans: %s,
		selectPlan(plan) {
			this.open = false;
			$dispatch('open-checkout-dialog', plan);
		}
	}`, string(plansJSON))
}
