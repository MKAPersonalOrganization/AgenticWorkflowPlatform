package domain

// Decider pattern - separates decision logic from state management.
// A Decider is a pure function that takes a command and current state,
// validates business rules, and returns events to be persisted.
//
// Pattern: (Command, State) -> []Event
//
// Benefits:
// - Pure functions (no side effects)
// - Easy to test
// - Clear separation of concerns
// - Command validation in one place

// Decider represents the decision function for an aggregate.
type Decider[TState any, TCommand any, TEvent any] struct {
	// Decide takes a command and current state, validates business rules,
	// and returns events to apply. Returns error if command is invalid.
	Decide func(command TCommand, state TState) ([]TEvent, error)

	// Evolve applies an event to state and returns the new state.
	// This should be a pure function with no side effects.
	Evolve func(state TState, event TEvent) TState

	// GetInitial returns the zero/initial state for the aggregate.
	GetInitial func() TState
}

// NewDecider creates a new Decider with the given functions.
func NewDecider[TState any, TCommand any, TEvent any](
	decide func(TCommand, TState) ([]TEvent, error),
	evolve func(TState, TEvent) TState,
	getInitial func() TState,
) *Decider[TState, TCommand, TEvent] {
	return &Decider[TState, TCommand, TEvent]{
		Decide:     decide,
		Evolve:     evolve,
		GetInitial: getInitial,
	}
}

// Handle processes a command through the full decide-evolve cycle.
// It decides what events to emit, then evolves the state with those events.
func (d *Decider[TState, TCommand, TEvent]) Handle(command TCommand, state TState) (TState, []TEvent, error) {
	events, err := d.Decide(command, state)
	if err != nil {
		return state, nil, err
	}

	// Evolve state with all events
	newState := state
	for _, event := range events {
		newState = d.Evolve(newState, event)
	}

	return newState, events, nil
}
