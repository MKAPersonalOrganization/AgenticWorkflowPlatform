package session

import (
	"sync"
	"time"
)

// Store manages in-memory thread storage with thread-safe operations
type Store struct {
	mu      sync.RWMutex
	threads map[string]*Thread
}

// NewStore creates a new in-memory session store
func NewStore() *Store {
	return &Store{
		threads: make(map[string]*Thread),
	}
}

// GetOrCreateThread retrieves an existing thread or creates a new one
func (s *Store) GetOrCreateThread(threadID string) *Thread {
	s.mu.Lock()
	defer s.mu.Unlock()

	if thread, exists := s.threads[threadID]; exists {
		return thread
	}

	thread := &Thread{
		ID:        threadID,
		Title:     "New Chat",
		Messages:  []Message{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.threads[threadID] = thread
	return thread
}

// AddMessage adds a message to a thread
func (s *Store) AddMessage(threadID string, msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if thread, exists := s.threads[threadID]; exists {
		thread.Messages = append(thread.Messages, msg)
		thread.UpdatedAt = time.Now()
	}
}

// GetMessages retrieves all messages from a thread
func (s *Store) GetMessages(threadID string) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if thread, exists := s.threads[threadID]; exists {
		return thread.Messages
	}
	return []Message{}
}

// GetThread retrieves a thread by ID
func (s *Store) GetThread(threadID string) (*Thread, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	thread, exists := s.threads[threadID]
	return thread, exists
}

// GetAllThreads retrieves all threads sorted by UpdatedAt (newest first)
func (s *Store) GetAllThreads() []Thread {
	s.mu.RLock()
	defer s.mu.RUnlock()

	threads := make([]Thread, 0, len(s.threads))
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
func (s *Store) UpdateThreadTitle(threadID string, title string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if thread, exists := s.threads[threadID]; exists {
		thread.Title = title
		thread.UpdatedAt = time.Now()
	}
}

// DeleteThread removes a thread from the store
func (s *Store) DeleteThread(threadID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.threads[threadID]; exists {
		delete(s.threads, threadID)
		return true
	}
	return false
}
