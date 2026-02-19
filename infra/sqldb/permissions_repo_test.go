package sqldb

import (
	"context"
	"os"
	"testing"
	"time"

	"school-with-ai/services/permissions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresPermissionsRepository(t *testing.T) {
	host := os.Getenv("SCHOOL_DB_HOST")
	if host == "" {
		host = "localhost:5435"
	}

	cfg := Config{
		User:         "schoolai",
		Password:     "dev-password",
		Host:         host,
		Name:         "projections",
		DisableTLS:   true,
		MaxIdleConns: 2,
		MaxOpenConns: 2,
	}

	db, err := Open(cfg)
	if err != nil {
		t.Skipf("DB not reachable: %v", err)
	}
	defer db.Close()

	repo := NewPostgresPermissionsRepository(db)
	ctx := context.Background()

	// Clean up
	db.Exec("DELETE FROM agent_tool_permissions")
	db.Exec("DELETE FROM agent_tool_set_permissions")

	// We need tool sets and tools existing due to FK constraints
	db.Exec("INSERT INTO tool_sets (id, name) VALUES ('ts-perm-1', 'Perm Set 1') ON CONFLICT (id) DO NOTHING")
	db.Exec("INSERT INTO tools (id, name, tool_set_id) VALUES ('t-perm-1', 'Perm Tool 1', 'ts-perm-1') ON CONFLICT (id) DO NOTHING")

	t.Run("Tool Set Permissions", func(t *testing.T) {
		userID := "test-user"
		agentID := "agent-1"
		toolSetID := "ts-perm-1"

		perm := &permissions.ToolSetPermission{
			UserID:    userID,
			AgentID:   agentID,
			ToolSetID: toolSetID,
			Enabled:   true,
			Config:    `{"limit": 10}`,
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		}

		err := repo.SaveToolSetPermission(ctx, perm)
		require.NoError(t, err)

		got, err := repo.GetToolSetPermission(ctx, userID, agentID, toolSetID)
		require.NoError(t, err)
		assert.Equal(t, perm.Enabled, got.Enabled)
		assert.Equal(t, perm.Config, got.Config)
		assert.Equal(t, perm.UserID, got.UserID)

		// List
		list, err := repo.ListToolSetPermissions(ctx, userID, agentID)
		require.NoError(t, err)
		assert.NotEmpty(t, list)

		// Delete
		err = repo.DeleteToolSetPermission(ctx, userID, agentID, toolSetID)
		require.NoError(t, err)

		_, err = repo.GetToolSetPermission(ctx, userID, agentID, toolSetID)
		assert.Error(t, err)
	})

	t.Run("Tool Permissions", func(t *testing.T) {
		userID := "test-user"
		agentID := "agent-1"
		toolSetID := "ts-perm-1"
		toolID := "t-perm-1"

		perm := &permissions.Permission{
			UserID:    userID,
			AgentID:   agentID,
			ToolSetID: toolSetID,
			ToolID:    toolID,
			Enabled:   true,
			Config:    `{"timeout": "5s"}`,
			CreatedAt: time.Now().Truncate(time.Microsecond),
			UpdatedAt: time.Now().Truncate(time.Microsecond),
		}

		err := repo.SavePermission(ctx, perm)
		require.NoError(t, err)

		got, err := repo.GetPermission(ctx, userID, agentID, toolSetID, toolID)
		require.NoError(t, err)
		assert.Equal(t, perm.Enabled, got.Enabled)
		assert.Equal(t, perm.Config, got.Config)
		assert.Equal(t, perm.UserID, got.UserID)

		// List
		list, err := repo.ListPermissions(ctx, userID, agentID, toolSetID)
		require.NoError(t, err)
		assert.NotEmpty(t, list)

		// Delete
		err = repo.DeletePermission(ctx, userID, agentID, toolSetID, toolID)
		require.NoError(t, err)

		_, err = repo.GetPermission(ctx, userID, agentID, toolSetID, toolID)
		assert.Error(t, err)
	})
}
