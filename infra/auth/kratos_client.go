package auth

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	client "github.com/ory/client-go"
)

// KratosClient wraps the Ory Kratos SDK client with convenience methods.
type KratosClient struct {
	frontend client.FrontendAPI
	identity client.IdentityAPI
	metadata client.MetadataAPI

	publicURL  string
	adminURL   string
	browserURL string
}

// NewKratosClient creates a new Kratos client from environment variables.
func NewKratosClient() *KratosClient {
	publicURL := requireEnv("KRATOS_PUBLIC_URL")
	adminURL := requireEnv("KRATOS_ADMIN_URL")
	browserURL := requireEnv("KRATOS_BROWSER_URL")

	// Configuration for public (frontend) API
	frontendConfig := client.NewConfiguration()
	frontendConfig.Servers = client.ServerConfigurations{
		{
			URL: publicURL,
		},
	}

	// Configuration for admin API (identity management)
	adminConfig := client.NewConfiguration()
	adminConfig.Servers = client.ServerConfigurations{
		{
			URL: adminURL,
		},
	}

	// Add admin API token if configured
	if token := os.Getenv("ORY_ADMIN_API_TOKEN"); token != "" {
		adminConfig.AddDefaultHeader("Authorization", "Bearer "+token)
	}

	apiClient := client.NewAPIClient(frontendConfig)

	return &KratosClient{
		frontend:   apiClient.FrontendAPI,
		identity:   client.NewAPIClient(adminConfig).IdentityAPI,
		metadata:   apiClient.MetadataAPI,
		publicURL:  publicURL,
		adminURL:   adminURL,
		browserURL: browserURL,
	}
}

// Frontend returns the frontend API for self-service flows.
func (k *KratosClient) Frontend() client.FrontendAPI {
	return k.frontend
}

// Identity returns the identity API for admin operations.
func (k *KratosClient) Identity() client.IdentityAPI {
	return k.identity
}

// Metadata returns the metadata API.
func (k *KratosClient) Metadata() client.MetadataAPI {
	return k.metadata
}

// PublicURL returns the Kratos public API URL (internal).
func (k *KratosClient) PublicURL() string {
	return k.publicURL
}

// AdminURL returns the Kratos admin API URL (internal).
func (k *KratosClient) AdminURL() string {
	return k.adminURL
}

// BrowserURL returns the Kratos browser URL (external, for redirects).
func (k *KratosClient) BrowserURL() string {
	return k.browserURL
}

// ToSession validates a session cookie and returns the session.
func (k *KratosClient) ToSession(ctx context.Context, cookie string) (*client.Session, error) {
	req := k.frontend.ToSession(ctx)
	req = req.Cookie(cookie)

	session, resp, err := req.Execute()
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		if resp != nil {
			slog.Debug("ToSession failed",
				"status", resp.StatusCode,
				"error", err)
		}

		return nil, fmt.Errorf("failed to validate session: %w", err)
	}

	return session, nil
}

// GetLoginFlow fetches a login flow by ID.
func (k *KratosClient) GetLoginFlow(ctx context.Context, flowID string, cookie string) (*client.LoginFlow, error) {
	req := k.frontend.GetLoginFlow(ctx).Id(flowID)
	if cookie != "" {
		req = req.Cookie(cookie)
	}

	flow, resp, err := req.Execute()
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		if resp != nil {
			slog.Debug("GetLoginFlow failed",
				"flowID", flowID,
				"status", resp.StatusCode,
				"error", err)
		}

		return nil, fmt.Errorf("failed to get login flow: %w", err)
	}

	return flow, nil
}

// GetRegistrationFlow fetches a registration flow by ID.
func (k *KratosClient) GetRegistrationFlow(ctx context.Context, flowID string, cookie string) (*client.RegistrationFlow, error) {
	req := k.frontend.GetRegistrationFlow(ctx).Id(flowID)
	if cookie != "" {
		req = req.Cookie(cookie)
	}

	flow, resp, err := req.Execute()
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		if resp != nil {
			slog.Debug("GetRegistrationFlow failed",
				"flowID", flowID,
				"status", resp.StatusCode,
				"error", err)
		}

		return nil, fmt.Errorf("failed to get registration flow: %w", err)
	}

	return flow, nil
}

// GetRecoveryFlow fetches a recovery flow by ID.
func (k *KratosClient) GetRecoveryFlow(ctx context.Context, flowID string, cookie string) (*client.RecoveryFlow, error) {
	req := k.frontend.GetRecoveryFlow(ctx).Id(flowID)
	if cookie != "" {
		req = req.Cookie(cookie)
	}

	flow, resp, err := req.Execute()
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		if resp != nil {
			slog.Debug("GetRecoveryFlow failed",
				"flowID", flowID,
				"status", resp.StatusCode,
				"error", err)
		}

		return nil, fmt.Errorf("failed to get recovery flow: %w", err)
	}

	return flow, nil
}

// GetVerificationFlow fetches a verification flow by ID.
func (k *KratosClient) GetVerificationFlow(ctx context.Context, flowID string, cookie string) (*client.VerificationFlow, error) {
	req := k.frontend.GetVerificationFlow(ctx).Id(flowID)
	if cookie != "" {
		req = req.Cookie(cookie)
	}

	flow, resp, err := req.Execute()
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		if resp != nil {
			slog.Debug("GetVerificationFlow failed",
				"flowID", flowID,
				"status", resp.StatusCode,
				"error", err)
		}

		return nil, fmt.Errorf("failed to get verification flow: %w", err)
	}

	return flow, nil
}

// GetSettingsFlow fetches a settings flow by ID.
func (k *KratosClient) GetSettingsFlow(ctx context.Context, flowID string, cookie string) (*client.SettingsFlow, error) {
	req := k.frontend.GetSettingsFlow(ctx).Id(flowID)
	if cookie != "" {
		req = req.Cookie(cookie)
	}

	flow, resp, err := req.Execute()
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		if resp != nil {
			slog.Debug("GetSettingsFlow failed",
				"flowID", flowID,
				"status", resp.StatusCode,
				"error", err)
		}

		return nil, fmt.Errorf("failed to get settings flow: %w", err)
	}

	return flow, nil
}

// CreateBrowserLogoutFlow creates a logout flow.
func (k *KratosClient) CreateBrowserLogoutFlow(ctx context.Context, cookie string) (*client.LogoutFlow, error) {
	req := k.frontend.CreateBrowserLogoutFlow(ctx)
	if cookie != "" {
		req = req.Cookie(cookie)
	}

	flow, resp, err := req.Execute()
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		if resp != nil {
			slog.Debug("CreateBrowserLogoutFlow failed",
				"status", resp.StatusCode,
				"error", err)
		}

		return nil, fmt.Errorf("failed to create logout flow: %w", err)
	}

	return flow, nil
}

// GetFlowError fetches the error details for a flow.
func (k *KratosClient) GetFlowError(ctx context.Context, errorID string) (*client.FlowError, error) {
	req := k.frontend.GetFlowError(ctx).Id(errorID)

	flowError, resp, err := req.Execute()
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		if resp != nil {
			slog.Debug("GetFlowError failed",
				"errorID", errorID,
				"status", resp.StatusCode,
				"error", err)
		}

		return nil, fmt.Errorf("failed to get flow error: %w", err)
	}

	return flowError, nil
}

// requireEnv gets an environment variable or panics if not set.
func requireEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}

	return value
}

// getEnv gets an environment variable with a default fallback.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}
