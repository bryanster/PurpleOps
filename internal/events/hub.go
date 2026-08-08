package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Topic names M2 publishes. M4 adds engagement-scoped prefixes; keep these
// stable — the SPA and any scripted subscriber keys off the string.
const (
	// TopicContentJobs carries every content job progress tick (admin).
	TopicContentJobs = "content.jobs"

	// TopicEngagementPrefix is the prefix for engagement-scoped topics:
	// "engagement.{engagementId}". No per-kind subtopics in M4.
	TopicEngagementPrefix = "engagement."
)

// TopicContentJob is the per-job topic for one content sync job.
func TopicContentJob(jobID string) string {
	return TopicContentJobs + "." + jobID
}

// EngagementTopic returns the topic name for one engagement's live stream.
func EngagementTopic(engagementID string) string {
	return TopicEngagementPrefix + engagementID
}

// Stable event type values on the wire. Progress ticks while a job runs;
// terminal fires once the job reaches a final status.
const (
	TypeContentJobProgress = "content.job.progress"
	TypeContentJobTerminal = "content.job.terminal"
	TypeStreamGap          = "stream.gap"
	TypeSyncRequired       = "sync.required"
)

// Presence event types (M4-006). Ephemeral: never persisted, never replayed
// on Last-Event-ID reconnect. Hub-generated ids only.
const (
	TypePresenceJoin   = "presence.join"
	TypePresenceLeave  = "presence.leave"
	TypePresenceUpdate = "presence.update"
)

// Event is one fan-out message. ID is a UUIDv7 so Last-Event-ID ordering is
// stable even though M2 does not replay against the activity log.
type Event struct {
	ID    string          `json:"id"`
	Topic string          `json:"topic"`
	Type  string          `json:"type"`
	At    time.Time       `json:"at"`
	Data  json.RawMessage `json:"data"`
}

// Subscription is what a caller asks the hub for. Topics are filtered through
// [Options.TopicAuthz] (when set) before the subscription is armed — never
// silently widened.
//
// Allow is a per-subscriber delivery filter: events that pass topic matching
// are still dropped for this subscriber when Allow returns false. Publish is
// unaffected. M4 uses this for blind-mode per-seat filtering.
//
// Modify is an optional per-subscriber transform that runs after Allow. It
// receives a copy of the event and may return a modified copy — the original
// is unchanged for other subscribers. M4-009 uses this to strip unrevealed
// focus from presence events for blue subscribers.
type Subscription struct {
	Topics []string
	Allow  func(Event) bool
	Modify func(Event) Event
}

// Options bounds hub memory and plugs M4 extension points.
type Options struct {
	// MaxSubscribers caps concurrent live subscriptions. Zero → 256.
	MaxSubscribers int
	// Buffer is the per-subscriber channel capacity. Zero → 16. A full buffer
	// drops that subscriber rather than blocking Publish.
	Buffer int
	// TopicAuthz decides whether a requested topic is visible to this
	// Subscribe call. nil accepts every well-formed topic the caller named.
	// Returning false filters the topic out; returning an error fails the
	// whole Subscribe. M4 uses this for engagement-scoped topic prefixes.
	TopicAuthz func(ctx context.Context, topic string) (bool, error)
}

// Defaults when Options left a zero.
const (
	defaultMaxSubscribers = 256
	defaultBuffer         = 16
)

// Hub is an in-process topic fan-out with backpressure and slow-client
// eviction. It has no store dependency — durable audit stays on [Log].
//
// Publish never blocks. A subscriber whose buffer fills is dropped and its
// channel closed; subsequent Publish calls keep returning promptly.
type Hub struct {
	maxSubs int
	buffer  int
	topicOK func(context.Context, string) (bool, error)
	mu      sync.Mutex
	subs    map[*subscriber]struct{}
}

// subscriber serializes send and close so Publish cannot race close(ch).
type subscriber struct {
	hub    *Hub
	topics map[string]struct{}
	allow  func(Event) bool
	modify func(Event) Event
	ch     chan Event
	stop   chan struct{}

	sendMu sync.Mutex
	closed bool
	// detachOnce runs hub detach + stop close exactly once.
	detachOnce sync.Once
}

// NewHub returns a hub. opts may be zero-valued.
func NewHub(opts Options) *Hub {
	max := opts.MaxSubscribers
	if max <= 0 {
		max = defaultMaxSubscribers
	}
	buf := opts.Buffer
	if buf <= 0 {
		buf = defaultBuffer
	}
	return &Hub{
		maxSubs: max,
		buffer:  buf,
		topicOK: opts.TopicAuthz,
		subs:    make(map[*subscriber]struct{}),
	}
}

// ErrUnknownTopic means the caller named a topic this build does not define.
// The HTTP layer maps it to 400; never silently ignore unknown names in a way
// that looks like a successful empty subscription to a typo.
var ErrUnknownTopic = errors.New("events: unknown topic")

// ErrNoTopics means every requested topic was filtered out by TopicAuthz, or
// the caller named none.
var ErrNoTopics = errors.New("events: no authorized topics")

// ErrTooManySubscribers means MaxSubscribers is already reached.
var ErrTooManySubscribers = errors.New("events: subscriber limit reached")

// Subscribe arms a filtered subscription. The channel is closed when
// unsubscribe is called, the context is cancelled, or the subscriber is
// evicted for falling behind.
//
// Unknown topics (not matching any known prefix this build defines) return
// [ErrUnknownTopic]. An empty intersection after authz returns [ErrNoTopics].
func (h *Hub) Subscribe(ctx context.Context, sub Subscription) (<-chan Event, func(), error) {
	if len(sub.Topics) == 0 {
		return nil, nil, ErrNoTopics
	}

	allowed := make(map[string]struct{}, len(sub.Topics))
	for _, topic := range sub.Topics {
		if topic == "" {
			return nil, nil, fmt.Errorf("%w: empty topic", ErrUnknownTopic)
		}
		if !knownTopic(topic) {
			return nil, nil, fmt.Errorf("%w: %s", ErrUnknownTopic, topic)
		}
		if h.topicOK != nil {
			ok, err := h.topicOK(ctx, topic)
			if err != nil {
				return nil, nil, err
			}
			if !ok {
				continue
			}
		}
		allowed[topic] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil, nil, ErrNoTopics
	}

	s := &subscriber{
		hub:    h,
		topics: allowed,
		allow:  sub.Allow,
		modify: sub.Modify,
		ch:     make(chan Event, h.buffer),
		stop:   make(chan struct{}),
	}

	h.mu.Lock()
	if len(h.subs) >= h.maxSubs {
		h.mu.Unlock()
		return nil, nil, ErrTooManySubscribers
	}
	h.subs[s] = struct{}{}
	h.mu.Unlock()

	// Context cancel drops the subscriber. The HTTP layer also calls
	// unsubscribe on handler return; detach is idempotent.
	if ctx != nil && ctx.Done() != nil {
		go func() {
			select {
			case <-ctx.Done():
				s.detach()
			case <-s.stop:
			}
		}()
	}

	return s.ch, s.detach, nil
}

// Publish fans evt out to every subscriber of topic, then each subscriber's
// [Subscription.Allow] (when set) may skip delivery. Modify (when set)
// transforms the event per-subscriber after Allow. Missing ID/At/Topic are
// filled in. Never blocks: a slow subscriber is dropped.
func (h *Hub) Publish(topic string, evt Event) {
	if topic == "" {
		topic = evt.Topic
	}
	if topic == "" {
		return
	}
	evt.Topic = topic
	if evt.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			// V7 only fails when the system clock is unreadable; fall back so
			// a clock blip cannot stall progress fan-out.
			id = uuid.New()
		}
		evt.ID = id.String()
	}
	if evt.At.IsZero() {
		evt.At = time.Now().UTC()
	} else {
		evt.At = evt.At.UTC()
	}

	h.mu.Lock()
	// Snapshot under the lock so Publish does not hold it across channel
	// sends (and so a drop mid-iteration is well-defined).
	targets := make([]*subscriber, 0, len(h.subs))
	for s := range h.subs {
		if _, ok := s.topics[topic]; ok {
			targets = append(targets, s)
		}
	}
	h.mu.Unlock()

	for _, s := range targets {
		candidate := evt
		if s.allow != nil && !s.allow(candidate) {
			continue
		}
		if s.modify != nil {
			candidate = s.modify(candidate)
		}
		s.trySend(candidate)
	}
}

// trySend delivers evt. On a full buffer the subscriber is detached so Publish
// never blocks on a slow client.
func (s *subscriber) trySend(evt Event) {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- evt:
	default:
		// Overflow: close under sendMu, then detach (hub lock after sendMu).
		s.closed = true
		close(s.ch)
		// detach after releasing sendMu to keep lock order: sendMu → hub.mu
		// is never inverted with hub.mu → sendMu (Publish snapshots first).
		go s.finishDetach()
	}
}

// detach unsubscribes. Safe to call many times from HTTP return, context
// cancel, or overflow eviction.
func (s *subscriber) detach() {
	s.sendMu.Lock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
	s.sendMu.Unlock()
	s.finishDetach()
}

func (s *subscriber) finishDetach() {
	s.detachOnce.Do(func() {
		s.hub.mu.Lock()
		delete(s.hub.subs, s)
		s.hub.mu.Unlock()
		close(s.stop)
	})
}

// SubscriberCount is the number of live subscriptions. Tests and diagnostics.
func (h *Hub) SubscriberCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}

// knownTopic reports whether topic is one this build will ever publish.
// Unknown names must not be accepted as an empty successful subscription.
//
// M2: content.jobs and content.jobs.{jobId}. M4 extends this (engagement.*).
func knownTopic(topic string) bool {
	if topic == TopicContentJobs {
		return true
	}
	if strings.HasPrefix(topic, TopicContentJobs+".") {
		return true
	}
	if strings.HasPrefix(topic, TopicEngagementPrefix) {
		engID := strings.TrimPrefix(topic, TopicEngagementPrefix)
		_, err := uuid.Parse(engID)
		return err == nil
	}
	return false
}
