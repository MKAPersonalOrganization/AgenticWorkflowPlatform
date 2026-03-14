package subscriber

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func TestTryAcquireAdvisoryLock_AcquiresOnce(t *testing.T) {
	// Integration test — requires a real PostgreSQL. Set TEST_DATABASE_URL
	// (e.g. "postgres://appuser:dev-password@localhost:5435/projections?sslmode=disable")
	// to run. Skipped otherwise.
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("no TEST_DATABASE_URL set — skipping advisory lock integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First connection acquires the lock.
	conn1, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn1: %v", err)
	}
	defer conn1.Close()

	acquired, err := tryAcquireAdvisoryLock(ctx, conn1, advisoryLockID)
	if err != nil {
		t.Fatalf("lock attempt 1: %v", err)
	}
	if !acquired {
		t.Fatal("expected first lock attempt to succeed")
	}

	// Second connection must NOT acquire the same lock.
	conn2, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn2: %v", err)
	}
	defer conn2.Close()

	acquired2, err := tryAcquireAdvisoryLock(ctx, conn2, advisoryLockID)
	if err != nil {
		t.Fatalf("lock attempt 2: %v", err)
	}
	if acquired2 {
		t.Fatal("expected second lock attempt to fail while first connection holds the lock")
	}

	// After closing the first connection the lock should be released.
	conn1.Close()

	// Small delay to let PG release session resources.
	time.Sleep(100 * time.Millisecond)

	acquired3, err := tryAcquireAdvisoryLock(ctx, conn2, advisoryLockID)
	if err != nil {
		t.Fatalf("lock attempt 3: %v", err)
	}
	if !acquired3 {
		t.Fatal("expected lock to be available after first connection was closed")
	}
}

func TestAdvisoryLockID_IsConstant(t *testing.T) {
	// Guard against accidental changes to the lock ID which would break
	// rolling upgrades (old pod holds lock with old ID, new pod uses new ID
	// → both think they are leader).
	if advisoryLockID != 7_301_924_817 {
		t.Fatalf("advisoryLockID changed — this will break rolling upgrades across pod versions")
	}
}
