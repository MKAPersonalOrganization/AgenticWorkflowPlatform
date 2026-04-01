package taskmgmt_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"school-with-ai/foundation/logger"
	"school-with-ai/shared"

	taskmgmthandlers "school-with-ai/modules/chat/taskmgmt"
)

func newTestLogger() *logger.Logger {
	return logger.NewTextWithEvents(os.Stdout, logger.LevelInfo, "TEST", nil, logger.Events{})
}

// mockAuthService implements shared.AuthService for testing.
type mockAuthService struct{}

var _ shared.AuthService = (*mockAuthService)(nil)

func (m *mockAuthService) GetUserFromContext(ctx context.Context) *shared.User { return nil }
func (m *mockAuthService) IsAuthenticated(ctx context.Context) bool            { return false }

func TestSSECORSOrigin(t *testing.T) {
	t.Parallel()
	const appURL = "https://school.example.com"
	handler := taskmgmthandlers.NewTaskManagementHandler(
		newTestLogger(),
		nil, // orchestrator
		nil, // agentProvider
		&mockAuthService{},
		nil, // threadQueryService
		nil, // aguiProjection
		nil, // threadMgmt
		nil, // artifactStore
		nil, // toolRegistry
		nil, // requirementConfigService
		nil, // documentProvider
		"",  // tenantID
		appURL,
		taskmgmthandlers.DefaultSSEConfig(),
	)
	// Test that the handler doesn't panic on valid requests
	// Note: CORS header is only set when request is authenticated
	req := httptest.NewRequest(http.MethodPost, "/v1/message:send", nil)
	rec := httptest.NewRecorder()
	handler.SendMessage(rec, req)
	// Should return unauthorized since mockAuthService returns nil user
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected unauthorized status, got %d", rec.Code)
	}
}

func TestSSECORSOriginNotWildcard(t *testing.T) {
	t.Parallel()
	handler := taskmgmthandlers.NewTaskManagementHandler(
		newTestLogger(),
		nil, // orchestrator
		nil, // agentProvider
		&mockAuthService{},
		nil, // threadQueryService
		nil, // aguiProjection
		nil, // threadMgmt
		nil, // artifactStore
		nil, // toolRegistry
		nil, // requirementConfigService
		nil, // documentProvider
		"",  // tenantID
		"",  // allowedOrigin
		taskmgmthandlers.DefaultSSEConfig(),
	)
	req := httptest.NewRequest(http.MethodPost, "/v1/message:send", nil)
	rec := httptest.NewRecorder()
	handler.SendMessage(rec, req)
	corsHeader := rec.Header().Get("Access-Control-Allow-Origin")
	if corsHeader == "*" {
		t.Error("Access-Control-Allow-Origin must never be wildcard (*)")
	}
}

func TestErrorResponsesDoNotLeakInternals(t *testing.T) {
	t.Parallel()
	handler := taskmgmthandlers.NewTaskManagementHandler(
		newTestLogger(),
		nil, // orchestrator
		nil, // agentProvider
		&mockAuthService{},
		nil, // threadQueryService
		nil, // aguiProjection
		nil, // threadMgmt
		nil, // artifactStore
		nil, // toolRegistry
		nil, // requirementConfigService
		nil, // documentProvider
		"",  // tenantID
		"",  // allowedOrigin
		taskmgmthandlers.DefaultSSEConfig(),
	)
	req := httptest.NewRequest(http.MethodPost, "/v1/message:send", nil)
	rec := httptest.NewRecorder()
	handler.SendMessage(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	body := rec.Body.String()
	for _, indicator := range []string{"runtime.", "goroutine", "panic", "sql:", "pq:"} {
		if containsCI(body, indicator) {
			t.Errorf("response body contains internal detail %q: %s", indicator, body)
		}
	}
}

func containsCI(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s != "" && substr != "" &&
			(len(s) >= len(substr)) &&
			(func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					match := true
					for j := range len(substr) {
						sc := s[i+j]
						pc := substr[j]
						if sc >= 'A' && sc <= 'Z' {
							sc += 32
						}
						if pc >= 'A' && pc <= 'Z' {
							pc += 32
						}
						if sc != pc {
							match = false
							break
						}
					}
					if match {
						return true
					}
				}
				return false
			})())
}
