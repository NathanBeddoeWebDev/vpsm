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

// --- Server show model ---

type serverShowModel struct {
	provider     domain.Provider
	providerName string

	// Either serverID (to fetch) or server (already fetched) is set.
	serverID string
	server   *domain.Server

	width  int
	height int

	loading bool
	spinner spinner.Model
	err     error

	action   string
	quitting bool
}

// RunServerShow starts the full-window server detail TUI.
func RunServerShow(provider domain.Provider, providerName string, serverID string) (*ShowResult, error) {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Blue)

	m := serverShowModel{
		provider:     provider,
		providerName: providerName,
		serverID:     serverID,
		loading:      true,
		spinner:      s,
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
		return tea.Batch(m.spinner.Tick, m.fetchServer())
	}
	return nil
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

func (m serverShowModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case serverDetailLoadedMsg:
		m.loading = false
		m.server = msg.server
		m.err = nil
		return m, nil

	case serverDetailErrorMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m serverShowModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		m.quitting = true
		return m, tea.Quit

	case "d":
		if m.server != nil {
			m.action = "delete"
			return m, tea.Quit
		}

	case "r":
		if m.server != nil {
			m.loading = true
			m.serverID = m.server.ID
			m.err = nil
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

	footerBindings := []components.KeyBinding{
		{Key: "d", Desc: "delete"},
		{Key: "r", Desc: "refresh"},
		{Key: "q", Desc: "back"},
	}
	footer := components.Footer(m.width, footerBindings)

	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	contentH := m.height - headerH - footerH
	if contentH < 1 {
		contentH = 1
	}

	content := m.renderContent(contentH)

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m serverShowModel) renderContent(height int) string {
	if m.loading {
		loadingText := m.spinner.View() + "  Fetching server details..."
		return lipgloss.Place(
			m.width, height,
			lipgloss.Center, lipgloss.Center,
			styles.MutedText.Render(loadingText),
		)
	}

	if m.err != nil {
		errText := styles.ErrorText.Render("Error: "+m.err.Error()) + "\n\n" +
			styles.MutedText.Render("Press q to go back.")
		return lipgloss.Place(
			m.width, height,
			lipgloss.Center, lipgloss.Center,
			errText,
		)
	}

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
