package threadmgmt

import (
	"context"
	"errors"
	"fmt"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
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
func (m *Manager) AddUserMessage(ctx context.Context, threadID, messageID, content string) (*domain.ThreadAggregate, *domain.MessageAdded, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if errors.Is(err, domain.ErrThreadNotFound) {
		thread, err = domain.OpenThread(threadID, "system", "New Chat")
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

	event := &domain.MessageAdded{
		ThreadID:  threadID,
		MessageID: messageID,
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}

	return thread, event, nil
}

// AddAssistantMessage adds an assistant message to the thread.
func (m *Manager) AddAssistantMessage(ctx context.Context, threadID, messageID, content string, toolCalls []domain.ToolCall, agentName string) (*domain.ThreadAggregate, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to load thread: %w", err)
	}

	err = thread.AddMessage(messageID, "assistant", content, toolCalls, "", agentName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to add assistant message: %w", err)
	}

	if err := m.threadRepo.Save(ctx, thread); err != nil {
		return nil, err
	}

	return thread, nil
}

// AddToolResultMessage adds a tool result message to the thread.
func (m *Manager) AddToolResultMessage(ctx context.Context, threadID, messageID, toolCallID, result, agentName string) (*domain.ThreadAggregate, error) {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to load thread: %w", err)
	}

	err = thread.AddMessage(messageID, "tool", result, nil, toolCallID, agentName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to add tool result: %w", err)
	}

	if err := m.threadRepo.Save(ctx, thread); err != nil {
		return nil, err
	}

	return thread, nil
}

// UpdateThreadTitle updates the title of a thread.
func (m *Manager) UpdateThreadTitle(ctx context.Context, threadID, newTitle string) (*domain.ThreadTitleUpdatedEvent, error) {
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

	event := &domain.ThreadTitleUpdatedEvent{
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

// ToAgentThread converts a ThreadAggregate to a core.AgentThread.
func (m *Manager) ToAgentThread(aggregate *domain.ThreadAggregate) core.AgentThread {
	agentThread := core.NewInMemoryThreadWithID(aggregate.ID)
	if len(aggregate.Messages) > 0 {
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
		}
		_ = agentThread.AddMessages(messages)
	}
	return agentThread
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
