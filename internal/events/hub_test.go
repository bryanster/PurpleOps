package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/events"
)

func TestPublishReachesMatchingSubscribersOnly(t *testing.T) {
	t.Parallel()
	hub := events.NewHub(events.Options{})

	all, unsubAll, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.TopicContentJobs},
	})
	if err != nil {
		t.Fatalf("subscribe all: %v", err)
	}
	defer unsubAll()

	jobTopic := events.TopicContentJob("job-1")
	one, unsubOne, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{jobTopic},
	})
	if err != nil {
		t.Fatalf("subscribe one: %v", err)
	}
	defer unsubOne()

	hub.Publish(events.TopicContentJobs, events.Event{
		Type: events.TypeContentJobProgress,
		Data: json.RawMessage(`{"jobId":"job-1"}`),
	})
	hub.Publish(jobTopic, events.Event{
		Type: events.TypeContentJobProgress,
		Data: json.RawMessage(`{"jobId":"job-1"}`),
	})

	gotAll := recvN(t, all, 1)
	if gotAll[0].Topic != events.TopicContentJobs {
		t.Fatalf("all topic = %q", gotAll[0].Topic)
	}
	gotOne := recvN(t, one, 1)
	if gotOne[0].Topic != jobTopic {
		t.Fatalf("one topic = %q", gotOne[0].Topic)
	}

	// The per-job subscriber must not have seen the broadcast-only event.
	select {
	case ev := <-one:
		t.Fatalf("per-job got unexpected event: %+v", ev)
	default:
	}
}

func TestUnknownTopicIsRefused(t *testing.T) {
	t.Parallel()
	hub := events.NewHub(events.Options{})
	_, _, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{"engagement.nope"},
	})
	if !errors.Is(err, events.ErrUnknownTopic) {
		t.Fatalf("err = %v, want ErrUnknownTopic", err)
	}
}

func TestTopicAuthzFilters(t *testing.T) {
	t.Parallel()
	hub := events.NewHub(events.Options{
		TopicAuthz: func(_ context.Context, topic string) (bool, error) {
			return topic == events.TopicContentJobs, nil
		},
	})

	// Mixed request: keep the allowed one.
	ch, unsub, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.TopicContentJobs, events.TopicContentJob("x")},
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	hub.Publish(events.TopicContentJobs, events.Event{Type: events.TypeContentJobProgress})
	hub.Publish(events.TopicContentJob("x"), events.Event{Type: events.TypeContentJobProgress})
	got := recvN(t, ch, 1)
	if got[0].Topic != events.TopicContentJobs {
		t.Fatalf("topic = %q", got[0].Topic)
	}

	// All filtered → ErrNoTopics.
	_, _, err = hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.TopicContentJob("x")},
	})
	if !errors.Is(err, events.ErrNoTopics) {
		t.Fatalf("err = %v, want ErrNoTopics", err)
	}
}

func TestCancelContextUnsubscribes(t *testing.T) {
	t.Parallel()
	hub := events.NewHub(events.Options{})
	ctx, cancel := context.WithCancel(t.Context())
	ch, unsub, err := hub.Subscribe(ctx, events.Subscription{
		Topics: []string{events.TopicContentJobs},
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer unsub()

	if hub.SubscriberCount() != 1 {
		t.Fatalf("count = %d", hub.SubscriberCount())
	}
	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for hub.SubscriberCount() != 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if hub.SubscriberCount() != 0 {
		t.Fatalf("subscriber survived context cancel")
	}
	// Channel must be closed.
	if _, ok := <-ch; ok {
		t.Fatal("channel still open after cancel")
	}
}

func TestSlowSubscriberIsEvictedAndPublishStaysPrompt(t *testing.T) {
	t.Parallel()
	hub := events.NewHub(events.Options{Buffer: 2})

	// Never-read subscriber fills the buffer and must be dropped.
	_, unsubSlow, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.TopicContentJobs},
	})
	if err != nil {
		t.Fatalf("slow sub: %v", err)
	}
	defer unsubSlow()

	fast, unsubFast, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.TopicContentJobs},
	})
	if err != nil {
		t.Fatalf("fast sub: %v", err)
	}
	defer unsubFast()

	// Fill the slow buffer (2) then more to force eviction.
	start := time.Now()
	for range 4 {
		hub.Publish(events.TopicContentJobs, events.Event{
			Type: events.TypeContentJobProgress,
			Data: json.RawMessage(`{"n":1}`),
		})
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("Publish blocked for %s; must never wait on a slow client", elapsed)
	}

	// Fast subscriber still receives (it drains).
	_ = recvN(t, fast, 1)

	// Slow one is gone; only the fast subscriber remains.
	deadline := time.Now().Add(2 * time.Second)
	for hub.SubscriberCount() > 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if hub.SubscriberCount() > 1 {
		t.Fatalf("slow subscriber was not evicted; count=%d", hub.SubscriberCount())
	}

	// A subsequent Publish still returns promptly with nobody blocked.
	start = time.Now()
	hub.Publish(events.TopicContentJobs, events.Event{Type: events.TypeContentJobTerminal})
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("Publish after eviction took %s", elapsed)
	}
}

func TestMaxSubscribers(t *testing.T) {
	t.Parallel()
	hub := events.NewHub(events.Options{MaxSubscribers: 1})
	_, unsub, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.TopicContentJobs},
	})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	defer unsub()

	_, _, err = hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.TopicContentJobs},
	})
	if !errors.Is(err, events.ErrTooManySubscribers) {
		t.Fatalf("err = %v, want ErrTooManySubscribers", err)
	}
}

func TestUnsubscribeIsIdempotent(t *testing.T) {
	t.Parallel()
	hub := events.NewHub(events.Options{})
	_, unsub, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.TopicContentJobs},
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	unsub()
	unsub()
	if hub.SubscriberCount() != 0 {
		t.Fatalf("count = %d", hub.SubscriberCount())
	}
}

// TestPublishConcurrentWithSubscribe exercises the race detector on the hot path.
func TestPublishConcurrentWithSubscribe(t *testing.T) {
	t.Parallel()
	hub := events.NewHub(events.Options{Buffer: 64, MaxSubscribers: 64})

	var pubs, subs sync.WaitGroup
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for range 8 {
		subs.Add(1)
		go func() {
			defer subs.Done()
			ch, unsub, err := hub.Subscribe(ctx, events.Subscription{
				Topics: []string{events.TopicContentJobs},
			})
			if err != nil {
				return
			}
			defer unsub()
			for range ch {
			}
		}()
	}
	for range 8 {
		pubs.Add(1)
		go func() {
			defer pubs.Done()
			for range 100 {
				hub.Publish(events.TopicContentJobs, events.Event{
					Type: events.TypeContentJobProgress,
					Data: json.RawMessage(`{}`),
				})
			}
		}()
	}
	pubs.Wait()
	cancel()
	subs.Wait()
}

func recvN(t *testing.T, ch <-chan events.Event, n int) []events.Event {
	t.Helper()
	out := make([]events.Event, 0, n)
	deadline := time.After(2 * time.Second)
	for len(out) < n {
		select {
		case ev, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed after %d/%d events", len(out), n)
			}
			if ev.ID == "" || ev.At.IsZero() || ev.Topic == "" {
				t.Fatalf("event missing required fields: %+v", ev)
			}
			out = append(out, ev)
		case <-deadline:
			t.Fatalf("timed out waiting for %d events; got %d", n, len(out))
		}
	}
	return out
}
