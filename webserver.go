package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// webAddrEnvVar overrides the address the dashboard listens on
// (default webDefaultAddr). Set to "off" to disable the web server entirely.
const webAddrEnvVar = "MAIL_MONITOR_WEB_ADDR"
const webDefaultAddr = ":18080"

// webSnapshotEvents caps how many recent events the /api/snapshot response
// carries — the SSE stream keeps the live feed current after that, so this
// only needs to cover what a freshly opened dashboard shows before any new
// mail arrives.
const webSnapshotEvents = 300

// webRankingLimit caps sender/receiver ranking entries sent to the
// dashboard — generous since the panel scrolls internally now, rather than
// the old top-10 cutoff that existed only because the panel couldn't scroll.
const webRankingLimit = 50

//go:embed web/dist
var webDistFS embed.FS

// webEvent is the JSON shape sent to the dashboard — a flattened, string-only
// view of Event so the frontend never needs to know about EventType's int
// encoding.
type webEvent struct {
	When          string `json:"when"`
	Type          string `json:"type"`
	Glyph         string `json:"glyph"`
	From          string `json:"from"`
	To            string `json:"to"`
	FromDisplay   string `json:"fromDisplay"`
	ToDisplay     string `json:"toDisplay"`
	OrigTo        string `json:"origTo"`
	Subject       string `json:"subject"`
	ResultSummary string `json:"result"`
	ResultDetail  string `json:"resultDetail"`
	FromIP        string `json:"fromIp"`
	ToIP          string `json:"toIp"`
	Raw           string `json:"raw"`
}

func toWebEvent(ev Event) webEvent {
	return webEvent{
		When:          ev.When,
		Type:          ev.Type.Label(),
		Glyph:         ev.Type.Glyph(),
		From:          ev.From,
		To:            ev.To,
		FromDisplay:   ev.FromDisplay,
		ToDisplay:     ev.ToDisplay,
		OrigTo:        ev.OrigTo,
		Subject:       ev.Subject,
		ResultSummary: ev.ResultSummary,
		ResultDetail:  ev.ResultDetail,
		FromIP:        ev.FromIP,
		ToIP:          ev.ToIP,
		Raw:           ev.Raw,
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
		SenderRanking:   toRankPayload(firstN(rankSenders(s.events), webRankingLimit)),
		ReceiverRanking: toRankPayload(firstN(rankReceivers(s.events), webRankingLimit)),
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

// releaseURL links a version to its specific GitHub release tag. version is
// "dev" for a local (non-goreleaser) build, which has no tag to link to —
// the dashboard falls back to the releases index for that case.
func releaseURL(version string) string {
	if version == "" || version == "dev" {
		return "https://github.com/LeeAn0121/mail-monitor/releases"
	}
	return "https://github.com/LeeAn0121/mail-monitor/releases/tag/v" + version
}

// parseTimeParam parses an RFC3339 datetime query param (what the
// dashboard's date/time range picker sends), returning nil for an empty or
// unparseable value so /api/history's range filter is simply skipped.
func parseTimeParam(v string) *time.Time {
	if v == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil
	}
	return &t
}

// startWebServer runs the dashboard's HTTP server in the background for the
// lifetime of the process. Errors (e.g. port already in use) are reported to
// stderr rather than crashing mail-monitor — the TUI keeps working either way.
func startWebServer(addr string, state *webState, hub *sseHub, db *sql.DB) {
	mux := http.NewServeMux()
	mux.Handle("/", webDistHandler())

	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"version":     version,
			"releaseUrl":  releaseURL(version),
			"releasesUrl": "https://github.com/LeeAn0121/mail-monitor/releases",
		})
	})

	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(state.snapshot())
	})

	// /api/history scans logPath + rotated logs on disk (via the same
	// searchHistory the TUI's `/` search uses) — unlike /api/snapshot, this
	// isn't limited to the in-memory event buffer, so it can find mail from
	// before mail-monitor started or from earlier days.
	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		from := parseTimeParam(r.URL.Query().Get("from"))
		to := parseTimeParam(r.URL.Query().Get("to"))
		res := searchHistory(db, q, from, to)
		w.Header().Set("Content-Type", "application/json")
		if res.err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": res.err.Error()})
			return
		}
		events := make([]webEvent, 0, len(res.events))
		for _, e := range res.events {
			events = append(events, toWebEvent(e))
		}
		json.NewEncoder(w).Encode(map[string]any{"events": events})
	})

	// /api/users lists the `users` directory table (email, name) — read-only
	// browser for the same table resolveName already queries to attach
	// display names to addresses elsewhere in the dashboard. Empty (not an
	// error) when MAIL_MONITOR_DB_DSN isn't configured.
	mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		users, err := listUsers(db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"users": users, "enabled": db != nil})
	})

	// /api/forwardings manages the `forwardings` table (source, destination
	// columns), joined against `users` for display names on either side.
	mux.HandleFunc("/api/forwardings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			list, err := listForwardings(db)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"forwardings": list, "enabled": db != nil})

		case http.MethodPost:
			var body struct {
				Source      string `json:"source"`
				Destination string `json:"destination"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "잘못된 요청입니다"})
				return
			}
			already, err := addForwarding(db, strings.TrimSpace(body.Source), strings.TrimSpace(body.Destination))
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"alreadyExists": already})

		case http.MethodDelete:
			source := strings.TrimSpace(r.URL.Query().Get("source"))
			destination := strings.TrimSpace(r.URL.Query().Get("destination"))
			found, err := deleteForwarding(db, source, destination)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"found": found})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	// /api/blocklist manages /etc/postfix/header_checks REJECT rules — the
	// same file and line format /usr/local/bin/block-sender uses, so either
	// one sees what the other added.
	mux.HandleFunc("/api/blocklist", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			list, err := listBlockedSenders()
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"blocked": list})

		case http.MethodPost:
			var body struct {
				Email string `json:"email"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "잘못된 요청입니다"})
				return
			}
			already, err := blockSender(strings.TrimSpace(body.Email))
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"alreadyBlocked": already})

		case http.MethodDelete:
			email := strings.TrimSpace(r.URL.Query().Get("email"))
			found, err := unblockSender(email)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"found": found})

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
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
