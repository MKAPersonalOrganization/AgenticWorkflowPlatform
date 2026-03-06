//go:build integration
// +build integration

package tasks

import (
	"context"
	"os"
	"testing"
	"time"

	"school-with-ai/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Integration_ListTasks(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	resp, err := service.List(ctx, tCtx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.NotNil(t, resp.Tasks)
	assert.NotNil(t, resp.Pagination)
	assert.NotNil(t, resp.Statistics)

	assert.GreaterOrEqual(t, resp.Pagination.Total, uint64(0))
	assert.GreaterOrEqual(t, resp.Pagination.Page, uint32(1))
	assert.GreaterOrEqual(t, resp.Pagination.PageSize, uint32(1))

	totalStats := resp.Statistics.Pending + resp.Statistics.Processing +
		resp.Statistics.Indexed + resp.Statistics.Failed + resp.Statistics.Cancelled
	assert.Equal(t, resp.Pagination.Total, totalStats, "Statistics should sum to total")
}

func TestService_Integration_ListTasksWithPagination(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	page := uint32(1)
	pageSize := uint32(5)
	params := &ListTasksParams{
		Page:     &page,
		PageSize: &pageSize,
	}

	resp, err := service.List(ctx, tCtx, params)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, page, resp.Pagination.Page)
	assert.Equal(t, pageSize, resp.Pagination.PageSize)
	assert.LessOrEqual(t, len(resp.Tasks), int(pageSize))
}

func TestService_Integration_ListTasksWithStatusFilter(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	status := "pending"
	params := &ListTasksParams{
		Status: &status,
	}

	resp, err := service.List(ctx, tCtx, params)
	require.NoError(t, err)
	require.NotNil(t, resp)

	for _, task := range resp.Tasks {
		assert.Equal(t, status, task.Status, "Task should have filtered status")
	}

	if len(resp.Tasks) > 0 {
		assert.Equal(t, resp.Pagination.Total, resp.Statistics.Pending)
	}
}

func TestService_Integration_ListTasksWithTaskTypeFilter(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	taskType := "upload"
	params := &ListTasksParams{
		TaskType: &taskType,
	}

	resp, err := service.List(ctx, tCtx, params)
	require.NoError(t, err)
	require.NotNil(t, resp)

	for _, task := range resp.Tasks {
		assert.Equal(t, taskType, task.TaskType, "Task should have filtered task type")
	}
}

func TestService_Integration_ListTasksWithSorting(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	sort := "created_at"
	order := "desc"
	params := &ListTasksParams{
		Sort:  &sort,
		Order: &order,
	}

	resp, err := service.List(ctx, tCtx, params)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if len(resp.Tasks) > 1 {
		for i := 0; i < len(resp.Tasks)-1; i++ {
			t1, err1 := time.Parse(time.RFC3339, resp.Tasks[i].CreatedAt)
			t2, err2 := time.Parse(time.RFC3339, resp.Tasks[i+1].CreatedAt)

			if err1 == nil && err2 == nil {
				assert.GreaterOrEqual(t, t1.Unix(), t2.Unix(),
					"Tasks should be sorted by created_at descending")
			}
		}
	}
}

func TestService_Integration_ListTasksWithMultipleFilters(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	status := "indexed"
	taskType := "upload"
	page := uint32(1)
	pageSize := uint32(50)
	sort := "updated_at"
	order := "asc"

	params := &ListTasksParams{
		Status:   &status,
		TaskType: &taskType,
		Page:     &page,
		PageSize: &pageSize,
		Sort:     &sort,
		Order:    &order,
	}

	resp, err := service.List(ctx, tCtx, params)
	require.NoError(t, err)
	require.NotNil(t, resp)

	for _, task := range resp.Tasks {
		assert.Equal(t, status, task.Status)
		assert.Equal(t, taskType, task.TaskType)
	}
}

func TestService_Integration_GetNonExistentTask(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	nonExistentTrackID := "nonexistent-track-id-" + uuid.New().String()
	_, err := service.Get(ctx, tCtx, nonExistentTrackID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestService_Integration_CancelNonExistentTask(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	nonExistentTrackID := "nonexistent-task-" + uuid.New().String()
	_, err := service.Cancel(ctx, tCtx, nonExistentTrackID)

	assert.Error(t, err)
}

func TestService_Integration_RetryNonExistentTask(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	nonExistentTrackID := "nonexistent-task-" + uuid.New().String()
	_, err := service.Retry(ctx, tCtx, nonExistentTrackID)

	assert.Error(t, err)
}

func TestService_Integration_TaskResponseFields(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	resp, err := service.List(ctx, tCtx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	if len(resp.Tasks) > 0 {
		task := resp.Tasks[0]

		assert.NotEmpty(t, task.TrackID, "TrackID should not be empty")
		assert.NotEmpty(t, task.TenantID, "TenantID should not be empty")
		assert.NotEmpty(t, task.WorkspaceID, "WorkspaceID should not be empty")
		assert.NotEmpty(t, task.TaskType, "TaskType should not be empty")
		assert.NotEmpty(t, task.Status, "Status should not be empty")
		assert.NotEmpty(t, task.CreatedAt, "CreatedAt should not be empty")
		assert.NotEmpty(t, task.UpdatedAt, "UpdatedAt should not be empty")

		assert.GreaterOrEqual(t, task.RetryCount, int32(0))
		assert.GreaterOrEqual(t, task.MaxRetries, int32(0))

		_, err := time.Parse(time.RFC3339, task.CreatedAt)
		assert.NoError(t, err, "CreatedAt should be valid RFC3339")

		_, err = time.Parse(time.RFC3339, task.UpdatedAt)
		assert.NoError(t, err, "UpdatedAt should be valid RFC3339")
	}
}

func TestService_Integration_StatisticsAccuracy(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	resp, err := service.List(ctx, tCtx, nil)
	require.NoError(t, err)
	require.NotNil(t, resp)

	statusCounts := map[string]int{}
	for _, task := range resp.Tasks {
		statusCounts[task.Status]++
	}

	if len(resp.Tasks) == int(resp.Pagination.Total) {
		assert.Equal(t, uint64(statusCounts["pending"]), resp.Statistics.Pending)
		assert.Equal(t, uint64(statusCounts["processing"]), resp.Statistics.Processing)
		assert.Equal(t, uint64(statusCounts["indexed"]), resp.Statistics.Indexed)
		assert.Equal(t, uint64(statusCounts["failed"]), resp.Statistics.Failed)
		assert.Equal(t, uint64(statusCounts["cancelled"]), resp.Statistics.Cancelled)
	}
}

func TestService_Integration_PaginationConsistency(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	page1 := uint32(1)
	pageSize := uint32(10)
	params1 := &ListTasksParams{
		Page:     &page1,
		PageSize: &pageSize,
	}

	resp1, err := service.List(ctx, tCtx, params1)
	require.NoError(t, err)

	if resp1.Pagination.TotalPages > 1 {
		page2 := uint32(2)
		params2 := &ListTasksParams{
			Page:     &page2,
			PageSize: &pageSize,
		}

		resp2, err := service.List(ctx, tCtx, params2)
		require.NoError(t, err)

		assert.Equal(t, resp1.Pagination.Total, resp2.Pagination.Total)
		assert.Equal(t, resp1.Statistics, resp2.Statistics)

		if len(resp1.Tasks) > 0 && len(resp2.Tasks) > 0 {
			assert.NotEqual(t, resp1.Tasks[0].TrackID, resp2.Tasks[0].TrackID,
				"Different pages should have different tasks")
		}
	}
}

func TestService_Integration_GetTaskIfExists(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	listResp, err := service.List(ctx, tCtx, nil)
	require.NoError(t, err)

	if len(listResp.Tasks) > 0 {
		trackID := listResp.Tasks[0].TrackID

		task, err := service.Get(ctx, tCtx, trackID)
		require.NoError(t, err)
		require.NotNil(t, task)

		assert.Equal(t, trackID, task.TrackID)
		assert.Equal(t, listResp.Tasks[0].Status, task.Status)
		assert.Equal(t, listResp.Tasks[0].TaskType, task.TaskType)
	}
}

func TestService_Integration_FilterByMultipleStatuses(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	statuses := []string{"pending", "processing", "indexed", "failed", "cancelled"}

	for _, status := range statuses {
		params := &ListTasksParams{
			Status: &status,
		}

		resp, err := service.List(ctx, tCtx, params)
		require.NoError(t, err)

		for _, task := range resp.Tasks {
			assert.Equal(t, status, task.Status,
				"All tasks should have status %s when filtered", status)
		}
	}
}

func TestService_Integration_SortOrderVerification(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()
	tenantID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	workspaceID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.MustParse("00000000-0000-0000-0000-000000000004").String(),
		Role:        "user",
	}

	sort := "created_at"
	orderAsc := "asc"
	paramsAsc := &ListTasksParams{
		Sort:  &sort,
		Order: &orderAsc,
	}

	respAsc, err := service.List(ctx, tCtx, paramsAsc)
	require.NoError(t, err)

	if len(respAsc.Tasks) > 1 {
		for i := 0; i < len(respAsc.Tasks)-1; i++ {
			t1, _ := time.Parse(time.RFC3339, respAsc.Tasks[i].CreatedAt)
			t2, _ := time.Parse(time.RFC3339, respAsc.Tasks[i+1].CreatedAt)
			assert.LessOrEqual(t, t1.Unix(), t2.Unix(),
				"Tasks should be sorted ascending")
		}
	}

	orderDesc := "desc"
	paramsDesc := &ListTasksParams{
		Sort:  &sort,
		Order: &orderDesc,
	}

	respDesc, err := service.List(ctx, tCtx, paramsDesc)
	require.NoError(t, err)

	if len(respDesc.Tasks) > 1 {
		for i := 0; i < len(respDesc.Tasks)-1; i++ {
			t1, _ := time.Parse(time.RFC3339, respDesc.Tasks[i].CreatedAt)
			t2, _ := time.Parse(time.RFC3339, respDesc.Tasks[i+1].CreatedAt)
			assert.GreaterOrEqual(t, t1.Unix(), t2.Unix(),
				"Tasks should be sorted descending")
		}
	}
}

func TestService_Integration_EmptyResultSet(t *testing.T) {
	baseURL := os.Getenv("EDGEQUAKE_BASE_URL")
	if baseURL == "" {
		t.Skip("EDGEQUAKE_BASE_URL not set")
	}

	service := NewService(baseURL, nil)
	ctx := context.Background()

	tenantID := uuid.New()
	workspaceID := uuid.New()
	tCtx := &domain.TenantContext{
		TenantID:    &tenantID,
		WorkspaceID: &workspaceID,
		UserID:      uuid.New().String(),
		Role:        "user",
	}

	resp, err := service.List(ctx, tCtx, nil)

	if err == nil {
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Tasks)
		assert.NotNil(t, resp.Pagination)
		assert.NotNil(t, resp.Statistics)
	}
}
