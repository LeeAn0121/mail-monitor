package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "github.com/go-sql-driver/mysql"
)

var version = "dev"

const (
	logPath   = "/var/log/mail.log"
	maxEvents = 500
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

func (e EventType) Icon() string {
	switch e {
	case EventLogin:
		return "🔐"
	case EventRecv:
		return "📥"
	case EventSent:
		return "📤"
	case EventBounce:
		return "❌"
	case EventReject:
		return "🚫"
	}
	return "?"
}

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
	Time time.Time
	Type EventType
	Text string
	Raw  string
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

// withSubject appends a truncated [Subject] suffix when known.
func withSubject(text, subject string) string {
	if subject == "" {
		return text
	}
	const maxLen = 60
	if len(subject) > maxLen {
		subject = subject[:maxLen] + "…"
	}
	return fmt.Sprintf("%s [%s]", text, subject)
}

// processLine parses one mail.log line into an Event. Postfix logs a
// message's from=, client IP, and eventual to=/status= on separate lines
// that share only a Queue-ID, so qidFrom/qidIP correlate them across lines.
func (m *model) processLine(line string) *Event {
	now := time.Now()

	if qm := qidLineRe.FindStringSubmatch(line); qm != nil {
		qid, rest := qm[1], qm[2]

		if from := extract(fromRe, rest); from != "-" {
			m.qidFrom[qid] = from
		}
		if cm := clientRe.FindStringSubmatch(rest); cm != nil {
			m.qidIP[qid] = cm[1]
		}
		if sm := subjectRe.FindStringSubmatch(rest); sm != nil {
			m.qidSubject[qid] = sm[1]
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
		from, ok := m.qidFrom[qid]
		if !ok {
			from = "-"
		}
		fromDisplay := fromWithIP(m.addr(from), m.qidIP[qid])
		subject := m.qidSubject[qid]

		switch {
		case strings.Contains(rest, "status=bounced"):
			return &Event{Time: now, Type: EventBounce, Raw: line,
				Text: withSubject(fmt.Sprintf("%s → %s", fromDisplay, to), subject)}
		case strings.Contains(rest, "status=sent"):
			relay := extract(relayRe, rest)
			if isLocalRelay(relay) {
				return &Event{Time: now, Type: EventRecv, Raw: line,
					Text: withSubject(fmt.Sprintf("%s → %s", fromDisplay, to), subject)}
			}
			return &Event{Time: now, Type: EventSent, Raw: line,
				Text: withSubject(fmt.Sprintf("%s → %s via %s", fromDisplay, to, relay), subject)}
		}
		return nil
	}

	switch {
	case loginRe.MatchString(line):
		user := m.addr(extract(userRe, line))
		rip := extract(ripRe, line)
		return &Event{Time: now, Type: EventLogin, Raw: line,
			Text: fmt.Sprintf("%s from %s", user, rip)}
	case strings.Contains(line, "reject:"):
		from := m.addr(extract(fromRe, line))
		to := m.addr(extract(toRe, line))
		reason := extract(rejectRe, line)
		ip := ""
		if im := bracketIP.FindStringSubmatch(line); im != nil {
			ip = im[1]
		}
		return &Event{Time: now, Type: EventReject, Raw: line,
			Text: fmt.Sprintf("%s → %s (%s)", fromWithIP(from, ip), to, reason)}
	}
	return nil
}

// --- log tailing ---

type logLineMsg string
type tailErrMsg error
type tickMsg time.Time

// nativeTail follows logPath by seeking to EOF up front and polling for
// appended data, avoiding the startup race of handing off to an external
// `tail` process (which may not have attached before the first lines land)
// and letting us read the file directly when permissions allow (no sudo).
func nativeTail(ch chan<- string, errCh chan<- error) {
	f, err := os.Open(logPath)
	if err != nil {
		errCh <- err
		return
	}
	defer f.Close()
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		errCh <- err
		return
	}

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
	Filter key.Binding
	Toggle key.Binding
	Pause  key.Binding
	Clear  key.Binding
	Bottom key.Binding
	Up     key.Binding
	Down   key.Binding
	Help   key.Binding
	Quit   key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Filter, k.Toggle, k.Pause, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Filter, k.Toggle, k.Pause, k.Clear},
		{k.Up, k.Down, k.Bottom},
		{k.Help, k.Quit},
	}
}

var keys = keyMap{
	Filter: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "필터")),
	Toggle: key.NewBinding(key.WithKeys("1", "2", "3", "4", "5"), key.WithHelp("1-5", "이벤트 토글")),
	Pause:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "정지/재개")),
	Clear:  key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "클리어")),
	Bottom: key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "최신으로")),
	Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "스크롤")),
	Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "스크롤")),
	Help:   key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "도움말")),
	Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "종료")),
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
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "user@domain.com"
	ti.CharLimit = 128
	ti.Prompt = "🔎 "

	var enabled [eventTypeCount]bool
	for i := range enabled {
		enabled[i] = true
	}

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
	}
}

func (m model) Init() tea.Cmd {
	go startTail(m.lineCh, m.errCh)
	return tea.Batch(waitForLine(m.lineCh), waitForErr(m.errCh), tick(), textinput.Blink)
}

func (m model) matchesFilter(e Event) bool {
	if m.filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(e.Raw), strings.ToLower(m.filter))
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

func (m *model) refreshViewport() {
	if !m.ready {
		return
	}
	m.viewport.SetContent(renderEvents(m.visibleEvents(), m.viewport.Width))
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

		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Filter):
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
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
		label := fmt.Sprintf("%s %-6s %d", t.Icon(), t.Label(), m.counts[t])
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

func renderEvents(events []Event, width int) string {
	if len(events) == 0 {
		return dimStyle.Render("  … 이벤트 대기 중 (mail.log 감시 중) …")
	}
	var b strings.Builder
	for i, e := range events {
		style := lipgloss.NewStyle().Foreground(eventColor[e.Type])
		bar := style.Render("▎")
		line := fmt.Sprintf("%s %s %s %-6s %s",
			bar, dimStyle.Render(e.Time.Format("15:04:05")), e.Type.Icon(),
			style.Bold(true).Render(e.Type.Label()), e.Text)
		b.WriteString(line)
		if i < len(events)-1 {
			b.WriteString("\n")
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
	title := titleStyle.Render("📧 Mail Server Monitor")
	clock := clockStyle.Render(m.now.Format("2006-01-02 15:04:05"))
	b.WriteString(justify(m.width, title, clock))
	b.WriteString("\n")

	// status row
	status := runBadge.Render("▶ RUNNING")
	if m.paused {
		status = pauseBadge.Render("⏸ PAUSED")
	}
	filterLabel := filterLabelStyle.Render("필터 ")
	filterVal := filterValueStyle.Render("전체")
	if m.filter != "" {
		filterVal = filterValueStyle.Render(m.filter)
	}
	left := filterLabel + filterVal
	if !m.followTail {
		left += "  " + dimStyle.Render("(스크롤 중 · G로 최신 이동)")
	}
	b.WriteString(justify(m.width, left, status))
	b.WriteString("\n")

	// stat cards
	b.WriteString(statCards(m))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errStyle.Render("⚠ " + m.err.Error()))
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
	} else if m.showAll {
		b.WriteString(m.help.FullHelpView(keys.FullHelp()))
	} else {
		b.WriteString(m.help.ShortHelpView(keys.ShortHelp()))
	}

	return b.String()
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println("mail-monitor " + version)
		return
	}

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
