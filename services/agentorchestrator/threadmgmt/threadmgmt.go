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
	notifier   core.EventNotifier
	agentName  string
}

// New creates a new thread management component.
func New(log *logger.Logger, threadRepo *domain.ThreadRepository, notifier core.EventNotifier, agentName string) *Manager {
	return &Manager{
		log:        log,
		threadRepo: threadRepo,
		notifier:   notifier,
		agentName:  agentName,
	}
}

// CreateThread creates a new chat thread.
func (m *Manager) CreateThread(ctx context.Context, threadID, title string) error {
	m.log.Debug(ctx, "CreateThread called", "threadID", threadID, "title", title)

	existingThread, err := m.threadRepo.Load(ctx, threadID)
	if err == nil {
		m.log.Debug(ctx, "Thread already exists, skipping create", "threadID", threadID, "version", existingThread.GetVersion())
		return nil
	}
	if !errors.Is(err, domain.ErrThreadNotFound) {
		return fmt.Errorf("failed to check existing thread: %w", err)
	}

	thread, err := domain.OpenThread(threadID, "system", title)
	if err != nil {
		return fmt.Errorf("failed to create thread: %w", err)
	}

	err = m.threadRepo.SaveNew(ctx, thread)
	if err != nil {
		if errors.Is(err, core.ErrConcurrencyConflict) {
			return nil
		}
		return fmt.Errorf("failed to save new thread: %w", err)
	}

	if m.notifier != nil {
		if err := m.notifier.HandleDomainEvent(&core.ThreadCreatedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "ThreadCreatedEvent",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			Title: title,
		}); err != nil {
			m.log.Error(ctx, "failed to emit thread created event", "error", err, "threadID", threadID)
		}
	}

	return nil
}

// AddUserMessage adds a user message to a thread.
func (m *Manager) AddUserMessage(ctx context.Context, threadID, messageID, content string) error {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if errors.Is(err, domain.ErrThreadNotFound) {
		thread, err = domain.OpenThread(threadID, "system", "New Chat")
		if err != nil {
			return fmt.Errorf("failed to create thread: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	isNewThread := thread.GetVersion() == 0

	err = thread.AddMessage(messageID, "user", content, nil, "", "", nil)
	if err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	if isNewThread {
		err = m.threadRepo.SaveNew(ctx, thread)
		if err != nil {
			if errors.Is(err, core.ErrConcurrencyConflict) {
				return nil
			}
			return fmt.Errorf("failed to save new thread: %w", err)
		}
	} else {
		err = m.threadRepo.Save(ctx, thread)
		if err != nil {
			return fmt.Errorf("failed to save thread: %w", err)
		}
	}

	if m.notifier != nil {
		if err := m.notifier.HandleDomainEvent(&core.UserMessageAddedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "UserMessageAddedEvent",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			MessageID: messageID,
			Content:   content,
		}); err != nil {
			m.log.Error(ctx, "failed to emit user message added event", "error", err, "threadID", threadID)
		}
	}

	return nil
}

// AddAssistantMessage adds an assistant message to the thread.
func (m *Manager) AddAssistantMessage(ctx context.Context, threadID, messageID, content string, toolCalls []domain.ToolCall) error {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	err = thread.AddMessage(messageID, "assistant", content, toolCalls, "", m.agentName, nil)
	if err != nil {
		return fmt.Errorf("failed to add assistant message: %w", err)
	}

	return m.threadRepo.Save(ctx, thread)
}

// AddToolResultMessage adds a tool result message to the thread.
func (m *Manager) AddToolResultMessage(ctx context.Context, threadID, messageID, toolCallID, result string) error {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	err = thread.AddMessage(messageID, "tool", result, nil, toolCallID, m.agentName, nil)
	if err != nil {
		return fmt.Errorf("failed to add tool result: %w", err)
	}

	return m.threadRepo.Save(ctx, thread)
}

// UpdateThreadTitle updates the title of a thread.
func (m *Manager) UpdateThreadTitle(ctx context.Context, threadID, newTitle string) error {
	thread, err := m.threadRepo.Load(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	err = thread.ChangeTitle(newTitle)
	if err != nil {
		return fmt.Errorf("failed to change title: %w", err)
	}

	err = m.threadRepo.Save(ctx, thread)
	if err != nil {
		return fmt.Errorf("failed to save thread: %w", err)
	}

	if m.notifier != nil {
		if err := m.notifier.HandleDomainEvent(&core.ThreadTitleUpdatedEvent{
			BaseEvent: core.BaseEvent{
				Type:      "ThreadTitleUpdatedEvent",
				Timestamp: time.Now(),
				ThreadID:  threadID,
			},
			NewTitle: newTitle,
		}); err != nil {
			m.log.Error(ctx, "failed to emit thread title updated event", "error", err, "threadID", threadID)
		}
	}

	return nil
}

// DeleteThread deletes a thread.
func (m *Manager) DeleteThread(ctx context.Context, threadID string) error {
	err := m.threadRepo.Delete(ctx, threadID)
	if err != nil {
		return fmt.Errorf("failed to delete thread: %w", err)
	}
	return nil
}
