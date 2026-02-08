package sqldb

import (
	"context"
	"os"
	"testing"
	"time"

	"school-with-ai/services/resources"

	"github.com/google/uuid"
)

func TestPostgresResourcesRepository(t *testing.T) {
	// Only run if DB host is set
	host := os.Getenv("SCHOOL_DB_HOST")
	if host == "" {
		host = "localhost:5435" // default from .env
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
		t.Skipf("DB not reachable, skipping integration test: %v", err)
	}
	defer db.Close()

	repo := NewPostgresResourcesRepository(db)
	ctx := context.Background()

	// 1. Test Create
	uid := "user-test-1"
	res := resources.Resource{
		ID:        uuid.New(),
		UserID:    &uid,
		Title:     "Integration Test Doc",
		FileType:  "application/pdf",
		FilePath:  "test/path/doc.pdf",
		Source:    resources.SourcePrivate,
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	if err := repo.Create(ctx, res); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 2. Test List
	list, err := repo.List(ctx, uid, resources.SourcePrivate)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, r := range list {
		if r.ID == res.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Created resource not found in list")
	}

	// 3. Test GetByID
	got, err := repo.GetByID(ctx, res.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Title != res.Title {
		t.Errorf("Expected title %s, got %s", res.Title, got.Title)
	}

	// 4. Test Delete
	if err := repo.Delete(ctx, res.ID, uid); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = repo.GetByID(ctx, res.ID)
	if err == nil {
		t.Errorf("Expected error getting deleted resource, got nil")
	}
}

func TestPostgresResourcesRepository_Public(t *testing.T) {
	host := os.Getenv("SCHOOL_DB_HOST")
	if host == "" {
		host = "localhost:5435"
	}

	cfg := Config{
		User:       "schoolai",
		Password:   "dev-password",
		Host:       host,
		Name:       "projections",
		DisableTLS: true,
	}

	db, err := Open(cfg)
	if err != nil {
		t.Skipf("DB not reachable, skipping integration test: %v", err)
	}
	defer db.Close()

	repo := NewPostgresResourcesRepository(db)
	ctx := context.Background()

	res := resources.Resource{
		ID:        uuid.New(),
		UserID:    nil,
		Title:     "Public Resource Test",
		FileType:  "text/markdown",
		FilePath:  "public/test.md",
		Source:    resources.SourcePublic,
		CreatedAt: time.Now().Truncate(time.Microsecond),
		UpdatedAt: time.Now().Truncate(time.Microsecond),
	}

	if err := repo.Create(ctx, res); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	list, err := repo.List(ctx, "any-user", resources.SourcePublic)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, r := range list {
		if r.ID == res.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Public resource not found in list")
	}

	// Cleanup
	repo.Delete(ctx, res.ID, "any-user")
}
