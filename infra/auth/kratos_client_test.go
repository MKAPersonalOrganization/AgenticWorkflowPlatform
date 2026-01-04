package auth

import (
	"context"
	"os"
	"testing"
)

func TestNewKratosClient(t *testing.T) {
	// Save original env vars
	origPublic := os.Getenv("KRATOS_PUBLIC_URL")
	origAdmin := os.Getenv("KRATOS_ADMIN_URL")
	origBrowser := os.Getenv("KRATOS_BROWSER_URL")

	// Clean up
	defer func() {
		os.Setenv("KRATOS_PUBLIC_URL", origPublic)
		os.Setenv("KRATOS_ADMIN_URL", origAdmin)
		os.Setenv("KRATOS_BROWSER_URL", origBrowser)
	}()

	tests := []struct {
		name            string
		publicURL       string
		adminURL        string
		browserURL      string
		expectedPublic  string
		expectedAdmin   string
		expectedBrowser string
	}{
		{
			name:            "env values",
			publicURL:       "http://127.0.0.1:4433",
			adminURL:        "http://127.0.0.1:4434",
			browserURL:      "http://127.0.0.1:4433",
			expectedPublic:  "http://127.0.0.1:4433",
			expectedAdmin:   "http://127.0.0.1:4434",
			expectedBrowser: "http://127.0.0.1:4433",
		},
		{
			name:            "custom values",
			publicURL:       "http://custom-public:8080",
			adminURL:        "http://custom-admin:8081",
			browserURL:      "http://localhost:8080",
			expectedPublic:  "http://custom-public:8080",
			expectedAdmin:   "http://custom-admin:8081",
			expectedBrowser: "http://localhost:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables (all required)
			os.Setenv("KRATOS_PUBLIC_URL", tt.publicURL)
			os.Setenv("KRATOS_ADMIN_URL", tt.adminURL)
			os.Setenv("KRATOS_BROWSER_URL", tt.browserURL)

			// Create client
			client := NewKratosClient()

			if client == nil {
				t.Fatal("NewKratosClient() returned nil")
			}

			if client.PublicURL() != tt.expectedPublic {
				t.Errorf("PublicURL() = %v, want %v", client.PublicURL(), tt.expectedPublic)
			}

			if client.AdminURL() != tt.expectedAdmin {
				t.Errorf("AdminURL() = %v, want %v", client.AdminURL(), tt.expectedAdmin)
			}

			if client.BrowserURL() != tt.expectedBrowser {
				t.Errorf("BrowserURL() = %v, want %v", client.BrowserURL(), tt.expectedBrowser)
			}

			if client.Frontend() == nil {
				t.Error("Frontend() returned nil")
			}

			if client.Identity() == nil {
				t.Error("Identity() returned nil")
			}

			if client.Metadata() == nil {
				t.Error("Metadata() returned nil")
			}
		})
	}
}

func TestKratosClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Set up environment for integration test
	os.Setenv("KRATOS_PUBLIC_URL", "http://127.0.0.1:4433")
	os.Setenv("KRATOS_ADMIN_URL", "http://127.0.0.1:4434")
	os.Setenv("KRATOS_BROWSER_URL", "http://127.0.0.1:4433")

	client := NewKratosClient()
	ctx := context.Background()

	t.Run("ToSession with invalid cookie", func(t *testing.T) {
		_, err := client.ToSession(ctx, "invalid-cookie")
		if err == nil {
			t.Error("Expected error for invalid cookie, got nil")
		}
	})

	t.Run("GetLoginFlow with invalid ID", func(t *testing.T) {
		_, err := client.GetLoginFlow(ctx, "invalid-flow-id", "")
		if err == nil {
			t.Error("Expected error for invalid flow ID, got nil")
		}
	})

	t.Run("GetFlowError with invalid ID", func(t *testing.T) {
		_, err := client.GetFlowError(ctx, "invalid-error-id")
		if err == nil {
			t.Error("Expected error for invalid error ID, got nil")
		}
	})
}

func TestGetEnv(t *testing.T) {
	// Save and restore env var
	orig := os.Getenv("TEST_VAR")
	defer func() {
		if orig != "" {
			os.Setenv("TEST_VAR", orig)
		} else {
			os.Unsetenv("TEST_VAR")
		}
	}()

	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "env var set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "custom",
			want:         "custom",
		},
		{
			name:         "env var not set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
			} else {
				os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
