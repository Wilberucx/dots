package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// HeaderStyle is used for section headers.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	// SuccessStyle is used for success messages.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("76"))

	// ErrorStyle is used for error messages.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	// WarningStyle is used for warnings.
	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	// InfoStyle is used for info messages.
	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	// DimStyle is used for dim/muted text.
	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))

	// DividerStyle is used for horizontal dividers.
	DividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("236"))

	// BoldStyle is used for bold text.
	BoldStyle = lipgloss.NewStyle().Bold(true)
)

// PrintHeader prints a section header.
func PrintHeader(msg string) {
	fmt.Println(HeaderStyle.Render("─── " + msg + " ───"))
}

// PrintSuccess prints a success message.
func PrintSuccess(msg string) {
	fmt.Println(SuccessStyle.Render("✔ " + msg))
}

// PrintError prints an error message.
func PrintError(msg string) {
	fmt.Println(ErrorStyle.Render("✘ " + msg))
}

// PrintWarning prints a warning message.
func PrintWarning(msg string) {
	fmt.Println(WarningStyle.Render("⚠ " + msg))
}

// PrintInfo prints an info message.
func PrintInfo(msg string) {
	fmt.Println("ℹ  " + msg)
}

// PrintDivider prints a horizontal divider.
func PrintDivider(width int) {
	if width <= 0 {
		width = 80
	}
	fmt.Println(DividerStyle.Render(string(repeatRune('━', width))))
}

// PrintTreeItem prints an indented tree item with an icon.
func PrintTreeItem(icon, label, detail string) {
	msg := icon + " " + label
	if detail != "" {
		msg += " " + DimStyle.Render(detail)
	}
	fmt.Println(msg)
}

func repeatRune(r rune, count int) []rune {
	s := make([]rune, count)
	for i := range s {
		s[i] = r
	}
	return s
}
