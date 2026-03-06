package custom

import (
	"context"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/services/threadmgmt"
)

// ThreadManager manages thread lifecycle and message persistence.
type ThreadManager interface {
	GetThreadManagement() *threadmgmt.Manager
}

// EventProcessor processes and broadcasts domain events.
type EventProcessor interface {
	ProcessEvents(ctx context.Context, events iter.Seq2[domain.Event, error]) iter.Seq2[domain.Event, error]
}
