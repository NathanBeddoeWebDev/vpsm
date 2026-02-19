package tui

import (
	"context"
	"fmt"

	"nathanbeddoewebdev/vpsm/internal/dns/domain"
	"nathanbeddoewebdev/vpsm/internal/dns/services"
	"nathanbeddoewebdev/vpsm/internal/tui/components"
	"nathanbeddoewebdev/vpsm/internal/tui/styles"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Messages ---

type dnsDomainsLoadedMsg struct {
	domains []domain.Domain
}

type dnsDomainsErrorMsg struct {
	err error
}

// --- Domain list model ---

type dnsDomainListModel struct {
	service      *services.Service
	providerName string

	domains []domain.Domain
	cursor  int

	width  int
	height int

	loading       bool
	spinner       spinner.Model
	err           error
	status        string
	statusIsError bool

	embedded bool
}

func newDNSDomainListModel(svc *services.Service, providerName string, embedded bool, width, height int) dnsDomainListModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Blue)

	return dnsDomainListModel{
		service:      svc,
		providerName: providerName,
		embedded:     embedded,
		width:        width,
		height:       height,
		loading:      true,
		spinner:      s,
	}
}

func (m dnsDomainListModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadDomainsCmd())
}

func (m dnsDomainListModel) loadDomainsCmd() tea.Cmd {
	return func() tea.Msg {
		domains, err := m.service.ListDomains(context.Background())
		if err != nil {
			return dnsDomainsErrorMsg{err}
		}
		return dnsDomainsLoadedMsg{domains}
	}
}

func (m dnsDomainListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if !m.embedded {
				return m, tea.Quit
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.domains)-1 {
				m.cursor++
			}
		case "r":
			m.loading = true
			m.err = nil
			return m, m.loadDomainsCmd()
		case "enter":
			if len(m.domains) > 0 {
				dom := m.domains[m.cursor]
				if m.embedded {
					return m, func() tea.Msg { return dnsNavigateToRecordListMsg{domain: dom} }
				}
				return m, tea.Quit
			}
		}

	case dnsDomainsLoadedMsg:
		m.loading = false
		m.domains = msg.domains
		m.cursor = 0
		if len(m.domains) == 0 {
			m.status = "No domains found."
		} else {
			m.status = fmt.Sprintf("Loaded %d domains.", len(m.domains))
		}

	case dnsDomainsErrorMsg:
		m.loading = false
		m.err = msg.err
		m.status = msg.err.Error()
		m.statusIsError = true

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m dnsDomainListModel) View() string {
	header := components.Header(m.width, "dns > domains", m.providerName)

	bindings := []components.KeyBinding{
		{Key: "j/k", Desc: "navigate"},
		{Key: "enter", Desc: "records"},
		{Key: "r", Desc: "refresh"},
	}
	if !m.embedded {
		bindings = append(bindings, components.KeyBinding{Key: "q", Desc: "quit"})
	}
	footer := components.Footer(m.width, bindings)

	statusBar := components.StatusBar(m.width, m.status, m.statusIsError)

	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	statusH := lipgloss.Height(statusBar)
	contentH := m.height - headerH - footerH - statusH
	if contentH < 1 {
		contentH = 1
	}

	var content string
	if m.loading {
		content = fmt.Sprintf("\n  %s Loading domains...", m.spinner.View())
	} else if m.err != nil {
		content = fmt.Sprintf("\n  %s", styles.ErrorText.Render(m.err.Error()))
	} else if len(m.domains) == 0 {
		content = "\n  No domains found in this account."
	} else {
		content = m.renderTable(contentH)
	}

	// Pad content to fill height
	lines := lipgloss.Height(content)
	if lines < contentH {
		content += lipgloss.NewStyle().Height(contentH - lines).Render("")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, statusBar, footer)
}

func (m dnsDomainListModel) renderTable(height int) string {
	if len(m.domains) == 0 {
		return ""
	}

	cols := []int{30, 15, 10, 20}

	header := styles.TableHeader.Render(
		fmt.Sprintf("  %-*s %-*s %-*s %-*s",
			cols[0], "DOMAIN",
			cols[1], "STATUS",
			cols[2], "TLD",
			cols[3], "EXPIRES",
		),
	)

	var rows []string
	rows = append(rows, header)

	// Simple pagination/viewport calculation
	start := 0
	if m.cursor >= height-2 {
		start = m.cursor - (height - 3)
	}
	end := start + height - 2
	if end > len(m.domains) {
		end = len(m.domains)
	}

	for i := start; i < end; i++ {
		d := m.domains[i]

		cursor := " "
		rowStyle := styles.TableCell
		if i == m.cursor {
			cursor = styles.AccentText.Render(">")
			rowStyle = styles.TableSelectedRow
		}

		status := d.Status
		if status == "ACTIVE" {
			status = styles.SuccessText.Render(status)
		}

		row := fmt.Sprintf("%s %-*s %-*s %-*s %-*s",
			cursor,
			cols[0], d.Name,
			cols[1], status,
			cols[2], d.TLD,
			cols[3], d.ExpireDate,
		)
		rows = append(rows, rowStyle.Render(row))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
