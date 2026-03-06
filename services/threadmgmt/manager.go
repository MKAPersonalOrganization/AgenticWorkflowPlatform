package threadmgmt

import (
	"context"
	"errors"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
	"time"
)

// Manager handles thread lifecycle and persistence using event sourcing.
type Manager struct {
	log        *logger.Logger
	threadRepo *domain.ThreadRepository
}

// New creates a new thread management component.
func New(log *logger.Logger, threadRepo *domain.ThreadRepository) *Manager {
	return &Manager{
		log:        log,
		threadRepo: threadRepo,
	}
}

// CreateThread creates a new chat thread.
func (m *Manager) CreateThread(ctx context.Context, threadID, userID, title string) (*domain.ThreadAggregate, *domain.ThreadCreated, error) {
	m.log.Debug(ctx, "CreateThread called", "threadID", threadID, "title", title)

	existingThread, err := m.threadRepo.Load(ctx, threadID)
	if err == nil {
		m.log.Debug(ctx, "Thread already exists, skipping create", "threadID", threadID, "version", existingThread.GetVersion())
		return existingThread, nil, nil
	}
	if !errors.Is(err, domain.ErrThreadNotFound) {
		return nil, nil, fmt.Errorf("failed to check existing thread: %w", err)
	}

	thread, err := domain.OpenThread(threadID, userID, title)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create thread: %w", err)
	}

	err = m.threadRepo.SaveNew(ctx, thread)
	if err != nil {
		if errors.Is(err, domain.ErrConcurrencyConflict) {
			// Reload the thread if it was already created by another request
			t, err := m.threadRepo.Load(ctx, threadID)
			return t, nil, err
		}
		return nil, nil, fmt.Errorf("failed to save new thread: %w", err)
	}

	event := &domain.ThreadCreated{
		ThreadID:  threadID,
		UserID:    userID,
		Title:     title,
		Timestamp: time.Now(),
	}

	return thread, event, nil
}

// AddUserMessage adds a user message to a thread.
// Returns the thread aggregate, the primary UserMessageAdded event, any additional
// side-effect events (e.g. ThreadTitleUpdated from auto-titling), and an error.
func (m *Manager) AddUserMessage(ctx context.Context, threadID, messageID, content, userID string, attachments ...domain.Attachment) (*domain.ThreadAggregate, *domain.UserMessageAdded, []domain.Event, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if errors.Is(err, domain.ErrThreadNotFound) {
		thread, err = domain.OpenThread(threadID, userID, "New Chat")
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create thread: %w", err)
		}
	} else if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load thread: %w", err)
	}

	isNewThread := thread.GetVersion() == 0

	err = thread.AddMessage(
		messageID,
		"user",
		content,
		nil,
		"",
		"",
		"",
		attachments...,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to add user message: %w", err)
	}

	// Track whether we changed the title for event emission
	var sideEffects []domain.Event

	// Auto-titling: if the thread is new or has the default "New Chat" title, update it
	// using the first 50 characters of the user message.
	if isNewThread || thread.Title == "New Chat" {
		newTitle := content
		if len(newTitle) > 50 {
			newTitle = newTitle[:50] + "..."
		}
		if err := thread.ChangeTitle(newTitle); err != nil {
			m.log.Warn(ctx, "failed to auto-generate thread title", "error", err)
		} else {
			sideEffects = append(sideEffects, &domain.ThreadTitleUpdated{
				ThreadID:  threadID,
				Title:     newTitle,
				Timestamp: time.Now(),
			})
		}
	}

	if isNewThread {
		err = m.threadRepo.SaveNew(ctx, thread)
		if err != nil {
			if errors.Is(err, domain.ErrConcurrencyConflict) {
				// Reload if already exists
				t, err := m.threadRepo.Load(ctx, threadID)
				return t, nil, nil, err
			}
			return nil, nil, nil, fmt.Errorf("failed to save new thread: %w", err)
		}
	} else {
		err = m.threadRepo.Save(ctx, thread)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to save thread: %w", err)
		}
	}

	event := &domain.UserMessageAdded{
		ThreadID:    threadID,
		MessageID:   messageID,
		Content:     content,
		Attachments: attachments,
		Timestamp:   time.Now(),
	}

	return thread, event, sideEffects, nil
}

// AddAssistantMessage adds an assistant message to the thread.
func (m *Manager) AddAssistantMessage(ctx context.Context, threadID, messageID, content string, toolCalls []domain.ToolCall, agentName string) (*domain.ThreadAggregate, *domain.AssistantMessageAdded, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load thread: %w", err)
	}

	err = thread.AddMessage(
		messageID,
		"assistant",
		content,
		toolCalls,
		"",
		"",
		agentName,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add assistant message: %w", err)
	}

	if err := m.threadRepo.Save(ctx, thread); err != nil {
		return nil, nil, err
	}

	event := &domain.AssistantMessageAdded{
		ThreadID:  threadID,
		MessageID: messageID,
		Content:   content,
		ToolCalls: toolCalls,
		AgentName: agentName,
		Timestamp: time.Now(),
	}

	return thread, event, nil
}

// AddTokenUsage records token usage for a message in the thread.
func (m *Manager) AddTokenUsage(ctx context.Context, event *domain.TokenUsageReported) (*domain.ThreadAggregate, error) {
	return m.retryUpdate(ctx, event.ThreadID, func(t *domain.ThreadAggregate) error {
		return t.RecordUsage(event)
	})
}

// RecordToolEvent records a tool lifecycle event in the thread.
func (m *Manager) RecordToolEvent(ctx context.Context, threadID string, event domain.Event) (*domain.ThreadAggregate, error) {
	return m.retryUpdate(ctx, threadID, func(t *domain.ThreadAggregate) error {
		return t.RecordToolEvent(event)
	})
}

// AddToolResultMessage adds a tool result message to the thread.
func (m *Manager) AddToolResultMessage(ctx context.Context, threadID, messageID, toolCallID, toolName, result, agentName string) (*domain.ThreadAggregate, *domain.ToolResultAdded, error) {
	thread, err := m.retryUpdate(ctx, threadID, func(t *domain.ThreadAggregate) error {
		return t.AddMessage(
			messageID,
			"tool",
			result,
			nil,
			toolCallID,
			toolName,
			agentName,
		)
	})
	if err != nil {
		return nil, nil, err
	}

	event := &domain.ToolResultAdded{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Result:     result,
		AgentName:  agentName,
		Timestamp:  time.Now(),
	}

	return thread, event, nil
}

// UpdateThreadTitle updates the title of a thread.
func (m *Manager) UpdateThreadTitle(ctx context.Context, threadID, newTitle string) (*domain.ThreadTitleUpdated, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to load thread: %w", err)
	}

	err = thread.ChangeTitle(newTitle)
	if err != nil {
		return nil, fmt.Errorf("failed to change title: %w", err)
	}

	err = m.threadRepo.Save(ctx, thread)
	if err != nil {
		return nil, fmt.Errorf("failed to save thread: %w", err)
	}

	event := &domain.ThreadTitleUpdated{
		ThreadID:  threadID,
		Title:     newTitle,
		Timestamp: time.Now(),
	}

	return event, nil
}

// DeleteThread deletes a thread and returns the deletion event.
func (m *Manager) DeleteThread(ctx context.Context, threadID string) (*domain.ThreadDeleted, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to load thread: %w", err)
	}

	if err := thread.Delete(); err != nil {
		return nil, fmt.Errorf("failed to delete aggregate: %w", err)
	}

	if err := m.threadRepo.Save(ctx, thread); err != nil {
		return nil, fmt.Errorf("failed to save thread: %w", err)
	}

	event := &domain.ThreadDeleted{
		ThreadID:  threadID,
		Timestamp: time.Now(),
	}

	return event, nil
}

// GetPendingInputToolCall returns the ID of a tool call awaiting user input.
func (m *Manager) GetPendingInputToolCall(ctx context.Context, threadID string) (string, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return "", err
	}

	for id, info := range thread.ActiveToolStates {
		if info.Status == "AwaitingInput" {
			return id, nil
		}
	}
	return "", nil
}

// GetPendingAuthToolCall returns the ID of a tool call awaiting authorization.
func (m *Manager) GetPendingAuthToolCall(ctx context.Context, threadID string) (string, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return "", err
	}

	for id, info := range thread.ActiveToolStates {
		if info.Status == "AwaitingAuth" {
			return id, nil
		}
	}
	return "", nil
}

// Load loads an existing thread aggregate from the repository.
// Returns domain.ErrThreadNotFound if the thread doesn't exist.
func (m *Manager) Load(ctx context.Context, threadID string) (*domain.ThreadAggregate, error) {
	agg, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return nil, err
	}
	return agg, nil
}

func (m *Manager) retryUpdate(ctx context.Context, threadID string, update func(*domain.ThreadAggregate) error) (*domain.ThreadAggregate, error) {
	const maxRetries = 10
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		// Check for context cancellation before each attempt
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		thread, err := m.threadRepo.Load(ctx, threadID)
		if err != nil {
			return nil, fmt.Errorf("failed to load thread: %w", err)
		}

		if err := update(thread); err != nil {
			return nil, err
		}

		err = m.threadRepo.Save(ctx, thread)
		if err == nil {
			return thread, nil
		}

		if !errors.Is(err, domain.ErrConcurrencyConflict) {
			return nil, fmt.Errorf("failed to save thread: %w", err)
		}

		lastErr = err
		// Exponential backoff with true random jitter to avoid thundering herd
		backoff := time.Duration(10*(i+1)) * time.Millisecond
		jitter := time.Duration(time.Now().UnixNano() % int64(10*time.Millisecond))
		wait := backoff + jitter
		m.log.Debug(ctx, "Concurrency conflict during update, retrying",
			"threadID", threadID,
			"attempt", i+1,
			"wait", wait,
		)
		// Use context-aware sleep so shutdown/cancellation interrupts immediately
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}

	return nil, fmt.Errorf("exceeded max retries (%d) after concurrency conflict: %w", maxRetries, lastErr)
}
