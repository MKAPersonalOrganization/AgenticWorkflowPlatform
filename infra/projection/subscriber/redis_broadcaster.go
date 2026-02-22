package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/logger"

	"github.com/redis/go-redis/v9"
)

// RedisBroadcaster handles real-time event broadcasting using Redis pub/sub.
// This enables horizontal scaling by allowing events to be broadcast across multiple pods.
type RedisBroadcaster struct {
	log    *logger.Logger
	client *redis.Client

	mu          sync.RWMutex
	subscribers map[string][]*redisSubscription // threadID -> local subscribers
}

// redisSubscription tracks a local subscriber and its Redis pubsub connection.
type redisSubscription struct {
	ch     chan domain.Event
	pubsub *redis.PubSub
	cancel context.CancelFunc
}

// NewRedisBroadcaster creates a new Redis-backed broadcaster.
func NewRedisBroadcaster(log *logger.Logger, redisAddr string, redisPassword string) *RedisBroadcaster {
	poolSize := 10 * runtime.GOMAXPROCS(0)
	if poolSize < 50 {
		poolSize = 50
	}

	client := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     redisPassword,
		PoolSize:     poolSize,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	return &RedisBroadcaster{
		log:         log,
		client:      client,
		subscribers: make(map[string][]*redisSubscription),
	}
}

// channelName returns the Redis channel name for a thread.
func channelName(threadID string) string {
	return fmt.Sprintf("thread:%s:events", threadID)
}

// Subscribe subscribes to real-time events for a thread.
// Events are received via Redis pub/sub, enabling cross-pod delivery.
func (b *RedisBroadcaster) Subscribe(threadID string) (<-chan domain.Event, func()) {
	ch := make(chan domain.Event, 100)

	ctx, cancel := context.WithCancel(context.Background())

	// Subscribe to Redis channel for this thread
	pubsub := b.client.Subscribe(ctx, channelName(threadID))

	sub := &redisSubscription{
		ch:     ch,
		pubsub: pubsub,
		cancel: cancel,
	}

	// Track subscription locally (for cleanup)
	b.mu.Lock()
	b.subscribers[threadID] = append(b.subscribers[threadID], sub)
	b.mu.Unlock()

	// Start goroutine to receive messages from Redis and forward to channel
	go func() {
		defer close(ch)
		defer func() {
			_ = pubsub.Close()
		}()

		redisCh := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-redisCh:
				if !ok {
					return
				}

				// Deserialize the event
				event, err := b.deserializeEvent(msg.Payload)
				if err != nil {
					b.log.Warn(ctx, "Failed to deserialize event from Redis",
						"threadID", threadID,
						"error", err)
					continue
				}

				// Forward to local channel directly
				if event != nil {
					select {
					case ch <- event:
					default:
						b.log.Warn(ctx, "Redis broadcaster channel full, dropping event",
							"threadID", threadID)
					}
				}
			}
		}
	}()

	// Return cancel function that cleans up the subscription
	return ch, func() {
		cancel()

		b.mu.Lock()
		defer b.mu.Unlock()

		subs := b.subscribers[threadID]
		for i, s := range subs {
			if s == sub {
				b.subscribers[threadID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		// Remove the map entry entirely if no subscribers remain to prevent memory leak
		if len(b.subscribers[threadID]) == 0 {
			delete(b.subscribers, threadID)
		}
	}
}

// Broadcast publishes an event to Redis, which delivers it to all pods' subscribers.
func (b *RedisBroadcaster) Broadcast(ctx context.Context, event domain.Event) error {
	threadID := event.AggregateID()
	if threadID == "" {
		return nil
	}

	// Serialize the event for Redis
	payload, err := b.serializeEvent(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Publish to Redis channel
	if err := b.client.Publish(ctx, channelName(threadID), payload).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	b.log.Debug(ctx, "RedisBroadcaster published event",
		"threadID", threadID,
		"eventType", event.EventType(),
		"channel", channelName(threadID))

	return nil
}

// eventEnvelope wraps an event with its type for deserialization.
type eventEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// serializeEvent serializes an event to JSON with type information.
func (b *RedisBroadcaster) serializeEvent(event domain.Event) (string, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return "", err
	}

	envelope := eventEnvelope{
		Type: event.EventType(),
		Data: data,
	}

	result, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

// deserializeEvent deserializes an event from JSON.
func (b *RedisBroadcaster) deserializeEvent(payload string) (domain.Event, error) {
	var envelope eventEnvelope
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal envelope: %w", err)
	}

	// Use the same event deserialization as ProjectionUpdater
	event, err := deserializeDomainEvent(envelope.Type, envelope.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize event type %s: %w", envelope.Type, err)
	}

	return event, nil
}

// Close cancels all active subscriptions and closes the Redis client connection.
func (b *RedisBroadcaster) Close() error {
	b.mu.Lock()
	for threadID, subs := range b.subscribers {
		for _, s := range subs {
			s.cancel()
		}
		delete(b.subscribers, threadID)
	}
	b.mu.Unlock()

	return b.client.Close()
}
