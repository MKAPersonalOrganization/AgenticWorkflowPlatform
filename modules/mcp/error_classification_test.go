package mcp

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

// TestIsAuthError verifies that isAuthError matches the exact error strings
// the go-sdk produces for 401 responses. The go-sdk formats non-2xx responses
// using http.StatusText(code), so the error message contains "Unauthorized"
// without the numeric 401.
func TestIsAuthError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		// ---- should match ----
		{
			name: "streamable_initialize_401",
			err:  fmt.Errorf(`calling "initialize": sending "initialize": %s`, http.StatusText(http.StatusUnauthorized)),
			want: true,
		},
		{
			name: "sse_connect_401",
			err:  fmt.Errorf("failed to connect: %s", http.StatusText(http.StatusUnauthorized)),
			want: true,
		},
		{
			name: "wrapped_401",
			err:  fmt.Errorf("connecting to MCP server: %w", fmt.Errorf(`sending "initialize": Unauthorized`)),
			want: true,
		},
		// ---- should NOT match ----
		{
			name: "nil_error",
			err:  nil,
			want: false,
		},
		{
			name: "403_forbidden",
			err:  fmt.Errorf("sending request: %s", http.StatusText(http.StatusForbidden)),
			want: false,
		},
		{
			name: "connection_refused",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "500_internal",
			err:  fmt.Errorf("server returned: %s", http.StatusText(http.StatusInternalServerError)),
			want: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isAuthError(tt.err); got != tt.want {
				t.Errorf("isAuthError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsHTTPClientError verifies that 4xx errors (excluding 401) are classified
// as permanent client errors using http.StatusText output.
// TestIsTransientError verifies that 5xx and network errors are classified as transient.
func TestIsTransientError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		// ---- 5xx status texts (go-sdk format) ----
		{
			name: "502_bad_gateway",
			err:  fmt.Errorf(`sending "tools/call": %s`, http.StatusText(http.StatusBadGateway)),
			want: true,
		},
		{
			name: "503_service_unavailable",
			err:  fmt.Errorf("request: %s", http.StatusText(http.StatusServiceUnavailable)),
			want: true,
		},
		{
			name: "504_gateway_timeout",
			err:  fmt.Errorf("request: %s", http.StatusText(http.StatusGatewayTimeout)),
			want: true,
		},
		// ---- network-level ----
		{
			name: "connection_refused",
			err:  errors.New("dial tcp 10.0.0.1:443: connection refused"),
			want: true,
		},
		{
			name: "connection_reset",
			err:  errors.New("read: connection reset by peer"),
			want: true,
		},
		{
			name: "timeout",
			err:  errors.New("context deadline exceeded"),
			want: true,
		},
		{
			name: "eof",
			err:  errors.New("unexpected EOF"),
			want: true,
		},
		// ---- should NOT match ----
		{
			name: "401_not_transient",
			err:  fmt.Errorf("request: %s", http.StatusText(http.StatusUnauthorized)),
			want: false,
		},
		{
			name: "403_not_transient",
			err:  fmt.Errorf("request: %s", http.StatusText(http.StatusForbidden)),
			want: false,
		},
		{
			name: "nil_error",
			err:  nil,
			want: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isTransientError(tt.err); got != tt.want {
				t.Errorf("isTransientError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStatusTextMatchesGoSDK is a safety net: it verifies that the status text
// constants we use for classification match what the go-sdk produces. If the Go
// standard library ever changes http.StatusText output, this test will catch it.
func TestStatusTextMatchesGoSDK(t *testing.T) {
	t.Parallel()

	t.Run("auth", func(t *testing.T) {
		t.Parallel()
		if authStatusText != "Unauthorized" {
			t.Fatalf("authStatusText = %q, want %q", authStatusText, "Unauthorized")
		}
	})

	t.Run("transient_errors_are_status_text", func(t *testing.T) {
		t.Parallel()
		expectedTexts := map[string]bool{
			"Bad Gateway":         true,
			"Service Unavailable": true,
			"Gateway Timeout":     true,
		}
		for _, text := range transientStatusTexts {
			if !expectedTexts[text] {
				t.Errorf("unexpected transient error text: %q", text)
			}
		}
	})
}
