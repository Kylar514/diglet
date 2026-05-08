package tui

import (
	"fmt"
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
	titleStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Bold(true)
	errorStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
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
)

type tunnelReadyMsg struct {
	name string
	err  error
}

type model struct {
	connections []config.Connection
	filtered    []config.Connection
	cursor      int
	mode        mode
	filter      textinput.Model
	sortType    string
	statusMsg   string
	connecting  map[string]bool
	width       int
	height      int
}

func newModel(cfg *config.Config) model {
	ti := textinput.New()
	ti.Placeholder = "fuzzy filter..."
	ti.CharLimit = 64

	return model{
		connections: cfg.Connections,
		filtered:    cfg.Connections,
		filter:      ti,
		sortType:    "all",
		connecting:  make(map[string]bool),
	}
}

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) applyFilter() {
	query := m.filter.Value()
	if query == "" {
		m.filtered = m.connections
		m.cursor = 0
		return
	}

	names := make([]string, len(m.connections))
	for i, c := range m.connections {
		names[i] = c.Name
	}

	matches := fuzzy.Find(query, names)
	m.filtered = make([]config.Connection, 0, len(matches))
	for _, match := range matches {
		m.filtered = append(m.filtered, m.connections[match.Index])
	}
	m.cursor = 0
}

func (m *model) applySort() {
	if m.sortType == "all" {
		m.filtered = m.connections
		return
	}
	filtered := make([]config.Connection, 0)
	for _, c := range m.connections {
		if c.TunnelType == m.sortType {
			filtered = append(filtered, c)
		}
	}
	m.filtered = filtered
	m.cursor = 0
}

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
				m.filtered = m.connections
				m.cursor = 0
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

		case "s":
			types := append([]string{"all"}, tunnel.RegisteredTypes()...)
			for i, t := range types {
				if t == m.sortType {
					m.sortType = types[(i+1)%len(types)]
					break
				}
			}
			m.applySort()

		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			conn := m.filtered[m.cursor]

			// Don't allow double-toggling while a probe is in flight.
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
	previewStyle := previewStyle.Width(width).Height(height)
	if len(m.filtered) == 0 {
		return previewStyle.Render(dimStyle.Render("no connections"))
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
	sb.WriteString(fmt.Sprintf("%-12s %s\n", "type:", conn.TunnelType))

	switch conn.TunnelType {
	case "ssh":
		sb.WriteString(fmt.Sprintf("%-12s %s\n", "host:", conn.SSHHost))
	case "kubectl":
		sb.WriteString(fmt.Sprintf("%-12s %s\n", "resource:", conn.Resource))
		if conn.Namespace != "" {
			sb.WriteString(fmt.Sprintf("%-12s %s\n", "namespace:", conn.Namespace))
		}
	case "docker":
		sb.WriteString(fmt.Sprintf("%-12s %s\n", "container:", conn.Container))
	}

	sb.WriteString(fmt.Sprintf("%-12s %d → %d\n", "ports:", conn.LocalPort, conn.RemotePort))
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("command:") + "\n")
	sb.WriteString(buildCommand(conn) + "\n")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("%-12s %s\n", "status:", status))

	return previewStyle.Render(sb.String())
}

func (m *model) renderList(width, height int) string {
	listStyle := listStyle.Width(width).Height(height)
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("connections") + "\n\n")

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
		sb.WriteString("/" + m.filter.View())
	} else {
		statusLine := dimStyle.Render("/ filter  s sort  enter toggle  q quit")
		if m.statusMsg != "" {
			statusLine = errorStyle.Render("! " + m.statusMsg)
		}
		sb.WriteString(statusLine)
	}

	return listStyle.Render(sb.String())
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

func Run(cfg *config.Config) error {
	m := newModel(cfg)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
