package toolregistry

import (
	"context"
	"os"
	"testing"
	"time"

	"school-with-ai/infra/sqldb"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresToolRegistryRepository(t *testing.T) {
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

	repo := NewPostgresToolRegistryRepository(db)
	ctx := context.Background()

	t.Run("ToolSets", func(t *testing.T) {
		ts := &ToolSet{
			ID:          uuid.New().String(),
			Name:        "Test ToolSet",
			Description: "A test tool set",
			Category:    "testing",
			ToolIDs:     []string{},
			CreatedAt:   time.Now().Truncate(time.Microsecond),
			UpdatedAt:   time.Now().Truncate(time.Microsecond),
		}

		err := repo.SaveToolSet(ctx, ts)
		require.NoError(t, err)

		got, err := repo.GetToolSet(ctx, ts.ID)
		require.NoError(t, err)
		assert.Equal(t, ts.Name, got.Name)
		assert.Equal(t, ts.Description, got.Description)
		assert.Equal(t, ts.Category, got.Category)

		list, err := repo.ListToolSets(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, list)

		err = repo.DeleteToolSet(ctx, ts.ID)
		require.NoError(t, err)

		_, err = repo.GetToolSet(ctx, ts.ID)
		assert.Error(t, err)
	})

	t.Run("Tools", func(t *testing.T) {
		tool := &Tool{
			ID:          uuid.New().String(),
			Name:        "Test Tool",
			Description: "A test tool",
			Category:    "testing",
			CreatedAt:   time.Now().Truncate(time.Microsecond),
			UpdatedAt:   time.Now().Truncate(time.Microsecond),
		}

		err := repo.SaveTool(ctx, tool)
		require.NoError(t, err)

		got, err := repo.GetTool(ctx, tool.ID)
		require.NoError(t, err)
		assert.Equal(t, tool.Name, got.Name)
		assert.Equal(t, tool.Description, got.Description)
		assert.Equal(t, tool.Category, got.Category)

		list, err := repo.ListTools(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, list)

		err = repo.DeleteTool(ctx, tool.ID)
		require.NoError(t, err)

		_, err = repo.GetTool(ctx, tool.ID)
		assert.Error(t, err)
	})
}
