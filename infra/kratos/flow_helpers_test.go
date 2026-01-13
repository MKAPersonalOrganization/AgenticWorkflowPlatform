package kratos

import (
	"context"
	"errors"
	"testing"

	client "github.com/ory/client-go"
)

// Test error sentinels for testing error handling.
var (
	errTest410FlowExpired      = errors.New("410 Gone: flow expired")
	errTest404NotFound         = errors.New("404 Not Found")
	errTestFlowExpired         = errors.New("flow expired")
	errTestFlowNotFound        = errors.New("flow not found")
	errTestNetworkError        = errors.New("network error")
	errTestSessionAAL2Required = errors.New("session_aal2_required")
	errTestAAL2Required        = errors.New("authentication level aal2 required")
	errTestInvalidCredentials  = errors.New("invalid credentials")
)

func TestInitFlowURL(t *testing.T) {
	t.Parallel()
	kratosClient := NewKratosClient(testConfig())

	tests := []struct {
		name     string
		flowType FlowType
		params   FlowParams
		contains []string
	}{
		{
			name:     "login flow basic",
			flowType: FlowTypeLogin,
			params:   FlowParams{},
			contains: []string{"/self-service/login/browser"},
		},
		{
			name:     "login flow with return_to",
			flowType: FlowTypeLogin,
			params: FlowParams{
				ReturnTo: "/dashboard",
			},
			contains: []string{"/self-service/login/browser", "return_to=%2Fdashboard"},
		},
		{
			name:     "login flow with AAL2",
			flowType: FlowTypeLogin,
			params: FlowParams{
				AAL:      "aal2",
				ReturnTo: "/settings",
			},
			contains: []string{"/self-service/login/browser", "aal=aal2", "return_to"},
		},
		{
			name:     "registration flow",
			flowType: FlowTypeRegistration,
			params:   FlowParams{},
			contains: []string{"/self-service/registration/browser"},
		},
		{
			name:     "recovery flow",
			flowType: FlowTypeRecovery,
			params:   FlowParams{},
			contains: []string{"/self-service/recovery/browser"},
		},
		{
			name:     "verification flow",
			flowType: FlowTypeVerification,
			params:   FlowParams{},
			contains: []string{"/self-service/verification/browser"},
		},
		{
			name:     "settings flow with refresh",
			flowType: FlowTypeSettings,
			params: FlowParams{
				Refresh: true,
			},
			contains: []string{"/self-service/settings/browser", "refresh=true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			url := kratosClient.InitFlowURL(tt.flowType, tt.params)

			for _, expected := range tt.contains {
				if !contains(url, expected) {
					t.Errorf("InitFlowURL() = %v, should contain %v", url, expected)
				}
			}
		})
	}
}

func TestExtractCSRFToken(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		nodes    []client.UiNode
		expected string
	}{
		{
			name:     "no nodes",
			nodes:    []client.UiNode{},
			expected: "",
		},
		{
			name: "csrf token present",
			nodes: []client.UiNode{
				{
					Group: "default",
					Attributes: client.UiNodeAttributes{
						UiNodeInputAttributes: &client.UiNodeInputAttributes{
							Name:  "csrf_token",
							Value: "test-csrf-token-123",
							Type:  "hidden",
						},
					},
				},
			},
			expected: "test-csrf-token-123",
		},
		{
			name: "multiple nodes with csrf",
			nodes: []client.UiNode{
				{
					Group: "password",
					Attributes: client.UiNodeAttributes{
						UiNodeInputAttributes: &client.UiNodeInputAttributes{
							Name:  "password",
							Value: "",
							Type:  "password",
						},
					},
				},
				{
					Group: "default",
					Attributes: client.UiNodeAttributes{
						UiNodeInputAttributes: &client.UiNodeInputAttributes{
							Name:  "csrf_token",
							Value: "another-token",
							Type:  "hidden",
						},
					},
				},
			},
			expected: "another-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ExtractCSRFToken(tt.nodes)
			if result != tt.expected {
				t.Errorf("ExtractCSRFToken() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractMessages(t *testing.T) {
	t.Parallel()
	messages := []client.UiText{
		{
			Id:   1010001,
			Text: "Please enter your email address",
			Type: "info",
		},
		{
			Id:   4000006,
			Text: "Invalid credentials",
			Type: "error",
			Context: map[string]any{
				"reason": "wrong password",
			},
		},
	}

	result := ExtractMessages(messages)

	if len(result) != 2 {
		t.Errorf("ExtractMessages() returned %d messages, want 2", len(result))
	}

	if result[0].ID != 1010001 {
		t.Errorf("First message ID = %d, want 1010001", result[0].ID)
	}

	if result[1].Type != "error" {
		t.Errorf("Second message type = %s, want error", result[1].Type)
	}

	if result[1].Context == nil {
		t.Error("Second message context is nil")
	}
}

func TestGetMessageType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"error", "error"},
		{"ERROR", "error"},
		{"success", "success"},
		{"SUCCESS", "success"},
		{"info", "info"},
		{"INFO", "info"},
		{"warning", "info"}, // Falls back to info
		{"unknown", "info"}, // Falls back to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			result := GetMessageType(tt.input)
			if result != tt.expected {
				t.Errorf("GetMessageType(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsFlowExpired(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "410 error",
			err:      errTest410FlowExpired,
			expected: true,
		},
		{
			name:     "404 error",
			err:      errTest404NotFound,
			expected: true,
		},
		{
			name:     "explicit flow expired",
			err:      errTestFlowExpired,
			expected: true,
		},
		{
			name:     "flow not found",
			err:      errTestFlowNotFound,
			expected: true,
		},
		{
			name:     "other error",
			err:      errTestNetworkError,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsFlowExpired(tt.err)
			if result != tt.expected {
				t.Errorf("IsFlowExpired() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestShouldRedirectToVerification(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		messages []client.UiText
		expected bool
	}{
		{
			name:     "no messages",
			messages: []client.UiText{},
			expected: false,
		},
		{
			name: "verification required message",
			messages: []client.UiText{
				{
					Id:   4000010,
					Text: "Account verification required",
					Type: "error",
				},
			},
			expected: true,
		},
		{
			name: "other messages",
			messages: []client.UiText{
				{
					Id:   1010001,
					Text: "Please enter your email",
					Type: "info",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ShouldRedirectToVerification(tt.messages)
			if result != tt.expected {
				t.Errorf("ShouldRedirectToVerification() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsAAL2Required(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "aal2 required",
			err:      errTestSessionAAL2Required,
			expected: true,
		},
		{
			name:     "aal2 in message",
			err:      errTestAAL2Required,
			expected: true,
		},
		{
			name:     "other error",
			err:      errTestInvalidCredentials,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := IsAAL2Required(tt.err)
			if result != tt.expected {
				t.Errorf("IsAAL2Required() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetNodeValue(t *testing.T) {
	t.Parallel()
	nodes := []client.UiNode{
		{
			Attributes: client.UiNodeAttributes{
				UiNodeInputAttributes: &client.UiNodeInputAttributes{
					Name:  "email",
					Value: "user@example.com",
				},
			},
		},
		{
			Attributes: client.UiNodeAttributes{
				UiNodeInputAttributes: &client.UiNodeInputAttributes{
					Name:  "password",
					Value: "",
				},
			},
		},
	}

	tests := []struct {
		name     string
		nodeName string
		expected string
	}{
		{
			name:     "email value",
			nodeName: "email",
			expected: "user@example.com",
		},
		{
			name:     "password value (empty)",
			nodeName: "password",
			expected: "",
		},
		{
			name:     "non-existent field",
			nodeName: "username",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := GetNodeValue(nodes, tt.nodeName)
			if result != tt.expected {
				t.Errorf("GetNodeValue() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFilterNodesByGroup(t *testing.T) {
	t.Parallel()
	nodes := []client.UiNode{
		{
			Group: "default",
			Attributes: client.UiNodeAttributes{
				UiNodeInputAttributes: &client.UiNodeInputAttributes{
					Name: "csrf_token",
				},
			},
		},
		{
			Group: "password",
			Attributes: client.UiNodeAttributes{
				UiNodeInputAttributes: &client.UiNodeInputAttributes{
					Name: "password",
				},
			},
		},
		{
			Group: "profile",
			Attributes: client.UiNodeAttributes{
				UiNodeInputAttributes: &client.UiNodeInputAttributes{
					Name: "first_name",
				},
			},
		},
	}

	tests := []struct {
		name     string
		groups   []string
		expected int
	}{
		{
			name:     "no filter",
			groups:   []string{},
			expected: 3,
		},
		{
			name:     "filter password group",
			groups:   []string{"password"},
			expected: 1,
		},
		{
			name:     "filter multiple groups",
			groups:   []string{"default", "profile"},
			expected: 2,
		},
		{
			name:     "filter non-existent group",
			groups:   []string{"oidc"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := FilterNodesByGroup(nodes, tt.groups...)
			if len(result) != tt.expected {
				t.Errorf("FilterNodesByGroup() returned %d nodes, want %d", len(result), tt.expected)
			}
		})
	}
}

// Integration tests (these will only run with -short=false and require running Kratos).
func TestKratosClient_GetLoginFlow_Integration(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	kratosClient := NewKratosClient(testConfig())
	ctx := context.Background()

	// Try to get a non-existent flow
	_, err := kratosClient.GetLoginFlow(ctx, "invalid-flow-id", "")
	if err == nil {
		t.Error("Expected error for invalid flow ID, got nil")
	}
}

func TestKratosClient_GetFlowError_Integration(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	kratosClient := NewKratosClient(testConfig())
	ctx := context.Background()

	// Try to get a non-existent error
	_, err := kratosClient.GetFlowError(ctx, "invalid-error-id")
	if err == nil {
		t.Error("Expected error for invalid error ID, got nil")
	}
}

// Helper function.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
