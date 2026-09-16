package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"golang.org/x/text/encoding/htmlindex"
)

var version = "dev"

const (
	logPath   = "/var/log/mail.log"
	maxEvents = 5000
)

type EventType int

const (
	EventLogin EventType = iota
	EventRecv
	EventSent
	EventForward
	EventBounce
	EventReject
	eventTypeCount
)

func (e EventType) Label() string {
	switch e {
	case EventLogin:
		return "LOGIN"
	case EventRecv:
		return "RECV"
	case EventSent:
		return "SENT"
	case EventForward:
		return "FWD"
	case EventBounce:
		return "BOUNCE"
	case EventReject:
		return "REJECT"
	}
	return "?"
}

// Glyph gives each event type a distinct shape, not just a color, so the
// log stays readable for colorblind users (RECV green vs. BOUNCE red is a
// hard pair for red-green colorblindness to tell apart on hue alone).
func (e EventType) Glyph() string {
	switch e {
	case EventLogin:
		return "●"
	case EventRecv:
		return "▼"
	case EventSent:
		return "▲"
	case EventForward:
		return "↪"
	case EventBounce:
		return "✕"
	case EventReject:
		return "■"
	}
	return "?"
}

type Event struct {
	When string // e.g. "08-04 15:34:59", taken from the log line itself
	Type EventType
	Text string
	Raw  string
	From string // raw sender address, undecorated (no name/IP) — for aggregation like the sender ranking view
	To   string // raw recipient address, undecorated — for aggregation like the receiver ranking view

	// Subject/ResultSummary/ResultDetail/FromIP/ToIP/OrigTo are the web
	// dashboard's structured columns (시간/유형/발신/수신/원본수신/내용/
	// 처리결과/발신자IP/수신자IP) — Text stays the TUI's single rendered
	// line, which mixes several of these together. OrigTo is only set for
	// FWD events: the alias/address the mail was originally addressed to
	// before forwarding put it in To. ResultSummary is the short status
	// shown in the table (e.g. "발송 완료"); ResultDetail is postfix's own
	// full status text, shown only in the row detail popup.
	Subject       string
	ResultSummary string
	ResultDetail  string
	FromIP        string
	ToIP          string
	OrigTo        string

	// FromDisplay/ToDisplay are From/To with a resolved display name
	// appended ("addr@example.com (홍길동)") when the users table (see
	// resolveName) has one — set by withNames, which every construction
	// site chains after withDetail.
	FromDisplay string
	ToDisplay   string

	// rawLower/textLower cache strings.ToLower(Raw)/(Text), computed once at
	// construction, so matchesFilter doesn't re-lowercase every event on
	// every keystroke/render pass.
	rawLower  string
	textLower string
}

// withDetail fills in the web dashboard's structured columns that the TUI's
// Text field doesn't carry. Chainable off newEvent so call sites stay
// one-liners: newEvent(...).withDetail(...).
func (e *Event) withDetail(subject, resultSummary, resultDetail, fromIP, toIP, origTo string) *Event {
	e.Subject, e.ResultSummary, e.ResultDetail = subject, resultSummary, resultDetail
	e.FromIP, e.ToIP, e.OrigTo = fromIP, toIP, origTo
	return e
}

// withNames resolves display names for From/To via the users table (MySQL,
// optional — see resolveName) into FromDisplay/ToDisplay, for the web
// dashboard's 발신/수신 columns. Takes m explicitly (rather than being a
// method on model) so it can wrap a return statement: withNames(m, newEvent(...)).
func withNames(m *model, ev *Event) *Event {
	ev.FromDisplay = m.nameSuffix(ev.From)
	ev.ToDisplay = m.nameSuffix(ev.To)
	return ev
}

// newEvent builds an Event and precomputes its lowercase filter-match cache.
func newEvent(when string, typ EventType, raw, from, to, text string) *Event {
	return &Event{When: when, Type: typ, Raw: raw, From: from, To: to, Text: text,
		rawLower: strings.ToLower(raw), textLower: strings.ToLower(text)}
}

// syslogTsRe captures a line's leading syslog timestamp ("Aug  4 15:34:59").
var syslogTsRe = regexp.MustCompile(`^(\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})`)

// extractWhen reads the real timestamp off the log line rather than using
// wall-clock time, so history search results (which can be processed long
// after the fact, from rotated files spanning many days) show when the
// event actually happened, not when mail-monitor happened to read it.
func extractWhen(line string) string {
	m := syslogTsRe.FindStringSubmatch(line)
	if m == nil {
		return time.Now().Format("01-02 15:04:05")
	}
	t, err := time.Parse("Jan _2 15:04:05", m[1])
	if err != nil {
		return time.Now().Format("01-02 15:04:05")
	}
	return t.Format("01-02 15:04:05")
}

var (
	// captures the postfix queue id and the remainder of the line after "QID: "
	// e.g. "... postfix/qmgr[2568950]: 933E0AC0470: from=<...>, size=..." -> ("933E0AC0470", "from=<...>, size=...")
	qidLineRe = regexp.MustCompile(`^\S+ +\d+ +\S+ +\S+ +\S+: ([0-9A-F]{9,14}): (.*)$`)

	loginRe        = regexp.MustCompile(`dovecot.*(?:auth.*Success|imap-login.*Login)`)
	userRe         = regexp.MustCompile(`user=<([^>]*)>`)
	ripRe          = regexp.MustCompile(`rip=([0-9.]+)`)
	fromRe         = regexp.MustCompile(`from=<([^>]*)>`)
	toRe           = regexp.MustCompile(`to=<([^>]*)>`)
	origToRe       = regexp.MustCompile(`orig_to=<([^>]*)>`)
	relayRe        = regexp.MustCompile(`relay=([^,\s]*)`)
	rejectRe       = regexp.MustCompile(`reject:\s*([^;]*)`)
	bounceReasonRe = regexp.MustCompile(`status=bounced \((.*)\)`)
	sentDetailRe   = regexp.MustCompile(`status=sent \((.*)\)`)
	clientRe       = regexp.MustCompile(`client=\S+\[(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\]`)
	bracketIP      = regexp.MustCompile(`\[(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\]`)
	subjectRe      = regexp.MustCompile(`warning: header Subject: (.*?) from \S+\[[0-9.]+\];`)
)

func extract(re *regexp.Regexp, line string) string {
	m := re.FindStringSubmatch(line)
	if len(m) < 2 {
		return "-"
	}
	if m[1] == "" {
		return "<>"
	}
	return m[1]
}

// isLocalRelay reports whether a postfix relay= value is a local delivery
// agent (virtual/local/lmtp/spamassassin/...) rather than a remote host,
// which always appears as "host[ip]:port".
func isLocalRelay(relay string) bool {
	return !strings.Contains(relay, "[")
}

// fromWithIP formats a sender address with its client IP, when known.
func fromWithIP(from, ip string) string {
	if ip == "" {
		return from
	}
	return fmt.Sprintf("%s (%s)", from, ip)
}

// toWithForward notes when postfix delivered to a different mailbox than
// the message was originally addressed to — an alias or forwarding rule
// expanded it (postfix logs this as orig_to=, alongside the final to=).
func toWithForward(to, origTo string) string {
	if origTo == "" || origTo == to {
		return to
	}
	return fmt.Sprintf("%s (%s에서 전달됨)", to, origTo)
}

// srsRe matches an SRS-rewritten bounce address (SRS0=hash=TT=domain=local@relay),
// which postfix/OpenSMTPD generates so bounces route back through the
// forwarding relay. The rewritten form is long and mostly noise for a human
// reader — shortenSRS pulls out the original local@domain it was rewritten
// from so the display stays short.
var srsRe = regexp.MustCompile(`^SRS0=[^=]+=[^=]+=([^=]+)=([^@]+)@`)

func shortenSRS(addr string) string {
	if m := srsRe.FindStringSubmatch(addr); m != nil {
		return m[2] + "@" + m[1]
	}
	return addr
}

// mimeWordDecoder decodes RFC 2047 encoded-words (e.g. "=?ks_c_5601-1987?B?...?=",
// common for Korean subjects) into UTF-8, resolving legacy MIME charset
// names like ks_c_5601-1987 (EUC-KR) via the IANA charset registry.
var mimeWordDecoder = &mime.WordDecoder{
	CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
		enc, err := htmlindex.Get(charset)
		if err != nil || enc == nil {
			return input, nil
		}
		return enc.NewDecoder().Reader(input), nil
	},
}

// foldArtifactRe matches the mangled junction Postfix leaves when it logs a
// header that was folded across multiple lines: it replaces the folding
// CRLF with literal "?" characters, turning "...?=\r\n =?UTF-8?B?..." into
// "...?=??=?UTF-8?B?..." (or a single "?" plus real whitespace). Both
// variants break RFC 2047 parsing, so normalize them back to a plain space
// before decoding — adjacent encoded-words separated only by whitespace are
// concatenated per spec, same as the original unfolded header intended.
var foldArtifactRe = regexp.MustCompile(`\?=\?+\s*=\?`)

// decodeSubject best-effort decodes a raw header value; on any failure it
// falls back to the original (still-encoded) text rather than dropping it.
func decodeSubject(s string) string {
	s = foldArtifactRe.ReplaceAllString(s, "?= =?")
	if decoded, err := mimeWordDecoder.DecodeHeader(s); err == nil {
		return decoded
	}
	return s
}

// withSubject appends a truncated [Subject] as its own continuation line,
// rather than tacking it onto the sender/recipient line, so a long address
// and a long subject don't compete for the same line width.
func withSubject(text, subject string) string {
	if subject == "" {
		return text
	}
	const maxLen = 60
	if r := []rune(subject); len(r) > maxLen {
		subject = string(r[:maxLen]) + "…"
	}
	return fmt.Sprintf("%s\n[%s]", text, subject)
}

// processLine parses one mail.log line into an Event. Postfix logs a
// message's from=, client IP, and eventual to=/status= on separate lines
// that share only a Queue-ID, so qidFrom/qidIP correlate them across lines.
func (m *model) processLine(line string) *Event {
	when := extractWhen(line)

	if qm := qidLineRe.FindStringSubmatch(line); qm != nil {
		qid, rest := qm[1], qm[2]

		if from := extract(fromRe, rest); from != "-" {
			m.qidFrom[qid] = from
		}
		if cm := clientRe.FindStringSubmatch(rest); cm != nil {
			m.qidIP[qid] = cm[1]
		}
		if sm := subjectRe.FindStringSubmatch(rest); sm != nil {
			m.qidSubject[qid] = decodeSubject(sm[1])
		}
		if strings.Contains(rest, ": removed") || rest == "removed" {
			delete(m.qidFrom, qid)
			delete(m.qidIP, qid)
			delete(m.qidSubject, qid)
			return nil
		}
		if len(m.qidFrom) > 20000 {
			m.qidFrom = make(map[string]string)
			m.qidIP = make(map[string]string)
			m.qidSubject = make(map[string]string)
		}

		toRaw := extract(toRe, rest)
		if toRaw == "-" {
			return nil
		}
		finalTo := m.addr(toRaw)
		to := finalTo
		forwarded := false
		var origTo string
		if om := origToRe.FindStringSubmatch(rest); om != nil && om[1] != "" {
			origTo = m.addr(om[1])
			forwarded = origTo != finalTo
			to = toWithForward(finalTo, origTo)
		}
		from, ok := m.qidFrom[qid]
		if !ok {
			from = "-"
		}
		fromDisplay := fromWithIP(m.addr(shortenSRS(from)), m.qidIP[qid])
		subject := m.qidSubject[qid]

		switch {
		case strings.Contains(rest, "status=bounced"):
			reason := extract(bounceReasonRe, rest)
			detail := reason
			if detail == "-" {
				detail = "반송"
			}
			return withNames(m, newEvent(when, EventBounce, line, from, toRaw,
				withSubject(fmt.Sprintf("발신: %s → 수신: %s", fromDisplay, to), subject)).
				withDetail(subject, "반송", detail, m.qidIP[qid], "", ""))
		case strings.Contains(rest, "status=sent"):
			relay := extract(relayRe, rest)
			detail := extract(sentDetailRe, rest)
			if isLocalRelay(relay) {
				typ := EventRecv
				summary := "수신 완료"
				evOrigTo := ""
				if forwarded {
					typ = EventForward
					summary = "전달 완료"
					evOrigTo = origTo
				}
				if detail == "-" {
					detail = summary
				}
				return withNames(m, newEvent(when, typ, line, from, toRaw,
					withSubject(fmt.Sprintf("발신: %s → 수신: %s", fromDisplay, to), subject)).
					withDetail(subject, summary, detail, m.qidIP[qid], "", evOrigTo))
			}
			if detail == "-" {
				detail = "발송 완료"
			}
			relayIP := extract(bracketIP, relay)
			return withNames(m, newEvent(when, EventSent, line, from, toRaw,
				withSubject(fmt.Sprintf("발신: %s → 수신: %s (via %s)", fromDisplay, to, relay), subject)).
				withDetail(subject, "발송 완료", detail, m.qidIP[qid], relayIP, ""))
		}
		return nil
	}

	switch {
	case loginRe.MatchString(line):
		user := m.addr(extract(userRe, line))
		rip := extract(ripRe, line)
		return withNames(m, newEvent(when, EventLogin, line, "", "", fmt.Sprintf("%s from %s", user, rip)).
			withDetail("", "로그인 성공", "로그인 성공", rip, "", ""))
	case strings.Contains(line, "reject:"):
		fromRaw := extract(fromRe, line)
		from := m.addr(shortenSRS(fromRaw))
		toRaw := extract(toRe, line)
		to := m.addr(toRaw)
		reason := extract(rejectRe, line)
		detail := reason
		if detail == "-" {
			detail = "거부"
		}
		ip := ""
		if im := bracketIP.FindStringSubmatch(line); im != nil {
			ip = im[1]
		}
		return withNames(m, newEvent(when, EventReject, line, fromRaw, toRaw,
			fmt.Sprintf("발신: %s → 수신: %s (%s)", fromWithIP(from, ip), to, reason)).
			withDetail("", "거부", detail, ip, "", ""))
	}
	return nil
}

// --- log tailing ---

type logLineMsg []string
type tailErrMsg error
type tickMsg time.Time

// todayDatePrefix returns today's date in the same "Jan _2" form syslogTsRe
// captures, so history replay can match it against each line's leading
// timestamp without a full time.Parse round-trip per line.
func todayDatePrefix() string {
	return time.Now().Format("Jan _2")
}

// nativeTail replays today's lines of logPath as history — the whole day so
// far, not a fixed line count, since startup is meant to answer "what's
// happened today" — then follows only new appends from that point on; it
// never rescans or reprints old lines during live-follow. This also avoids
// the startup race of handing off to an external `tail` process (which may
// not have attached before the first lines land) and lets us read the file
// directly when permissions allow (no sudo).
func nativeTail(ch chan<- string, errCh chan<- error) {
	f, err := os.Open(logPath)
	if err != nil {
		errCh <- err
		return
	}
	defer f.Close()

	if _, err := f.Stat(); err == nil {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		today := todayDatePrefix()
		for scanner.Scan() {
			line := scanner.Text()
			if m := syslogTsRe.FindStringSubmatch(line); m != nil && strings.HasPrefix(m[1], today) {
				ch <- line
			}
		}
	}
	f.Seek(0, io.SeekEnd)

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err == nil {
			ch <- strings.TrimRight(line, "\n")
			continue
		}
		if fi, statErr := os.Stat(logPath); statErr == nil {
			if cur, _ := f.Seek(0, io.SeekCurrent); fi.Size() < cur {
				f.Seek(0, io.SeekStart)
				reader = bufio.NewReader(f)
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func subprocessTail(ch chan<- string, errCh chan<- error) {
	// Mirrors nativeTail's today-only history replay when direct read isn't
	// permitted: grep today's lines via sudo first, then follow new appends.
	if out, err := exec.Command("sudo", "grep", "-a", "^"+todayDatePrefix(), logPath).Output(); err == nil {
		for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
			if l != "" {
				ch <- l
			}
		}
	}
	cmd := exec.Command("sudo", "tail", "-F", "-n", "0", logPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		errCh <- err
		return
	}
	if err := cmd.Start(); err != nil {
		errCh <- err
		return
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		ch <- scanner.Text()
	}
}

func startTail(ch chan<- string, errCh chan<- error) {
	if f, err := os.Open(logPath); err == nil {
		f.Close()
		nativeTail(ch, errCh)
		return
	}
	subprocessTail(ch, errCh)
}

// --- history search (scans logPath + rotated logs on disk) ---

const maxHistoryResults = 3000

var rotatedSuffixRe = regexp.MustCompile(`\.(\d+)(\.gz)?$`)

// listRotatedLogs returns logPath and any logrotate-style rotated
// siblings (mail.log.1, mail.log.2.gz, ...), oldest first, current
// logPath last — chronological order for search results.
func listRotatedLogs() []string {
	matches, _ := filepath.Glob(logPath + ".*")
	type entry struct {
		path string
		n    int
	}
	var entries []entry
	for _, p := range matches {
		m := rotatedSuffixRe.FindStringSubmatch(p)
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		entries = append(entries, entry{p, n})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].n > entries[j].n })

	files := make([]string, 0, len(entries)+1)
	for _, e := range entries {
		files = append(files, e.path)
	}
	if _, err := os.Stat(logPath); err == nil {
		files = append(files, logPath)
	}
	return files
}

// cmdReadCloser adapts a subprocess's stdout pipe to io.ReadCloser, waiting
// on the process (to release its resources) when the reader is closed.
type cmdReadCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdReadCloser) Close() error {
	c.ReadCloser.Close()
	return c.cmd.Wait()
}

// openForScan opens path directly when readable, same as os.Open. When that
// fails on a permission error, it falls back to `sudo tail -c +1` — the same
// binary (and the same NOPASSWD sudoers rule, see README) startTail already
// relies on for the live log, so history search works without requiring a
// second sudoers entry for cat/zcat. `-c +1` reads the whole file from byte
// 1 as raw bytes (unlike `-n`, byte offsets don't risk mis-splitting binary
// gzip data), so the result is identical to os.Open — .gz still needs
// gzip.NewReader on top either way.
func openForScan(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err == nil {
		return f, nil
	}
	if !os.IsPermission(err) {
		return nil, err
	}
	cmd := exec.Command("sudo", "-n", "tail", "-c", "+1", path)
	stdout, perr := cmd.StdoutPipe()
	if perr != nil {
		return nil, err
	}
	if perr := cmd.Start(); perr != nil {
		return nil, err
	}
	return &cmdReadCloser{stdout, cmd}, nil
}

// scanFile reads path line by line, transparently decompressing .gz.
func scanFile(path string, fn func(line string)) error {
	rc, err := openForScan(path)
	if err != nil {
		return err
	}
	defer rc.Close()

	var r io.Reader = rc
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(rc)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		fn(scanner.Text())
	}
	return scanner.Err()
}

type historyResultsMsg struct {
	query  string
	events []Event
	err    error
}

// parseEventWhen recovers a full time.Time from an Event.When string
// ("MM-DD HH:MM:SS", no year — see extractWhen) for date-range filtering.
// Since the log itself never records a year, this assumes ref's year and
// rolls back one year if that would put the timestamp more than a day in
// ref's future — the one case that actually happens in practice is a
// rotated log from late December being searched in early January.
func parseEventWhen(when string, ref time.Time) (time.Time, bool) {
	t, err := time.Parse("01-02 15:04:05", when)
	if err != nil {
		return time.Time{}, false
	}
	t = time.Date(ref.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, ref.Location())
	if t.After(ref.Add(24 * time.Hour)) {
		t = t.AddDate(-1, 0, 0)
	}
	return t, true
}

// searchHistory scans logPath and its rotated siblings for events matching
// query, using a correlation state independent of the live model's (own
// qidFrom/qidIP/qidSubject/nameCache) so it can run concurrently on its own
// goroutine without racing the live view. It shares the *sql.DB handle,
// which is safe for concurrent use. from/to optionally bound the search to
// a date range (either or both may be nil); the TUI's own `/` search always
// passes nil, nil.
func searchHistory(db *sql.DB, query string, from, to *time.Time) historyResultsMsg {
	sm := &model{
		qidFrom:    make(map[string]string),
		qidIP:      make(map[string]string),
		qidSubject: make(map[string]string),
		nameCache:  make(map[string]string),
		db:         db,
	}

	needle := strings.ToLower(query)
	now := time.Now()
	// ring holds only the last maxHistoryResults matches. Rather than
	// shifting the slice left on every overflow (O(n) per match, O(n²) over
	// a large scan), write into a fixed-size circular buffer — O(1) per
	// match — and reorder into chronological order once at the end.
	ring := make([]Event, maxHistoryResults)
	total := 0
	var lastErr error
	opened := 0

	for _, path := range listRotatedLogs() {
		err := scanFile(path, func(line string) {
			ev := sm.processLine(line)
			if ev == nil {
				return
			}
			if needle != "" && !strings.Contains(ev.rawLower, needle) &&
				!strings.Contains(ev.textLower, needle) {
				return
			}
			if from != nil || to != nil {
				t, ok := parseEventWhen(ev.When, now)
				if !ok {
					return
				}
				if from != nil && t.Before(*from) {
					return
				}
				if to != nil && t.After(*to) {
					return
				}
			}
			ring[total%maxHistoryResults] = *ev
			total++
		})
		if err != nil {
			lastErr = err
			continue
		}
		opened++
	}

	if opened == 0 && lastErr != nil {
		return historyResultsMsg{query: query, err: lastErr}
	}
	var results []Event
	if total <= maxHistoryResults {
		results = ring[:total]
	} else {
		start := total % maxHistoryResults
		results = append(results, ring[start:]...)
		results = append(results, ring[:start]...)
	}
	return historyResultsMsg{query: query, events: results}
}

func searchHistoryCmd(db *sql.DB, query string) tea.Cmd {
	return func() tea.Msg {
		return searchHistory(db, query, nil, nil)
	}
}

// waitForLine blocks for the first available line, then drains any further
// lines already buffered on the channel (non-blocking) into the same batch,
// up to lineBatchLimit. A busy server can log faster than bubbletea's
// Update/View cycle — without batching, each line triggers its own full
// refreshViewport() (rescans/regroups every visible event), which turns a
// burst of N lines into O(N * visibleEvents) work instead of O(N + one
// refresh).
const lineBatchLimit = 200

func waitForLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		batch := []string{<-ch}
		for len(batch) < lineBatchLimit {
			select {
			case l := <-ch:
				batch = append(batch, l)
			default:
				return logLineMsg(batch)
			}
		}
		return logLineMsg(batch)
	}
}

func waitForErr(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		return tailErrMsg(<-ch)
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// --- name directory (optional MySQL lookup) ---

const dsnEnvVar = "MAIL_MONITOR_DB_DSN"

func openDirectory() *sql.DB {
	dsn := os.Getenv(dsnEnvVar)
	if dsn == "" {
		return nil
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil
	}
	return db
}

// resolveName looks up a display name for an email address in the `users`
// table (email, name columns), caching results (including misses) so a
// forwarding alias fanning out to many recipients only queries each once.
func (m *model) resolveName(email string) string {
	if m.db == nil || email == "" || email == "-" || email == "<>" {
		return ""
	}
	if name, ok := m.nameCache[email]; ok {
		return name
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	var name string
	err := m.db.QueryRowContext(ctx, "SELECT name FROM users WHERE email = ?", email).Scan(&name)
	if err != nil {
		name = ""
	}
	m.nameCache[email] = name
	return name
}

// addr renders an address as "Name <email>" when a name is known.
func (m *model) addr(email string) string {
	if name := m.resolveName(email); name != "" {
		return fmt.Sprintf("%s <%s>", name, email)
	}
	return email
}

// nameSuffix renders an address as "email (Name)" when a name is known —
// the web dashboard's 발신/수신 column format, email-first so it still sorts
// and filters the same as the raw address.
func (m *model) nameSuffix(email string) string {
	if name := m.resolveName(email); name != "" {
		return fmt.Sprintf("%s (%s)", email, name)
	}
	return email
}

// --- keymap ---

type keyMap struct {
	Filter   key.Binding
	History  key.Binding
	Rank     key.Binding
	RankRecv key.Binding
	Export   key.Binding
	Toggle   key.Binding
	Pause    key.Binding
	Clear    key.Binding
	Bottom   key.Binding
	Up       key.Binding
	Down     key.Binding
	Help     key.Binding
	Quit     key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Filter, k.History, k.Rank, k.Toggle, k.Pause, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Filter, k.History, k.Rank, k.RankRecv, k.Export, k.Toggle, k.Pause, k.Clear},
		{k.Up, k.Down, k.Bottom},
		{k.Help, k.Quit},
	}
}

var keys = keyMap{
	Filter:   key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "필터(버퍼)")),
	History:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "이력 검색")),
	Rank:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "발신량 랭킹")),
	RankRecv: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "수신량 랭킹")),
	Export:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "CSV 내보내기")),
	Toggle:   key.NewBinding(key.WithKeys("1", "2", "3", "4", "5", "6"), key.WithHelp("1-6", "이벤트 토글")),
	Pause:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "정지/재개")),
	Clear:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "클리어")),
	Bottom:   key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "최신으로")),
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "스크롤")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "스크롤")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "도움말")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "종료")),
}

// --- model ---

type model struct {
	events      []Event
	counts      [eventTypeCount]int
	enabled     [eventTypeCount]bool
	filter      string
	filterInput textinput.Model
	filtering   bool
	paused      bool
	now         time.Time
	err         error
	lineCh      chan string
	errCh       chan error
	width       int
	height      int
	dayStart    time.Time

	viewport   viewport.Model
	ready      bool
	followTail bool

	help    help.Model
	showAll bool

	qidFrom    map[string]string
	qidIP      map[string]string
	qidSubject map[string]string

	db        *sql.DB
	nameCache map[string]string

	hsInput   textinput.Model
	hsTyping  bool
	hsActive  bool
	hsLoading bool
	hsQuery   string
	hsResults []Event
	hsErr     error

	rankActive bool
	rankByRecv bool // false = sender ranking (SENT), true = receiver ranking (RECV/FORWARD)

	// exportMsg/exportAt show a transient status-bar confirmation (or error)
	// after pressing e; the message clears itself once exportMsgTTL elapses.
	exportMsg string
	exportAt  time.Time

	spark     sparkline.Model
	lastTotal int

	// lastBadTotal/alertUntil drive a spike warning: when BOUNCE+REJECT grows
	// by bounceRejectSpikeThreshold or more within one tick (a burst, not
	// steady background noise), a warning badge shows for alertBadgeTTL.
	lastBadTotal int
	alertUntil   time.Time

	// pendingNew counts events buffered while paused or scrolled away from
	// the tail (followTail == false) — a "new events" cue so nothing that
	// arrived off-screen goes unnoticed. Reset once the view returns to the
	// live tail.
	pendingNew int

	// webState/webHub feed the live web dashboard: every parsed event and
	// spike-alert update mirrors into them alongside the TUI's own state.
	webState *webState
	webHub   *sseHub
}

func initialModel(ws *webState, hub *sseHub, db *sql.DB) model {
	ti := textinput.New()
	ti.Placeholder = "user@domain.com"
	ti.CharLimit = 128
	ti.Prompt = "필터: "

	hsi := textinput.New()
	hsi.Placeholder = "user@domain.com / IP / 제목 키워드"
	hsi.CharLimit = 128
	hsi.Prompt = "이력 검색: "

	// LOGIN starts hidden: dovecot logs a login+logout pair per IMAP
	// session, so it dominates the list by volume. Counts still track it;
	// press 1 to show it.
	var enabled [eventTypeCount]bool
	for i := range enabled {
		enabled[i] = true
	}
	enabled[EventLogin] = false

	h := help.New()

	return model{
		enabled:     enabled,
		filterInput: ti,
		now:         time.Now(),
		lineCh:      make(chan string, 256),
		errCh:       make(chan error, 4),
		dayStart:    time.Now(),
		followTail:  true,
		help:        h,
		qidFrom:     make(map[string]string),
		qidIP:       make(map[string]string),
		qidSubject:  make(map[string]string),
		db:          db,
		nameCache:   make(map[string]string),
		hsInput:     hsi,
		spark:       sparkline.New(40, 2, sparkline.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("214")))),
		webState:    ws,
		webHub:      hub,
	}
}

func (m model) Init() tea.Cmd {
	go startTail(m.lineCh, m.errCh)
	return tea.Batch(waitForLine(m.lineCh), waitForErr(m.errCh), tick(), textinput.Blink)
}

// matchesFilter checks the raw log line (addresses, IPs, hostnames) and the
// rendered event text (decoded subject, resolved names) so search covers
// both what postfix logged and what mail-monitor derived from it.
func (m model) matchesFilter(e Event) bool {
	if m.filter == "" {
		return true
	}
	needle := strings.ToLower(m.filter)
	return strings.Contains(e.rawLower, needle) ||
		strings.Contains(e.textLower, needle)
}

// visibleEvents returns events passing the current type/filter settings.
func (m model) visibleEvents() []Event {
	out := make([]Event, 0, len(m.events))
	for _, e := range m.events {
		if m.enabled[e.Type] && m.matchesFilter(e) {
			out = append(out, e)
		}
	}
	return out
}

const (
	headerHeight = 12
	footerHeight = 2
)

// bounceRejectSpikeThreshold/alertBadgeTTL tune the spike-warning badge:
// a burst of at least this many new BOUNCE+REJECT events within one tick
// (1s) triggers the badge, which then stays up for alertBadgeTTL so a
// glance at the screen a few seconds later still catches it.
const (
	bounceRejectSpikeThreshold = 3
	alertBadgeTTL              = 10 * time.Second
)

// trafficLabel prefixes the header sparkline (a real ntcharts sparkline, not
// a hand-drawn bar) that tracks total events/sec so a traffic spike — a
// broadcast landing, a flood of LOGIN retries — is visible before scrolling
// the log at all.
const trafficLabel = "TRAFFIC "

// historyVisible applies the current type toggles (1-5) to search results;
// the query text itself already narrowed them at scan time.
func (m model) historyVisible() []Event {
	out := make([]Event, 0, len(m.hsResults))
	for _, e := range m.hsResults {
		if m.enabled[e.Type] {
			out = append(out, e)
		}
	}
	return out
}

type rankEntry struct {
	addr  string
	count int
}

// rankByKey aggregates events matching keep, keyed by key(e), highest count
// first — the shared engine behind both the sender and receiver rankings.
func rankByKey(events []Event, keep func(Event) bool, key func(Event) string) []rankEntry {
	counts := make(map[string]int)
	for _, e := range events {
		if !keep(e) {
			continue
		}
		if k := key(e); k != "" && k != "-" {
			counts[k]++
		}
	}
	out := make([]rankEntry, 0, len(counts))
	for addr, n := range counts {
		out = append(out, rankEntry{addr, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].count != out[j].count {
			return out[i].count > out[j].count
		}
		return out[i].addr < out[j].addr
	})
	return out
}

// rankSenders aggregates SENT event counts by raw sender address, highest
// first — a compromised account blasting spam shows up at the top.
func rankSenders(events []Event) []rankEntry {
	return rankByKey(events,
		func(e Event) bool { return e.Type == EventSent },
		func(e Event) string { return e.From })
}

// rankReceivers aggregates RECV/FORWARD event counts by raw recipient
// address, highest first — surfaces mailboxes receiving unusual volume,
// including ones only reachable via a forwarding alias.
func rankReceivers(events []Event) []rankEntry {
	return rankByKey(events,
		func(e Event) bool { return e.Type == EventRecv || e.Type == EventForward },
		func(e Event) string { return e.To })
}

// rankBarColor shades bars from the busiest sender (alert red-orange) down
// to the quietest (the normal SENT color), so a runaway sender — the
// clearest sign of a compromised account — visually jumps out without
// having to read the numbers.
func rankBarColor(rank, total int) lipgloss.Color {
	if total <= 1 || rank == 0 {
		return lipgloss.Color("196")
	}
	t := float64(rank) / float64(total-1)
	// interpolate 196 (red) -> 214 (amber/SENT) across the field
	return lipgloss.Color(fmt.Sprintf("%d", 196+int(t*18)))
}

const maxRankBars = 15

// renderRanking draws sender volume as a real horizontal bar chart
// (ntcharts) instead of a plain numbered list, so the relative gap between
// the top sender and the rest reads at a glance.
func renderRanking(entries []rankEntry, width int, emptyMsg string) string {
	if len(entries) == 0 {
		return dimStyle.Render("  " + emptyMsg)
	}
	if len(entries) > maxRankBars {
		entries = entries[:maxRankBars]
	}
	maxLabel := 0
	for _, e := range entries {
		if l := lipgloss.Width(e.addr); l > maxLabel {
			maxLabel = l
		}
	}
	if maxLabel > 34 {
		maxLabel = 34
	}
	bars := make([]barchart.BarData, len(entries))
	for i, e := range entries {
		label := truncateToWidth(e.addr, maxLabel)
		bars[i] = barchart.BarData{
			Label: label,
			Values: []barchart.BarValue{{
				Name:  label,
				Value: float64(e.count),
				Style: lipgloss.NewStyle().Foreground(rankBarColor(i, len(entries))),
			}},
		}
	}
	w := width
	if w < 24 {
		w = 24
	}
	bc := barchart.New(w, len(bars),
		barchart.WithHorizontalBars(),
		barchart.WithBarGap(0),
		barchart.WithStyles(dimStyle, lipgloss.NewStyle().Foreground(lipgloss.Color("250"))))
	bc.PushAll(bars)
	// Resize (not just Draw) is required here — it's what recomputes the
	// axis origin from the label widths just pushed; without it every bar
	// renders flush against the left edge with no label column at all.
	bc.Resize(w, len(bars))
	bc.Draw()
	return bc.View()
}

func (m *model) refreshViewport() {
	if !m.ready {
		return
	}
	if m.rankActive {
		if m.rankByRecv {
			m.viewport.SetContent(renderRanking(rankReceivers(m.events), m.viewport.Width, "집계할 RECV/FWD 이벤트 없음"))
		} else {
			m.viewport.SetContent(renderRanking(rankSenders(m.events), m.viewport.Width, "집계할 SENT 이벤트 없음"))
		}
		return
	}
	if m.hsActive {
		m.viewport.SetContent(renderEvents(m.historyVisible(), m.viewport.Width, "일치하는 이력 없음"))
		return
	}
	m.viewport.SetContent(renderEvents(m.visibleEvents(), m.viewport.Width, "이벤트 대기 중 (mail.log 감시 중)"))
	if m.followTail {
		m.viewport.GotoBottom()
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		vh := msg.Height - headerHeight - footerHeight
		if vh < 3 {
			vh = 3
		}
		sparkWidth := msg.Width - lipgloss.Width(trafficLabel)
		if sparkWidth < 10 {
			sparkWidth = 10
		}
		m.spark.Resize(sparkWidth, m.spark.Height())
		if !m.ready {
			m.viewport = viewport.New(msg.Width, vh)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = vh
		}
		m.refreshViewport()
		return m, nil

	case tickMsg:
		m.now = time.Time(msg)
		total := 0
		for t := EventType(0); t < eventTypeCount; t++ {
			total += m.counts[t]
		}
		m.spark.Push(float64(total - m.lastTotal))
		m.spark.Draw()
		m.lastTotal = total

		badTotal := m.counts[EventBounce] + m.counts[EventReject]
		if badTotal-m.lastBadTotal >= bounceRejectSpikeThreshold {
			m.alertUntil = m.now.Add(alertBadgeTTL)
			if m.webState != nil {
				m.webState.setAlertUntil(m.alertUntil)
			}
		}
		m.lastBadTotal = badTotal
		return m, tick()

	case tailErrMsg:
		m.err = msg
		return m, waitForErr(m.errCh)

	case logLineMsg:
		cmd := waitForLine(m.lineCh)
		changed := false
		for _, line := range msg {
			if ev := m.processLine(line); ev != nil {
				m.counts[ev.Type]++
				m.events = append(m.events, *ev)
				changed = true
				if m.paused || !m.followTail {
					m.pendingNew++
				}
				if m.webState != nil {
					m.webState.addEvent(*ev)
					m.webHub.broadcastEvent(*ev)
				}
			}
		}
		if len(m.events) > maxEvents {
			m.events = m.events[len(m.events)-maxEvents:]
		}
		if changed && !m.paused {
			m.refreshViewport()
		}
		return m, cmd

	case historyResultsMsg:
		m.hsLoading = false
		m.hsErr = msg.err
		m.hsResults = msg.events
		m.hsQuery = msg.query
		m.hsActive = true
		m.refreshViewport()
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		if m.filtering {
			switch msg.Type {
			case tea.KeyEnter:
				m.filter = strings.TrimSpace(m.filterInput.Value())
				m.filtering = false
				m.filterInput.Blur()
				m.refreshViewport()
				return m, nil
			case tea.KeyEsc:
				m.filtering = false
				m.filterInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			return m, cmd
		}

		if m.hsTyping {
			switch msg.Type {
			case tea.KeyEnter:
				q := strings.TrimSpace(m.hsInput.Value())
				m.hsTyping = false
				m.hsInput.Blur()
				if q == "" {
					return m, nil
				}
				m.hsLoading = true
				m.hsActive = true
				m.refreshViewport()
				return m, searchHistoryCmd(m.db, q)
			case tea.KeyEsc:
				m.hsTyping = false
				m.hsInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.hsInput, cmd = m.hsInput.Update(msg)
			return m, cmd
		}

		if (m.hsActive || m.rankActive) && msg.String() == "esc" {
			m.hsActive = false
			m.rankActive = false
			m.refreshViewport()
			m.viewport.GotoBottom()
			return m, nil
		}

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Filter):
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case key.Matches(msg, keys.History):
			m.rankActive = false
			m.hsTyping = true
			m.hsInput.Focus()
			return m, textinput.Blink
		case key.Matches(msg, keys.Rank):
			m.hsActive = false
			m.rankActive = true
			m.rankByRecv = false
			m.refreshViewport()
			m.viewport.GotoTop()
			return m, nil
		case key.Matches(msg, keys.RankRecv):
			m.hsActive = false
			m.rankActive = true
			m.rankByRecv = true
			m.refreshViewport()
			m.viewport.GotoTop()
			return m, nil
		case key.Matches(msg, keys.Toggle):
			idx := int(msg.String()[0] - '1')
			m.enabled[idx] = !m.enabled[idx]
			m.refreshViewport()
			return m, nil
		case key.Matches(msg, keys.Export):
			var toExport []Event
			if m.hsActive {
				toExport = m.historyVisible()
			} else {
				toExport = m.visibleEvents()
			}
			path, err := exportEvents(toExport)
			m.exportAt = time.Now()
			if err != nil {
				m.exportMsg = "내보내기 실패: " + err.Error()
			} else {
				m.exportMsg = fmt.Sprintf("내보냄: %s (%d건)", path, len(toExport))
			}
			return m, nil
		case key.Matches(msg, keys.Pause):
			m.paused = !m.paused
			if !m.paused {
				m.refreshViewport()
				m.pendingNew = 0
			}
			return m, nil
		case key.Matches(msg, keys.Clear):
			m.events = nil
			for i := range m.counts {
				m.counts[i] = 0
			}
			m.pendingNew = 0
			m.refreshViewport()
			return m, nil
		case key.Matches(msg, keys.Bottom):
			m.followTail = true
			m.pendingNew = 0
			m.viewport.GotoBottom()
			return m, nil
		case key.Matches(msg, keys.Help):
			m.showAll = !m.showAll
			return m, nil
		case key.Matches(msg, keys.Up), key.Matches(msg, keys.Down),
			msg.String() == "pgup", msg.String() == "pgdown":
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			m.followTail = m.viewport.AtBottom()
			if m.followTail {
				m.pendingNew = 0
			}
			return m, cmd
		}
	}
	return m, nil
}

// --- styles ---

var (
	appBorder = lipgloss.RoundedBorder()

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("219")).Background(lipgloss.Color("235"))
	headerBar  = lipgloss.NewStyle().Background(lipgloss.Color("235")).Padding(0, 1)
	dateStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Background(lipgloss.Color("235"))
	clockStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250")).Background(lipgloss.Color("235"))

	runBadge   = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("232")).Background(lipgloss.Color("42"))
	pauseBadge = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("232")).Background(lipgloss.Color("214"))
	newBadge   = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("232")).Background(lipgloss.Color("219"))
	alertBadge = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("196"))

	filterLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	filterValueStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))

	sepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	errStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)

	eventColor = map[EventType]lipgloss.Color{
		EventLogin:   lipgloss.Color("75"),
		EventRecv:    lipgloss.Color("42"),
		EventSent:    lipgloss.Color("214"),
		EventForward: lipgloss.Color("117"),
		EventBounce:  lipgloss.Color("203"),
		EventReject:  lipgloss.Color("161"),
	}
)

func cardStyle(color lipgloss.Color, on bool) lipgloss.Style {
	s := lipgloss.NewStyle().Border(appBorder).Padding(0, 1)
	if on {
		return s.BorderForeground(color).Foreground(color).Bold(true)
	}
	return s.BorderForeground(lipgloss.Color("238")).Foreground(lipgloss.Color("238"))
}

// miniBar renders a fixed-width block bar showing count's share of the
// busiest event type right now — a quick at-a-glance read of which kind of
// traffic currently dominates, without having to compare the raw numbers.
func miniBar(count, maxCount, width int) string {
	filled := 0
	if maxCount > 0 {
		filled = count * width / maxCount
	}
	if filled == 0 && count > 0 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// statCards renders one card per event type with a fixed-width count field
// so a jump from single to double/triple digits (e.g. LOGIN hitting 82)
// doesn't widen that card relative to its neighbors, a mini bar under the
// count showing its share of the busiest type, and dims every card whose
// type is currently toggled off so the active ones stand out.
func statCards(m model) string {
	const barWidth = 10
	maxCount := 1
	for t := EventType(0); t < eventTypeCount; t++ {
		if m.counts[t] > maxCount {
			maxCount = m.counts[t]
		}
	}
	cards := make([]string, 0, eventTypeCount)
	for t := EventType(0); t < eventTypeCount; t++ {
		label := fmt.Sprintf("%s %-6s %4d", t.Glyph(), t.Label(), m.counts[t])
		bar := miniBar(m.counts[t], maxCount, barWidth)
		cards = append(cards, cardStyle(eventColor[t], m.enabled[t]).Render(label+"\n"+bar))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

// exportEvents writes events to a timestamped CSV file in the current
// directory, one row per event with its multi-line Text flattened to " | "
// so it survives a single CSV field, and returns the path written.
func exportEvents(events []Event) (string, error) {
	path := fmt.Sprintf("mail-monitor-%s.csv", time.Now().Format("20060102-150405"))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"When", "Type", "From", "To", "Text"}); err != nil {
		return "", err
	}
	for _, e := range events {
		row := []string{e.When, e.Type.Label(), e.From, e.To, strings.ReplaceAll(e.Text, "\n", " | ")}
		if err := w.Write(row); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return path, nil
}

// enabledTypesSummary renders one dimmed/colored glyph per event type so the
// currently active 1-6 toggles are visible in the status bar itself, not
// just inferable from which stat cards look dim.
func enabledTypesSummary(m model) string {
	var b strings.Builder
	for t := EventType(0); t < eventTypeCount; t++ {
		g := t.Glyph()
		if m.enabled[t] {
			b.WriteString(lipgloss.NewStyle().Foreground(eventColor[t]).Render(g))
		} else {
			b.WriteString(dimStyle.Render(g))
		}
		b.WriteString(" ")
	}
	return dimStyle.Render("표시:") + " " + strings.TrimRight(b.String(), " ")
}

// justify lays `left` and `right` across `width`, padding the gap.
func justify(width int, left, right string) string {
	lw, rw := lipgloss.Width(left), lipgloss.Width(right)
	gap := width - lw - rw
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// continuationIndent aligns wrapped lines (e.g. the "수신:" line of a
// multi-line RECV/SENT/BOUNCE/REJECT event) under the text column, matching
// the plain-text width of "| 08-04 15:34:59 LOGIN   " (bar, date+time,
// padded label, separators).
const continuationIndent = "                        "

// truncateToWidth cuts s to fit within max display cells (wide runes like
// Korean count as 2), appending an ellipsis, so a long SRS0=... address or
// subject can't push the line into an ugly wrap.
func truncateToWidth(s string, max int) string {
	if lipgloss.Width(s) <= max {
		return s
	}
	r := []rune(s)
	for i := len(r); i > 0; i-- {
		cand := string(r[:i]) + "…"
		if lipgloss.Width(cand) <= max {
			return cand
		}
	}
	return "…"
}

// colorizeLine dims the literal "발신:"/"수신:" labels so the addresses
// after them stand out more. Runs after truncation only — it inserts ANSI
// codes, and truncating a string that already contains them could slice
// through an escape sequence.
func colorizeLine(s string) string {
	s = strings.Replace(s, "발신: ", dimStyle.Render("발신:")+" ", 1)
	s = strings.Replace(s, " 수신: ", " "+dimStyle.Render("수신:")+" ", 1)
	return s
}

// recvLineParts splits a RECV event's Text into the "발신: X → 수신: " prefix,
// the recipient that follows it on the same line, and an optional subject
// continuation line — the pieces groupRecvBroadcasts needs to fold repeated
// deliveries of one broadcast message into a single line.
func recvLineParts(e Event) (prefix, recipient, subject string, ok bool) {
	if e.Type != EventRecv && e.Type != EventForward {
		return "", "", "", false
	}
	lines := strings.SplitN(e.Text, "\n", 2)
	const marker = "수신: "
	idx := strings.Index(lines[0], marker)
	if idx == -1 {
		return "", "", "", false
	}
	prefix = lines[0][:idx+len(marker)]
	recipient = lines[0][idx+len(marker):]
	if len(lines) > 1 {
		subject = lines[1]
	}
	return prefix, recipient, subject, true
}

// forwardNoteRe splits a toWithForward recipient like "hslee@koolsign.net
// (management@koolsign.net에서 전달됨)" into the bare email and the note.
var forwardNoteRe = regexp.MustCompile(`^(.*) \((.+에서 전달됨)\)$`)

func splitForwardNote(recipient string) (email, note string) {
	if m := forwardNoteRe.FindStringSubmatch(recipient); m != nil {
		return m[1], m[2]
	}
	return recipient, ""
}

// namedAddrRe matches the "이름 <email>" form m.addr() produces when a users
// table lookup resolves a real name for the address.
var namedAddrRe = regexp.MustCompile(`^(.+) <[^>]+>$`)

// shortRecipient reduces a recipient (possibly "이름 <email>" if resolveName
// found a match, otherwise a bare email) to the short form used in a grouped
// broadcast's recipient list: the resolved name when there is one, otherwise
// just the local part (domain dropped).
func shortRecipient(s string) string {
	if m := namedAddrRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	if idx := strings.LastIndex(s, "@"); idx > 0 {
		return s[:idx]
	}
	return s
}

// groupRecvBroadcasts folds consecutive RECV events that share a timestamp,
// sender, and subject — the same message BCC'd/expanded to several local
// mailboxes at once (a newsletter, an alias fan-out) — into three lines:
// a "수신: N명 (forwarding note)" summary, the subject, and a "수신: N명
// (a, b, c)" line naming every recipient by local part (domain dropped, no
// per-name forwarding notes — those vary per recipient depending on which
// alias they're behind, and repeating them per name got noisy fast).
func groupRecvBroadcasts(events []Event) []Event {
	out := make([]Event, 0, len(events))
	var groupKey string
	var recips []string
	var prefix, subject string
	flush := func() {
		if len(recips) < 2 {
			return
		}
		emails := make([]string, len(recips))
		notes := make([]string, len(recips))
		sharedNote := ""
		for i, r := range recips {
			emails[i], notes[i] = splitForwardNote(r)
			if sharedNote == "" && notes[i] != "" {
				sharedNote = notes[i]
			}
		}
		locals := make([]string, len(emails))
		for i, email := range emails {
			locals[i] = shortRecipient(email)
		}
		summary := fmt.Sprintf("%s%d명", prefix, len(recips))
		if sharedNote != "" {
			summary += " (" + sharedNote + ")"
		}
		names := fmt.Sprintf("%s%d명 (%s)", prefix, len(recips), strings.Join(locals, ", "))
		lines := []string{summary}
		if subject != "" {
			lines = append(lines, subject)
		}
		lines = append(lines, names)
		out[len(out)-1].Text = strings.Join(lines, "\n")
	}
	for _, e := range events {
		p, r, s, ok := recvLineParts(e)
		if !ok {
			flush()
			groupKey = ""
			out = append(out, e)
			continue
		}
		key := e.When + "\x00" + p + "\x00" + s
		if key == groupKey {
			recips = append(recips, r)
			continue
		}
		flush()
		groupKey, prefix, subject, recips = key, p, s, []string{r}
		out = append(out, e)
	}
	flush()
	return out
}

func renderEvents(events []Event, width int, emptyMsg string) string {
	if len(events) == 0 {
		return dimStyle.Render("  " + emptyMsg)
	}
	events = groupRecvBroadcasts(events)
	maxText := width - len(continuationIndent)
	if maxText < 20 {
		maxText = 20
	}
	var b strings.Builder
	for i, e := range events {
		style := lipgloss.NewStyle().Foreground(eventColor[e.Type])
		bar := style.Render(e.Type.Glyph())
		lines := strings.Split(e.Text, "\n")
		first := colorizeLine(truncateToWidth(lines[0], maxText))
		b.WriteString(fmt.Sprintf("%s %s %-6s %s",
			bar, dimStyle.Render(e.When),
			style.Bold(true).Render(e.Type.Label()), first))
		for _, l := range lines[1:] {
			b.WriteString("\n" + continuationIndent + colorizeLine(truncateToWidth(l, maxText)))
		}
		if i < len(events)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

func (m model) View() string {
	if !m.ready {
		return "초기화 중…"
	}

	var b strings.Builder

	// title row — solid background bar so it reads as a header, not just text
	title := titleStyle.Render(" Mail Server Monitor ")
	clock := dateStyle.Render(m.now.Format("2006-01-02")) + clockStyle.Render(" "+m.now.Format("15:04:05")+" ")
	inner := justify(m.width-2, title, clock)
	b.WriteString(headerBar.Render(inner))
	b.WriteString("\n")

	// status row
	status := runBadge.Render("RUNNING")
	if m.paused {
		status = pauseBadge.Render("PAUSED")
	}
	if m.pendingNew > 0 {
		status = newBadge.Render(fmt.Sprintf("▲ 새 이벤트 %d건", m.pendingNew)) + " " + status
	}
	if m.now.Before(m.alertUntil) {
		status = alertBadge.Render("⚠ BOUNCE/REJECT 급증") + " " + status
	}

	var left string
	switch {
	case m.hsLoading:
		left = pauseBadge.Render("검색 중") + "  " + dimStyle.Render(m.hsQuery)
	case m.hsActive:
		left = filterLabelStyle.Render("이력 검색 ") + filterValueStyle.Render(m.hsQuery) +
			dimStyle.Render(fmt.Sprintf("  (%d건 · esc로 복귀)", len(m.hsResults)))
		if m.hsErr != nil {
			left += "  " + errStyle.Render(m.hsErr.Error())
		}
	case m.rankActive && m.rankByRecv:
		left = filterLabelStyle.Render("수신량 랭킹") +
			dimStyle.Render(" (RECV/FWD, 현재 버퍼 기준 · esc로 복귀)")
	case m.rankActive:
		left = filterLabelStyle.Render("발신량 랭킹") +
			dimStyle.Render(" (SENT, 현재 버퍼 기준 · esc로 복귀)")
	default:
		filterLabel := filterLabelStyle.Render("필터 ")
		filterVal := filterValueStyle.Render("전체")
		if m.filter != "" {
			filterVal = filterValueStyle.Render(m.filter)
		}
		left = filterLabel + filterVal + "  " + enabledTypesSummary(m)
		if !m.followTail {
			left += "  " + dimStyle.Render("(스크롤 중 · G로 최신 이동)")
		}
	}
	if m.exportMsg != "" && time.Since(m.exportAt) < 3*time.Second {
		left += "  " + filterValueStyle.Render(m.exportMsg)
	}
	b.WriteString(justify(m.width, left, status))
	b.WriteString("\n")

	// stat cards
	b.WriteString(statCards(m))
	b.WriteString("\n")

	// traffic sparkline — total events/sec over the last window
	b.WriteString(dimStyle.Render(trafficLabel) + m.spark.View())
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errStyle.Render("ERROR: " + m.err.Error()))
		b.WriteString("\n")
	}

	b.WriteString(sepStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	// event viewport
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	b.WriteString(sepStyle.Render(strings.Repeat("─", m.width)))
	b.WriteString("\n")

	if m.filtering {
		b.WriteString(m.filterInput.View())
	} else if m.hsTyping {
		b.WriteString(m.hsInput.View())
	} else if m.showAll {
		b.WriteString(m.help.FullHelpView(keys.FullHelp()))
	} else {
		b.WriteString(m.help.ShortHelpView(keys.ShortHelp()))
	}

	return b.String()
}

// envFilePaths lists .env locations in priority order: a system-wide file
// for service deployments, then the current directory for local runs.
var envFilePaths = []string{"/etc/mail-monitor/.env", ".env"}

// loadEnvFile loads the first .env file that's actually readable.
// godotenv.Load never overwrites variables already set in the process
// environment. It's tempting to gate this on os.Stat first, but stat only
// needs directory search permission — it can succeed on a file the process
// then can't open (e.g. root:root, mode 600, run as a regular user), and
// stopping at that path would silently skip the working fallback below it.
// Trying Load directly on every candidate avoids that trap.
func loadEnvFile() {
	for _, path := range envFilePaths {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
}

// runDaemon runs the log-tailing/parsing core without the bubbletea TUI —
// for the systemd service, which has no terminal to attach to. It reuses
// model.processLine (and the same webState/sseHub mirroring) so the parsing
// logic never forks between the two run modes.
func runDaemon(ws *webState, hub *sseHub, db *sql.DB) {
	m := initialModel(ws, hub, db)
	go startTail(m.lineCh, m.errCh)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case line := <-m.lineCh:
			if ev := m.processLine(line); ev != nil {
				m.counts[ev.Type]++
				m.events = append(m.events, *ev)
				if len(m.events) > maxEvents {
					m.events = m.events[len(m.events)-maxEvents:]
				}
				ws.addEvent(*ev)
				hub.broadcastEvent(*ev)
			}
		case <-ticker.C:
			badTotal := m.counts[EventBounce] + m.counts[EventReject]
			if badTotal-m.lastBadTotal >= bounceRejectSpikeThreshold {
				m.alertUntil = time.Now().Add(alertBadgeTTL)
				ws.setAlertUntil(m.alertUntil)
			}
			m.lastBadTotal = badTotal
		case err := <-m.errCh:
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println("mail-monitor " + version)
		return
	}

	loadEnvFile()

	db := openDirectory()
	ws := newWebState()
	hub := newSSEHub()
	if addr := webAddr(); addr != "" {
		startWebServer(addr, ws, hub, db)
	}

	if len(os.Args) > 1 && os.Args[1] == "--daemon" {
		runDaemon(ws, hub, db)
		return
	}

	p := tea.NewProgram(initialModel(ws, hub, db), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
