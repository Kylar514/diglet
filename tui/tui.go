package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// tunnelReadyMsg is sent back to Update() from the async probe goroutine.
type tunnelReadyMsg struct {
	name string
	err  error
}

// editorFinishedMsg is sent back to Update() after the editor process exits.
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
	connecting  map[string]bool
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
		connecting:  make(map[string]bool),
		cfgPath:     cfgPath,
	}
}

func (m *model) Init() tea.Cmd {
	return nil
}

// typeList returns the full ordered list of selectable type entries: "all" first,
// then registered tunnel types alphabetically.
func (m *model) typeList() []string {
	return append([]string{"all"}, tunnel.RegisteredTypes()...)
}

// typeCount returns the number of connections matching a given type ("all" returns total).
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

// connectionsOfType returns all connections matching a given type ("all" returns all).
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

// openEditor suspends the TUI, opens $EDITOR on the config file, then resumes.
// The result is delivered back as an editorFinishedMsg.
func openEditor(cfgPath string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Return a plain function that sends the error as a msg — no exec needed.
		return func() tea.Msg {
			return editorFinishedMsg{err: fmt.Errorf("$EDITOR is not set")}
		}
	}
	return tea.ExecProcess(exec.Command(editor, cfgPath), func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

// startTunnel returns a tea.Cmd that launches the tunnel process then probes
// the local port. The result is delivered back as a tunnelReadyMsg.
func startTunnel(conn config.Connection) tea.Cmd {
	return func() tea.Msg {
		if err := tunnel.StartProcess(conn); err != nil {
			return tunnelReadyMsg{name: conn.Name, err: err}
		}
		if err := tunnel.Probe(conn.LocalPort); err != nil {
			tunnel.Abort(conn.Name)
			return tunnelReadyMsg{name: conn.Name, err: err}
		}
		if err := tunnel.Confirm(conn.Name); err != nil {
			_ = err
		}
		return tunnelReadyMsg{name: conn.Name, err: nil}
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tunnelReadyMsg:
		delete(m.connecting, msg.name)
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("%s: %s", msg.name, msg.err.Error())
		}
		return m, nil

	case editorFinishedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("editor: %s", msg.err.Error())
			return m, nil
		}
		// Reload config; on parse error keep existing connections and show error.
		newCfg, err := config.Load(m.cfgPath)
		if err != nil {
			m.statusMsg = fmt.Sprintf("config reload failed: %s", err.Error())
			return m, nil
		}
		m.connections = newCfg.Connections
		// Re-apply current filters so the list reflects the updated config.
		m.applyTypeFilter()
		if m.filter.Value() != "" {
			m.applyFilter()
		}
		// Clamp cursor in case the list shrank.
		if m.cursor >= len(m.filtered) {
			m.cursor = max(0, len(m.filtered)-1)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		m.statusMsg = ""

		// --- filter mode ---
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

		// --- type mode ---
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
				// Clear any active fuzzy filter when switching type.
				m.filter.SetValue("")
				m.mode = modeNormal
			}
			return m, nil
		}

		// --- normal mode ---
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

		case "t":
			m.mode = modeType
			// Position the type cursor on the currently active filter.
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

			if m.connecting[conn.Name] {
				return m, nil
			}

			if tunnel.IsActive(conn.Name) {
				if err := tunnel.Stop(conn.Name); err != nil {
					m.statusMsg = err.Error()
				}
				return m, nil
			}

			m.connecting[conn.Name] = true
			return m, startTunnel(conn)
		}
	}

	return m, nil
}

// hotkey renders a single [key] label pair.
func hotkey(key, label string) string {
	return keyStyle.Render("["+key+"]") + " " + dimStyle.Render(label)
}

// buildCommand returns the CLI command string for the preview pane.
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

func (m *model) renderPreview(width, height int) string {
	ps := previewStyle.Width(width).Height(height)

	// In type mode: show the connections belonging to the hovered type.
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
				var indicator string
				switch {
				case m.connecting[c.Name]:
					indicator = connectingStyle.Render("◌")
				case tunnel.IsActive(c.Name):
					indicator = activeStyle.Render("●")
				default:
					indicator = dimStyle.Render("○")
				}
				sb.WriteString(fmt.Sprintf("%s  %s\n", indicator, c.Name))
			}
		}

		return ps.Render(sb.String())
	}

	// Normal / filter mode: show the selected connection detail.
	if len(m.filtered) == 0 {
		return ps.Render(dimStyle.Render("no connections"))
	}

	conn := m.filtered[m.cursor]

	var status string
	switch {
	case m.connecting[conn.Name]:
		status = connectingStyle.Render("◌ connecting...")
	case tunnel.IsActive(conn.Name):
		status = activeStyle.Render("● active")
	default:
		status = dimStyle.Render("● inactive")
	}

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

	// --- type mode ---
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

	// --- normal / filter mode ---
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

		var indicator, name string
		switch {
		case m.connecting[conn.Name]:
			indicator = connectingStyle.Render("◌")
			name = connectingStyle.Render(conn.Name)
		case tunnel.IsActive(conn.Name):
			indicator = activeStyle.Render("●")
			name = activeStyle.Render(conn.Name)
		default:
			indicator = dimStyle.Render("○")
			name = conn.Name
		}

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
