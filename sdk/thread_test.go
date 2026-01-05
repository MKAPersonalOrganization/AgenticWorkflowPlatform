package sdk

import (
	"fmt"
	"testing"
	"time"

	"school-with-ai/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInMemoryThreadCreation verifies that in-memory threads can be created correctly
func TestInMemoryThreadCreation(t *testing.T) {
	t.Run("create new thread", func(t *testing.T) {
		thread := NewInMemoryThread()

		assert.NotEmpty(t, thread.GetID(), "thread should have an ID")
		assert.Equal(t, ThreadTypeLocal, thread.GetType(), "thread type should be local")
		assert.Empty(t, thread.GetMessages(), "new thread should have no messages")
	})

	t.Run("create thread with ID", func(t *testing.T) {
		expectedID := "test-thread-123"
		thread := NewInMemoryThreadWithID(expectedID)

		assert.Equal(t, expectedID, thread.GetID(), "thread should have specified ID")
		assert.Equal(t, ThreadTypeLocal, thread.GetType())
		assert.Empty(t, thread.GetMessages())
	})

	t.Run("create thread with initial messages", func(t *testing.T) {
		messages := []domain.Message{
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

		retrievedMessages := thread.GetMessages()
		assert.Len(t, retrievedMessages, 2, "thread should have 2 messages")
		assert.Equal(t, messages[0].Content, retrievedMessages[0].Content)
		assert.Equal(t, messages[1].Content, retrievedMessages[1].Content)
	})
}

// TestThreadMessageManagement verifies message CRUD operations
func TestThreadMessageManagement(t *testing.T) {
	t.Run("add and retrieve messages", func(t *testing.T) {
		thread := NewInMemoryThread()

		messages := []domain.Message{
			{
				ID:        "msg1",
				Role:      "user",
				Content:   "What is 2+2?",
				Timestamp: time.Now(),
			},
		}

		err := thread.AddMessages(messages)
		require.NoError(t, err)

		retrievedMessages := thread.GetMessages()
		assert.Len(t, retrievedMessages, 1)
		assert.Equal(t, "What is 2+2?", retrievedMessages[0].Content)
		assert.Equal(t, "user", retrievedMessages[0].Role)
	})

	t.Run("add multiple message batches", func(t *testing.T) {
		thread := NewInMemoryThread()

		batch1 := []domain.Message{
			{ID: "msg1", Role: "user", Content: "Hello", Timestamp: time.Now()},
		}
		batch2 := []domain.Message{
			{ID: "msg2", Role: "assistant", Content: "Hi", Timestamp: time.Now()},
			{ID: "msg3", Role: "user", Content: "How are you?", Timestamp: time.Now()},
		}

		err := thread.AddMessages(batch1)
		require.NoError(t, err)

		err = thread.AddMessages(batch2)
		require.NoError(t, err)

		messages := thread.GetMessages()
		assert.Len(t, messages, 3, "should have all 3 messages")
		assert.Equal(t, "Hello", messages[0].Content)
		assert.Equal(t, "Hi", messages[1].Content)
		assert.Equal(t, "How are you?", messages[2].Content)
	})

	t.Run("messages are immutable copies", func(t *testing.T) {
		thread := NewInMemoryThread()

		original := []domain.Message{
			{ID: "msg1", Role: "user", Content: "Original", Timestamp: time.Now()},
		}

		err := thread.AddMessages(original)
		require.NoError(t, err)

		retrieved1 := thread.GetMessages()
		retrieved2 := thread.GetMessages()

		// Modify one of the retrieved slices
		retrieved1[0].Content = "Modified"

		// Verify thread still has original content
		current := thread.GetMessages()
		assert.Equal(t, "Original", current[0].Content, "thread content should not be affected")
		assert.Equal(t, "Original", retrieved2[0].Content, "other retrieved copy should not be affected")
	})

	t.Run("message store operations", func(t *testing.T) {
		thread := NewInMemoryThread()
		store := thread.GetMessageStore()

		messages := []domain.Message{
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

// TestThreadSerialization verifies thread serialization and deserialization
func TestThreadSerialization(t *testing.T) {
	t.Run("serialize and deserialize thread", func(t *testing.T) {
		// Create thread with messages
		originalThread := NewInMemoryThreadWithID("thread-123")
		messages := []domain.Message{
			{
				ID:        "msg1",
				Role:      "user",
				Content:   "Hello, world!",
				Timestamp: time.Now(),
			},
			{
				ID:        "msg2",
				Role:      "assistant",
				Content:   "Hi there!",
				Timestamp: time.Now(),
			},
		}

		err := originalThread.AddMessages(messages)
		require.NoError(t, err)

		// Serialize
		data, err := originalThread.Serialize()
		require.NoError(t, err)
		assert.NotEmpty(t, data, "serialized data should not be empty")

		// Deserialize
		restoredThread, err := NewInMemoryThreadFromState(data)
		require.NoError(t, err)
		require.NotNil(t, restoredThread)

		// Verify restored thread matches original
		assert.Equal(t, originalThread.GetID(), restoredThread.GetID(), "thread IDs should match")
		assert.Equal(t, originalThread.GetType(), restoredThread.GetType(), "thread types should match")

		restoredMessages := restoredThread.GetMessages()
		assert.Len(t, restoredMessages, 2, "should have restored 2 messages")
		assert.Equal(t, "Hello, world!", restoredMessages[0].Content)
		assert.Equal(t, "Hi there!", restoredMessages[1].Content)
	})

	t.Run("serialize empty thread", func(t *testing.T) {
		thread := NewInMemoryThread()

		data, err := thread.Serialize()
		require.NoError(t, err)

		restoredThread, err := NewInMemoryThreadFromState(data)
		require.NoError(t, err)
		assert.Empty(t, restoredThread.GetMessages())
	})

	t.Run("deserialize invalid data", func(t *testing.T) {
		invalidData := []byte("not valid json")

		thread, err := NewInMemoryThreadFromState(invalidData)
		assert.Error(t, err)
		assert.Nil(t, thread)
	})

	t.Run("deserialize empty data", func(t *testing.T) {
		thread, err := NewInMemoryThreadFromState([]byte{})
		assert.Error(t, err)
		assert.Nil(t, thread)
	})
}

// TestThreadIsolation verifies that multiple threads don't interfere with each other
func TestThreadIsolation(t *testing.T) {
	t.Run("multiple threads are independent", func(t *testing.T) {
		thread1 := NewInMemoryThreadWithID("thread-1")
		thread2 := NewInMemoryThreadWithID("thread-2")

		messages1 := []domain.Message{
			{ID: "msg1", Role: "user", Content: "Thread 1 message", Timestamp: time.Now()},
		}
		messages2 := []domain.Message{
			{ID: "msg2", Role: "user", Content: "Thread 2 message", Timestamp: time.Now()},
		}

		err := thread1.AddMessages(messages1)
		require.NoError(t, err)

		err = thread2.AddMessages(messages2)
		require.NoError(t, err)

		// Verify isolation
		msgs1 := thread1.GetMessages()
		msgs2 := thread2.GetMessages()

		assert.Len(t, msgs1, 1)
		assert.Len(t, msgs2, 1)
		assert.Equal(t, "Thread 1 message", msgs1[0].Content)
		assert.Equal(t, "Thread 2 message", msgs2[0].Content)
	})

	t.Run("concurrent access to different threads", func(t *testing.T) {
		thread1 := NewInMemoryThread()
		thread2 := NewInMemoryThread()

		done := make(chan bool)

		// Goroutine 1: writes to thread1
		go func() {
			for i := 0; i < 100; i++ {
				msg := []domain.Message{
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
			for i := 0; i < 100; i++ {
				msg := []domain.Message{
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
		assert.Len(t, thread1.GetMessages(), 100)
		assert.Len(t, thread2.GetMessages(), 100)

		// Verify content is correct
		msgs1 := thread1.GetMessages()
		msgs2 := thread2.GetMessages()
		assert.Equal(t, "user", msgs1[0].Role)
		assert.Equal(t, "assistant", msgs2[0].Role)
	})
}

// TestAgentThreadFactory verifies Agent thread factory methods
func TestAgentThreadFactory(t *testing.T) {
	t.Run("GetNewThread creates new thread", func(t *testing.T) {
		agent := &Agent{}

		thread := agent.GetNewThread()
		require.NotNil(t, thread)

		assert.NotEmpty(t, thread.GetID())
		assert.Equal(t, ThreadTypeLocal, thread.GetType())
		assert.Empty(t, thread.GetMessages())
	})

	t.Run("DeserializeThread restores thread", func(t *testing.T) {
		agent := &Agent{}

		// Create and serialize a thread
		originalThread := NewInMemoryThreadWithID("test-thread")
		messages := []domain.Message{
			{ID: "msg1", Role: "user", Content: "Test", Timestamp: time.Now()},
		}
		_ = originalThread.AddMessages(messages)

		data, err := originalThread.Serialize()
		require.NoError(t, err)

		// Deserialize using agent
		restoredThread, err := agent.DeserializeThread(data)
		require.NoError(t, err)
		require.NotNil(t, restoredThread)

		assert.Equal(t, "test-thread", restoredThread.GetID())
		assert.Len(t, restoredThread.GetMessages(), 1)
		assert.Equal(t, "Test", restoredThread.GetMessages()[0].Content)
	})

	t.Run("DeserializeThread with invalid data", func(t *testing.T) {
		agent := &Agent{}

		thread, err := agent.DeserializeThread([]byte("invalid"))
		assert.Error(t, err)
		assert.Nil(t, thread)
	})
}

// TestMessageStoreThreadSafety verifies concurrent access to message store
func TestMessageStoreThreadSafety(t *testing.T) {
	t.Run("concurrent writes and reads", func(t *testing.T) {
		store := NewInMemoryMessageStore()
		done := make(chan bool)

		// Writer goroutine
		go func() {
			for i := 0; i < 50; i++ {
				msg := []domain.Message{
					{ID: fmt.Sprintf("msg-%d", i), Role: "user", Content: "Test", Timestamp: time.Now()},
				}
				_ = store.AddMessages(msg)
			}
			done <- true
		}()

		// Reader goroutine
		go func() {
			for i := 0; i < 50; i++ {
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

// TestThreadGetService verifies service retrieval pattern
func TestThreadGetService(t *testing.T) {
	t.Run("get thread service", func(t *testing.T) {
		thread := NewInMemoryThread()

		var agentThread AgentThread
		service := thread.GetService(&agentThread)
		assert.NotNil(t, service)
	})

	t.Run("get message store service", func(t *testing.T) {
		thread := NewInMemoryThread()

		var messageStore MessageStore
		service := thread.GetService(&messageStore)
		assert.NotNil(t, service)
	})

	t.Run("get unknown service returns nil", func(t *testing.T) {
		thread := NewInMemoryThread()

		type UnknownService interface{}
		var unknown UnknownService
		service := thread.GetService(&unknown)
		assert.Nil(t, service)
	})
}
