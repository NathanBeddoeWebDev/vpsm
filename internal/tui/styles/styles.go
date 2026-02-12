package styles

import "github.com/charmbracelet/lipgloss"

// --- Typography ---

var (
	// Title is the main header text style.
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(White)

	// Subtitle is used for secondary headings.
	Subtitle = lipgloss.NewStyle().
			Foreground(Gray)

	// Label is used for field names in detail views.
	Label = lipgloss.NewStyle().
		Foreground(Gray).
		Bold(true)

	// Value is used for field values in detail views.
	Value = lipgloss.NewStyle().
		Foreground(White)

	// MutedText is for help text, hints, and less important info.
	MutedText = lipgloss.NewStyle().
			Foreground(Muted)

	// AccentText is for highlighted interactive elements.
	AccentText = lipgloss.NewStyle().
			Foreground(Blue)

	// ErrorText is for error messages.
	ErrorText = lipgloss.NewStyle().
			Foreground(Red).
			Bold(true)

	// SuccessText is for success messages.
	SuccessText = lipgloss.NewStyle().
			Foreground(Green).
			Bold(true)

	// WarningText is for warning messages.
	WarningText = lipgloss.NewStyle().
			Foreground(Yellow).
			Bold(true)
)

// --- Status badges ---

// StatusStyle returns a styled string for a server status value.
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "running":
		return lipgloss.NewStyle().Foreground(Green).Bold(true)
	case "starting", "rebuilding", "migrating":
		return lipgloss.NewStyle().Foreground(Yellow).Bold(true)
	case "stopping", "deleting":
		return lipgloss.NewStyle().Foreground(Yellow)
	case "off", "stopped":
		return lipgloss.NewStyle().Foreground(Red)
	default:
		return lipgloss.NewStyle().Foreground(Gray)
	}
}

// StatusIndicator returns a small dot + status text with appropriate color.
func StatusIndicator(status string) string {
	style := StatusStyle(status)
	dot := style.Render("●")
	text := style.Render(status)
	return dot + " " + text
}

// --- Layout components ---

var (
	// Border is the default subtle border style.
	Border = lipgloss.RoundedBorder()

	// Card is a rounded-border panel for content sections.
	Card = lipgloss.NewStyle().
		Border(Border).
		BorderForeground(DimGray).
		Padding(1, 2)

	// CardActive is a card with an accent border for focused elements.
	CardActive = lipgloss.NewStyle().
			Border(Border).
			BorderForeground(Blue).
			Padding(1, 2)
)

// --- Key binding hint styles ---

var (
	// KeyStyle is used for key labels in the footer (e.g. "q").
	KeyStyle = lipgloss.NewStyle().
			Foreground(Blue).
			Bold(true)

	// KeyDescStyle is used for key descriptions in the footer (e.g. "quit").
	KeyDescStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// KeySepStyle is used for separators between key bindings.
	KeySepStyle = lipgloss.NewStyle().
			Foreground(DimGray)
)

// FormatKeyBinding formats a single key binding for the footer.
func FormatKeyBinding(key, desc string) string {
	return KeyStyle.Render(key) + " " + KeyDescStyle.Render(desc)
}

// --- Table styles ---

var (
	// TableHeader is the style for table header cells.
	TableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(Gray).
			Padding(0, 1)

	// TableCell is the style for table data cells.
	TableCell = lipgloss.NewStyle().
			Foreground(White).
			Padding(0, 1)

	// TableSelectedRow is the style for the currently selected table row.
	TableSelectedRow = lipgloss.NewStyle().
				Foreground(White).
				Background(DarkBlue).
				Bold(true).
				Padding(0, 1)
)

// --- Input field styles ---

var (
	// InputFocused is the style for focused input fields.
	InputFocused = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Blue).
			Padding(0, 1)

	// InputBlurred is the style for unfocused input fields.
	InputBlurred = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(DimGray).
			Padding(0, 1)
)

// --- Layout helpers ---

// AppFrame returns the full-window frame style with padding.
func AppFrame(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height)
}

// CenterText centers text horizontally within the given width.
func CenterText(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
}
