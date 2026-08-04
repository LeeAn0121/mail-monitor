package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	logPath   = "/var/log/mail.log"
	maxEvents = 500
	visible   = 20
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
	loginRe  = regexp.MustCompile(`dovecot.*(?:auth.*Success|imap-login.*Login)`)
	userRe   = regexp.MustCompile(`user=<([^>]*)>`)
	ripRe    = regexp.MustCompile(`rip=([0-9.]+)`)
	fromRe   = regexp.MustCompile(`from=<([^>]*)>`)
	toRe     = regexp.MustCompile(`to=<([^>]*)>`)
	relayRe  = regexp.MustCompile(`relay=([^,\s]*)`)
	rejectRe = regexp.MustCompile(`reject:\s*(.*)`)
)

func extract(re *regexp.Regexp, line string) string {
	m := re.FindStringSubmatch(line)
	if len(m) < 2 {
		return "-"
	}
	return m[1]
}

func parseLine(line string) *Event {
	now := time.Now()
	switch {
	case loginRe.MatchString(line):
		user := extract(userRe, line)
		rip := extract(ripRe, line)
		return &Event{Time: now, Type: EventLogin, Raw: line,
			Text: fmt.Sprintf("%s from %s", user, rip)}
	case strings.Contains(line, "status=bounced"):
		from, to := extract(fromRe, line), extract(toRe, line)
		return &Event{Time: now, Type: EventBounce, Raw: line,
			Text: fmt.Sprintf("%s → %s", from, to)}
	case strings.Contains(line, "status=sent"):
		from, to, relay := extract(fromRe, line), extract(toRe, line), extract(relayRe, line)
		return &Event{Time: now, Type: EventSent, Raw: line,
			Text: fmt.Sprintf("%s → %s via %s", from, to, relay)}
	case strings.Contains(line, "status=delivered"):
		from, to := extract(fromRe, line), extract(toRe, line)
		return &Event{Time: now, Type: EventRecv, Raw: line,
			Text: fmt.Sprintf("%s → %s", from, to)}
	case strings.Contains(line, "reject:"):
		to := extract(toRe, line)
		reason := extract(rejectRe, line)
		return &Event{Time: now, Type: EventReject, Raw: line,
			Text: fmt.Sprintf("to %s (%s)", to, reason)}
	}
	return nil
}

// --- log tailing ---

type logLineMsg string
type tailErrMsg error
type tickMsg time.Time

func startTail(ch chan<- string, errCh chan<- error) {
	args := []string{"-F", "-n", "0", logPath}
	var cmd *exec.Cmd
	if f, err := os.Open(logPath); err == nil {
		f.Close()
		cmd = exec.Command("tail", args...)
	} else {
		cmd = exec.Command("sudo", append([]string{"tail"}, args...)...)
	}
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
}

func initialModel() model {
	ti := textinput.New()
	ti.Placeholder = "user@domain.com"
	ti.CharLimit = 128
	ti.Width = 40

	var enabled [eventTypeCount]bool
	for i := range enabled {
		enabled[i] = true
	}

	return model{
		enabled:     enabled,
		filterInput: ti,
		now:         time.Now(),
		lineCh:      make(chan string, 256),
		errCh:       make(chan error, 4),
		dayStart:    time.Now(),
	}
}

func (m model) Init() tea.Cmd {
	go startTail(m.lineCh, m.errCh)
	return tea.Batch(waitForLine(m.lineCh), waitForErr(m.errCh), tick())
}

func (m model) matchesFilter(e Event) bool {
	if m.filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(e.Raw), strings.ToLower(m.filter))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
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
		if ev := parseLine(string(msg)); ev != nil {
			m.counts[ev.Type]++
			if m.enabled[ev.Type] && m.matchesFilter(*ev) {
				m.events = append(m.events, *ev)
				if len(m.events) > maxEvents {
					m.events = m.events[len(m.events)-maxEvents:]
				}
			}
		}
		return m, cmd

	case tea.KeyMsg:
		if m.filtering {
			switch msg.Type {
			case tea.KeyEnter:
				m.filter = strings.TrimSpace(m.filterInput.Value())
				m.filtering = false
				m.filterInput.Blur()
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

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "f":
			m.filtering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "1", "2", "3", "4", "5":
			idx := int(msg.String()[0] - '1')
			m.enabled[idx] = !m.enabled[idx]
			return m, nil
		case " ":
			m.paused = !m.paused
			return m, nil
		case "c":
			m.events = nil
			return m, nil
		}
	}
	return m, nil
}

// --- styles ---

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Background(lipgloss.Color("62")).Padding(0, 1)
	statStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	runStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	pauseStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	sepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	eventColor = map[EventType]lipgloss.Color{
		EventLogin:  lipgloss.Color("39"),
		EventRecv:   lipgloss.Color("42"),
		EventSent:   lipgloss.Color("214"),
		EventBounce: lipgloss.Color("196"),
		EventReject: lipgloss.Color("160"),
	}
)

func (m model) View() string {
	var b strings.Builder

	title := fmt.Sprintf("📧 Mail Server Monitor | %s", m.now.Format("15:04:05"))
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n")

	status := runStyle.Render("[▶ RUNNING]")
	if m.paused {
		status = pauseStyle.Render("[⏸ PAUSED]")
	}
	filterLabel := m.filter
	if filterLabel == "" {
		filterLabel = "(all)"
	}
	b.WriteString(fmt.Sprintf(" User: %s | %s\n", filterLabel, status))

	total := fmt.Sprintf("📊 %s Total: 🔐%d 📥%d 📤%d ❌%d 🚫%d",
		m.dayStart.Format("2006-01-02"),
		m.counts[EventLogin], m.counts[EventRecv], m.counts[EventSent],
		m.counts[EventBounce], m.counts[EventReject])
	b.WriteString(statStyle.Render(total))
	b.WriteString("\n")
	b.WriteString(sepStyle.Render(strings.Repeat("─", 60)))
	b.WriteString("\n")

	if m.err != nil {
		b.WriteString(errStyle.Render(fmt.Sprintf("error: %v", m.err)))
		b.WriteString("\n")
	}

	rows := visible
	if m.height > 10 {
		rows = m.height - 10
	}
	start := 0
	if len(m.events) > rows {
		start = len(m.events) - rows
	}
	for _, e := range m.events[start:] {
		style := lipgloss.NewStyle().Foreground(eventColor[e.Type])
		line := fmt.Sprintf("  [%s] %s %-6s %s",
			e.Time.Format("15:04:05"), e.Type.Icon(), e.Type.Label(), e.Text)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}

	b.WriteString(sepStyle.Render(strings.Repeat("─", 60)))
	b.WriteString("\n")

	if m.filtering {
		b.WriteString("필터 (사용자/도메인): " + m.filterInput.View() + "\n")
	}

	help := " [f]필터 | [1]로그인 [2]수신 [3]송신 [4]반송 [5]거절 (토글) | [space]정지 | [c]클리어 | [q]종료"
	b.WriteString(footerStyle.Render(help))

	toggles := " on: "
	for i := EventType(0); i < eventTypeCount; i++ {
		if m.enabled[i] {
			toggles += i.Icon() + " "
		}
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(toggles))

	return b.String()
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
