# Threads Pagination Implementation Plan

Implement cursor-based pagination for `GET /api/agui/threads` using `updated_at` as the cursor.

## Current State
- `ListThreads()` returns all thread IDs as `[]string`
- No pagination support
- Frontend fetches all threads at once

## Target State
- `ListThreads(cursor, limit)` returns paginated results with next cursor
- Default limit: 20 threads per page
- Cursor: RFC3339 timestamp of last item's `updated_at`

---

## Step 1: Define Pagination Types

**File:** `infra/projection/agui/agui_projection.go`

**Changes:**
- Add `ThreadListRequest` struct with `Cursor` and `Limit` fields
- Add `ThreadListResponse` struct with `Threads`, `NextCursor`, and `HasMore` fields
- Add `ThreadSummary` struct (ID, Title, UpdatedAt) for list items

**Expectations:**
- Types compile without errors
- Types are exported and documented

**Test:**
```bash
go build ./infra/projection/agui/...
```

---

## Step 2: Update Interface

**File:** `infra/projection/agui/agui_projection.go`

**Changes:**
- Change `ListThreads() []string` to `ListThreads(req ThreadListRequest) (*ThreadListResponse, error)`

**Expectations:**
- Interface updated
- Build fails (implementation doesn't match) - this is expected

**Test:**
```bash
go build ./... 2>&1 | grep "does not implement"
# Should show PostgresAguiProjection doesn't implement AguiProjection
```

---

## Step 3: Update PostgreSQL Implementation

**File:** `infra/projection/agui/postgres_projection.go`

**Changes:**
- Update `ListThreads` method signature
- Add SQL query with cursor filtering and LIMIT
- Return `ThreadListResponse` with threads and next cursor
- Handle edge cases: empty cursor (first page), no more results

**SQL Query:**
```sql
SELECT thread_id, title, updated_at
FROM agui_threads
WHERE user_id = $1
  AND is_deleted = false
  AND ($2 = '' OR updated_at < $2::timestamp)
ORDER BY updated_at DESC
LIMIT $3 + 1  -- fetch one extra to determine hasMore
```

**Expectations:**
- Method compiles and matches interface
- Returns correct page size
- Returns correct `nextCursor` (updated_at of last item)
- Returns `hasMore = true` when more pages exist

**Test:**
```bash
go build ./infra/projection/agui/...
go test ./infra/projection/agui/... -v
```

---

## Step 4: Update Handler

**File:** `handlers/agui/handlers.go`

**Changes:**
- Update `ListThreads` handler to parse `cursor` and `limit` query params
- Default limit to 20, max limit to 100
- Return JSON response with `threads`, `nextCursor`, `hasMore`

**Request:** `GET /api/agui/threads?cursor=2026-01-19T10:30:00Z&limit=20`

**Response:**
```json
{
  "threads": [
    {"id": "abc", "title": "Math Help", "updatedAt": "2026-01-19T10:00:00Z"},
    ...
  ],
  "nextCursor": "2026-01-18T15:30:00Z",
  "hasMore": true
}
```

**Expectations:**
- Handler parses query params correctly
- Returns 400 for invalid cursor format
- Returns paginated response
- Works without cursor (first page)

**Test:**
```bash
go build ./handlers/agui/...
# Manual test with curl after running server
```

---

## Step 5: Update Frontend

**File:** `assets/js/chat/chat-sessions.js`

**Changes:**
- Store `nextCursor`, `hasMore`, and `isLoadingMore` in state
- Add scroll event listener on sidebar chat history container
- Detect when user scrolls near bottom (e.g., 100px from end)
- Call `fetchMoreThreads(cursor)` when threshold reached
- Append new threads to existing list (don't replace)
- Debounce/guard to prevent multiple simultaneous fetches

**Implementation:**
```javascript
// In Alpine component
__onSidebarScroll(event) {
    const el = event.target;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100;

    if (nearBottom && this.hasMore && !this.isLoadingMore) {
        this.__loadMoreThreads();
    }
}
```

**Expectations:**
- Initial load fetches first page (20 threads)
- Scrolling to bottom auto-fetches next page
- Loading state prevents duplicate requests
- Stops fetching when `hasMore = false`
- No duplicate threads in list

**Test:**
- Manual: Open app, verify first 20 threads load
- Manual: Scroll down in sidebar, verify next page loads automatically
- Manual: Verify loading stops when all threads loaded

---

## Step 6: Integration Test

**File:** `handlers/agui/handlers_test.go` (new or existing)

**Changes:**
- Add test for pagination with multiple pages
- Test edge cases: empty results, exactly one page, cursor at end

**Test Cases:**
1. First page returns correct limit
2. Cursor returns next page (no overlap)
3. Empty cursor returns first page
4. Invalid cursor returns 400
5. Last page has `hasMore = false`

**Test:**
```bash
go test ./handlers/agui/... -v -run TestListThreadsPagination
```

---

## Summary

| Step | File(s) | Verification |
|------|---------|--------------|
| 1 | agui_projection.go | `go build` |
| 2 | agui_projection.go | `go build` (expect fail) |
| 3 | postgres_projection.go | `go build && go test` |
| 4 | handlers.go | `go build` + curl |
| 5 | chat-sessions.js | Manual browser test |
| 6 | handlers_test.go | `go test` |
