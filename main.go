package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"database/sql"
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
	case EventBounce:
		return "BOUNCE"
	case EventReject:
		return "REJECT"
	}
	return "?"
}

type Event struct {
	When string // e.g. "08-04 15:34:59", taken from the log line itself
	Type EventType
	Text string
	Raw  string
	From string // raw sender address, undecorated (no name/IP) — for aggregation like the sender ranking view
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

	loginRe   = regexp.MustCompile(`dovecot.*(?:auth.*Success|imap-login.*Login)`)
	userRe    = regexp.MustCompile(`user=<([^>]*)>`)
	ripRe     = regexp.MustCompile(`rip=([0-9.]+)`)
	fromRe    = regexp.MustCompile(`from=<([^>]*)>`)
	toRe      = regexp.MustCompile(`to=<([^>]*)>`)
	origToRe  = regexp.MustCompile(`orig_to=<([^>]*)>`)
	relayRe   = regexp.MustCompile(`relay=([^,\s]*)`)
	rejectRe  = regexp.MustCompile(`reject:\s*([^;]*)`)
	clientRe  = regexp.MustCompile(`client=\S+\[(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\]`)
	bracketIP = regexp.MustCompile(`\[(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\]`)
	subjectRe = regexp.MustCompile(`warning: header Subject: (.*?) from \S+\[[0-9.]+\];`)
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
		to := m.addr(toRaw)
		if om := origToRe.FindStringSubmatch(rest); om != nil && om[1] != "" {
			to = toWithForward(to, m.addr(om[1]))
		}
		from, ok := m.qidFrom[qid]
		if !ok {
			from = "-"
		}
		fromDisplay := fromWithIP(m.addr(shortenSRS(from)), m.qidIP[qid])
		subject := m.qidSubject[qid]

		switch {
		case strings.Contains(rest, "status=bounced"):
			return &Event{When: when, Type: EventBounce, Raw: line, From: from,
				Text: withSubject(fmt.Sprintf("발신: %s → 수신: %s", fromDisplay, to), subject)}
		case strings.Contains(rest, "status=sent"):
			relay := extract(relayRe, rest)
			if isLocalRelay(relay) {
				return &Event{When: when, Type: EventRecv, Raw: line, From: from,
					Text: withSubject(fmt.Sprintf("발신: %s → 수신: %s", fromDisplay, to), subject)}
			}
			return &Event{When: when, Type: EventSent, Raw: line, From: from,
				Text: withSubject(fmt.Sprintf("발신: %s → 수신: %s (via %s)", fromDisplay, to, relay), subject)}
		}
		return nil
	}

	switch {
	case loginRe.MatchString(line):
		user := m.addr(extract(userRe, line))
		rip := extract(ripRe, line)
		return &Event{When: when, Type: EventLogin, Raw: line,
			Text: fmt.Sprintf("%s from %s", user, rip)}
	case strings.Contains(line, "reject:"):
		fromRaw := extract(fromRe, line)
		from := m.addr(shortenSRS(fromRaw))
		to := m.addr(extract(toRe, line))
		reason := extract(rejectRe, line)
		ip := ""
		if im := bracketIP.FindStringSubmatch(line); im != nil {
			ip = im[1]
		}
		return &Event{When: when, Type: EventReject, Raw: line, From: fromRaw,
			Text: fmt.Sprintf("발신: %s → 수신: %s (%s)", fromWithIP(from, ip), to, reason)}
	}
	return nil
}

// --- log tailing ---

type logLineMsg string
type tailErrMsg error
type tickMsg time.Time

// historyLines is how many recent lines nativeTail replays as history on
// startup, before switching to pure live-follow. A busy server's log can
// have hundreds of thousands of lines (dovecot alone logs a login+logout
// pair per IMAP session), so this must stay small or it reads as a flood.
const historyLines = 100

// historyScanBytes is how far back we look to find those last historyLines
// lines — generous relative to typical line length, cheap to read regardless
// of how large logPath has grown since rotation.
const historyScanBytes = 256 * 1024

// nativeTail replays the last historyLines lines of logPath as history, then
// follows only new appends from that point on — it never rescans or reprints
// old lines during live-follow. This also avoids the startup race of handing
// off to an external `tail` process (which may not have attached before the
// first lines land) and lets us read the file directly when permissions
// allow (no sudo).
func nativeTail(ch chan<- string, errCh chan<- error) {
	f, err := os.Open(logPath)
	if err != nil {
		errCh <- err
		return
	}
	defer f.Close()

	if fi, err := f.Stat(); err == nil {
		start := int64(0)
		if fi.Size() > historyScanBytes {
			start = fi.Size() - historyScanBytes
		}
		f.Seek(start, io.SeekStart)
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		var recent []string
		for scanner.Scan() {
			recent = append(recent, scanner.Text())
		}
		if start > 0 && len(recent) > 0 {
			recent = recent[1:] // drop the partial line at our scan start
		}
		if len(recent) > historyLines {
			recent = recent[len(recent)-historyLines:]
		}
		for _, l := range recent {
			ch <- l
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
	// -n 100 matches nativeTail's historyLines: recent history, then follow.
	cmd := exec.Command("sudo", "tail", "-F", "-n", fmt.Sprint(historyLines), logPath)
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

// scanFile reads path line by line, transparently decompressing .gz.
func scanFile(path string, fn func(line string)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
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

// searchHistory scans logPath and its rotated siblings for events matching
// query, using a correlation state independent of the live model's (own
// qidFrom/qidIP/qidSubject/nameCache) so it can run concurrently on its own
// goroutine without racing the live view. It shares the *sql.DB handle,
// which is safe for concurrent use.
func searchHistory(db *sql.DB, query string) historyResultsMsg {
	sm := &model{
		qidFrom:    make(map[string]string),
		qidIP:      make(map[string]string),
		qidSubject: make(map[string]string),
		nameCache:  make(map[string]string),
		db:         db,
	}

	needle := strings.ToLower(query)
	var results []Event
	var lastErr error
	opened := 0

	for _, path := range listRotatedLogs() {
		err := scanFile(path, func(line string) {
			ev := sm.processLine(line)
			if ev == nil {
				return
			}
			if needle != "" && !strings.Contains(strings.ToLower(ev.Raw), needle) &&
				!strings.Contains(strings.ToLower(ev.Text), needle) {
				return
			}
			results = append(results, *ev)
			if len(results) > maxHistoryResults {
				results = results[1:]
			}
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
	return historyResultsMsg{query: query, events: results}
}

func searchHistoryCmd(db *sql.DB, query string) tea.Cmd {
	return func() tea.Msg {
		return searchHistory(db, query)
	}
}

func waitForLine(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		return logLineMsg(<-ch)
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

// --- keymap ---

type keyMap struct {
	Filter  key.Binding
	History key.Binding
	Rank    key.Binding
	Toggle  key.Binding
	Pause   key.Binding
	Clear   key.Binding
	Bottom  key.Binding
	Up      key.Binding
	Down    key.Binding
	Help    key.Binding
	Quit    key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Filter, k.History, k.Rank, k.Toggle, k.Pause, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Filter, k.History, k.Rank, k.Toggle, k.Pause, k.Clear},
		{k.Up, k.Down, k.Bottom},
		{k.Help, k.Quit},
	}
}

var keys = keyMap{
	Filter:  key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "필터(버퍼)")),
	History: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "이력 검색")),
	Rank:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "발신량 랭킹")),
	Toggle:  key.NewBinding(key.WithKeys("1", "2", "3", "4", "5"), key.WithHelp("1-5", "이벤트 토글")),
	Pause:   key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "정지/재개")),
	Clear:   key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "클리어")),
	Bottom:  key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "최신으로")),
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "스크롤")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "스크롤")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "도움말")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "종료")),
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
}

func initialModel() model {
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
		db:          openDirectory(),
		nameCache:   make(map[string]string),
		hsInput:     hsi,
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
	return strings.Contains(strings.ToLower(e.Raw), needle) ||
		strings.Contains(strings.ToLower(e.Text), needle)
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
	headerHeight = 8
	footerHeight = 2
)

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

// rankSenders aggregates SENT event counts by raw sender address, highest
// first — a compromised account blasting spam shows up at the top.
func rankSenders(events []Event) []rankEntry {
	counts := make(map[string]int)
	for _, e := range events {
		if e.Type != EventSent || e.From == "" || e.From == "-" {
			continue
		}
		counts[e.From]++
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

func renderRanking(entries []rankEntry) string {
	if len(entries) == 0 {
		return dimStyle.Render("  집계할 SENT 이벤트 없음")
	}
	var b strings.Builder
	for i, e := range entries {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("%3d.  %4d건  %s", i+1, e.count, e.addr))
	}
	return b.String()
}

func (m *model) refreshViewport() {
	if !m.ready {
		return
	}
	if m.rankActive {
		m.viewport.SetContent(renderRanking(rankSenders(m.events)))
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
		return m, tick()

	case tailErrMsg:
		m.err = msg
		return m, waitForErr(m.errCh)

	case logLineMsg:
		cmd := waitForLine(m.lineCh)
		if m.paused {
			return m, cmd
		}
		if ev := m.processLine(string(msg)); ev != nil {
			m.counts[ev.Type]++
			m.events = append(m.events, *ev)
			if len(m.events) > maxEvents {
				m.events = m.events[len(m.events)-maxEvents:]
			}
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
			m.refreshViewport()
			m.viewport.GotoTop()
			return m, nil
		case key.Matches(msg, keys.Toggle):
			idx := int(msg.String()[0] - '1')
			m.enabled[idx] = !m.enabled[idx]
			m.refreshViewport()
			return m, nil
		case key.Matches(msg, keys.Pause):
			m.paused = !m.paused
			return m, nil
		case key.Matches(msg, keys.Clear):
			m.events = nil
			for i := range m.counts {
				m.counts[i] = 0
			}
			m.refreshViewport()
			return m, nil
		case key.Matches(msg, keys.Bottom):
			m.followTail = true
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
			return m, cmd
		}
	}
	return m, nil
}

// --- styles ---

var (
	appBorder = lipgloss.RoundedBorder()

	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("219"))
	clockStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	runBadge   = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("232")).Background(lipgloss.Color("42"))
	pauseBadge = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(lipgloss.Color("232")).Background(lipgloss.Color("214"))

	filterLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	filterValueStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("117"))

	sepStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
	errStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)

	eventColor = map[EventType]lipgloss.Color{
		EventLogin:  lipgloss.Color("75"),
		EventRecv:   lipgloss.Color("42"),
		EventSent:   lipgloss.Color("214"),
		EventBounce: lipgloss.Color("203"),
		EventReject: lipgloss.Color("161"),
	}
)

func cardStyle(color lipgloss.Color, on bool) lipgloss.Style {
	s := lipgloss.NewStyle().Border(appBorder).Padding(0, 1)
	if on {
		return s.BorderForeground(color).Foreground(color).Bold(true)
	}
	return s.BorderForeground(lipgloss.Color("237")).Foreground(lipgloss.Color("240"))
}

func statCards(m model) string {
	cards := make([]string, 0, eventTypeCount)
	for t := EventType(0); t < eventTypeCount; t++ {
		label := fmt.Sprintf("%-6s %d", t.Label(), m.counts[t])
		cards = append(cards, cardStyle(eventColor[t], m.enabled[t]).Render(label))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
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
	if e.Type != EventRecv {
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

// groupRecvBroadcasts folds consecutive RECV events that share a timestamp,
// sender, and subject — the same message BCC'd/expanded to several local
// mailboxes at once (a newsletter, an alias fan-out) — into a single
// "수신 N명: a, b, c" line instead of repeating the sender block per recipient.
func groupRecvBroadcasts(events []Event) []Event {
	out := make([]Event, 0, len(events))
	var groupKey string
	var recips []string
	var prefix, subject string
	flush := func() {
		if len(recips) < 2 {
			return
		}
		text := fmt.Sprintf("%s%d명: %s", prefix, len(recips), strings.Join(recips, ", "))
		if subject != "" {
			text += "\n" + subject
		}
		out[len(out)-1].Text = text
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
		bar := style.Render("|")
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

	// title row
	title := titleStyle.Render("Mail Server Monitor")
	clock := clockStyle.Render(m.now.Format("2006-01-02 15:04:05"))
	b.WriteString(justify(m.width, title, clock))
	b.WriteString("\n")

	// status row
	status := runBadge.Render("RUNNING")
	if m.paused {
		status = pauseBadge.Render("PAUSED")
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
	case m.rankActive:
		left = filterLabelStyle.Render("발신량 랭킹") +
			dimStyle.Render(" (SENT, 현재 버퍼 기준 · esc로 복귀)")
	default:
		filterLabel := filterLabelStyle.Render("필터 ")
		filterVal := filterValueStyle.Render("전체")
		if m.filter != "" {
			filterVal = filterValueStyle.Render(m.filter)
		}
		left = filterLabel + filterVal
		if !m.followTail {
			left += "  " + dimStyle.Render("(스크롤 중 · G로 최신 이동)")
		}
	}
	b.WriteString(justify(m.width, left, status))
	b.WriteString("\n")

	// stat cards
	b.WriteString(statCards(m))
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

// loadEnvFile loads the first .env file found. godotenv.Load never
// overwrites variables already set in the process environment.
func loadEnvFile() {
	for _, path := range envFilePaths {
		if _, err := os.Stat(path); err == nil {
			godotenv.Load(path)
			return
		}
	}
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println("mail-monitor " + version)
		return
	}

	loadEnvFile()

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
