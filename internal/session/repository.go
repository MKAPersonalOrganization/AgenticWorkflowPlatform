package session

// Repository defines the interface for thread and message storage operations
type Repository interface {
	// GetOrCreateThread retrieves an existing thread or creates a new one
	GetOrCreateThread(threadID string) *Thread

	// AddMessage adds a message to a thread
	AddMessage(threadID string, msg Message)

	// GetMessages retrieves all messages from a thread
	GetMessages(threadID string) []Message

	// GetThread retrieves a thread by ID
	GetThread(threadID string) (*Thread, bool)

	// GetAllThreads retrieves all threads sorted by UpdatedAt (newest first)
	GetAllThreads() []Thread

	// UpdateThreadTitle updates the title of a thread
	UpdateThreadTitle(threadID string, title string)

	// DeleteThread removes a thread from the store
	DeleteThread(threadID string) bool
}
