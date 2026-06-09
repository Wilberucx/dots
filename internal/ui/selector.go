package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Model ──────────────────────────────────────────────────────────────────

type moduleItem struct {
	name    string
	checked bool
	cursor  bool
}

type selectorModel struct {
	items        []moduleItem
	cursor       int
	selected     []string
	done         bool
	quitting     bool
	preselectAll bool
	width        int
	height       int
}

func (m selectorModel) Init() tea.Cmd {
	return nil
}

func (m selectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			m.done = true
			return m, tea.Quit

		case "enter":
			m.done = true
			m.selected = m.collectSelected()
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}

		case " ", "tab":
			m.items[m.cursor].checked = !m.items[m.cursor].checked

		case "a":
			// Toggle all
			allChecked := true
			for _, item := range m.items {
				if !item.checked {
					allChecked = false
					break
				}
			}
			for i := range m.items {
				m.items[i].checked = !allChecked
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, nil
}

func (m selectorModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder
	b.WriteString(QuestionStyle.Render("Select modules (↑↓/jk navigate · Space/Tab toggle · a toggle-all · Enter confirm):"))
	b.WriteString("\n\n")

	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = SelectedStyle.Render("▸ ")
		}

		checkbox := "[ ]"
		if item.checked {
			checkbox = CheckedStyle.Render("[✓]")
		}

		line := fmt.Sprintf("%s %s %s", cursor, checkbox, item.name)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("(↑↓/jk navigate · Space/Tab toggle · a toggle-all · Enter confirm · q/Ctrl+c cancel)"))

	return b.String()
}

func (m selectorModel) collectSelected() []string {
	var selected []string
	for _, item := range m.items {
		if item.checked {
			selected = append(selected, item.name)
		}
	}
	return selected
}

// ─── Styles ─────────────────────────────────────────────────────────────────

var (
	QuestionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39"))

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39"))

	CheckedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("76"))

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243"))
)

// ─── Public API ──────────────────────────────────────────────────────────────

// RunModuleSelector runs an interactive checkbox-style module selection TUI.
// Returns the list of selected module names, or nil if cancelled.
func RunModuleSelector(names []string, preselectAll bool) ([]string, error) {
	items := make([]moduleItem, len(names))
	for i, name := range names {
		items[i] = moduleItem{
			name:    name,
			checked: preselectAll,
		}
	}

	model := selectorModel{
		items:        items,
		cursor:       0,
		preselectAll: preselectAll,
	}

	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running selector: %v\n", err)
		return nil, err
	}

	finalModel, ok := result.(selectorModel)
	if !ok {
		return nil, fmt.Errorf("unexpected result type")
	}

	if finalModel.quitting {
		return nil, nil
	}

	return finalModel.selected, nil
}

// ─── Single-select picker model ──────────────────────────────────────────────

type pickerModel struct {
	title    string
	items    []string
	cursor   int
	selected string
	done     bool
	quitting bool
	width    int
	height   int
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			m.done = true
			return m, tea.Quit
		case "enter":
			m.done = true
			m.selected = m.items[m.cursor]
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m pickerModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder
	b.WriteString(QuestionStyle.Render(m.title))
	b.WriteString("\n\n")

	for i, item := range m.items {
		cursor := "  "
		if i == m.cursor {
			cursor = SelectedStyle.Render("▸ ")
		}
		line := fmt.Sprintf("%s%s", cursor, item)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("(↑↓/jk navigate · Enter confirm · q/Ctrl+c cancel)"))
	return b.String()
}

// RunModulePicker runs an interactive single-select module picker TUI.
// Returns the selected module name, or empty string if cancelled.
func RunModulePicker(names []string) string {
	if len(names) == 0 {
		return ""
	}

	model := pickerModel{
		title: "Select a module (↑↓/jk navigate · Enter confirm):",
		items: names,
	}

	return runPicker(model)
}

// RunVariantPicker runs an interactive single-select variant picker TUI.
// Returns the selected variant name, or empty string if cancelled.
func RunVariantPicker(moduleName string, variants []string) string {
	if len(variants) == 0 {
		return ""
	}

	model := pickerModel{
		title: fmt.Sprintf("Select a variant for %q (↑↓/jk navigate · Enter confirm):", moduleName),
		items: variants,
	}

	return runPicker(model)
}

// runPicker runs a pickerModel and returns the selected item.
func runPicker(model pickerModel) string {
	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return ""
	}

	finalModel, ok := result.(pickerModel)
	if !ok || finalModel.quitting {
		return ""
	}

	return finalModel.selected
}

// RunPrompt asks the user a text question with a default value.
func RunPrompt(message, defaultValue string) string {
	fmt.Printf("%s %s ", QuestionStyle.Render("?"), message)
	fmt.Printf("%s ", HelpStyle.Render(fmt.Sprintf("[%s]", defaultValue)))

	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(response)

	if response == "" {
		return defaultValue
	}
	return response
}

// RunConfirm runs a simple yes/no confirmation prompt.
func RunConfirm(message string, defaultValue bool) bool {
	fmt.Printf("%s %s ", QuestionStyle.Render("?"), message)

	defaultStr := "y/N"
	if defaultValue {
		defaultStr = "Y/n"
	}
	fmt.Printf("%s ", HelpStyle.Render(fmt.Sprintf("[%s]", defaultStr)))

	var response string
	fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "" {
		return defaultValue
	}
	return response == "y" || response == "yes"
}
