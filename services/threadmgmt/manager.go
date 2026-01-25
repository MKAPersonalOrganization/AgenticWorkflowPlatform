package threadmgmt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
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
func (m *Manager) CreateThread(ctx context.Context, threadID, title string) (*domain.ThreadAggregate, *domain.ThreadCreated, error) {
	m.log.Debug(ctx, "CreateThread called", "threadID", threadID, "title", title)

	existingThread, err := m.threadRepo.Load(ctx, threadID)
	if err == nil {
		m.log.Debug(ctx, "Thread already exists, skipping create", "threadID", threadID, "version", existingThread.GetVersion())
		return existingThread, nil, nil
	}
	if !errors.Is(err, domain.ErrThreadNotFound) {
		return nil, nil, fmt.Errorf("failed to check existing thread: %w", err)
	}

	thread, err := domain.OpenThread(threadID, "system", title)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create thread: %w", err)
	}

	err = m.threadRepo.SaveNew(ctx, thread)
	if err != nil {
		if errors.Is(err, core.ErrConcurrencyConflict) {
			// Reload the thread if it was already created by another request
			t, err := m.threadRepo.Load(ctx, threadID)
			return t, nil, err
		}
		return nil, nil, fmt.Errorf("failed to save new thread: %w", err)
	}

	event := &domain.ThreadCreated{
		ThreadID:  threadID,
		UserID:    "system", // Assuming system for now
		Title:     title,
		Timestamp: time.Now(),
	}

	return thread, event, nil
}

// AddUserMessage adds a user message to a thread.
func (m *Manager) AddUserMessage(ctx context.Context, threadID, messageID, content, userID string) (*domain.ThreadAggregate, *domain.UserMessageAdded, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if errors.Is(err, domain.ErrThreadNotFound) {
		thread, err = domain.OpenThread(threadID, userID, "New Chat")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create thread: %w", err)
		}
	} else if err != nil {
		return nil, nil, fmt.Errorf("failed to load thread: %w", err)
	}

	isNewThread := thread.GetVersion() == 0

	err = thread.AddMessage(messageID, "user", content, nil, "", "", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add message: %w", err)
	}

	if isNewThread {
		err = m.threadRepo.SaveNew(ctx, thread)
		if err != nil {
			if errors.Is(err, core.ErrConcurrencyConflict) {
				// Reload if already exists
				t, err := m.threadRepo.Load(ctx, threadID)
				return t, nil, err
			}
			return nil, nil, fmt.Errorf("failed to save new thread: %w", err)
		}
	} else {
		err = m.threadRepo.Save(ctx, thread)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to save thread: %w", err)
		}
	}

	event := &domain.UserMessageAdded{
		ThreadID:  threadID,
		MessageID: messageID,
		Content:   content,
		Timestamp: time.Now(),
	}

	return thread, event, nil
}

// AddAssistantMessage adds an assistant message to the thread.
func (m *Manager) AddAssistantMessage(ctx context.Context, threadID, messageID, content string, toolCalls []domain.ToolCall, agentName string) (*domain.ThreadAggregate, *domain.AssistantMessageAdded, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load thread: %w", err)
	}

	err = thread.AddMessage(messageID, "assistant", content, toolCalls, "", agentName, nil)
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
func (m *Manager) AddTokenUsage(ctx context.Context, event domain.TokenUsageReported) (*domain.ThreadAggregate, error) {
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

// RecordEngineStateChange records a change in the engine's high-level state.
func (m *Manager) RecordEngineStateChange(ctx context.Context, event domain.EngineStateChanged) (*domain.ThreadAggregate, error) {
	return m.retryUpdate(ctx, event.ThreadID, func(t *domain.ThreadAggregate) error {
		return t.RecordEngineStateChange(event)
	})
}

// AddToolResultMessage adds a tool result message to the thread.
func (m *Manager) AddToolResultMessage(ctx context.Context, threadID, messageID, toolCallID, result, agentName string) (*domain.ThreadAggregate, *domain.ToolResultAdded, error) {
	thread, err := m.retryUpdate(ctx, threadID, func(t *domain.ThreadAggregate) error {
		return t.AddMessage(messageID, "tool", result, nil, toolCallID, agentName, nil)
	})
	if err != nil {
		return nil, nil, err
	}

	event := &domain.ToolResultAdded{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
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

// DeleteThread deletes a thread.
func (m *Manager) DeleteThread(ctx context.Context, threadID string) error {
	err := m.threadRepo.Delete(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to delete thread: %w", err)
	}
	return nil
}

// GetEngineState retrieves the current engine state for a thread.
func (m *Manager) GetEngineState(ctx context.Context, threadID string) (string, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return "", err
	}
	return thread.EngineState, nil
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

// ToAgentThread converts a ThreadAggregate to a core.AgentThread.
// Load loads an existing thread aggregate from the repository.
// Returns domain.ErrThreadNotFound if the thread doesn't exist.
func (m *Manager) Load(ctx context.Context, threadID string) (*domain.ThreadAggregate, error) {
	return m.threadRepo.Load(ctx, threadID)
}

func (m *Manager) ToAgentThread(aggregate *domain.ThreadAggregate) core.AgentThread {
	agentThread := core.NewInMemoryThreadWithID(aggregate.ID)
	messages := m.ToCoreMessages(aggregate)
	if len(messages) > 0 {
		_ = agentThread.AddMessages(messages)
	}
	return agentThread
}

// ToCoreMessages converts domain messages in an aggregate to core messages.
func (m *Manager) ToCoreMessages(aggregate *domain.ThreadAggregate) []core.Message {
	if len(aggregate.Messages) == 0 {
		return nil
	}
	messages := make([]core.Message, len(aggregate.Messages))
	for i, dm := range aggregate.Messages {
		messages[i] = core.Message{
			ID:         dm.ID,
			Role:       dm.Role,
			Content:    dm.Content,
			Timestamp:  dm.Timestamp,
			ToolCalls:  toCoreToolCalls(dm.ToolCalls),
			ToolCallID: dm.ToolCallID,
			AgentName:  dm.AgentName,
			Metadata:   dm.Metadata,
		}

		// Inject active tool states if present
		if len(dm.ToolCalls) > 0 && len(aggregate.ActiveToolStates) > 0 {
			toolStates := make(map[string]domain.ToolCallState)
			hasActive := false
			for _, tc := range dm.ToolCalls {
				if state, ok := aggregate.ActiveToolStates[tc.ID]; ok {
					toolStates[tc.ID] = state
					hasActive = true
				}
			}

			if hasActive {
				if messages[i].Metadata == nil {
					messages[i].Metadata = make(map[string]any)
				}
				messages[i].Metadata["tool_states"] = toolStates
			}
		}
	}
	return messages
}

func (m *Manager) retryUpdate(ctx context.Context, threadID string, update func(*domain.ThreadAggregate) error) (*domain.ThreadAggregate, error) {
	const maxRetries = 5
	var lastErr error

	for i := 0; i < maxRetries; i++ {
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

		if !errors.Is(err, core.ErrConcurrencyConflict) {
			return nil, fmt.Errorf("failed to save thread: %w", err)
		}

		lastErr = err
		// Wait a tiny bit to avoid tight loops? Or just continue
		m.log.Debug(ctx, "Concurrency conflict during update, retrying", "threadID", threadID, "attempt", i+1)
		time.Sleep(time.Duration(i*10) * time.Millisecond)
	}

	return nil, fmt.Errorf("exceeded max retries (%d) after concurrency conflict: %w", maxRetries, lastErr)
}

func toCoreToolCalls(dtcs []domain.ToolCall) []core.ToolCall {
	if dtcs == nil {
		return nil
	}
	result := make([]core.ToolCall, len(dtcs))
	for i, dtc := range dtcs {
		result[i] = core.ToolCall{
			ID:   dtc.ID,
			Type: dtc.Type,
			Function: core.ToolFunction{
				Name:      dtc.Function.Name,
				Arguments: dtc.Function.Arguments,
			},
		}
	}
	return result
}
