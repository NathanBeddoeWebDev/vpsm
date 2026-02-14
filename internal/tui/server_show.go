package tui

import (
	"context"
	"fmt"
	"strings"

	"nathanbeddoewebdev/vpsm/internal/domain"
	"nathanbeddoewebdev/vpsm/internal/tui/components"
	"nathanbeddoewebdev/vpsm/internal/tui/styles"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Messages ---

type serverDetailLoadedMsg struct {
	server *domain.Server
}

type serverDetailErrorMsg struct {
	err error
}

// --- Show result ---

// ShowResult holds the outcome of the server show TUI.
type ShowResult struct {
	Server *domain.Server
	Action string // "delete" or ""
}

// --- Phases ---

type showPhase int

const (
	showPhaseSelect showPhase = iota // selecting a server from the list
	showPhaseDetail                  // displaying server details
)

// --- Server show model ---

type serverShowModel struct {
	provider     domain.Provider
	providerName string

	phase showPhase

	// Select phase.
	servers   []domain.Server
	cursor    int
	listStart int

	// Detail phase.
	serverID string
	server   *domain.Server

	// Whether we came from the select phase (enables going back).
	fromSelect bool

	width  int
	height int

	loading bool
	spinner spinner.Model
	err     error

	// Status bar state.
	status        string
	statusIsError bool

	// toggling is true while an async start/stop API call is in flight.
	toggling bool
	// toggleResult holds a success message to display after the post-toggle
	// refresh completes.
	toggleResult string

	action   string
	quitting bool
}

// RunServerShow starts the full-window server detail TUI.
// If serverID is empty, the TUI first shows a server selection list.
func RunServerShow(provider domain.Provider, providerName string, serverID string) (*ShowResult, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Blue)

	m := serverShowModel{
		provider:     provider,
		providerName: providerName,
		loading:      true,
		spinner:      s,
	}

	if serverID != "" {
		// Direct detail fetch.
		m.phase = showPhaseDetail
		m.serverID = serverID
	} else {
		// Select-then-detail flow.
		m.phase = showPhaseSelect
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run server show: %w", err)
	}

	final := result.(serverShowModel)
	if final.quitting || final.server == nil {
		return nil, nil
	}
	return &ShowResult{Server: final.server, Action: final.action}, nil
}

// RunServerShowDirect starts the detail view with an already-loaded server.
func RunServerShowDirect(provider domain.Provider, providerName string, server *domain.Server) (*ShowResult, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Blue)

	m := serverShowModel{
		provider:     provider,
		providerName: providerName,
		phase:        showPhaseDetail,
		server:       server,
		loading:      false,
		spinner:      s,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run server show: %w", err)
	}

	final := result.(serverShowModel)
	if final.quitting {
		return nil, nil
	}
	return &ShowResult{Server: final.server, Action: final.action}, nil
}

func (m serverShowModel) Init() tea.Cmd {
	if m.loading {
		switch m.phase {
		case showPhaseSelect:
			return tea.Batch(m.spinner.Tick, m.fetchServers())
		case showPhaseDetail:
			return tea.Batch(m.spinner.Tick, m.fetchServer())
		}
	}
	return nil
}

func (m serverShowModel) fetchServers() tea.Cmd {
	return func() tea.Msg {
		servers, err := m.provider.ListServers(context.Background())
		if err != nil {
			return serversErrorMsg{err: err}
		}
		return serversLoadedMsg{servers: servers}
	}
}

func (m serverShowModel) fetchServer() tea.Cmd {
	return func() tea.Msg {
		server, err := m.provider.GetServer(context.Background(), m.serverID)
		if err != nil {
			return serverDetailErrorMsg{err: err}
		}
		return serverDetailLoadedMsg{server: server}
	}
}

// toggleServer fires an async start or stop API call based on the server's
// current status.
func (m serverShowModel) toggleServer(server *domain.Server) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		switch server.Status {
		case "running":
			if err := m.provider.StopServer(ctx, server.ID); err != nil {
				return serverToggleErrorMsg{err: fmt.Errorf("failed to stop server %q: %w", server.Name, err)}
			}
			return serverToggleDoneMsg{serverName: server.Name, action: "stopped"}
		case "off", "stopped":
			if err := m.provider.StartServer(ctx, server.ID); err != nil {
				return serverToggleErrorMsg{err: fmt.Errorf("failed to start server %q: %w", server.Name, err)}
			}
			return serverToggleDoneMsg{serverName: server.Name, action: "started"}
		default:
			return serverToggleErrorMsg{
				err: fmt.Errorf("cannot start/stop server %q: current status is %q", server.Name, server.Status),
			}
		}
	}
}

func (m serverShowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case serversLoadedMsg:
		m.loading = false
		m.servers = msg.servers
		m.err = nil
		return m, nil

	case serversErrorMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case serverDetailLoadedMsg:
		m.loading = false
		m.server = msg.server
		m.err = nil
		if m.toggleResult != "" {
			m.status = m.toggleResult
			m.statusIsError = false
			m.toggleResult = ""
		} else {
			m.status = ""
			m.statusIsError = false
		}
		return m, nil

	case serverDetailErrorMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case serverToggleDoneMsg:
		m.toggling = false
		m.toggleResult = fmt.Sprintf("Server %q %s successfully", msg.serverName, msg.action)
		if m.phase == showPhaseDetail && m.server != nil {
			// Refresh the detail view.
			m.loading = true
			m.err = nil
			m.serverID = m.server.ID
			return m, tea.Batch(m.spinner.Tick, m.fetchServer())
		}
		return m, nil

	case serverToggleErrorMsg:
		m.toggling = false
		m.status = msg.err.Error()
		m.statusIsError = true
		return m, nil

	case spinner.TickMsg:
		if m.loading || m.toggling {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m serverShowModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	// Block input while loading or toggling.
	if m.loading || m.toggling {
		return m, nil
	}

	switch m.phase {
	case showPhaseSelect:
		return m.handleSelectKey(msg)
	case showPhaseDetail:
		return m.handleDetailKey(msg)
	}

	return m, nil
}

func (m serverShowModel) handleSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		if msg.String() == "q" || msg.String() == "esc" {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "q", "esc":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.servers)-1 {
			m.cursor++
		}
	case "g":
		m.cursor = 0
	case "G":
		if len(m.servers) > 0 {
			m.cursor = len(m.servers) - 1
		}
	case "enter":
		if len(m.servers) > 0 {
			selected := m.servers[m.cursor]
			m.serverID = selected.ID
			m.phase = showPhaseDetail
			m.fromSelect = true
			m.loading = true
			m.err = nil
			m.status = ""
			m.statusIsError = false
			return m, tea.Batch(m.spinner.Tick, m.fetchServer())
		}
	}

	return m, nil
}

func (m serverShowModel) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		if m.fromSelect {
			// Go back to the select phase.
			m.phase = showPhaseSelect
			m.server = nil
			m.serverID = ""
			m.err = nil
			m.status = ""
			m.statusIsError = false
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "d":
		if m.server != nil {
			m.action = "delete"
			return m, tea.Quit
		}

	case "s":
		if m.server != nil {
			switch m.server.Status {
			case "running":
				m.toggling = true
				m.status = fmt.Sprintf("Stopping server %q...", m.server.Name)
				m.statusIsError = false
				return m, tea.Batch(m.spinner.Tick, m.toggleServer(m.server))
			case "off", "stopped":
				m.toggling = true
				m.status = fmt.Sprintf("Starting server %q...", m.server.Name)
				m.statusIsError = false
				return m, tea.Batch(m.spinner.Tick, m.toggleServer(m.server))
			default:
				m.status = fmt.Sprintf("Cannot start/stop server %q: status is %q", m.server.Name, m.server.Status)
				m.statusIsError = true
			}
		}

	case "r":
		if m.server != nil {
			m.loading = true
			m.serverID = m.server.ID
			m.err = nil
			m.status = ""
			m.statusIsError = false
			return m, tea.Batch(m.spinner.Tick, m.fetchServer())
		}
	}

	return m, nil
}

func (m serverShowModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	header := components.Header(m.width, "server show", m.providerName)

	var footerBindings []components.KeyBinding
	switch {
	case m.loading || m.toggling:
		footerBindings = []components.KeyBinding{{Key: "ctrl+c", Desc: "quit"}}
	case m.phase == showPhaseSelect:
		footerBindings = []components.KeyBinding{
			{Key: "j/k", Desc: "navigate"},
			{Key: "enter", Desc: "select"},
			{Key: "q", Desc: "quit"},
		}
	case m.phase == showPhaseDetail:
		bindings := []components.KeyBinding{
			{Key: "s", Desc: "start/stop"},
			{Key: "d", Desc: "delete"},
			{Key: "r", Desc: "refresh"},
		}
		if m.fromSelect {
			bindings = append(bindings, components.KeyBinding{Key: "esc", Desc: "back"})
		}
		bindings = append(bindings, components.KeyBinding{Key: "q", Desc: "quit"})
		footerBindings = bindings
	}
	footer := components.Footer(m.width, footerBindings)

	statusBar := ""
	if m.err != nil {
		// Errors are rendered inline in the content area, not in the status bar.
	} else if m.status != "" {
		statusBar = components.StatusBar(m.width, m.status, m.statusIsError)
	}

	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	statusH := lipgloss.Height(statusBar)
	contentH := m.height - headerH - footerH - statusH
	if contentH < 1 {
		contentH = 1
	}

	content := m.renderContent(contentH)

	// Assemble the full layout.
	sections := []string{header, content}
	if statusBar != "" {
		sections = append(sections, statusBar)
	}
	sections = append(sections, footer)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m serverShowModel) renderContent(height int) string {
	if m.loading {
		var loadingLabel string
		switch m.phase {
		case showPhaseSelect:
			loadingLabel = "Fetching servers..."
		default:
			loadingLabel = "Fetching server details..."
		}
		loadingText := m.spinner.View() + "  " + loadingLabel
		return lipgloss.Place(
			m.width, height,
			lipgloss.Center, lipgloss.Center,
			styles.MutedText.Render(loadingText),
		)
	}

	if m.toggling {
		toggleText := m.spinner.View() + "  " + m.status
		return lipgloss.Place(
			m.width, height,
			lipgloss.Center, lipgloss.Center,
			styles.MutedText.Render(toggleText),
		)
	}

	if m.err != nil {
		backHint := "Press q to go back."
		if m.fromSelect && m.phase == showPhaseDetail {
			backHint = "Press esc to go back."
		}
		errText := styles.ErrorText.Render("Error: "+m.err.Error()) + "\n\n" +
			styles.MutedText.Render(backHint)
		return lipgloss.Place(
			m.width, height,
			lipgloss.Center, lipgloss.Center,
			errText,
		)
	}

	switch m.phase {
	case showPhaseSelect:
		return m.renderSelectPhase(height)
	case showPhaseDetail:
		return m.renderDetailPhase(height)
	}

	return ""
}

func (m serverShowModel) renderSelectPhase(height int) string {
	if len(m.servers) == 0 {
		return lipgloss.Place(
			m.width, height,
			lipgloss.Center, lipgloss.Center,
			styles.MutedText.Render("No servers found."),
		)
	}

	title := styles.Title.Render("Select a server")

	maxVisible := height - 4
	if maxVisible < 3 {
		maxVisible = 3
	}

	// Scrolling.
	start := m.listStart
	if m.cursor < start {
		start = m.cursor
	}
	if m.cursor >= start+maxVisible {
		start = m.cursor - maxVisible + 1
	}

	end := start + maxVisible
	if end > len(m.servers) {
		end = len(m.servers)
	}

	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		s := m.servers[i]
		prefix := "  "
		if i == m.cursor {
			prefix = styles.AccentText.Render("> ")
		}

		label := serverOptionLabel(s)
		if i == m.cursor {
			label = styles.Value.Bold(true).Render(label)
		} else {
			label = styles.MutedText.Render(label)
		}

		rows = append(rows, prefix+label)
	}

	listContent := strings.Join(rows, "\n")

	combined := lipgloss.JoinVertical(lipgloss.Left, title, "", listContent)
	return lipgloss.Place(
		m.width, height,
		lipgloss.Center, lipgloss.Center,
		combined,
	)
}

func (m serverShowModel) renderDetailPhase(height int) string {
	if m.server == nil {
		return lipgloss.Place(
			m.width, height,
			lipgloss.Center, lipgloss.Center,
			styles.MutedText.Render("No server data."),
		)
	}

	return m.renderDetail(height)
}

func (m serverShowModel) renderDetail(height int) string {
	s := m.server

	// Calculate card width (responsive, capped).
	cardWidth := m.width - 8
	if cardWidth > 72 {
		cardWidth = 72
	}
	if cardWidth < 30 {
		cardWidth = 30
	}

	labelWidth := 14
	valueWidth := cardWidth - labelWidth - 8 // padding + border

	renderField := func(label, value string) string {
		l := styles.Label.Width(labelWidth).Render(label)
		v := styles.Value.Width(valueWidth).Render(value)
		return l + v
	}

	// Server name + status header.
	nameTitle := styles.Title.Render(s.Name)
	statusBadge := styles.StatusIndicator(s.Status)
	titleLine := nameTitle + "  " + statusBadge

	// --- Overview section ---
	overviewFields := []string{
		renderField("ID", s.ID),
		renderField("Provider", s.Provider),
		renderField("Type", s.ServerType),
		renderField("Region", s.Region),
	}
	if s.Image != "" {
		overviewFields = append(overviewFields, renderField("Image", s.Image))
	}
	if !s.CreatedAt.IsZero() {
		overviewFields = append(overviewFields, renderField("Created", s.CreatedAt.UTC().Format("2006-01-02 15:04:05 UTC")))
	}

	overviewContent := strings.Join(overviewFields, "\n")

	// --- Network section ---
	var networkFields []string
	if s.PublicIPv4 != "" {
		networkFields = append(networkFields, renderField("IPv4", s.PublicIPv4))
	}
	if s.PublicIPv6 != "" {
		networkFields = append(networkFields, renderField("IPv6", s.PublicIPv6))
	}
	if s.PrivateIPv4 != "" {
		networkFields = append(networkFields, renderField("Private IP", s.PrivateIPv4))
	}

	// Build sections.
	sectionStyle := styles.Card.Width(cardWidth)

	sections := []string{
		titleLine,
		"",
		sectionStyle.Render(
			styles.Subtitle.Render("Overview") + "\n\n" + overviewContent,
		),
	}

	if len(networkFields) > 0 {
		networkContent := strings.Join(networkFields, "\n")
		sections = append(sections, sectionStyle.Render(
			styles.Subtitle.Render("Network")+"\n\n"+networkContent,
		))
	}

	detail := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Center the detail card in available space.
	return lipgloss.Place(
		m.width, height,
		lipgloss.Center, lipgloss.Center,
		detail,
	)
}