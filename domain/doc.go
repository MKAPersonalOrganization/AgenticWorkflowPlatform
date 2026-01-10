// Package domain provides event-sourced domain models using patterns from EventSourcing.NetCore.
//
// # Event Sourcing Architecture
//
// This implementation follows these key patterns:
//
// 1. **Aggregate Pattern**: ThreadAggregate embeds AggregateState for event sourcing capabilities
// 2. **Decider Pattern**: Pure functions for command validation and event generation
// 3. **Repository Pattern**: Handles persistence and optimistic concurrency
// 4. **Evolve Pattern**: Pure state transitions from events
//
// # Quick Start
//
// ## Creating a new thread:
//
//	// 1. Create repository
//	repo := domain.NewThreadRepository(eventStore)
//
//	// 2. Create aggregate
//	thread := domain.NewThreadAggregate("thread-123")
//
//	// 3. Use decider to process command
//	decider := domain.NewThreadDecider()
//	cmd := domain.CreateThread{
//	    ThreadID: "thread-123",
//	    UserID:   "user-456",
//	    Title:    "My Conversation",
//	}
//
//	events, err := decider.Decide(cmd, thread)
//	if err != nil {
//	    // Command validation failed
//	    return err
//	}
//
//	// 4. Enqueue events and save
//	for _, event := range events {
//	    thread.Enqueue(event)
//	    thread.Evolve(event) // Update local state
//	}
//
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
//	// 2. Create command
//	cmd := domain.AddMessage{
//	    ThreadID:  "thread-123",
//	    MessageID: "msg-789",
//	    Role:      "user",
//	    Content:   "Hello, world!",
//	}
//
//	// 3. Use decider to validate and generate events
//	decider := domain.NewThreadDecider()
//	events, err := decider.Decide(cmd, thread)
//	if err != nil {
//	    return err // Business rule violation
//	}
//
//	// 4. Apply events and save
//	for _, event := range events {
//	    thread.Enqueue(event)
//	    thread.Evolve(event)
//	}
//
//	err = repo.Save(ctx, thread) // Uses optimistic concurrency
//	if err == core.ErrConcurrencyConflict {
//	    // Another process modified the thread - reload and retry
//	}
//
// # Key Concepts
//
// ## Commands vs Events
//
// Commands represent intentions and can fail:
//   - CreateThread, AddMessage, ChangeThreadTitle
//   - Validated by Decider
//   - Can return errors
//
// Events represent facts that happened:
//   - ThreadCreated, MessageAdded, ThreadTitleChanged
//   - Immutable and persisted in EventStore
//   - Cannot fail (they already happened)
//
// ## Decider Pattern
//
// The Decider separates business logic into three pure functions:
//
//	type Decider[State, Command, Event] struct {
//	    Decide     func(Command, State) ([]Event, error)  // Validate and generate events
//	    Evolve     func(State, Event) State               // Apply event to state
//	    GetInitial func() State                           // Create zero state
//	}
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
// - Uncommitted events queue
// - Event replay (LoadFromHistory)
//
// Your aggregate embeds AggregateState and implements Evolve:
//
//	type ThreadAggregate struct {
//	    AggregateState      // Embedded state management
//	    Title     string    // Domain state
//	    Messages  []Message // Domain state
//	}
//
//	func (t *ThreadAggregate) Evolve(event Event) {
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
//	    decider := domain.NewThreadDecider()
//	    thread := domain.NewThreadAggregate("thread-1")
//
//	    cmd := domain.CreateThread{
//	        ThreadID: "thread-1",
//	        UserID:   "user-1",
//	        Title:    "Test",
//	    }
//
//	    events, err := decider.Decide(cmd, thread)
//	    assert.NoError(t, err)
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
