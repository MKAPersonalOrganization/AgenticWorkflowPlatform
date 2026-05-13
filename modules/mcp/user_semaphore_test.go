package mcp

import (
	"context"
	"sync"
	"testing"
)

func TestUserSemaphore_BasicAcquireRelease(t *testing.T) {
	t.Parallel()

	us := newUserSemaphore(3)

	ctx := context.Background()
	userA := "user-a"

	// Acquire 3 slots — should succeed.
	for i := 0; i < 3; i++ {
		if err := us.Acquire(ctx, userA); err != nil {
			t.Fatalf("acquire %d: unexpected error: %v", i, err)
		}
	}

	if got := us.InUse(userA); got != 3 {
		t.Fatalf("expected 3 in-use, got %d", got)
	}

	// 4th acquire should fail.
	if err := us.Acquire(ctx, userA); err == nil {
		t.Fatal("expected error on 4th acquire, got nil")
	}

	// Release one, then acquire should succeed again.
	us.Release(userA)
	if err := us.Acquire(ctx, userA); err != nil {
		t.Fatalf("acquire after release: unexpected error: %v", err)
	}

	// Release all.
	for i := 0; i < 3; i++ {
		us.Release(userA)
	}

	if got := us.InUse(userA); got != 0 {
		t.Fatalf("expected 0 in-use after full release, got %d", got)
	}

	// Map entry should be cleaned up.
	if got := us.ActiveUsers(); got != 0 {
		t.Fatalf("expected 0 active users, got %d", got)
	}
}

func TestUserSemaphore_IsolatesBetweenUsers(t *testing.T) {
	t.Parallel()

	us := newUserSemaphore(2)

	ctx := context.Background()
	userA := "user-a"
	userB := "user-b"

	// Fill user A's slots.
	for i := 0; i < 2; i++ {
		if err := us.Acquire(ctx, userA); err != nil {
			t.Fatalf("acquire userA %d: %v", i, err)
		}
	}

	// User A is at limit.
	if err := us.Acquire(ctx, userA); err == nil {
		t.Fatal("expected userA to be at limit")
	}

	// User B should be unaffected.
	for i := 0; i < 2; i++ {
		if err := us.Acquire(ctx, userB); err != nil {
			t.Fatalf("acquire userB %d: %v", i, err)
		}
	}

	if got := us.ActiveUsers(); got != 2 {
		t.Fatalf("expected 2 active users, got %d", got)
	}

	// Clean up.
	us.Release(userA)
	us.Release(userA)
	us.Release(userB)
	us.Release(userB)
}

func TestUserSemaphore_ConcurrentSafety(t *testing.T) {
	t.Parallel()

	us := newUserSemaphore(5)
	ctx := context.Background()
	userID := "user-concurrent"

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Launch 20 goroutines, each trying to acquire and release.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := us.Acquire(ctx, userID); err != nil {
				errors <- err
				return
			}
			// Simulate work.
			us.Release(userID)
		}()
	}

	wg.Wait()
	close(errors)

	// Some goroutines may have hit the limit, that's expected.
	// The key property: no panics, no data races, final count is 0.
	if got := us.InUse(userID); got != 0 {
		t.Fatalf("expected 0 in-use after all goroutines complete, got %d", got)
	}
}
