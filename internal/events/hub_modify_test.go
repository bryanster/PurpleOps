package events_test

import (
	"encoding/json"
	"testing"

	"github.com/bryanster/blacklight/internal/events"
)

func TestModifyIsAppliedPerSubscriber(t *testing.T) {
	hub := events.NewHub(events.Options{})

	engID := "01900000-0000-7000-8000-0000000000a1"

	var sub1Modified bool
	ch1, unsub1, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.EngagementTopic(engID)},
		Modify: func(ev events.Event) events.Event {
			sub1Modified = true
			var data map[string]any
			if err := json.Unmarshal(ev.Data, &data); err != nil {
				return ev
			}
			delete(data, "secret")
			b, err := json.Marshal(data)
			if err != nil {
				return ev
			}
			ev.Data = b
			return ev
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer unsub1()

	ch2, unsub2, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.EngagementTopic(engID)},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer unsub2()

	hub.Publish(events.EngagementTopic(engID), events.Event{
		Type: events.TypePresenceJoin,
		Data: json.RawMessage(`{"userId":"u1","secret":"hidden"}`),
	})

	ev1 := <-ch1
	var d1 map[string]any
	if err := json.Unmarshal(ev1.Data, &d1); err != nil {
		t.Fatal(err)
	}
	if _, ok := d1["secret"]; ok {
		t.Error("subscriber 1 saw 'secret' field, want stripped")
	}
	if !sub1Modified {
		t.Error("Modify was not called for subscriber 1")
	}

	ev2 := <-ch2
	var d2 map[string]any
	if err := json.Unmarshal(ev2.Data, &d2); err != nil {
		t.Fatal(err)
	}
	if _, ok := d2["secret"]; !ok {
		t.Error("subscriber 2 should see 'secret' field intact")
	}
}

func TestModifyDoesNotAffectOtherSubscribers(t *testing.T) {
	hub := events.NewHub(events.Options{})

	engID := "01900000-0000-7000-8000-0000000000a2"

	ch1, unsub1, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.EngagementTopic(engID)},
		Modify: func(ev events.Event) events.Event {
			ev.Data = json.RawMessage(`{"mutated":true}`)
			return ev
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer unsub1()

	ch2, unsub2, err := hub.Subscribe(t.Context(), events.Subscription{
		Topics: []string{events.EngagementTopic(engID)},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer unsub2()

	hub.Publish(events.EngagementTopic(engID), events.Event{
		Type: events.TypePresenceJoin,
		Data: json.RawMessage(`{"original":true}`),
	})

	ev2 := <-ch2
	var d2 map[string]any
	if err := json.Unmarshal(ev2.Data, &d2); err != nil {
		t.Fatal(err)
	}
	if _, ok := d2["mutated"]; ok {
		t.Error("subscriber 2 saw mutated data from subscriber 1's Modify")
	}
	if _, ok := d2["original"]; !ok {
		t.Error("subscriber 2 should see original data")
	}

	<-ch1
}
