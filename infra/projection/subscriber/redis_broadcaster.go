package subscriber

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/maphash"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	es "school-with-ai/foundation/eventsourcing"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/projection/agui"

	nats "github.com/nats-io/nats.go"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	// replayBufferSize caps the number of events retained per thread in the
	// JetStream stream (MaxMsgsPerSubject) and the maximum returned by Replay.
	replayBufferSize int64 = 2000

	// topicShards is the number of independent shards for the in-process topic
	// map. Each shard has its own mutex — operations on different threads never
	// contend. 64 shards → ~1.5 % collision probability at any instant.
	topicShards = 64

	// natsStreamName is the single JetStream stream that holds all thread events.
	natsStreamName = "THREADS"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

// BroadcasterConfig holds tunable parameters shared across all transports.
type BroadcasterConfig struct {
	// RingBufferSize is the number of events each in-process topic keeps in its
	// ring buffer. A subscriber that falls more than RingBufferSize events behind
	// is disconnected; the client reconnects with ?lastEventId= for lossless
	// replay via JetStream.
	RingBufferSize int

	// TopicIdleTimeout is how long a topic can have no reads or writes before
	// the reaper removes it from memory and cancels its NATS subscription.
	TopicIdleTimeout time.Duration

	// ReaperInterval is how often the reaper scans for idle topics.
	ReaperInterval time.Duration
}

// NATSBroadcasterConfig extends BroadcasterConfig with NATS-specific settings.
type NATSBroadcasterConfig struct {
	BroadcasterConfig

	// StreamReplicas is the JetStream replication factor.
	// Set to 1 for single-node, 3 for a 3-node cluster.
	StreamReplicas int
}

// DefaultBroadcasterConfig returns production-safe in-process defaults.
func DefaultBroadcasterConfig() BroadcasterConfig {
	return BroadcasterConfig{
		RingBufferSize:   1024,
		TopicIdleTimeout: 10 * time.Minute,
		ReaperInterval:   60 * time.Second,
	}
}

// DefaultNATSBroadcasterConfig returns production-safe defaults for a single node.
func DefaultNATSBroadcasterConfig() NATSBroadcasterConfig {
	return NATSBroadcasterConfig{
		BroadcasterConfig: DefaultBroadcasterConfig(),
		StreamReplicas:    1,
	}
}

// ---------------------------------------------------------------------------
// signal — one-shot wake-up channel
// ---------------------------------------------------------------------------

type signal struct{ ch chan struct{} }

func newSignal() *signal { return &signal{ch: make(chan struct{})} }

// ---------------------------------------------------------------------------
// topicBuffer — per-thread ring buffer with lock-free wake via atomic pointer
// ---------------------------------------------------------------------------

// topicBuffer stores recent events for a single thread in a fixed-size ring
// buffer. Subscribers are woken by closing a signal channel — O(1) per event
// regardless of the number of local subscribers.
type topicBuffer struct {
	ring []es.BroadcastEnvelope
	size uint64

	mu       sync.RWMutex
	writePos uint64
	closed   bool

	sig        atomic.Pointer[signal]
	lastAccess atomic.Int64 // UnixNano
}

func newTopicBuffer(ringSize int) *topicBuffer {
	t := &topicBuffer{
		ring: make([]es.BroadcastEnvelope, ringSize),
		size: uint64(ringSize),
	}
	t.sig.Store(newSignal())
	t.lastAccess.Store(time.Now().UnixNano())
	return t
}

func (t *topicBuffer) publish(env es.BroadcastEnvelope) {
	t.mu.Lock()
	t.ring[t.writePos%t.size] = env
	t.writePos++
	t.mu.Unlock()

	old := t.sig.Swap(newSignal())
	close(old.ch)

	t.lastAccess.Store(time.Now().UnixNano())
}

func (t *topicBuffer) closeTopic() {
	t.mu.Lock()
	if !t.closed {
		t.closed = true
		old := t.sig.Swap(newSignal())
		close(old.ch)
	}
	t.mu.Unlock()
}

// ---------------------------------------------------------------------------
// subscription — implements agui.Subscription (pull-based, zero-registration)
// ---------------------------------------------------------------------------

// subscription is returned by Subscribe. It reads from a topicBuffer using a
// cursor. The broadcaster never tracks this struct — when the HTTP handler
// stops calling Ready/Recv the subscription is GC'd with no cleanup needed.
type subscription struct {
	topic  *topicBuffer
	cursor uint64
}

func (s *subscription) Ready() <-chan struct{} {
	t := s.topic
	t.lastAccess.Store(time.Now().UnixNano())

	t.mu.RLock()
	closed := t.closed
	behind := s.cursor < t.writePos
	t.mu.RUnlock()

	if closed || behind {
		return alwaysReady
	}
	return t.sig.Load().ch
}

func (s *subscription) Recv() ([]es.BroadcastEnvelope, bool) {
	t := s.topic
	t.lastAccess.Store(time.Now().UnixNano())

	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed || s.cursor >= t.writePos {
		return nil, true
	}
	if t.writePos > t.size && s.cursor < t.writePos-t.size {
		return nil, false // fallen behind — caller should disconnect
	}

	count := t.writePos - s.cursor
	result := make([]es.BroadcastEnvelope, count)
	for i := uint64(0); i < count; i++ {
		result[i] = t.ring[(s.cursor+i)%t.size]
	}
	s.cursor = t.writePos
	return result, true
}

var alwaysReady = func() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

var _ agui.Subscription = (*subscription)(nil)

// ---------------------------------------------------------------------------
// topicShard — one of 64 independent shards of the topic map
// ---------------------------------------------------------------------------

type topicShard struct {
	mu     sync.RWMutex
	topics map[string]*topicBuffer // keyed by threadID
}

var shardSeed = maphash.MakeSeed()

func shardFor(threadID string) uint64 {
	var h maphash.Hash
	h.SetSeed(shardSeed)
	h.WriteString(threadID)
	return h.Sum64() % topicShards
}

// ---------------------------------------------------------------------------
// NATSBroadcaster
// ---------------------------------------------------------------------------

// NATSBroadcaster delivers real-time events via NATS.
//
// Architecture:
//
//  1. One js.Publish per event — JetStream persists for replay.
//     A UnixNano SeqID is generated before publish and embedded in the
//     payload envelope so subscribers can read it without JetStream metadata.
//
//  2. Core NATS nc.Subscribe for live delivery — zero server-side consumer
//     state per subscription. The server routes matching subjects to interested
//     connections via a pure in-memory routing table. This eliminates the
//     millions of JetStream consumers that would otherwise be created at scale.
//
//  3. Replay via ephemeral ordered consumer — on reconnect with ?lastEventId=,
//     an ephemeral consumer starts at nats.StartTime(parseNano(lastEventId))
//     and drains synchronously. meta.NumPending == 0 signals the end.
//
//  4. In-process ring buffer + sharded topic map — local fan-out to many SSE
//     connections on the same pod is O(1) and lock-free on the hot path.
type NATSBroadcaster struct {
	log      *logger.Logger
	nc       *nats.Conn
	js       nats.JetStreamContext
	registry *es.DeserializerRegistry
	cfg      NATSBroadcasterConfig

	shards [topicShards]topicShard

	// natsSubs holds one *nats.Subscription per active threadID for cleanup.
	natsSubs sync.Map // threadID (string) → *nats.Subscription

	closed atomic.Bool
	cancel context.CancelFunc
}

// NewNATSBroadcaster connects to NATS, ensures the JetStream stream exists,
// and starts the reaper. Returns an error if the connection or stream setup fails.
func NewNATSBroadcaster(
	log *logger.Logger,
	natsURL string,
	natsPassword string,
	registry *es.DeserializerRegistry,
	cfg NATSBroadcasterConfig,
) (*NATSBroadcaster, error) {
	nc, err := nats.Connect(natsURL,
		nats.UserInfo("nats", natsPassword),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
		nats.PingInterval(20*time.Second),
		nats.MaxPingsOutstanding(3),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Warn(context.Background(), "NATSBroadcaster: disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info(context.Background(), "NATSBroadcaster: reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("NATS connect: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("NATS JetStream context: %w", err)
	}

	replicas := cfg.StreamReplicas
	if replicas < 1 {
		replicas = 1
	}
	if err := ensureStream(js, replicas); err != nil {
		nc.Close()
		return nil, fmt.Errorf("NATS stream setup: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	b := &NATSBroadcaster{
		log:      log,
		nc:       nc,
		js:       js,
		registry: registry,
		cfg:      cfg,
		cancel:   cancel,
	}
	for i := range b.shards {
		b.shards[i].topics = make(map[string]*topicBuffer)
	}

	go b.reapLoop(ctx)

	return b, nil
}

// ensureStream creates the THREADS stream if absent, or updates its config if
// already present. Idempotent — safe to call on every startup.
func ensureStream(js nats.JetStreamContext, replicas int) error {
	cfg := &nats.StreamConfig{
		Name:              natsStreamName,
		Subjects:          []string{"thread.*"},
		MaxMsgsPerSubject: replayBufferSize,
		MaxAge:            2 * time.Hour,
		MaxMsgSize:        1 << 20, // 1 MB
		Storage:           nats.FileStorage,
		Replicas:          replicas,
		Retention:         nats.LimitsPolicy,
		Discard:           nats.DiscardOld,
	}
	_, err := js.StreamInfo(natsStreamName)
	if err == nats.ErrStreamNotFound {
		_, err = js.AddStream(cfg)
		return err
	}
	if err != nil {
		return err
	}
	_, err = js.UpdateStream(cfg)
	return err
}

// natsSubject returns the NATS subject for a thread.
func natsSubject(threadID string) string { return "thread." + threadID }

// seqToStr formats a UnixNano timestamp as the SSE last-event-id.
func seqToStr(nano int64) string { return strconv.FormatInt(nano, 10) }

// strToNano parses an SSE last-event-id (UnixNano decimal string) back to
// a time.Time for JetStream StartTime replay. Returns zero time for empty
// strings or legacy Redis Stream IDs (contain "-"), causing Replay to start
// from the beginning of the retained buffer.
func strToNano(s string) time.Time {
	if s == "" || strings.Contains(s, "-") {
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

// ---------------------------------------------------------------------------
// Subscribe
// ---------------------------------------------------------------------------

// Subscribe returns a pull-based Subscription for real-time events on threadID.
// Nothing to unsubscribe — stop calling Ready/Recv and the subscription is GC'd.
func (b *NATSBroadcaster) Subscribe(threadID string) agui.Subscription {
	topic := b.getOrCreateTopic(threadID)

	topic.mu.RLock()
	cursor := topic.writePos
	topic.mu.RUnlock()

	return &subscription{topic: topic, cursor: cursor}
}

func (b *NATSBroadcaster) getOrCreateTopic(threadID string) *topicBuffer {
	shard := &b.shards[shardFor(threadID)]

	shard.mu.RLock()
	if t, ok := shard.topics[threadID]; ok {
		shard.mu.RUnlock()
		return t
	}
	shard.mu.RUnlock()

	shard.mu.Lock()
	if t, ok := shard.topics[threadID]; ok {
		shard.mu.Unlock()
		return t
	}
	t := newTopicBuffer(b.cfg.RingBufferSize)
	shard.topics[threadID] = t
	shard.mu.Unlock()

	// Start the core NATS subscription outside the shard lock.
	b.startLiveSub(threadID, t)
	return t
}

// startLiveSub creates a core NATS subscription for threadID that delivers
// new messages into the local ring buffer. Unlike JetStream push consumers,
// core NATS subscriptions have ZERO server-side state — the server simply
// routes matching subjects to interested connections via an in-memory table.
// This is the key scalability fix: 10M concurrent subscriptions create no
// server-side consumer objects, no raft log entries, no delivery tracking.
//
// The callback reads SeqID from the payload (embedded at publish time) instead
// of from JetStream metadata (which core NATS messages don't carry).
func (b *NATSBroadcaster) startLiveSub(threadID string, topic *topicBuffer) {
	if _, exists := b.natsSubs.Load(threadID); exists {
		return
	}

	sub, err := b.nc.Subscribe(natsSubject(threadID), func(msg *nats.Msg) {
		var env natsEnvelope
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			b.log.Warn(context.Background(), "NATSBroadcaster: unmarshal failed",
				"threadID", threadID, "error", err)
			return
		}

		event, err := deserializeDomainEvent(b.registry, env.Type, env.Data)
		if err != nil || event == nil {
			if err != nil {
				b.log.Warn(context.Background(), "NATSBroadcaster: deserialize failed",
					"threadID", threadID, "error", err)
			}
			return
		}

		topic.publish(es.BroadcastEnvelope{
			SeqID: env.SeqID,
			Event: event,
		})
	})
	if err != nil {
		b.log.Warn(context.Background(), "NATSBroadcaster: live subscribe failed",
			"threadID", threadID, "error", err)
		return
	}

	// LoadOrStore: if two goroutines race here, keep the first and discard the second.
	if _, loaded := b.natsSubs.LoadOrStore(threadID, sub); loaded {
		sub.Unsubscribe() //nolint:errcheck
	}
}

// ---------------------------------------------------------------------------
// Broadcast
// ---------------------------------------------------------------------------

// Broadcast appends the event to the THREADS JetStream stream. A UnixNano
// SeqID is generated before publish and embedded in the envelope — this lets
// core NATS subscribers (live path) read the sequence ID from the payload
// without JetStream metadata. JetStream stores the message for replay;
// the embedded timestamp is also used by Replay with nats.StartTime.
func (b *NATSBroadcaster) Broadcast(ctx context.Context, event es.Event) error {
	threadID := event.AggregateID()
	if threadID == "" {
		return nil
	}

	seqID := seqToStr(time.Now().UnixNano())

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	payload, err := json.Marshal(natsEnvelope{
		SeqID: seqID,
		Type:  event.EventType(),
		Data:  json.RawMessage(data),
	})
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	if _, err := b.js.Publish(natsSubject(threadID), payload); err != nil {
		return fmt.Errorf("NATS publish: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Replay
// ---------------------------------------------------------------------------

// Replay returns events from the THREADS stream that arrived strictly after
// afterSeqID. afterSeqID is a UnixNano timestamp as a decimal string (the SSE
// last-event-id value). Pass "" to replay from the beginning of the retained
// buffer.
//
// Uses an ephemeral ordered consumer with nats.StartTime — the embedded
// UnixNano SeqID maps directly to the NATS message timestamp, so replay
// picks up right where the client left off.
// meta.NumPending == 0 signals the last available message without polling.
func (b *NATSBroadcaster) Replay(threadID string, afterSeqID string) []es.BroadcastEnvelope {
	startTime := strToNano(afterSeqID)

	var startOpt nats.SubOpt
	if !startTime.IsZero() {
		// StartTime delivers messages with timestamps >= the given time.
		// Add 1ns so we get events strictly after afterSeqID.
		startOpt = nats.StartTime(startTime.Add(1 * time.Nanosecond))
	} else {
		startOpt = nats.DeliverAll()
	}

	sub, err := b.js.SubscribeSync(natsSubject(threadID),
		nats.OrderedConsumer(),
		startOpt,
	)
	if err != nil {
		if err != nats.ErrStreamNotFound {
			b.log.Warn(context.Background(), "NATSBroadcaster: Replay subscribe failed",
				"threadID", threadID, "error", err)
		}
		return nil
	}
	defer sub.Unsubscribe() //nolint:errcheck

	var envelopes []es.BroadcastEnvelope
	for int64(len(envelopes)) < replayBufferSize {
		msg, err := sub.NextMsg(2 * time.Second)
		if err != nil {
			break // ErrTimeout — no more messages available
		}

		meta, err := msg.Metadata()
		if err != nil {
			continue
		}

		var env natsEnvelope
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			continue
		}

		event, err := deserializeDomainEvent(b.registry, env.Type, env.Data)
		if err != nil || event == nil {
			continue
		}

		envelopes = append(envelopes, es.BroadcastEnvelope{
			SeqID: env.SeqID,
			Event: event,
		})

		if meta.NumPending == 0 {
			break // last available message — exit without waiting for timeout
		}
	}

	return envelopes
}

// ---------------------------------------------------------------------------
// Reaper
// ---------------------------------------------------------------------------

func (b *NATSBroadcaster) reapLoop(ctx context.Context) {
	ticker := time.NewTicker(b.cfg.ReaperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.reapIdleTopics()
		}
	}
}

func (b *NATSBroadcaster) reapIdleTopics() {
	now := time.Now()
	var toRemove []string

	for i := range b.shards {
		shard := &b.shards[i]
		shard.mu.Lock()
		for threadID, topic := range shard.topics {
			if now.Sub(time.Unix(0, topic.lastAccess.Load())) > b.cfg.TopicIdleTimeout {
				topic.closeTopic()
				delete(shard.topics, threadID)
				toRemove = append(toRemove, threadID)
			}
		}
		shard.mu.Unlock()
	}

	for _, threadID := range toRemove {
		if v, ok := b.natsSubs.LoadAndDelete(threadID); ok {
			v.(*nats.Subscription).Unsubscribe() //nolint:errcheck
		}
	}

	if len(toRemove) > 0 {
		b.log.Debug(context.Background(), "NATSBroadcaster reaped idle topics",
			"count", len(toRemove))
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

// Close cancels the reaper, unsubscribes all core NATS subscriptions, closes
// all topic buffers, and drains the NATS connection (letting in-flight publishes complete).
func (b *NATSBroadcaster) Close() error {
	b.closed.Store(true)
	b.cancel()

	b.natsSubs.Range(func(_, value any) bool {
		value.(*nats.Subscription).Unsubscribe() //nolint:errcheck
		return true
	})

	for i := range b.shards {
		shard := &b.shards[i]
		shard.mu.Lock()
		for _, topic := range shard.topics {
			topic.closeTopic()
		}
		shard.topics = make(map[string]*topicBuffer)
		shard.mu.Unlock()
	}

	return b.nc.Drain()
}

// ---------------------------------------------------------------------------
// natsEnvelope — payload stored in JetStream
// ---------------------------------------------------------------------------

// natsEnvelope is the JSON payload written to every JetStream message.
// SeqID is a UnixNano timestamp generated before publish and embedded in the
// payload. This lets core NATS subscribers (which have no JetStream metadata)
// read a monotonic sequence ID directly from the message body, eliminating
// the need for a server-side JetStream consumer per live subscription.
type natsEnvelope struct {
	SeqID string          `json:"seq_id"`
	Type  string          `json:"type"`
	Data  json.RawMessage `json:"data"`
}
