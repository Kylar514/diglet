package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kylar514/diglet/config"
	"github.com/kylar514/diglet/tunnel"
	"github.com/sahilm/fuzzy"
)

var (
	activeStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	connectingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	dimStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cursorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("4")).Bold(true)
	titleStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	keyStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	previewStyle    = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1)
	listStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("8")).
			Padding(0, 1)
)

type mode int

const (
	modeNormal mode = iota
	modeFilter
	modeType
)

type tickMsg struct {
	statuses map[string]tunnel.Info
	revision int
	err      error
}

type tunnelStartedMsg struct {
	name string
	pid  int
	err  error
}

type editorFinishedMsg struct{ err error }

type model struct {
	connections []config.Connection
	filtered    []config.Connection
	cursor      int
	typeCursor  int
	mode        mode
	filter      textinput.Model
	typeFilter  string
	statusMsg   string
	statuses    map[string]tunnel.Info
	revision    int
	cfgPath     string
	width       int
	height      int
}

func newModel(cfg *config.Config, cfgPath string) model {
	ti := textinput.New()
	ti.Placeholder = "fuzzy filter..."
	ti.CharLimit = 64

	return model{
		connections: cfg.Connections,
		filtered:    cfg.Connections,
		filter:      ti,
		typeFilter:  "all",
		cfgPath:     cfgPath,
		statuses:    map[string]tunnel.Info{},
	}
}

func (m *model) Init() tea.Cmd {
	return tick(m.connections, m.revision)
}

func tick(connections []config.Connection, revision int) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		statuses, err := tunnel.Statuses(connections)
		return tickMsg{statuses: statuses, revision: revision, err: err}
	})
}

func (m *model) typeList() []string {
	return append([]string{"all"}, tunnel.RegisteredTypes()...)
}

func (m *model) typeCount(t string) int {
	if t == "all" {
		return len(m.connections)
	}
	n := 0
	for _, c := range m.connections {
		if c.TunnelType == t {
			n++
		}
	}
	return n
}

func (m *model) connectionsOfType(t string) []config.Connection {
	if t == "all" {
		return m.connections
	}
	out := make([]config.Connection, 0)
	for _, c := range m.connections {
		if c.TunnelType == t {
			out = append(out, c)
		}
	}
	return out
}

func (m *model) applyFilter() {
	query := m.filter.Value()
	base := m.connectionsOfType(m.typeFilter)
	if query == "" {
		m.filtered = base
		m.cursor = 0
		return
	}

	names := make([]string, len(base))
	for i, c := range base {
		names[i] = c.Name
	}

	matches := fuzzy.Find(query, names)
	m.filtered = make([]config.Connection, 0, len(matches))
	for _, match := range matches {
		m.filtered = append(m.filtered, base[match.Index])
	}
	m.cursor = 0
}

func (m *model) applyTypeFilter() {
	m.filtered = m.connectionsOfType(m.typeFilter)
	m.cursor = 0
}

func openEditor(cfgPath string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		candidates := []string{"vim", "vi"}
		if runtime.GOOS == "windows" {
			candidates = []string{"notepad"}
		}
		for _, candidate := range candidates {
			if _, err := exec.LookPath(candidate); err == nil {
				editor = candidate
				break
			}
		}
	}
	if editor == "" {
		return func() tea.Msg {
			return editorFinishedMsg{err: fmt.Errorf("no editor found: set $EDITOR")}
		}
	}
	return tea.ExecProcess(exec.Command(editor, cfgPath), func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func startTunnel(conn config.Connection) tea.Cmd {
	return func() tea.Msg {
		pid, err := tunnel.Start(conn)
		if err != nil {
			return tunnelStartedMsg{name: conn.Name, err: err}
		}

		if err := tunnel.StartProbe(conn.Name, conn.LocalPort, pid); err != nil {
			if stopErr := tunnel.Stop(conn); stopErr != nil {
				err = errors.Join(err, fmt.Errorf("cleanup failed: %w", stopErr))
			}
			return tunnelStartedMsg{name: conn.Name, err: err}
		}

		return tunnelStartedMsg{name: conn.Name, pid: pid}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if msg.revision != m.revision {
			return m, tick(m.connections, m.revision)
		}
		if msg.err != nil {
			m.statusMsg = msg.err.Error()
		} else {
			for name, info := range m.statuses {
				if info.Status == tunnel.StatusConnecting && info.PID == 0 && msg.statuses[name].Status == tunnel.StatusInactive {
					msg.statuses[name] = info
				}
			}
			m.statuses = msg.statuses
		}
		return m, tick(m.connections, m.revision)

	case tunnelStartedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("%s: %s", msg.name, msg.err.Error())
			m.statuses[msg.name] = tunnel.Info{Status: tunnel.StatusInactive}
		} else {
			m.statuses[msg.name] = tunnel.Info{Status: tunnel.StatusConnecting, PID: msg.pid}
		}
		m.revision++
		return m, nil

	case editorFinishedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("editor: %s", msg.err.Error())
			return m, nil
		}
		newCfg, err := config.Load(m.cfgPath)
		if err != nil {
			m.statusMsg = fmt.Sprintf("config reload failed: %s", err.Error())
			return m, nil
		}
		m.connections = newCfg.Connections
		m.revision++
		m.applyTypeFilter()
		if m.filter.Value() != "" {
			m.applyFilter()
		}
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		m.statusMsg = ""

		if m.mode == modeFilter {
			switch msg.String() {
			case "esc":
				m.mode = modeNormal
				m.filter.Blur()
				m.filter.SetValue("")
				m.applyTypeFilter()
				return m, nil
			case "enter":
				m.mode = modeNormal
				m.filter.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			m.applyFilter()
			return m, cmd
		}

		if m.mode == modeType {
			types := m.typeList()
			switch msg.String() {
			case "esc", "t":
				m.mode = modeNormal
			case "j":
				if m.typeCursor < len(types)-1 {
					m.typeCursor++
				}
			case "k":
				if m.typeCursor > 0 {
					m.typeCursor--
				}
			case "g":
				m.typeCursor = 0
			case "G":
				m.typeCursor = len(types) - 1
			case "enter":
				m.typeFilter = types[m.typeCursor]
				m.applyTypeFilter()
				m.filter.SetValue("")
				m.mode = modeNormal
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "g":
			m.cursor = 0

		case "G":
			m.cursor = len(m.filtered) - 1

		case "/":
			m.mode = modeFilter
			m.filter.Focus()
			return m, textinput.Blink

		case "e":
			return m, openEditor(m.cfgPath)

		case "K":
			names, errs := tunnel.StopAll()
			if len(names) == 0 && len(errs) == 0 {
				m.statusMsg = "no active tunnels"
				return m, nil
			}
			if len(errs) == 0 {
				for _, name := range names {
					m.statuses[name] = tunnel.Info{Status: tunnel.StatusInactive}
				}
				tunnel.Notify("diglet", "all tunnels stopped\n"+strings.Join(names, "\n"))
				m.statusMsg = "all tunnels stopped"
			} else {
				m.statusMsg = fmt.Sprintf("stopped with %d error(s): %s", len(errs), errs[0].Error())
			}
			m.revision++
			return m, nil

		case "t":
			m.mode = modeType
			types := m.typeList()
			for i, t := range types {
				if t == m.typeFilter {
					m.typeCursor = i
					break
				}
			}

		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			conn := m.filtered[m.cursor]

			switch m.statuses[conn.Name].Status {
			case tunnel.StatusConnecting:
				if m.statuses[conn.Name].PID == 0 {
					return m, nil
				}
				fallthrough
			case tunnel.StatusActive:
				if err := tunnel.Stop(conn); err != nil {
					m.statusMsg = err.Error()
				} else {
					m.statuses[conn.Name] = tunnel.Info{Status: tunnel.StatusInactive}
					tunnel.Notify("diglet", conn.Name+": disconnected")
				}
				m.revision++
				return m, nil
			case tunnel.StatusOccupied:
				if m.statuses[conn.Name].PID > 0 {
					if err := tunnel.Stop(conn); err != nil {
						m.statusMsg = err.Error()
					} else {
						m.statuses[conn.Name] = tunnel.Info{Status: tunnel.StatusInactive}
					}
					m.revision++
					return m, nil
				}
				if owner := m.statuses[conn.Name].Owner; owner != "" {
					m.statusMsg = fmt.Sprintf("port %d is currently used by tunnel %q", conn.LocalPort, owner)
				} else {
					m.statusMsg = fmt.Sprintf("port %d is occupied by a process not owned by diglet", conn.LocalPort)
				}
				return m, nil
			}

			m.statuses[conn.Name] = tunnel.Info{Status: tunnel.StatusConnecting}
			m.revision++
			return m, startTunnel(conn)
		}
	}

	return m, nil
}

func hotkey(key, label string) string {
	return keyStyle.Render("["+key+"]") + " " + dimStyle.Render(label)
}

func buildCommand(conn config.Connection) string {
	switch conn.TunnelType {
	case "ssh":
		return fmt.Sprintf("ssh -L %d:localhost:%d -N %s",
			conn.LocalPort, conn.RemotePort, conn.SSHHost)
	case "kubectl":
		resource := conn.Resource
		if resource == "" {
			resource = "(resource not set)"
		}
		ns := ""
		if conn.Namespace != "" {
			ns = " -n " + conn.Namespace
		}
		return fmt.Sprintf("kubectl port-forward %s %d:%d%s",
			resource, conn.LocalPort, conn.RemotePort, ns)
	case "docker":
		return "docker  (not yet implemented)"
	default:
		return "unknown tunnel type"
	}
}

func (m *model) statusIndicator(conn config.Connection) (glyph, name string) {
	switch m.statuses[conn.Name].Status {
	case tunnel.StatusActive:
		return activeStyle.Render("●"), activeStyle.Render(conn.Name)
	case tunnel.StatusConnecting:
		return connectingStyle.Render("◌"), connectingStyle.Render(conn.Name)
	case tunnel.StatusOccupied:
		return errorStyle.Render("!"), errorStyle.Render(conn.Name)
	}
	return dimStyle.Render("○"), conn.Name
}

func (m *model) statusLabel(conn config.Connection) string {
	switch m.statuses[conn.Name].Status {
	case tunnel.StatusActive:
		return activeStyle.Render("● active")
	case tunnel.StatusConnecting:
		return connectingStyle.Render("◌ connecting")
	case tunnel.StatusOccupied:
		if owner := m.statuses[conn.Name].Owner; owner != "" {
			return errorStyle.Render("! occupied by " + owner)
		}
		return errorStyle.Render("! port occupied")
	}
	return dimStyle.Render("○ inactive")
}

func (m *model) renderPreview(width, height int) string {
	ps := previewStyle.Width(width).Height(height)

	if m.mode == modeType {
		types := m.typeList()
		hovered := types[m.typeCursor]
		conns := m.connectionsOfType(hovered)

		var sb strings.Builder
		label := hovered
		if hovered == "all" {
			label = "all types"
		}
		sb.WriteString(titleStyle.Render(label) + "\n\n")

		if len(conns) == 0 {
			sb.WriteString(dimStyle.Render("no connections of this type"))
		} else {
			for _, c := range conns {
				indicator, _ := m.statusIndicator(c)
				sb.WriteString(fmt.Sprintf("%s  %s\n", indicator, c.Name))
			}
		}

		return ps.Render(sb.String())
	}

	if len(m.filtered) == 0 {
		return ps.Render(dimStyle.Render("no connections"))
	}

	conn := m.filtered[m.cursor]
	status := m.statusLabel(conn)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render(conn.Name) + "\n\n")
	sb.WriteString(fmt.Sprintf("%-12s %s\n", dimStyle.Render("type"), conn.TunnelType))

	switch conn.TunnelType {
	case "ssh":
		sb.WriteString(fmt.Sprintf("%-12s %s\n", dimStyle.Render("host"), conn.SSHHost))
	case "kubectl":
		sb.WriteString(fmt.Sprintf("%-12s %s\n", dimStyle.Render("resource"), conn.Resource))
		if conn.Namespace != "" {
			sb.WriteString(fmt.Sprintf("%-12s %s\n", dimStyle.Render("namespace"), conn.Namespace))
		}
	case "docker":
		sb.WriteString(fmt.Sprintf("%-12s %s\n", dimStyle.Render("container"), conn.Container))
	}

	sb.WriteString(fmt.Sprintf("%-12s %d → %d\n", dimStyle.Render("ports"), conn.LocalPort, conn.RemotePort))
	sb.WriteString(fmt.Sprintf("%-12s %s\n", dimStyle.Render("status"), status))
	pid := m.statuses[conn.Name].PID
	pidStr := "n/a"
	if pid > 0 {
		pidStr = fmt.Sprintf("%d", pid)
	}
	sb.WriteString(fmt.Sprintf("%-12s %s\n", dimStyle.Render("pid"), pidStr))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("command") + "\n")
	sb.WriteString(dimStyle.Render("  "+buildCommand(conn)) + "\n")

	if m.statusMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(errorStyle.Render("! " + m.statusMsg))
	}

	return ps.Render(sb.String())
}

func (m *model) renderList(width, height int) string {
	ls := listStyle.Width(width).Height(height)
	var sb strings.Builder

	if m.mode == modeType {
		types := m.typeList()
		sb.WriteString(titleStyle.Render("type filter") +
			"  " + dimStyle.Render(fmt.Sprintf("[%d]", len(types))) + "\n\n")

		for i, t := range types {
			cursor := "  "
			if i == m.typeCursor {
				cursor = cursorStyle.Render("▶ ")
			}

			indicator := dimStyle.Render("○")
			label := t
			if t == m.typeFilter {
				indicator = activeStyle.Render("●")
				label = activeStyle.Render(t)
			}

			count := dimStyle.Render(fmt.Sprintf("(%d)", m.typeCount(t)))
			sb.WriteString(fmt.Sprintf("%s%s  %-12s %s\n", cursor, indicator, label, count))
		}

		sb.WriteString("\n")
		sb.WriteString(hotkey("↵", "select") + "   " + hotkey("esc", "cancel"))
		return ls.Render(sb.String())
	}

	total := len(m.connections)
	shown := len(m.filtered)
	header := titleStyle.Render("connections")
	if m.mode == modeFilter && m.filter.Value() != "" {
		header += "  " + dimStyle.Render(fmt.Sprintf("[%d/%d]", shown, total)) +
			"  " + dimStyle.Render(`"`+m.filter.Value()+`"`)
	} else if m.typeFilter != "all" {
		header += "  " + dimStyle.Render(fmt.Sprintf("[%d/%d]", shown, total)) +
			"  " + dimStyle.Render("· "+m.typeFilter)
	} else {
		header += "  " + dimStyle.Render(fmt.Sprintf("[%d]", total))
	}
	sb.WriteString(header + "\n\n")

	for i, conn := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▶ ")
		}

		indicator, name := m.statusIndicator(conn)
		typeTag := dimStyle.Render("[" + conn.TunnelType + "]")
		sb.WriteString(fmt.Sprintf("%s%s %s %s\n", cursor, indicator, name, typeTag))
	}

	sb.WriteString("\n")
	if m.mode == modeFilter {
		sb.WriteString(dimStyle.Render("/") + " " + m.filter.View() + "\n")
		sb.WriteString(hotkey("↵", "confirm") + "   " + hotkey("esc", "cancel"))
	} else {
		sb.WriteString(
			hotkey("/", "search") + "   " +
				hotkey("t", "type") + "   " +
				hotkey("↵", "connect") + "   " +
				hotkey("K", "kill all") + "   " +
				hotkey("e", "edit") + "   " +
				hotkey("q", "quit"),
		)
	}

	return ls.Render(sb.String())
}

func (m *model) View() string {
	if m.width == 0 {
		return "loading..."
	}

	halfWidth := m.width/2 - 2

	list := m.renderList(halfWidth, m.height-2)
	preview := m.renderPreview(halfWidth, m.height-2)

	return lipgloss.JoinHorizontal(lipgloss.Top, list, preview)
}

func Run(cfg *config.Config, cfgPath string) error {
	m := newModel(cfg, cfgPath)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
