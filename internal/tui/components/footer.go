package components

import (
	"strings"

	"nathanbeddoewebdev/vpsm/internal/tui/styles"

	"github.com/charmbracelet/lipgloss"
)

// KeyBinding represents a single key binding for the footer.
type KeyBinding struct {
	Key  string
	Desc string
}

// Footer renders the key binding help bar at the bottom of the screen.
func Footer(width int, bindings []KeyBinding) string {
	if width < 10 || len(bindings) == 0 {
		return ""
	}

	sep := styles.KeySepStyle.Render("  ")
	parts := make([]string, len(bindings))
	for i, b := range bindings {
		parts[i] = styles.FormatKeyBinding(b.Key, b.Desc)
	}

	content := strings.Join(parts, sep)

	bar := lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		BorderStyle(lipgloss.Border{Top: "─"}).
		BorderTop(true).
		BorderForeground(styles.DimGray).
		Render(content)

	return bar
}
