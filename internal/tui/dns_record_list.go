package tui

import (
	"context"
	"fmt"
	"strings"

	"nathanbeddoewebdev/vpsm/internal/dns/domain"
	"nathanbeddoewebdev/vpsm/internal/dns/services"
	"nathanbeddoewebdev/vpsm/internal/tui/components"
	"nathanbeddoewebdev/vpsm/internal/tui/styles"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Messages ---

type dnsRecordsLoadedMsg struct {
	records []domain.Record
}

type dnsRecordsErrorMsg struct {
	err error
}

// --- Record list model ---

type dnsRecordListModel struct {
	service      *services.Service
	providerName string
	domain       string

	records   []domain.Record
	filtered  []domain.Record
	cursor    int
	listStart int // for scrolling

	typeFilter string // e.g. "A", "CNAME", "" for all
	typeTypes  []string

	width  int
	height int

	loading          bool
	spinner          spinner.Model
	err              error
	status           string
	statusIsError    bool
	persistentStatus string

	embedded bool
}

func newDNSRecordListModel(svc *services.Service, providerName, domainName string, embedded bool, width, height int) dnsRecordListModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Blue)

	return dnsRecordListModel{
		service:      svc,
		providerName: providerName,
		domain:       domainName,
		typeTypes:    []string{"", "A", "AAAA", "CNAME", "MX", "TXT"},
		typeFilter:   "",
		embedded:     embedded,
		width:        width,
		height:       height,
		loading:      true,
		spinner:      s,
	}
}

func (m dnsRecordListModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadRecordsCmd())
}

func (m dnsRecordListModel) loadRecordsCmd() tea.Cmd {
	return func() tea.Msg {
		records, err := m.service.ListRecords(context.Background(), m.domain)
		if err != nil {
			return dnsRecordsErrorMsg{err}
		}
		return dnsRecordsLoadedMsg{records}
	}
}

func (m *dnsRecordListModel) applyFilter() {
	m.filtered = make([]domain.Record, 0)
	for _, r := range m.records {
		if m.typeFilter == "" || strings.EqualFold(string(r.Type), m.typeFilter) {
			m.filtered = append(m.filtered, r)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	}
	if m.listStart >= len(m.filtered) {
		m.listStart = 0
	}
}

func (m dnsRecordListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			if m.embedded {
				return m, func() tea.Msg { return dnsNavigateBackMsg{} }
			}
			return m, tea.Quit
		case "q":
			if !m.embedded {
				return m, tea.Quit
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "f":
			// Cycle type filter
			idx := 0
			for i, t := range m.typeTypes {
				if t == m.typeFilter {
					idx = i
					break
				}
			}
			idx = (idx + 1) % len(m.typeTypes)
			m.typeFilter = m.typeTypes[idx]
			m.applyFilter()
		case "r":
			m.loading = true
			m.err = nil
			return m, m.loadRecordsCmd()
		case "enter":
			if len(m.filtered) > 0 {
				rec := m.filtered[m.cursor]
				if m.embedded {
					return m, func() tea.Msg { return dnsNavigateToRecordShowMsg{record: rec, domain: m.domain} }
				}
			}
		case "c":
			if m.embedded {
				return m, func() tea.Msg { return dnsNavigateToRecordCreateMsg{domain: m.domain} }
			}
		case "d":
			if len(m.filtered) > 0 {
				rec := m.filtered[m.cursor]
				if m.embedded {
					return m, func() tea.Msg { return dnsNavigateToRecordDeleteMsg{record: rec, domain: m.domain} }
				}
			}
		case "e":
			if len(m.filtered) > 0 {
				rec := m.filtered[m.cursor]
				if m.embedded {
					return m, func() tea.Msg { return dnsNavigateToRecordEditMsg{record: rec, domain: m.domain} }
				}
			}
		}

	case dnsRecordsLoadedMsg:
		m.loading = false
		m.records = msg.records
		m.applyFilter()

		status := fmt.Sprintf("Loaded %d records.", len(m.records))
		if m.persistentStatus != "" {
			status = m.persistentStatus + " | " + status
			// We keep persistentStatus around until the user does something else
		}
		m.status = status

	case dnsRecordsErrorMsg:
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

func (m dnsRecordListModel) View() string {
	header := components.Header(m.width, "dns > "+m.domain, m.providerName)

	bindings := []components.KeyBinding{
		{Key: "j/k", Desc: "nav"},
		{Key: "enter", Desc: "show"},
		{Key: "c", Desc: "create"},
		{Key: "d", Desc: "delete"},
		{Key: "e", Desc: "edit"},
		{Key: "f", Desc: "filter"},
		{Key: "esc", Desc: "back"},
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
		content = fmt.Sprintf("\n  %s Loading records...", m.spinner.View())
	} else if m.err != nil {
		content = fmt.Sprintf("\n  %s", styles.ErrorText.Render(m.err.Error()))
	} else if len(m.records) == 0 {
		content = "\n  No records found for this domain."
	} else {
		content = m.renderFilterBar() + "\n" + m.renderTable(contentH-2)
	}

	lines := lipgloss.Height(content)
	if lines < contentH {
		content += lipgloss.NewStyle().Height(contentH - lines).Render("")
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, statusBar, footer)
}

func (m dnsRecordListModel) renderFilterBar() string {
	var parts []string
	parts = append(parts, "  Filter: ")

	for _, t := range m.typeTypes {
		label := t
		if t == "" {
			label = "All"
		}

		if t == m.typeFilter {
			parts = append(parts, fmt.Sprintf("[%s]", styles.AccentText.Render(label)))
		} else {
			parts = append(parts, fmt.Sprintf(" %s ", styles.MutedText.Render(label)))
		}
	}

	return strings.Join(parts, "")
}

func (m dnsRecordListModel) renderTable(height int) string {
	if len(m.filtered) == 0 {
		return "\n  No records match current filter."
	}

	cols := []int{30, 8, 40, 8}

	header := styles.TableHeader.Render(
		fmt.Sprintf("  %-*s %-*s %-*s %-*s",
			cols[0], "NAME",
			cols[1], "TYPE",
			cols[2], "CONTENT",
			cols[3], "TTL",
		),
	)

	var rows []string
	rows = append(rows, header)

	if m.cursor < m.listStart {
		m.listStart = m.cursor
	} else if m.cursor >= m.listStart+(height-1) {
		m.listStart = m.cursor - (height - 2)
	}

	end := m.listStart + height - 1
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	for i := m.listStart; i < end; i++ {
		r := m.filtered[i]

		cursor := " "
		rowStyle := styles.TableCell
		if i == m.cursor {
			cursor = styles.AccentText.Render(">")
			rowStyle = styles.TableSelectedRow
		}

		// Truncate content if too long
		contentStr := r.Content
		if len(contentStr) > cols[2]-2 {
			contentStr = contentStr[:cols[2]-5] + "..."
		}

		typeColor := styles.Value
		switch r.Type {
		case "A", "AAAA":
			typeColor = lipgloss.NewStyle().Foreground(styles.Green)
		case "CNAME":
			typeColor = lipgloss.NewStyle().Foreground(styles.Yellow)
		case "MX":
			typeColor = lipgloss.NewStyle().Foreground(styles.Blue)
		case "TXT":
			typeColor = styles.MutedText
		}

		row := fmt.Sprintf("%s %-*s %-*s %-*s %-*d",
			cursor,
			cols[0], r.Name,
			cols[1], typeColor.Render(string(r.Type)),
			cols[2], contentStr,
			cols[3], r.TTL,
		)
		rows = append(rows, rowStyle.Render(row))
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
