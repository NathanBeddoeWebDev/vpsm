package components

import (
	"nathanbeddoewebdev/vpsm/internal/tui/styles"

	"github.com/charmbracelet/lipgloss"
)

// StatusBar renders a status message line between the content and footer.
func StatusBar(width int, message string, isError bool) string {
	if message == "" {
		return ""
	}

	style := styles.MutedText
	if isError {
		style = styles.ErrorText
	}

	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		Render(style.Render(message))
}
