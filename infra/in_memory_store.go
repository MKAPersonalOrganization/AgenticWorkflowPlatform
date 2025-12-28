package infra

import (
	"sync"
	"time"

	"school-with-ai/domain"
)

// InMemoryStore manages in-memory thread storage with thread-safe operations
type InMemoryStore struct {
	mu      sync.RWMutex
	threads map[string]*domain.Thread
}

// NewInMemoryStore creates a new in-memory session store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		threads: make(map[string]*domain.Thread),
	}
}

// GetOrCreateThread retrieves an existing thread or creates a new one
func (s *InMemoryStore) GetOrCreateThread(threadID string) *domain.Thread {
	s.mu.Lock()
	defer s.mu.Unlock()

	if thread, exists := s.threads[threadID]; exists {
		return thread
	}

	thread := &domain.Thread{
		ID:        threadID,
		Title:     "New Chat",
		Messages:  []domain.Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.threads[threadID] = thread
	return thread
}

// AddMessage adds a message to a thread
func (s *InMemoryStore) AddMessage(threadID string, msg domain.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if thread, exists := s.threads[threadID]; exists {
		thread.Messages = append(thread.Messages, msg)
		thread.UpdatedAt = time.Now()
	}
}

// GetMessages retrieves all messages from a thread
func (s *InMemoryStore) GetMessages(threadID string) []domain.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if thread, exists := s.threads[threadID]; exists {
		return thread.Messages
	}
	return []domain.Message{}
}

// GetThread retrieves a thread by ID
func (s *InMemoryStore) GetThread(threadID string) (*domain.Thread, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	thread, exists := s.threads[threadID]
	return thread, exists
}

// GetAllThreads retrieves all threads sorted by UpdatedAt (newest first)
func (s *InMemoryStore) GetAllThreads() []domain.Thread {
	s.mu.RLock()
	defer s.mu.RUnlock()

	threads := make([]domain.Thread, 0, len(s.threads))
	for _, thread := range s.threads {
		threads = append(threads, *thread)
	}

	// Sort by UpdatedAt descending (newest first)
	for i := 0; i < len(threads)-1; i++ {
		for j := i + 1; j < len(threads); j++ {
			if threads[i].UpdatedAt.Before(threads[j].UpdatedAt) {
				threads[i], threads[j] = threads[j], threads[i]
			}
		}
	}

	return threads
}

// UpdateThreadTitle updates the title of a thread
func (s *InMemoryStore) UpdateThreadTitle(threadID string, title string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if thread, exists := s.threads[threadID]; exists {
		thread.Title = title
		thread.UpdatedAt = time.Now()
	}
}

// DeleteThread removes a thread from the store
func (s *InMemoryStore) DeleteThread(threadID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.threads[threadID]; exists {
		delete(s.threads, threadID)
		return true
	}
	return false
}
