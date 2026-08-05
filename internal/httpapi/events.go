package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// SubscribeEvents opens a text/event-stream over the shared hub (M2-004).
//
// Authz (content.sync + session-only) is decided by the middleware before this
// runs. This method only validates topics, arms the subscription, and writes
// frames until the client disconnects or is evicted for falling behind.
//
// Last-Event-ID is accepted and ignored in M2 — no activity-log catch-up yet.
func (h *handlers) SubscribeEvents(ctx context.Context, request gen.SubscribeEventsRequestObject) (gen.SubscribeEventsResponseObject, error) {
	if h.hub == nil {
		return nil, apierr.Internal(errors.New("events hub is not configured"))
	}

	// Best-effort only in M2: accept the header so clients can send it, do not
	// replay. M4 owns guaranteed catch-up against the activity log.
	_ = request.Params.LastEventID

	var topics []string
	if request.Params.Topics != nil {
		topics = *request.Params.Topics
	}

	ch, unsub, err := h.hub.Subscribe(ctx, events.Subscription{Topics: topics})
	if err != nil {
		switch {
		case errors.Is(err, events.ErrUnknownTopic):
			return nil, apierr.Validation(apierr.Field("topics", err.Error()))
		case errors.Is(err, events.ErrNoTopics):
			return nil, apierr.Validation(apierr.Field("topics", "at least one topic is required"))
		case errors.Is(err, events.ErrTooManySubscribers):
			return nil, apierr.Internal(fmt.Errorf("event subscriber limit reached"))
		default:
			return nil, apierr.Internal(fmt.Errorf("subscribe: %w", err))
		}
	}
	pr, pw := io.Pipe()
	heartbeat := h.eventsHeartbeat
	if heartbeat <= 0 {
		heartbeat = 15 * time.Second
	}

	go streamEvents(ctx, pw, ch, unsub, heartbeat, h.log)

	cacheControl := "no-cache"
	contentType := "text/event-stream"
	return gen.SubscribeEvents200TexteventStreamResponse{
		Body: pr,
		Headers: gen.SubscribeEvents200ResponseHeaders{
			CacheControl: &cacheControl,
			ContentType:  &contentType,
		},
	}, nil
}

// streamEvents writes SSE frames until ctx ends, the hub closes ch, or a write
// fails. unsub always runs so a slow-client eviction and a clean disconnect
// both detach the hub entry.
func streamEvents(ctx context.Context, w *io.PipeWriter, ch <-chan events.Event, unsub func(), heartbeat time.Duration, log *slog.Logger) {
	defer unsub()
	defer w.Close()

	// Leading retry hint: browsers reconnect after this many ms on drop.
	if _, err := io.WriteString(w, "retry: 3000\n\n"); err != nil {
		return
	}

	tick := time.NewTicker(heartbeat)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			// Comment frame: keeps idle proxies from closing the socket.
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
		case ev, ok := <-ch:
			if !ok {
				// Evicted or unsubscribed.
				return
			}
			if err := writeSSE(w, ev); err != nil {
				if log != nil {
					log.DebugContext(ctx, "events: sse write ended", "error", err)
				}
				return
			}
		}
	}
}

func writeSSE(w io.Writer, ev events.Event) error {
	// id / event / data per the SSE spec. data is one JSON object.
	var b strings.Builder
	if ev.ID != "" {
		b.WriteString("id: ")
		b.WriteString(ev.ID)
		b.WriteByte('\n')
	}
	if ev.Type != "" {
		b.WriteString("event: ")
		b.WriteString(ev.Type)
		b.WriteByte('\n')
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	b.WriteString("data: ")
	b.Write(payload)
	b.WriteString("\n\n")
	_, err = io.WriteString(w, b.String())
	return err
}

// bridgeContentProgress maps the runner's in-process progress channel onto the
// hub. One process-lifetime goroutine; terminal statuses become
// content.job.terminal, everything else content.job.progress. Each tick is
// published on both the broadcast topic and the per-job topic.
func bridgeContentProgress(hub *events.Hub, ch <-chan content.ProgressEvent, unsub func(), log *slog.Logger) {
	defer unsub()
	for ev := range ch {
		typ := events.TypeContentJobProgress
		switch ev.Status {
		case storecontent.JobStatusSucceeded, storecontent.JobStatusFailed,
			storecontent.JobStatusCancelled, storecontent.JobStatusInterrupted:
			typ = events.TypeContentJobTerminal
		}
		data, err := json.Marshal(contentJobEventData{
			JobID:   ev.JobID,
			Phase:   ev.Phase,
			Current: ev.Current,
			Total:   ev.Total,
			Message: ev.Message,
			Status:  string(ev.Status),
		})
		if err != nil {
			if log != nil {
				log.Error("events: marshal content progress", "error", err, "job_id", ev.JobID)
			}
			continue
		}
		hub.Publish(events.TopicContentJobs, events.Event{Type: typ, Data: data})
		hub.Publish(events.TopicContentJob(ev.JobID), events.Event{Type: typ, Data: data})
	}
}

// contentJobEventData is the JSON body inside a content.job.* SSE event.
type contentJobEventData struct {
	JobID   string `json:"jobId"`
	Phase   string `json:"phase,omitempty"`
	Current int64  `json:"current"`
	Total   int64  `json:"total"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status"`
}

// eventsPath is the API path the timeout middleware must not deadline.
const eventsPath = BasePath + "/events"

// isSSEPath reports whether r is the long-lived events stream. Used by the
// timeout middleware to leave the request context unbounded.
func isSSEPath(r *http.Request) bool {
	return r.URL != nil && r.URL.Path == eventsPath
}
