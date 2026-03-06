package sqldb

import (
	"context"
	"os"
	"school-with-ai/services/toolregistry"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresToolRegistryRepository(t *testing.T) {
	// Only run if DB host is set
	host := os.Getenv("SCHOOL_DB_HOST")
	if host == "" {
		host = "localhost:5435" // default from .env
	}

	password := os.Getenv("SCHOOL_DB_PASSWORD")
	if password == "" {
		password = "dev-password"
	}

	cfg := Config{
		User:         "schoolai",
		Password:     password,
		Host:         host,
		Name:         "projections",
		DisableTLS:   true,
		MaxIdleConns: 2,
		MaxOpenConns: 2,
	}

	db, err := Open(cfg)
	if err != nil {
		t.Skipf("DB connection failed, skipping integration test: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("DB not reachable, skipping integration test: %v", err)
	}

	repo := NewPostgresToolRegistryRepository(db)
	ctx := context.Background()

	// Clean up tables
	db.Exec("DELETE FROM agent_tool_permissions")
	db.Exec("DELETE FROM agent_tool_set_permissions")
	db.Exec("DELETE FROM tools")
	db.Exec("DELETE FROM tool_sets")

	t.Run("Tool Sets", func(t *testing.T) {
		tsID := "ts-" + uuid.New().String()
		ts := &toolregistry.ToolSet{
			ID:          tsID,
			Name:        "Test Tool Set",
			Description: "A tool set for testing",
			Category:    "Testing",
			ToolIDs:     []string{"tool-1", "tool-2"},
			Metadata:    map[string]string{"version": "1.0"},
			CreatedAt:   time.Now().Truncate(time.Microsecond),
			UpdatedAt:   time.Now().Truncate(time.Microsecond),
		}

		// Create
		err := repo.SaveToolSet(ctx, ts)
		require.NoError(t, err)

		// Get
		got, err := repo.GetToolSet(ctx, tsID)
		require.NoError(t, err)
		assert.Equal(t, ts.Name, got.Name)
		assert.ElementsMatch(t, ts.ToolIDs, got.ToolIDs)
		assert.Equal(t, ts.Metadata, got.Metadata)

		// List
		list, err := repo.ListToolSets(ctx)
		require.NoError(t, err)
		found := false
		for _, item := range list {
			if item.ID == tsID {
				found = true
				break
			}
		}
		assert.True(t, found)

		// Update
		ts.Name = "Updated Tool Set"
		err = repo.SaveToolSet(ctx, ts)
		require.NoError(t, err)
		got, err = repo.GetToolSet(ctx, tsID)
		require.NoError(t, err)
		assert.Equal(t, "Updated Tool Set", got.Name)

		// Delete
		err = repo.DeleteToolSet(ctx, tsID)
		require.NoError(t, err)
		_, err = repo.GetToolSet(ctx, tsID)
		assert.Error(t, err)
	})

	t.Run("Tools", func(t *testing.T) {
		// Need a tool set first for FK constraint if we use it, but tools table has nullable FK
		toolID := "tool-" + uuid.New().String()
		tool := &toolregistry.Tool{
			ID:          toolID,
			Name:        "Test Tool",
			Description: "A tool for testing",
			Category:    "Testing",
			Metadata:    map[string]string{"type": "function"},
			CreatedAt:   time.Now().Truncate(time.Microsecond),
			UpdatedAt:   time.Now().Truncate(time.Microsecond),
		}

		// Create
		err := repo.SaveTool(ctx, tool)
		require.NoError(t, err)

		// Get
		got, err := repo.GetTool(ctx, toolID)
		require.NoError(t, err)
		assert.Equal(t, tool.Name, got.Name)
		assert.Equal(t, tool.Metadata, got.Metadata)

		// List
		list, err := repo.ListTools(ctx)
		require.NoError(t, err)
		found := false
		for _, item := range list {
			if item.ID == toolID {
				found = true
				break
			}
		}
		assert.True(t, found)

		// Delete
		err = repo.DeleteTool(ctx, toolID)
		require.NoError(t, err)
		_, err = repo.GetTool(ctx, toolID)
		assert.Error(t, err)
	})
}
