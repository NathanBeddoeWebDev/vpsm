// Package styles provides the centralized color palette and style definitions
// for the vpsm TUI. All visual constants live here so the rest of the TUI
// code can reference a single source of truth.
package styles

import "github.com/charmbracelet/lipgloss"

// --- Color palette (professional & minimal) ---

var (
	// Core text
	White   = lipgloss.Color("#E2E2E2")
	Gray    = lipgloss.Color("#888888")
	Muted   = lipgloss.Color("#555555")
	DimGray = lipgloss.Color("#444444")
	Dark    = lipgloss.Color("#333333")

	// Accent
	Blue     = lipgloss.Color("#5FAFFF")
	DimBlue  = lipgloss.Color("#3A6FA0")
	DarkBlue = lipgloss.Color("#1A2F40")

	// Status
	Green  = lipgloss.Color("#5FD787")
	Yellow = lipgloss.Color("#FFD787")
	Red    = lipgloss.Color("#FF8787")

	// Surfaces
	SurfaceBg   = lipgloss.Color("#1A1A2E")
	SurfaceDim  = lipgloss.Color("#16213E")
	SurfaceCard = lipgloss.Color("#0F3460")
)
