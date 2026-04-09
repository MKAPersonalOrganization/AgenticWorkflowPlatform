package injection

import (
	"aicompass/shared"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

const (
	MaxInjectionContentBytes = 100 * 1024
)

// Service provides access to the EdgeQuake Knowledge Injection API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Injection service.
func NewService(baseURL string, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Service{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// PutInjectionRequest for creating/updating a knowledge injection (text mode).
type PutInjectionRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// UpdateInjectionRequest for updating an existing injection entry.
type UpdateInjectionRequest struct {
	Name    *string `json:"name,omitempty"`
	Content *string `json:"content,omitempty"`
}

// PutInjectionResponse for PUT injection (accepted for processing).
type PutInjectionResponse struct {
	InjectionID string `json:"injection_id"`
	WorkspaceID string `json:"workspace_id"`
	Version     uint32 `json:"version"`
	Status      string `json:"status"`
}

// InjectionDetailResponse for GET injection (single entry).
type InjectionDetailResponse struct {
	InjectionID string  `json:"injection_id"`
	Name        string  `json:"name"`
	Content     string  `json:"content"`
	Version     uint32  `json:"version"`
	Status      string  `json:"status"`
	EntityCount uint32  `json:"entity_count"`
	SourceType  string  `json:"source_type"`
	Error       *string `json:"error,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// InjectionSummary for list view.
type InjectionSummary struct {
	InjectionID string  `json:"injection_id"`
	Name        string  `json:"name"`
	Status      string  `json:"status"`
	EntityCount uint32  `json:"entity_count"`
	SourceType  string  `json:"source_type"`
	Error       *string `json:"error,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// ListInjectionsResponse for listing injection entries.
type ListInjectionsResponse struct {
	Items []InjectionSummary `json:"items"`
	Total int                `json:"total"`
}

// DeleteInjectionResponse for DELETE injection.
type DeleteInjectionResponse struct {
	Deleted bool   `json:"deleted"`
	Message string `json:"message"`
}

func (s *Service) doRequest(ctx context.Context, tCtx *shared.TenantContext, method, url string, body io.Reader) (*http.Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if tCtx != nil {
		if tCtx.TenantID != nil {
			httpReq.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			httpReq.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		if tCtx.UserID != "" {
			httpReq.Header.Set("X-User-ID", tCtx.UserID)
		}
		if tCtx.Role != "" {
			httpReq.Header.Set("X-User-Role", tCtx.Role)
		}
	}

	return s.httpClient.Do(httpReq)
}

// PutInjection creates or updates a knowledge injection entry.
func (s *Service) PutInjection(ctx context.Context, tCtx *shared.TenantContext, workspaceID string, req *PutInjectionRequest) (*PutInjectionResponse, error) {
	if len(req.Content) > MaxInjectionContentBytes {
		return nil, fmt.Errorf("content exceeds %d byte limit", MaxInjectionContentBytes)
	}

	url := fmt.Sprintf("%s/api/v1/workspaces/%s/injection", s.baseURL, workspaceID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := s.doRequest(ctx, tCtx, "PUT", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result PutInjectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// PutInjectionFile creates a knowledge injection from a file.
func (s *Service) PutInjectionFile(ctx context.Context, tCtx *shared.TenantContext, workspaceID, name, filename string, content []byte) (*PutInjectionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/injection/file", s.baseURL, workspaceID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormField("name")
	if err != nil {
		return nil, fmt.Errorf("failed to create form field: %w", err)
	}
	if _, err := part.Write([]byte(name)); err != nil {
		return nil, err
	}

	part, err = writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PUT", url, body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	if tCtx != nil {
		if tCtx.TenantID != nil {
			httpReq.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			httpReq.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		if tCtx.UserID != "" {
			httpReq.Header.Set("X-User-ID", tCtx.UserID)
		}
		if tCtx.Role != "" {
			httpReq.Header.Set("X-User-Role", tCtx.Role)
		}
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result PutInjectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListInjections returns all injection entries for a workspace.
func (s *Service) ListInjections(ctx context.Context, tCtx *shared.TenantContext, workspaceID string) (*ListInjectionsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/injections", s.baseURL, workspaceID)

	resp, err := s.doRequest(ctx, tCtx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ListInjectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetInjection returns a single injection entry by ID.
func (s *Service) GetInjection(ctx context.Context, tCtx *shared.TenantContext, workspaceID, injectionID string) (*InjectionDetailResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/injections/%s", s.baseURL, workspaceID, injectionID)

	resp, err := s.doRequest(ctx, tCtx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result InjectionDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// UpdateInjection updates an existing injection entry.
func (s *Service) UpdateInjection(ctx context.Context, tCtx *shared.TenantContext, workspaceID, injectionID string, req *UpdateInjectionRequest) (*InjectionDetailResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/injections/%s", s.baseURL, workspaceID, injectionID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := s.doRequest(ctx, tCtx, "PATCH", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result InjectionDetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// DeleteInjection removes an injection entry.
func (s *Service) DeleteInjection(ctx context.Context, tCtx *shared.TenantContext, workspaceID, injectionID string) (*DeleteInjectionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/workspaces/%s/injections/%s", s.baseURL, workspaceID, injectionID)

	resp, err := s.doRequest(ctx, tCtx, "DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result DeleteInjectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
