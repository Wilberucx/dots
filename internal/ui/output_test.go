package ui

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

// visibleRunes returns the visible runes (after stripping ANSI codes and trimming).
func visibleRunes(s string) []rune {
	return []rune(strings.TrimSpace(stripANSI(s)))
}

// captureStdout runs fn and returns everything written to stdout.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

// ─── Output function tests ──────────────────────────────────────────────────.

func TestPrintHeader(t *testing.T) {
	out := captureStdout(func() {
		PrintHeader("Test Header")
	})
	assert.Contains(t, out, "Test Header")
	assert.Contains(t, out, "───")
}

func TestPrintSuccess(t *testing.T) {
	out := captureStdout(func() {
		PrintSuccess("All good")
	})
	assert.Contains(t, out, "✔")
	assert.Contains(t, out, "All good")
}

func TestPrintError(t *testing.T) {
	out := captureStdout(func() {
		PrintError("Something broke")
	})
	assert.Contains(t, out, "✘")
	assert.Contains(t, out, "Something broke")
}

func TestPrintWarning(t *testing.T) {
	out := captureStdout(func() {
		PrintWarning("Be careful")
	})
	assert.Contains(t, out, "⚠")
	assert.Contains(t, out, "Be careful")
}

func TestPrintInfo(t *testing.T) {
	out := captureStdout(func() {
		PrintInfo("FYI")
	})
	assert.Contains(t, out, "ℹ")
	assert.Contains(t, out, "FYI")
}

func TestPrintDivider(t *testing.T) {
	t.Run("custom width", func(t *testing.T) {
		out := captureStdout(func() {
			PrintDivider(10)
		})
		// Strip ANSI codes before counting visible chars
		visible := visibleRunes(out)
		assert.Len(t, visible, 10)
	})

	t.Run("zero width defaults to 80", func(t *testing.T) {
		out := captureStdout(func() {
			PrintDivider(0)
		})
		visible := visibleRunes(out)
		assert.Len(t, visible, 80)
	})

	t.Run("negative width defaults to 80", func(t *testing.T) {
		out := captureStdout(func() {
			PrintDivider(-1)
		})
		visible := visibleRunes(out)
		assert.Len(t, visible, 80)
	})
}

func TestPrintTreeItem(t *testing.T) {
	t.Run("with detail", func(t *testing.T) {
		out := captureStdout(func() {
			PrintTreeItem("📦", "Nvim", "config")
		})
		assert.Contains(t, out, "📦")
		assert.Contains(t, out, "Nvim")
		assert.Contains(t, out, "config")
	})

	t.Run("without detail", func(t *testing.T) {
		out := captureStdout(func() {
			PrintTreeItem("✔", "Zsh", "")
		})
		assert.Contains(t, out, "✔")
		assert.Contains(t, out, "Zsh")
	})
}

func TestRepeatRune(t *testing.T) {
	t.Run("positive count", func(t *testing.T) {
		result := repeatRune('━', 5)
		assert.Equal(t, []rune{'━', '━', '━', '━', '━'}, result)
	})

	t.Run("zero count", func(t *testing.T) {
		result := repeatRune('━', 0)
		assert.Empty(t, result)
	})

	t.Run("single count", func(t *testing.T) {
		result := repeatRune('a', 1)
		assert.Equal(t, []rune{'a'}, result)
	})

	t.Run("unicode rune", func(t *testing.T) {
		result := repeatRune('📦', 3)
		assert.Len(t, result, 3)
		assert.Equal(t, '📦', result[0])
	})
}

// ─── Theme tests ────────────────────────────────────────────────────────────.

func TestDefaultPromptStyle(t *testing.T) {
	assert.NotNil(t, DefaultPromptStyle.Question)
	assert.NotNil(t, DefaultPromptStyle.Answer)
	assert.NotNil(t, DefaultPromptStyle.Help)
	assert.NotNil(t, DefaultPromptStyle.Error)

	// Verify style renders — lipgloss styles may or may not produce ANSI
	// depending on the terminal environment, so just check the text.
	assert.Contains(t, DefaultPromptStyle.Question.Render("?"), "?")
}

func TestStateStyle_AllStates(t *testing.T) {
	expectedStates := []string{"linked", "unlinked", "conflict", "missing", "unsafe"}
	for _, state := range expectedStates {
		s, ok := StateStyle[state]
		assert.True(t, ok, "StateStyle should have entry for %q", state)
		assert.NotEmpty(t, s.Label, "Label for %q should not be empty", state)
		assert.NotEmpty(t, s.Color, "Color for %q should not be empty", state)
	}
}

func TestStateStyle_UnknownState(t *testing.T) {
	_, ok := StateStyle["nonexistent"]
	assert.False(t, ok, "StateStyle should not have entry for unknown state")
}

func TestIcons_NotEmpty(t *testing.T) {
	icons := map[string]string{
		"IconLinked":    IconLinked,
		"IconConflict":  IconConflict,
		"IconError":     IconError,
		"IconPending":   IconPending,
		"IconVariant":   IconVariant,
		"IconActiveVar": IconActiveVar,
		"IconSwap":      IconSwap,
		"IconModule":    IconModule,
	}
	for name, icon := range icons {
		assert.NotEmpty(t, icon, "%s should not be empty", name)
	}
}

// ─── Selector model tests ───────────────────────────────────────────────────.
// These use actual tea.KeyMsg and tea.WindowSizeMsg so they exercise the real
// type switches in the Update methods.

func TestSelectorModel_Init(t *testing.T) {
	m := selectorModel{items: []moduleItem{{name: "Nvim"}, {name: "Zsh"}}}
	cmd := m.Init()
	assert.Nil(t, cmd, "Init should return nil")
}

func TestSelectorModel_CursorNavigation(t *testing.T) {
	m := selectorModel{
		items: []moduleItem{
			{name: "Nvim"},
			{name: "Zsh"},
			{name: "Git"},
		},
	}

	assert.Equal(t, 0, m.cursor)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(selectorModel)
	assert.Equal(t, 1, m.cursor)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(selectorModel)
	assert.Equal(t, 2, m.cursor)

	// Cannot go past last item
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(selectorModel)
	assert.Equal(t, 2, m.cursor)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(selectorModel)
	assert.Equal(t, 1, m.cursor)

	// Can't go above first
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(selectorModel)
	assert.Equal(t, 0, m.cursor)
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(selectorModel)
	assert.Equal(t, 0, m.cursor)
}

func TestSelectorModel_VimKeys(t *testing.T) {
	m := selectorModel{
		items: []moduleItem{
			{name: "Nvim"},
			{name: "Zsh"},
		},
	}

	// j = down (rune-based key)
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = result.(selectorModel)
	assert.Equal(t, 1, m.cursor)

	// k = up
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = result.(selectorModel)
	assert.Equal(t, 0, m.cursor)
}

func TestSelectorModel_ToggleItem(t *testing.T) {
	m := selectorModel{
		items: []moduleItem{
			{name: "Nvim"},
			{name: "Zsh"},
		},
	}

	assert.False(t, m.items[0].checked)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = result.(selectorModel)
	assert.True(t, m.items[0].checked)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = result.(selectorModel)
	assert.False(t, m.items[0].checked)
}

func TestSelectorModel_ToggleTab(t *testing.T) {
	m := selectorModel{
		items: []moduleItem{
			{name: "Nvim"},
			{name: "Zsh"},
		},
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = result.(selectorModel)
	assert.True(t, m.items[0].checked)
}

func TestSelectorModel_ToggleAll(t *testing.T) {
	m := selectorModel{
		items: []moduleItem{
			{name: "Nvim", checked: false},
			{name: "Zsh", checked: false},
			{name: "Git", checked: false},
		},
	}

	// Toggle all on (rune-based 'a')
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = result.(selectorModel)
	for _, item := range m.items {
		assert.True(t, item.checked, "all items should be checked after 'a'")
	}

	// Toggle all off
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = result.(selectorModel)
	for _, item := range m.items {
		assert.False(t, item.checked, "all items should be unchecked after second 'a'")
	}
}

func TestSelectorModel_EnterCollectsSelected(t *testing.T) {
	m := selectorModel{
		items: []moduleItem{
			{name: "Nvim", checked: true},
			{name: "Zsh", checked: false},
			{name: "Git", checked: true},
		},
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := result.(selectorModel)

	assert.True(t, final.done)
	assert.Equal(t, []string{"Nvim", "Git"}, final.selected)
}

func TestSelectorModel_Quit(t *testing.T) {
	t.Run("ctrl+c", func(t *testing.T) {
		m := selectorModel{items: []moduleItem{{name: "Nvim"}}}
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		final := result.(selectorModel)
		assert.True(t, final.done)
		assert.True(t, final.quitting)
	})

	t.Run("q key", func(t *testing.T) {
		m := selectorModel{items: []moduleItem{{name: "Nvim"}}}
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		final := result.(selectorModel)
		assert.True(t, final.done)
		assert.True(t, final.quitting)
	})
}

func TestSelectorModel_CollectSelected(t *testing.T) {
	m := selectorModel{
		items: []moduleItem{
			{name: "Nvim", checked: true},
			{name: "Zsh", checked: false},
			{name: "Git", checked: true},
		},
	}
	selected := m.collectSelected()
	assert.Equal(t, []string{"Nvim", "Git"}, selected)
}

func TestSelectorModel_View(t *testing.T) {
	m := selectorModel{
		items: []moduleItem{
			{name: "Nvim", checked: true},
			{name: "Zsh"},
		},
		cursor: 0,
	}

	view := m.View()
	assert.Contains(t, view, "Nvim")
	assert.Contains(t, view, "Zsh")
	assert.Contains(t, view, "[✓]")
	assert.Contains(t, view, "[ ]")
	assert.Contains(t, view, "Select modules")
}

func TestSelectorModel_ViewAfterDone(t *testing.T) {
	m := selectorModel{done: true}
	assert.Empty(t, m.View(), "View should return empty when done")
}

func TestSelectorModel_WindowSize(t *testing.T) {
	m := selectorModel{width: 0, height: 0}
	result, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	final := result.(selectorModel)
	assert.Equal(t, 100, final.width)
	assert.Equal(t, 30, final.height)
}

// ─── Picker model tests ─────────────────────────────────────────────────────.

func TestPickerModel_Init(t *testing.T) {
	m := pickerModel{items: []string{"Nvim", "Zsh"}}
	cmd := m.Init()
	assert.Nil(t, cmd)
}

func TestPickerModel_Navigation(t *testing.T) {
	m := pickerModel{
		items: []string{"Nvim", "Zsh", "Git"},
	}

	assert.Equal(t, 0, m.cursor)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(pickerModel)
	assert.Equal(t, 1, m.cursor)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(pickerModel)
	assert.Equal(t, 2, m.cursor)

	// Can't go past end
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = result.(pickerModel)
	assert.Equal(t, 2, m.cursor)

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = result.(pickerModel)
	assert.Equal(t, 1, m.cursor)
}

func TestPickerModel_Select(t *testing.T) {
	m := pickerModel{
		items:  []string{"Nvim", "Zsh"},
		cursor: 1,
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	final := result.(pickerModel)

	assert.True(t, final.done)
	assert.Equal(t, "Zsh", final.selected)
}

func TestPickerModel_Quit(t *testing.T) {
	m := pickerModel{items: []string{"Nvim"}}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	final := result.(pickerModel)
	assert.True(t, final.quitting)
	assert.True(t, final.done)
}

func TestPickerModel_View(t *testing.T) {
	m := pickerModel{
		title:  "Pick a module",
		items:  []string{"Nvim", "Zsh"},
		cursor: 0,
	}

	view := m.View()
	assert.Contains(t, view, "Pick a module")
	assert.Contains(t, view, "Nvim")
	assert.Contains(t, view, "Zsh")
}

func TestPickerModel_ViewAfterDone(t *testing.T) {
	m := pickerModel{done: true}
	assert.Empty(t, m.View())
}

func TestPickerModel_WindowSize(t *testing.T) {
	m := pickerModel{width: 0, height: 0}
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	final := result.(pickerModel)
	assert.Equal(t, 120, final.width)
	assert.Equal(t, 40, final.height)
}

func TestRunModulePicker_Empty(t *testing.T) {
	result := RunModulePicker(nil)
	assert.Empty(t, result)

	result = RunModulePicker([]string{})
	assert.Empty(t, result)
}

func TestRunVariantPicker_Empty(t *testing.T) {
	result := RunVariantPicker("Nvim", nil)
	assert.Empty(t, result)

	result = RunVariantPicker("Nvim", []string{})
	assert.Empty(t, result)
}

// ─── Prompt/Confirm tests ───────────────────────────────────────────────────.

func TestRunConfirm_DefaultTrue(t *testing.T) {
	// With no stdin input, returns default (true)
	result := RunConfirm("Continue?", true)
	assert.True(t, result)
}

func TestRunConfirm_DefaultFalse(t *testing.T) {
	result := RunConfirm("Continue?", false)
	assert.False(t, result)
}

// ─── Style rendering tests ──────────────────────────────────────────────────.

func TestStyles_Render(t *testing.T) {
	t.Run("QuestionStyle", func(t *testing.T) {
		rendered := QuestionStyle.Render("question")
		assert.Contains(t, rendered, "question")
	})

	t.Run("SelectedStyle", func(t *testing.T) {
		rendered := SelectedStyle.Render("selected")
		assert.Contains(t, rendered, "selected")
	})

	t.Run("CheckedStyle", func(t *testing.T) {
		rendered := CheckedStyle.Render("checked")
		assert.Contains(t, rendered, "checked")
	})

	t.Run("HelpStyle", func(t *testing.T) {
		rendered := HelpStyle.Render("help")
		assert.Contains(t, rendered, "help")
	})
}

func TestPrintTreeItem_AllIcons(t *testing.T) {
	tests := []struct {
		name   string
		icon   string
		label  string
		detail string
	}{
		{"linked", IconLinked, "Nvim", "linked"},
		{"conflict", IconConflict, "Nvim", "conflict"},
		{"error", IconError, "Nvim", "error"},
		{"pending", IconPending, "Nvim", "pending"},
		{"module", IconModule, "Zsh", "config"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(func() {
				PrintTreeItem(tt.icon, tt.label, tt.detail)
			})
			assert.Contains(t, out, tt.icon)
			assert.Contains(t, out, tt.label)
		})
	}
}
