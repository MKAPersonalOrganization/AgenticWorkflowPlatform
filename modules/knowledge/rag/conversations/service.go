package conversations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"school-with-ai/shared"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUnauthorized = fmt.Errorf("conversations: unauthorized")
	ErrForbidden    = fmt.Errorf("conversations: access forbidden")
)

// Service handles conversation-related operations against the Edgequake API.
type Service struct {
	baseURL    string
	httpClient *http.Client
}

// NewService creates a new Conversation service.
func NewService(baseURL string, httpClient *http.Client) *Service {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &Service{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// List retrieves a paginated list of conversations.
func (s *Service) List(ctx context.Context, tCtx *shared.TenantContext, params *ListConversationsParams) (*PaginatedConversationsResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/conversations", s.baseURL)

	query := url.Values{}
	if params != nil {
		if params.Cursor != nil {
			query.Set("cursor", *params.Cursor)
		}
		if params.Limit != nil {
			query.Set("limit", fmt.Sprintf("%d", *params.Limit))
		}
		if params.FilterMode != nil {
			query.Set("filter[mode]", *params.FilterMode)
		}
		if params.FilterArchived != nil {
			query.Set("filter[archived]", fmt.Sprintf("%t", *params.FilterArchived))
		}
		if params.FilterPinned != nil {
			query.Set("filter[pinned]", fmt.Sprintf("%t", *params.FilterPinned))
		}
		if params.FilterFolderID != nil {
			query.Set("filter[folder_id]", params.FilterFolderID.String())
		}
		if params.FilterSearch != nil {
			query.Set("filter[search]", *params.FilterSearch)
		}
		if params.Sort != nil {
			query.Set("sort", *params.Sort)
		}
		if params.Order != nil {
			query.Set("order", *params.Order)
		}
	}

	if len(query) > 0 {
		urlStr += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response PaginatedConversationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Create creates a new conversation.
func (s *Service) Create(ctx context.Context, tCtx *shared.TenantContext, req *CreateConversationRequest) (*ConversationResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/conversations", s.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ConversationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Get retrieves a conversation by ID.
func (s *Service) Get(ctx context.Context, tCtx *shared.TenantContext, conversationID uuid.UUID) (*ConversationWithMessagesResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/conversations/%s", s.baseURL, conversationID.String())

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ConversationWithMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Update updates a conversation.
func (s *Service) Update(ctx context.Context, tCtx *shared.TenantContext, conversationID uuid.UUID, req *UpdateConversationRequest) (*ConversationResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/conversations/%s", s.baseURL, conversationID.String())

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response ConversationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// Delete deletes a conversation.
func (s *Service) Delete(ctx context.Context, tCtx *shared.TenantContext, conversationID uuid.UUID) error {
	urlStr := fmt.Sprintf("%s/api/v1/conversations/%s", s.baseURL, conversationID.String())

	req, err := http.NewRequestWithContext(ctx, "DELETE", urlStr, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return s.handleError(resp)
}

// ListMessages retrieves messages for a conversation.
func (s *Service) ListMessages(ctx context.Context, tCtx *shared.TenantContext, conversationID uuid.UUID, params *ListMessagesParams) (*PaginatedMessagesResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/conversations/%s/messages", s.baseURL, conversationID.String())

	query := url.Values{}
	if params != nil {
		if params.Cursor != nil {
			query.Set("cursor", *params.Cursor)
		}
		if params.Limit != nil {
			query.Set("limit", fmt.Sprintf("%d", *params.Limit))
		}
	}

	if len(query) > 0 {
		urlStr += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response PaginatedMessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// CreateMessage creates a new message in a conversation.
func (s *Service) CreateMessage(ctx context.Context, tCtx *shared.TenantContext, conversationID uuid.UUID, req *CreateMessageRequest) (*MessageResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/conversations/%s/messages", s.baseURL, conversationID.String())

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// UpdateMessage updates a message.
func (s *Service) UpdateMessage(ctx context.Context, tCtx *shared.TenantContext, messageID uuid.UUID, req *UpdateMessageRequest) (*MessageResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/messages/%s", s.baseURL, messageID.String())

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "PATCH", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response MessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// DeleteMessage deletes a message.
func (s *Service) DeleteMessage(ctx context.Context, tCtx *shared.TenantContext, messageID uuid.UUID) error {
	urlStr := fmt.Sprintf("%s/api/v1/messages/%s", s.baseURL, messageID.String())

	req, err := http.NewRequestWithContext(ctx, "DELETE", urlStr, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	s.injectHeaders(req, tCtx)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return s.handleError(resp)
}

// BulkDelete deletes multiple conversations.
func (s *Service) BulkDelete(ctx context.Context, tCtx *shared.TenantContext, req *BulkOperationRequest) (*BulkOperationResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/conversations/bulk/delete", s.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response BulkOperationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// BulkArchive archives or unarchives multiple conversations.
func (s *Service) BulkArchive(ctx context.Context, tCtx *shared.TenantContext, req *BulkArchiveRequest) (*BulkOperationResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/conversations/bulk/archive", s.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response BulkOperationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// BulkMove moves multiple conversations to a folder.
func (s *Service) BulkMove(ctx context.Context, tCtx *shared.TenantContext, req *BulkMoveRequest) (*BulkOperationResponse, error) {
	urlStr := fmt.Sprintf("%s/api/v1/conversations/bulk/move", s.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	s.injectHeaders(httpReq, tCtx)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if err := s.handleError(resp); err != nil {
		return nil, err
	}

	var response BulkOperationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

func (s *Service) injectHeaders(req *http.Request, tCtx *shared.TenantContext) {
	if tCtx != nil {
		if tCtx.TenantID != nil {
			req.Header.Set("X-Tenant-ID", tCtx.TenantID.String())
		}
		if tCtx.WorkspaceID != nil {
			req.Header.Set("X-Workspace-ID", tCtx.WorkspaceID.String())
		}
		req.Header.Set("X-User-ID", tCtx.UserID)
		req.Header.Set("X-User-Role", tCtx.Role)
	}
}

func (s *Service) handleError(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return ErrForbidden
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("conversation not found")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}
