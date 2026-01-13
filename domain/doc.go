// Package domain provides event-sourced domain models following patterns from EventSourcing.NetCore.
//
// # Event Sourcing Architecture
//
// This implementation follows these key patterns:
//
// 1. **Aggregate Pattern**: Business logic implemented as methods on the aggregate
// 2. **Repository Pattern**: Handles persistence and optimistic concurrency
// 3. **Apply Pattern**: Pure state transitions from events
//
// # Quick Start
//
// ## Creating a new thread:
//
//	// 1. Create repository
//	repo := domain.NewThreadRepository(eventStore)
//
//	// 2. Create thread using static factory method
//	thread, err := domain.OpenThread("thread-123", "user-456", "My Conversation")
//	if err != nil {
//	    // Validation failed
//	    return err
//	}
//
//	// 3. Save the new thread
//	err = repo.SaveNew(ctx, thread)
//	if err != nil {
//	    // Persistence failed
//	    return err
//	}
//
// ## Adding a message to existing thread:
//
//	// 1. Load thread from event store
//	thread, err := repo.Load(ctx, "thread-123")
//	if err != nil {
//	    return err
//	}
//
//	// 2. Call business method on aggregate
//	err = thread.AddMessage("msg-789", "user", "Hello, world!", nil, "", "", nil)
//	if err != nil {
//	    return err // Business rule violation
//	}
//
//	// 3. Save changes
//	err = repo.Save(ctx, thread) // Uses optimistic concurrency
//	if err == core.ErrConcurrencyConflict {
//	    // Another process modified the thread - reload and retry
//	}
//
// # Key Concepts
//
// ## Business Logic in Aggregate Methods
//
// Following EventSourcing.NetCore pattern, business logic lives in aggregate methods:
//   - OpenThread() - Static factory for creating new threads
//   - AddMessage() - Adds a message with validation
//   - ChangeTitle() - Changes title (idempotent if same)
//   - Delete() - Marks thread as deleted
//
// Each method:
//   - Validates business rules
//   - Creates appropriate events
//   - Enqueues and applies events automatically
//   - Returns error if validation fails
//
// ## Events
//
// Events represent facts that happened:
//   - ThreadCreated, MessageAdded, ThreadTitleUpdatedEvent
//   - Immutable and persisted in EventStore
//   - Applied via the Apply() method to update state
//     Decide     func(Command, State) ([]Event, error)  // Validate and generate events
//     Evolve     func(State, Event) State               // Apply event to state
//     GetInitial func() State                           // Create zero state
//     }
//
// Benefits:
// - Easy to test (pure functions)
// - No side effects in business logic
// - Command validation separate from persistence
//
// ## Optimistic Concurrency
//
// The EventStore tracks stream versions to prevent conflicting updates:
//
//	// First writer wins
//	_, err := eventStore.AppendWithVersion(ctx, streamID, expectedVersion, events...)
//	if err == core.ErrConcurrencyConflict {
//	    // Someone else modified the stream - reload and retry
//	}
//
// Use -1 as expectedVersion to skip concurrency checks (not recommended for production).
//
// ## Aggregate State Management
//
// AggregateState provides:
// - Version tracking (number of events applied)
//
// ## Aggregate State Management
//
// Each aggregate embeds BaseAggregate which provides:
// - Version tracking for optimistic concurrency
// - Uncommitted events queue
// - Event replay (LoadFromHistory)
//
// Your aggregate embeds BaseAggregate and implements Apply:
//
//	type ThreadAggregate struct {
//	    BaseAggregate       // Embedded state management
//	    Title     string    // Domain state
//	    Messages  []Message // Domain state
//	}
//
//	func (t *ThreadAggregate) Apply(event Event) {
//	    switch e := event.(type) {
//	    case *MessageAdded:
//	        t.Messages = append(t.Messages, ...)
//	    }
//	}
//
// # Testing
//
// Event sourcing makes testing easy - just verify events:
//
//	func TestCreateThread(t *testing.T) {
//	    thread, err := domain.OpenThread("thread-1", "user-1", "Test")
//	    assert.NoError(t, err)
//
//	    events := thread.DequeueUncommittedEvents()
//	    assert.Len(t, events, 1)
//
//	    created := events[0].(*domain.ThreadCreated)
//	    assert.Equal(t, "thread-1", created.ThreadID)
//	    assert.Equal(t, "user-1", created.UserID)
//	}
//
// # Advanced Patterns
//
// ## Event Upcasting
//
// When event schemas change, create upcasters:
//
//	type EventUpcaster func(oldEvent Event) Event
//
// ## Snapshots
//
// For aggregates with many events, periodically save snapshots:
//
//	type Snapshot struct {
//	    AggregateID string
//	    Version     int64
//	    State       []byte  // Serialized aggregate state
//	}
//
// Load snapshot first, then replay events after snapshot version.
//
// ## Process Managers
//
// For long-running workflows spanning multiple aggregates, use process managers
// (also called sagas) that listen to events and issue commands.
//
// ## Projections
//
// Build read models by subscribing to event streams:
//
//	type ThreadListProjection struct {
//	    Threads map[string]ThreadSummary
//	}
//
//	func (p *ThreadListProjection) Apply(event Event) {
//	    switch e := event.(type) {
//	    case *ThreadCreated:
//	        p.Threads[e.ThreadID] = ThreadSummary{...}
//	    }
//	}
package domain
