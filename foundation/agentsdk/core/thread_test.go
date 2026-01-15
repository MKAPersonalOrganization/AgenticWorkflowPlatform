package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInMemoryThreadCreation verifies that in-memory threads can be created correctly.
func TestInMemoryThreadCreation(t *testing.T) {
	t.Parallel()
	t.Run("create new thread", func(t *testing.T) {
		t.Parallel()
		thread := NewInMemoryThread()

		assert.NotEmpty(t, thread.GetID(), "thread should have an ID")

		assert.Empty(t, thread.RetrieveFullConversationHistory(), "new thread should have no messages")
	})

	t.Run("create thread with ID", func(t *testing.T) {
		t.Parallel()
		expectedID := "test-thread-123"
		thread := NewInMemoryThreadWithID(expectedID)

		assert.Equal(t, expectedID, thread.GetID(), "thread should have specified ID")

		assert.Empty(t, thread.RetrieveFullConversationHistory())
	})

	t.Run("create thread with initial messages", func(t *testing.T) {
		t.Parallel()
		messages := []Message{
			{
				ID:        "msg1",
				Role:      "user",
				Content:   "Hello",
				Timestamp: time.Now(),
			},
			{
				ID:        "msg2",
				Role:      "assistant",
				Content:   "Hi there!",
				Timestamp: time.Now(),
			},
		}

		thread, err := NewInMemoryThreadWithMessages(messages)
		require.NoError(t, err)
		require.NotNil(t, thread)

		retrievedMessages := thread.RetrieveFullConversationHistory()
		assert.Len(t, retrievedMessages, 2, "thread should have 2 messages")
		assert.Equal(t, messages[0].Content, retrievedMessages[0].Content)
		assert.Equal(t, messages[1].Content, retrievedMessages[1].Content)
	})
}

// TestThreadMessageManagement verifies message CRUD operations.
func TestThreadMessageManagement(t *testing.T) {
	t.Parallel()
	t.Run("add and retrieve messages", func(t *testing.T) {
		t.Parallel()
		thread := NewInMemoryThread()

		messages := []Message{
			{
				ID:        "msg1",
				Role:      "user",
				Content:   "What is 2+2?",
				Timestamp: time.Now(),
			},
		}

		err := thread.AddMessages(messages)
		require.NoError(t, err)

		retrievedMessages := thread.RetrieveFullConversationHistory()
		assert.Len(t, retrievedMessages, 1)
		assert.Equal(t, "What is 2+2?", retrievedMessages[0].Content)
		assert.Equal(t, "user", retrievedMessages[0].Role)
	})

	t.Run("add multiple message batches", func(t *testing.T) {
		t.Parallel()
		thread := NewInMemoryThread()

		batch1 := []Message{
			{ID: "msg1", Role: "user", Content: "Hello", Timestamp: time.Now()},
		}
		batch2 := []Message{
			{ID: "msg2", Role: "assistant", Content: "Hi", Timestamp: time.Now()},
			{ID: "msg3", Role: "user", Content: "How are you?", Timestamp: time.Now()},
		}

		err := thread.AddMessages(batch1)
		require.NoError(t, err)

		err = thread.AddMessages(batch2)
		require.NoError(t, err)

		messages := thread.RetrieveFullConversationHistory()
		assert.Len(t, messages, 3, "should have all 3 messages")
		assert.Equal(t, "Hello", messages[0].Content)
		assert.Equal(t, "Hi", messages[1].Content)
		assert.Equal(t, "How are you?", messages[2].Content)
	})

	t.Run("messages are immutable copies", func(t *testing.T) {
		t.Parallel()
		thread := NewInMemoryThread()

		original := []Message{
			{ID: "msg1", Role: "user", Content: "Original", Timestamp: time.Now()},
		}

		err := thread.AddMessages(original)
		require.NoError(t, err)

		retrieved1 := thread.RetrieveFullConversationHistory()
		retrieved2 := thread.RetrieveFullConversationHistory()

		// Modify one of the retrieved slices
		retrieved1[0].Content = "Modified"

		// Verify thread still has original content
		current := thread.RetrieveFullConversationHistory()
		assert.Equal(t, "Original", current[0].Content, "thread content should not be affected")
		assert.Equal(t, "Original", retrieved2[0].Content, "other retrieved copy should not be affected")
	})

	t.Run("message store operations", func(t *testing.T) {
		t.Parallel()
		thread := NewInMemoryThread()
		store := thread.GetMessageStore()

		messages := []Message{
			{ID: "msg1", Role: "user", Content: "User message", Timestamp: time.Now()},
			{ID: "msg2", Role: "assistant", Content: "Assistant message", Timestamp: time.Now()},
			{ID: "msg3", Role: "user", Content: "Another user message", Timestamp: time.Now()},
		}

		err := store.AddMessages(messages)
		require.NoError(t, err)

		// Test count
		assert.Equal(t, 3, store.Count())

		// Test filter by role
		userMessages := store.GetMessagesByRole("user")
		assert.Len(t, userMessages, 2, "should have 2 user messages")

		assistantMessages := store.GetMessagesByRole("assistant")
		assert.Len(t, assistantMessages, 1, "should have 1 assistant message")

		// Test clear
		err = store.Clear()
		require.NoError(t, err)
		assert.Equal(t, 0, store.Count(), "store should be empty after clear")
	})
}

// TestThreadIsolation verifies that multiple threads don't interfere with each other.
func TestThreadIsolation(t *testing.T) {
	t.Parallel()
	t.Run("multiple threads are independent", func(t *testing.T) {
		t.Parallel()
		thread1 := NewInMemoryThreadWithID("thread-1")
		thread2 := NewInMemoryThreadWithID("thread-2")

		messages1 := []Message{
			{ID: "msg1", Role: "user", Content: "Thread 1 message", Timestamp: time.Now()},
		}
		messages2 := []Message{
			{ID: "msg2", Role: "user", Content: "Thread 2 message", Timestamp: time.Now()},
		}

		err := thread1.AddMessages(messages1)
		require.NoError(t, err)

		err = thread2.AddMessages(messages2)
		require.NoError(t, err)

		// Verify isolation
		msgs1 := thread1.RetrieveFullConversationHistory()
		msgs2 := thread2.RetrieveFullConversationHistory()

		assert.Len(t, msgs1, 1)
		assert.Len(t, msgs2, 1)
		assert.Equal(t, "Thread 1 message", msgs1[0].Content)
		assert.Equal(t, "Thread 2 message", msgs2[0].Content)
	})

	t.Run("concurrent access to different threads", func(t *testing.T) {
		t.Parallel()
		thread1 := NewInMemoryThread()
		thread2 := NewInMemoryThread()

		done := make(chan bool)

		// Goroutine 1: writes to thread1
		go func() {
			for i := range 100 {
				msg := []Message{
					{
						ID:        fmt.Sprintf("msg-%d", i),
						Role:      "user",
						Content:   fmt.Sprintf("Message %d", i),
						Timestamp: time.Now(),
					},
				}
				_ = thread1.AddMessages(msg)
			}
			done <- true
		}()

		// Goroutine 2: writes to thread2
		go func() {
			for i := range 100 {
				msg := []Message{
					{
						ID:        fmt.Sprintf("msg-%d", i),
						Role:      "assistant",
						Content:   fmt.Sprintf("Response %d", i),
						Timestamp: time.Now(),
					},
				}
				_ = thread2.AddMessages(msg)
			}
			done <- true
		}()

		// Wait for both goroutines
		<-done
		<-done

		// Verify both threads have their messages
		assert.Len(t, thread1.RetrieveFullConversationHistory(), 100)
		assert.Len(t, thread2.RetrieveFullConversationHistory(), 100)

		// Verify content is correct
		msgs1 := thread1.RetrieveFullConversationHistory()
		msgs2 := thread2.RetrieveFullConversationHistory()
		assert.Equal(t, "user", msgs1[0].Role)
		assert.Equal(t, "assistant", msgs2[0].Role)
	})
}

// TestMessageStoreThreadSafety verifies concurrent access to message store.
func TestMessageStoreThreadSafety(t *testing.T) {
	t.Parallel()
	t.Run("concurrent writes and reads", func(t *testing.T) {
		t.Parallel()
		store := NewInMemoryMessageStore()
		done := make(chan bool)

		// Writer goroutine
		go func() {
			for i := range 50 {
				msg := []Message{
					{ID: fmt.Sprintf("msg-%d", i), Role: "user", Content: "Test", Timestamp: time.Now()},
				}
				_ = store.AddMessages(msg)
			}
			done <- true
		}()

		// Reader goroutine
		go func() {
			for range 50 {
				_ = store.GetMessages()
				_ = store.Count()
			}
			done <- true
		}()

		// Wait for both
		<-done
		<-done

		assert.Equal(t, 50, store.Count())
	})
}
