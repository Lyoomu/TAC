package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type conflictAction int

const (
	conflictSkip      conflictAction = iota
	conflictOverwrite
	conflictRename
)

type conflictEntry struct {
	Name         string
	ResourceType string
	ExistingInfo string
	Action       conflictAction
	NewName      string
}

type conflictResolverModel struct {
	active        bool
	title         string
	entries       []*conflictEntry
	cursor        int
	editing       int
	editInput     textinput.Model
	existingNames map[string]bool
	errorMsg      string
}

func newConflictResolverModel() *conflictResolverModel {
	ti := textinput.New()
	ti.CharLimit = 128
	ti.Placeholder = "enter new name..."
	return &conflictResolverModel{
		editInput: ti,
	}
}

func (r *conflictResolverModel) IsActive() bool {
	return r.active
}

func (r *conflictResolverModel) start(title string, entries []*conflictEntry, existingNames map[string]bool) {
	r.active = true
	r.title = title
	r.entries = entries
	r.cursor = 0
	r.editing = -1
	r.existingNames = existingNames
	r.errorMsg = ""
	for _, e := range entries {
		e.Action = conflictSkip
		e.NewName = ""
	}
}

func (r *conflictResolverModel) cancel() {
	r.active = false
	r.entries = nil
	r.editing = -1
}

func (r *conflictResolverModel) Update(msg tea.Msg) (done bool, cancelled bool) {
	if !r.active {
		return false, false
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		if r.editing >= 0 {
			var cmd tea.Cmd
			r.editInput, cmd = r.editInput.Update(msg)
			_ = cmd
		}
		return false, false
	}

	key := keyMsg.String()

	if r.editing >= 0 {
		switch key {
		case "enter":
			newName := strings.TrimSpace(r.editInput.Value())
			if newName == "" {
				r.errorMsg = "Name cannot be empty"
				return false, false
			}
			if r.existingNames[newName] {
				r.errorMsg = fmt.Sprintf("Name '%s' already exists", newName)
				return false, false
			}
			for i, e := range r.entries {
				if i != r.editing && e.Action == conflictRename && e.NewName == newName {
					r.errorMsg = fmt.Sprintf("Another item is also renaming to '%s'", newName)
					return false, false
				}
			}
			r.entries[r.editing].NewName = newName
			r.editing = -1
			r.editInput.Blur()
			r.errorMsg = ""
			return false, false
		case "esc":
			r.editing = -1
			r.editInput.Blur()
			r.errorMsg = ""
			return false, false
		default:
			var cmd tea.Cmd
			r.editInput, cmd = r.editInput.Update(msg)
			_ = cmd
			return false, false
		}
	}

	switch key {
	case "up", "k":
		if r.cursor > 0 {
			r.cursor--
		}
	case "down", "j":
		if r.cursor < len(r.entries)-1 {
			r.cursor++
		}
	case "s", "S":
		if r.cursor >= 0 && r.cursor < len(r.entries) {
			r.entries[r.cursor].Action = conflictSkip
			r.entries[r.cursor].NewName = ""
		}
	case "o", "O":
		if r.cursor >= 0 && r.cursor < len(r.entries) {
			r.entries[r.cursor].Action = conflictOverwrite
			r.entries[r.cursor].NewName = ""
		}
	case "r", "R":
		if r.cursor >= 0 && r.cursor < len(r.entries) {
			r.entries[r.cursor].Action = conflictRename
			r.editing = r.cursor
			r.editInput.SetValue(r.entries[r.cursor].Name)
			r.editInput.Focus()
			r.editInput.CursorEnd()
		}
	case "enter":
		r.active = false
		r.editing = -1
		return true, false
	case "esc":
		r.cancel()
		return false, true
	}

	return false, false
}

func (r *conflictResolverModel) View() string {
	if !r.active {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  " + r.title))
	b.WriteString("\n\n")

	b.WriteString(hintStyle.Render("  s=Skip  o=Overwrite  r=Rename  [Enter] Confirm  [Esc] Cancel"))
	b.WriteString("\n\n")

	for i, e := range r.entries {
		var actionLabel string
		switch e.Action {
		case conflictSkip:
			actionLabel = "[SKIP      ]"
		case conflictOverwrite:
			actionLabel = "[OVERWRITE ]"
		case conflictRename:
			actionLabel = "[RENAME    ]"
		}

		line := fmt.Sprintf("  %s %s", actionLabel, e.Name)
		if e.ExistingInfo != "" {
			line += fmt.Sprintf("  (%s)", e.ExistingInfo)
		}
		if e.Action == conflictRename && e.NewName != "" {
			line += fmt.Sprintf("  →  %s", e.NewName)
		}

		if i == r.cursor {
			b.WriteString(tableSelectedStyle.Render(line))
			b.WriteString("\n")
		} else {
			b.WriteString(tableRowStyle.Render(line))
			b.WriteString("\n")
		}

		if r.editing == i {
			b.WriteString(valueStyle.Render("    New name: "))
			b.WriteString(r.editInput.View())
			b.WriteString("\n")
			if r.errorMsg != "" {
				b.WriteString(errorStyle.Render("    " + r.errorMsg))
				b.WriteString("\n")
			}
		}
	}

	var skip, overwrite, rename int
	for _, e := range r.entries {
		switch e.Action {
		case conflictSkip:
			skip++
		case conflictOverwrite:
			overwrite++
		case conflictRename:
			rename++
		}
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render(fmt.Sprintf("  %d conflicts: %d skip · %d overwrite · %d rename", len(r.entries), skip, overwrite, rename)))

	return b.String()
}

func (r *conflictResolverModel) GetResolved() map[string]*conflictEntry {
	result := make(map[string]*conflictEntry)
	for _, e := range r.entries {
		result[e.Name] = e
	}
	return result
}
