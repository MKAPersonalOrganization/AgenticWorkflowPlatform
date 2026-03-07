package workspace

import (
	"context"
	"os"
	"testing"
	"time"

	"school-with-ai/infra/sqldb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresWorkspacePermissionsRepository(t *testing.T) {
	host := os.Getenv("SCHOOL_DB_HOST")
	if host == "" {
		host = "localhost:5435"
	}

	password := os.Getenv("SCHOOL_DB_PASSWORD")
	if password == "" {
		password = "dev-password"
	}

	cfg := sqldb.Config{
		User:         "schoolai",
		Password:     password,
		Host:         host,
		Name:         "projections",
		DisableTLS:   true,
		MaxIdleConns: 2,
		MaxOpenConns: 2,
	}

	db, err := sqldb.Open(cfg)
	if err != nil {
		t.Skipf("DB connection failed, skipping integration test: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("DB not reachable, skipping integration test: %v", err)
	}

	repo := NewPostgresWorkspacePermissionsRepository(db)
	ctx := context.Background()

	// Clean up
	db.Exec("DELETE FROM agent_workspace_permissions")

	t.Run("Workspace Permissions", func(t *testing.T) {
		userID := "test-user"
		agentID := "agent-1"
		workspaceID := "ws-1"

		perm := &WorkspacePermission{
			UserID:      userID,
			AgentID:     agentID,
			WorkspaceID: workspaceID,
			Enabled:     true,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
			UpdatedAt:   time.Now().Truncate(time.Microsecond),
		}

		err := repo.SavePermission(ctx, perm)
		require.NoError(t, err)

		got, err := repo.GetPermission(ctx, userID, agentID, workspaceID)
		require.NoError(t, err)
		assert.Equal(t, perm.Enabled, got.Enabled)
		assert.Equal(t, perm.UserID, got.UserID)

		// List
		list, err := repo.ListPermissionsByAgent(ctx, userID, agentID)
		require.NoError(t, err)
		assert.NotEmpty(t, list)
		assert.Equal(t, 1, len(list))

		// Delete
		err = repo.DeletePermission(ctx, userID, agentID, workspaceID)
		require.NoError(t, err)

		got, err = repo.GetPermission(ctx, userID, agentID, workspaceID)
		assert.Nil(t, got) // Service expects nil if not found
		assert.Nil(t, err)
	})
}
