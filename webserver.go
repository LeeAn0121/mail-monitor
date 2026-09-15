package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"sync"
	"time"
)

// webAddrEnvVar overrides the address the dashboard listens on
// (default webDefaultAddr). Set to "off" to disable the web server entirely.
const webAddrEnvVar = "MAIL_MONITOR_WEB_ADDR"
const webDefaultAddr = ":8080"

// webSnapshotEvents caps how many recent events the /api/snapshot response
// carries — the SSE stream keeps the live feed current after that, so this
// only needs to cover what a freshly opened dashboard shows before any new
// mail arrives.
const webSnapshotEvents = 300

//go:embed web/dist
var webDistFS embed.FS

// webEvent is the JSON shape sent to the dashboard — a flattened, string-only
// view of Event so the frontend never needs to know about EventType's int
// encoding.
type webEvent struct {
	When  string `json:"when"`
	Type  string `json:"type"`
	Glyph string `json:"glyph"`
	Text  string `json:"text"`
	From  string `json:"from"`
	To    string `json:"to"`
}

func toWebEvent(ev Event) webEvent {
	return webEvent{
		When:  ev.When,
		Type:  ev.Type.Label(),
		Glyph: ev.Type.Glyph(),
		Text:  ev.Text,
		From:  ev.From,
		To:    ev.To,
	}
}

type rankEntryPayload struct {
	Addr  string `json:"addr"`
	Count int    `json:"count"`
}

func toRankPayload(entries []rankEntry) []rankEntryPayload {
	out := make([]rankEntryPayload, 0, len(entries))
	for _, e := range entries {
		out = append(out, rankEntryPayload{Addr: e.addr, Count: e.count})
	}
	return out
}

type snapshotPayload struct {
	Events          []webEvent         `json:"events"`
	Counts          map[string]int     `json:"counts"`
	SenderRanking   []rankEntryPayload `json:"senderRanking"`
	ReceiverRanking []rankEntryPayload `json:"receiverRanking"`
	AlertActive     bool               `json:"alertActive"`
}

// webState mirrors the parts of model the dashboard needs, kept in a
// separate mutex-guarded copy because model itself lives on bubbletea's
// single-goroutine Update loop and must never be touched from the HTTP
// server's goroutines.
type webState struct {
	mu         sync.RWMutex
	events     []Event
	alertUntil time.Time
}

func newWebState() *webState {
	return &webState{}
}

func (s *webState) addEvent(ev Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
	if len(s.events) > maxEvents {
		s.events = s.events[len(s.events)-maxEvents:]
	}
}

func (s *webState) setAlertUntil(t time.Time) {
	s.mu.Lock()
	s.alertUntil = t
	s.mu.Unlock()
}

func (s *webState) snapshot() snapshotPayload {
	s.mu.RLock()
	defer s.mu.RUnlock()

	start := 0
	if len(s.events) > webSnapshotEvents {
		start = len(s.events) - webSnapshotEvents
	}
	recent := s.events[start:]

	events := make([]webEvent, 0, len(recent))
	counts := make(map[string]int, eventTypeCount)
	for _, e := range s.events {
		counts[e.Type.Label()]++
	}
	for _, e := range recent {
		events = append(events, toWebEvent(e))
	}

	return snapshotPayload{
		Events:          events,
		Counts:          counts,
		SenderRanking:   toRankPayload(firstN(rankSenders(s.events), 10)),
		ReceiverRanking: toRankPayload(firstN(rankReceivers(s.events), 10)),
		AlertActive:     time.Now().Before(s.alertUntil),
	}
}

func firstN(entries []rankEntry, n int) []rankEntry {
	if len(entries) > n {
		return entries[:n]
	}
	return entries
}

// sseHub fans out newly parsed events to every connected dashboard tab over
// Server-Sent Events. A slow or stalled subscriber gets events dropped
// (never blocks the sender) rather than backing up the broadcaster.
type sseHub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func newSSEHub() *sseHub {
	return &sseHub{subs: make(map[chan []byte]struct{})}
}

func (h *sseHub) subscribe() chan []byte {
	ch := make(chan []byte, 32)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *sseHub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *sseHub) broadcast(data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- data:
		default:
		}
	}
}

func (h *sseHub) broadcastEvent(ev Event) {
	data, err := json.Marshal(toWebEvent(ev))
	if err != nil {
		return
	}
	h.broadcast(data)
}

// webDistHandler serves the React dashboard build embedded at compile time
// via web/dist. A placeholder index.html ships in the repo so `go build`
// always has something to embed; `npm run build` in web/ overwrites it with
// the real bundle before a release.
func webDistHandler() http.Handler {
	sub, err := fs.Sub(webDistFS, "web/dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	return http.FileServer(http.FS(sub))
}

// webAddr resolves the dashboard listen address from MAIL_MONITOR_WEB_ADDR,
// falling back to webDefaultAddr. Returns "" if the web server should be
// disabled (env var set to "off").
func webAddr() string {
	v := os.Getenv(webAddrEnvVar)
	if v == "off" {
		return ""
	}
	if v == "" {
		return webDefaultAddr
	}
	return v
}

// startWebServer runs the dashboard's HTTP server in the background for the
// lifetime of the process. Errors (e.g. port already in use) are reported to
// stderr rather than crashing mail-monitor — the TUI keeps working either way.
func startWebServer(addr string, state *webState, hub *sseHub) {
	mux := http.NewServeMux()
	mux.Handle("/", webDistHandler())

	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state.snapshot())
	})

	mux.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		ch := hub.subscribe()
		defer hub.unsubscribe(ch)
		for {
			select {
			case data, ok := <-ch:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	})

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Fprintf(os.Stderr, "web server error: %v\n", err)
		}
	}()
}
