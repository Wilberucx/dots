package ui

import "github.com/charmbracelet/lipgloss"

// PromptStyle defines the color scheme for interactive prompts.
type PromptStyle struct {
	Question lipgloss.Style
	Answer   lipgloss.Style
	Help     lipgloss.Style
	Error    lipgloss.Style
}

// DefaultPromptStyle is the default interactive prompt style.
var DefaultPromptStyle = PromptStyle{
	Question: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")),
	Answer:   lipgloss.NewStyle().Foreground(lipgloss.Color("76")),
	Help:     lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
	Error:    lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
}

// StateStyle maps state names to (label, color) pairs.
var StateStyle = map[string]struct {
	Label string
	Color lipgloss.Color
}{
	"linked":   {"linked", lipgloss.Color("76")},
	"pending":  {"unlinked", lipgloss.Color("243")},
	"conflict": {"broken", lipgloss.Color("196")},
	"missing":  {"missing", lipgloss.Color("214")},
	"unsafe":   {"unsafe", lipgloss.Color("196")},
}

// Icon constants for tree display.
const (
	IconLinked    = "✔"
	IconConflict  = "⚠"
	IconError     = "✘"
	IconPending   = "ℹ"
	IconVariant   = "○"
	IconActiveVar = "●"
	IconSwap      = "↔"
	IconModule    = "📦"
)
