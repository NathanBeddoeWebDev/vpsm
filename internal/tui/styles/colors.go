// Package styles provides the centralized color palette and style definitions
// for the vpsm TUI. All visual constants live here so the rest of the TUI
// code can reference a single source of truth.
package styles

import "github.com/charmbracelet/lipgloss"

// --- Color palette (professional & minimal) ---

var (
	// Core text
	White   = lipgloss.AdaptiveColor{Light: "#121212", Dark: "#E6E6E6"}
	Gray    = lipgloss.AdaptiveColor{Light: "#3F3F3F", Dark: "#CFCFCF"}
	Muted   = lipgloss.AdaptiveColor{Light: "#4A4A4A", Dark: "#A0A0A0"}
	DimGray = lipgloss.AdaptiveColor{Light: "#B0B0B0", Dark: "#3D3D3D"}
	Dark    = lipgloss.AdaptiveColor{Light: "#1E1E1E", Dark: "#B8B8B8"}

	// Accent
	Blue     = lipgloss.AdaptiveColor{Light: "#005F9E", Dark: "#6EB6FF"}
	DimBlue  = lipgloss.AdaptiveColor{Light: "#2A5C8A", Dark: "#4C86B8"}
	DarkBlue = lipgloss.AdaptiveColor{Light: "#D7E6F5", Dark: "#1A2F40"}

	// Status
	Green  = lipgloss.AdaptiveColor{Light: "#1B6F3A", Dark: "#5FD787"}
	Yellow = lipgloss.AdaptiveColor{Light: "#7A5200", Dark: "#FFD787"}
	Red    = lipgloss.AdaptiveColor{Light: "#A02424", Dark: "#FF8787"}

	// Surfaces
	SurfaceBg   = lipgloss.AdaptiveColor{Light: "#F7F7F7", Dark: "#1A1A2E"}
	SurfaceDim  = lipgloss.AdaptiveColor{Light: "#EFEFEF", Dark: "#16213E"}
	SurfaceCard = lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#0F3460"}
)
