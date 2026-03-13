package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"time"

	es "school-with-ai/foundation/eventsourcing"
	"school-with-ai/foundation/logger"

	"github.com/redis/go-redis/v9"
)

// RedisBroadcaster handles real-time event broadcasting using Redis pub/sub.
// It uses a single shared PubSub connection for all subscribed threads and one
// reader goroutine that fans out messages to local subscriber channels.
// This enables horizontal scaling across multiple pods.
type RedisBroadcaster struct {
	log      *logger.Logger
	client   *redis.Client
	registry *es.DeserializerRegistry
	pubsub   *redis.PubSub

	mu     sync.Mutex
	subs   map[string]map[chan es.Event]struct{} // channelName → local subscriber channels
	closed bool
	cancel context.CancelFunc
}

// NewRedisBroadcaster creates a new Redis-backed broadcaster.
func NewRedisBroadcaster(log *logger.Logger, redisAddr string, redisPassword string, registry *es.DeserializerRegistry) *RedisBroadcaster {
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

	ctx, cancel := context.WithCancel(context.Background())

	// Create a single PubSub connection with no initial subscriptions.
	// Channels are added/removed dynamically as SSE clients connect/disconnect.
	pubsub := client.Subscribe(ctx)

	b := &RedisBroadcaster{
		log:      log,
		client:   client,
		registry: registry,
		pubsub:   pubsub,
		subs:     make(map[string]map[chan es.Event]struct{}),
		cancel:   cancel,
	}

	// Single reader goroutine for the lifetime of the broadcaster.
	go b.readLoop(ctx)

	return b
}

// channelName returns the Redis channel name for a thread.
func channelName(threadID string) string {
	return fmt.Sprintf("thread:%s:events", threadID)
}

// Subscribe subscribes to real-time events for a thread.
// Events are received via Redis pub/sub, enabling cross-pod delivery.
//
// The first local subscriber for a given threadID adds the channel to the
// shared Redis PubSub connection. Subsequent subscribers for the same
// threadID re-use it with zero additional Redis resources.
func (b *RedisBroadcaster) Subscribe(threadID string) (<-chan es.Event, func()) {
	ch := make(chan es.Event, 100)
	chName := channelName(threadID)

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		close(ch)
		return ch, func() {}
	}

	topicSubs, exists := b.subs[chName]
	if !exists {
		// First subscriber for this thread — add channel to shared PubSub.
		topicSubs = make(map[chan es.Event]struct{})
		b.subs[chName] = topicSubs

		if err := b.pubsub.Subscribe(context.Background(), chName); err != nil {
			b.log.Warn(context.Background(), "Redis PubSub subscribe failed",
				"channel", chName, "error", err)
		}
	}

	topicSubs[ch] = struct{}{}
	b.mu.Unlock()

	// Unsubscribe function: removes this local sub. When the last subscriber
	// for a channel leaves, the channel is removed from the shared PubSub.
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()

		if _, ok := b.subs[chName]; !ok {
			return
		}
		delete(b.subs[chName], ch)
		close(ch)

		if len(b.subs[chName]) == 0 {
			// Last subscriber — remove channel from shared PubSub.
			_ = b.pubsub.Unsubscribe(context.Background(), chName)
			delete(b.subs, chName)
		}
	}
}

// readLoop is the single goroutine that reads from the shared Redis PubSub
// connection and fans out each message to the correct local subscribers.
func (b *RedisBroadcaster) readLoop(ctx context.Context) {
	redisCh := b.pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-redisCh:
			if !ok {
				return
			}

			event, err := b.deserializeEvent(msg.Payload)
			if err != nil {
				b.log.Warn(ctx, "Failed to deserialize event from Redis",
					"channel", msg.Channel,
					"error", err)
				continue
			}
			if event == nil {
				continue
			}

			// Fan-out to local subscribers for this channel.
			b.mu.Lock()
			for ch := range b.subs[msg.Channel] {
				select {
				case ch <- event:
				default:
					b.log.Warn(ctx, "Redis broadcaster channel full, dropping event",
						"channel", msg.Channel)
				}
			}
			b.mu.Unlock()
		}
	}
}

// Broadcast publishes an event to Redis, which delivers it to all pods' subscribers.
func (b *RedisBroadcaster) Broadcast(ctx context.Context, event es.Event) error {
	threadID := event.AggregateID()
	if threadID == "" {
		return nil
	}

	payload, err := b.serializeEvent(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

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
func (b *RedisBroadcaster) serializeEvent(event es.Event) (string, error) {
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
func (b *RedisBroadcaster) deserializeEvent(payload string) (es.Event, error) {
	var envelope eventEnvelope
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal envelope: %w", err)
	}

	event, err := deserializeDomainEvent(b.registry, envelope.Type, envelope.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize event type %s: %w", envelope.Type, err)
	}

	return event, nil
}

// Close cancels all active subscriptions and closes the Redis client connection.
func (b *RedisBroadcaster) Close() error {
	b.mu.Lock()
	b.closed = true
	for chName, topicSubs := range b.subs {
		for ch := range topicSubs {
			close(ch)
		}
		delete(b.subs, chName)
	}
	b.mu.Unlock()

	// Cancel the reader goroutine, then close the shared PubSub + client.
	b.cancel()
	_ = b.pubsub.Close()
	return b.client.Close()
}
