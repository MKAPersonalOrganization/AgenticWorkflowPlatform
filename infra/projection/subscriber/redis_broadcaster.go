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

const (
	// replayBufferSize is the maximum number of events kept in the Redis Stream
	// per thread (XADD MAXLEN) and the maximum returned on a single replay read.
	replayBufferSize int64 = 2000

	// streamTTL is how long a thread's replay stream survives after its last
	// event. Long enough for any plausible client reconnect.
	streamTTL = 2 * time.Hour
)

// RedisBroadcaster handles real-time event broadcasting using Redis pub/sub.
// It uses a single shared PubSub connection for all subscribed threads and one
// reader goroutine that fans out messages to local subscriber channels.
// This enables horizontal scaling across multiple pods.
//
// Replay: every broadcast is also written to a per-thread Redis Stream
// (XADD). Reconnecting clients pass ?lastEventId= with the last Redis Stream
// entry ID they received; the server calls Replay() to fetch missed events from
// the stream before switching to live delivery.
type RedisBroadcaster struct {
	log      *logger.Logger
	client   *redis.Client
	registry *es.DeserializerRegistry
	pubsub   *redis.PubSub

	mu     sync.Mutex
	subs   map[string]map[chan es.BroadcastEnvelope]struct{} // channelName → local subscriber channels
	closed bool
	cancel context.CancelFunc
}

// NewRedisBroadcaster creates a new Redis-backed broadcaster.
func NewRedisBroadcaster(log *logger.Logger, redisAddr string, redisPassword string, registry *es.DeserializerRegistry) *RedisBroadcaster {
	poolSize := max(10*runtime.GOMAXPROCS(0), 50)

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
		subs:     make(map[string]map[chan es.BroadcastEnvelope]struct{}),
		cancel:   cancel,
	}

	// Single reader goroutine for the lifetime of the broadcaster.
	go b.readLoop(ctx)

	return b
}

// channelName returns the Redis pub/sub channel name for a thread.
func channelName(threadID string) string {
	return fmt.Sprintf("thread:%s:events", threadID)
}

// streamKey returns the Redis Stream key used for cross-pod replay.
// Pub/sub channels are not Redis keys, so this name can safely match
// channelName without any key-type conflict.
func streamKey(threadID string) string {
	return fmt.Sprintf("thread:%s:stream", threadID)
}

// Subscribe subscribes to real-time events for a thread.
//
// The first local subscriber for a given threadID adds the channel to the
// shared Redis PubSub connection. Subsequent subscribers for the same
// threadID re-use it with zero additional Redis resources.
//
// If the subscriber's channel is full (client too slow), the channel is closed
// and the subscriber is removed. The HTTP handler detects the closed channel
// and ends the SSE response; the client reconnects with ?lastEventId= and
// receives missed events via Replay.
func (b *RedisBroadcaster) Subscribe(threadID string) (<-chan es.BroadcastEnvelope, func()) {
	ch := make(chan es.BroadcastEnvelope, 100)
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
		topicSubs = make(map[chan es.BroadcastEnvelope]struct{})
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

			var envelope pubsubEnvelope
			if err := json.Unmarshal([]byte(msg.Payload), &envelope); err != nil {
				b.log.Warn(ctx, "Failed to unmarshal pub/sub envelope",
					"channel", msg.Channel, "error", err)
				continue
			}

			event, err := deserializeDomainEvent(b.registry, envelope.Type, envelope.Data)
			if err != nil {
				b.log.Warn(ctx, "Failed to deserialize event from Redis",
					"channel", msg.Channel, "error", err)
				continue
			}
			if event == nil {
				continue
			}

			benv := es.BroadcastEnvelope{SeqID: envelope.StreamID, Event: event}

			// Fan-out to local subscribers for this channel.
			b.mu.Lock()
			for ch := range b.subs[msg.Channel] {
				select {
				case ch <- benv:
				default:
					// Channel full: disconnect this subscriber so the client
					// reconnects and replays missed events via the Redis Stream.
					b.log.Warn(ctx, "subscriber channel full, disconnecting slow client",
						"channel", msg.Channel)
					close(ch)
					delete(b.subs[msg.Channel], ch)
				}
			}
			b.mu.Unlock()
		}
	}
}

// Broadcast publishes an event to Redis. It:
//  1. Appends the event to the thread's Redis Stream (for cross-pod replay).
//  2. Publishes to the pub/sub channel (for low-latency live delivery),
//     embedding the Stream entry ID so subscribers can use it as the SSE id:.
func (b *RedisBroadcaster) Broadcast(ctx context.Context, event es.Event) error {
	threadID := event.AggregateID()
	if threadID == "" {
		return nil
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// 1. Append to Redis Stream — get back the globally unique entry ID.
	streamID, xaddErr := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey(threadID),
		MaxLen: replayBufferSize,
		Approx: true, // ~MAXLEN: O(1) trimming
		Values: map[string]any{
			"type": event.EventType(),
			"data": string(data),
		},
	}).Result()
	if xaddErr != nil {
		// Non-fatal: live delivery still works; replay for this event will be
		// missing, but that is better than dropping the live broadcast.
		b.log.Warn(ctx, "RedisBroadcaster: failed to append to stream",
			"threadID", threadID, "error", xaddErr)
		streamID = ""
	} else {
		// Refresh TTL so the stream survives for at least streamTTL after the
		// last event (fire-and-forget; ignore error).
		b.client.Expire(ctx, streamKey(threadID), streamTTL) //nolint:errcheck
	}

	// 2. Embed the stream ID in the pub/sub envelope so live subscribers
	//    receive it as their SSE id: without an extra Redis round-trip.
	envelope := pubsubEnvelope{
		Type:     event.EventType(),
		StreamID: streamID,
		Data:     json.RawMessage(data),
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal pub/sub envelope: %w", err)
	}

	if err := b.client.Publish(ctx, channelName(threadID), string(payload)).Err(); err != nil {
		return fmt.Errorf("failed to publish to Redis: %w", err)
	}

	b.log.Debug(ctx, "RedisBroadcaster published event",
		"threadID", threadID,
		"eventType", event.EventType(),
		"streamID", streamID)

	return nil
}

// Replay returns events from the thread's Redis Stream that arrived strictly
// after afterSeqID. afterSeqID is a Redis Stream entry ID (e.g.
// "1717000000000-0"); pass "0-0" to replay from the beginning.
//
// Returns an empty slice when no events are buffered, the stream has expired,
// or on error.
func (b *RedisBroadcaster) Replay(threadID string, afterSeqID string) []es.BroadcastEnvelope {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := b.client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{streamKey(threadID), afterSeqID},
		Count:   replayBufferSize,
		// Block is zero (default), so this is a non-blocking read.
	}).Result()
	if err != nil {
		if err != redis.Nil {
			b.log.Warn(context.Background(), "RedisBroadcaster: XREAD failed",
				"threadID", threadID, "error", err)
		}
		return nil
	}
	if len(result) == 0 {
		return nil
	}

	messages := result[0].Messages
	envelopes := make([]es.BroadcastEnvelope, 0, len(messages))
	for _, msg := range messages {
		eventType, _ := msg.Values["type"].(string)
		dataStr, _ := msg.Values["data"].(string)

		event, err := deserializeDomainEvent(b.registry, eventType, []byte(dataStr))
		if err != nil {
			b.log.Warn(context.Background(), "RedisBroadcaster: failed to deserialize stream event",
				"id", msg.ID, "error", err)
			continue
		}
		envelopes = append(envelopes, es.BroadcastEnvelope{SeqID: msg.ID, Event: event})
	}
	return envelopes
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

// pubsubEnvelope is the JSON payload written to the Redis pub/sub channel.
// StreamID carries the Redis Stream entry ID so live subscribers receive the
// same identifier that replay uses, without an extra round-trip.
type pubsubEnvelope struct {
	Type     string          `json:"type"`
	StreamID string          `json:"stream_id"`
	Data     json.RawMessage `json:"data"`
}
