package adkgo

import (
	"aicompass/foundation/auth"
	"context"
	"errors"
	"strings"
	"testing"
)

type stubWorkspacePermissions struct {
	ids []string
	err error
}

func (s *stubWorkspacePermissions) GetEnabledWorkspaces(context.Context, string, string) ([]string, error) {
	return s.ids, s.err
}

func TestKnowledgeBasePromptInjectorBuildPromptAddition(t *testing.T) {
	t.Parallel()

	ctx := auth.SetUserInContext(context.Background(), &auth.User{ID: "u1"})
	injector := NewKnowledgeBasePromptInjector(&stubWorkspacePermissions{ids: []string{"ws-1", "ws-2"}})

	got := injector.BuildPromptAddition(ctx, "agent-1", "")
	if !strings.Contains(got, "ws-1") || !strings.Contains(got, "ws-2") {
		t.Fatalf("prompt addition missing workspace IDs: %q", got)
	}
}

func TestKnowledgeBasePromptInjectorBuildPromptAdditionEmptyCases(t *testing.T) {
	t.Parallel()

	injector := NewKnowledgeBasePromptInjector(&stubWorkspacePermissions{err: errors.New("boom")})
	if got := injector.BuildPromptAddition(context.Background(), "agent-1", ""); got != "" {
		t.Fatalf("expected empty prompt without user, got %q", got)
	}

	ctx := auth.SetUserInContext(context.Background(), &auth.User{ID: "u1"})
	if got := injector.BuildPromptAddition(ctx, "agent-1", ""); got != "" {
		t.Fatalf("expected empty prompt on permission error, got %q", got)
	}
}
