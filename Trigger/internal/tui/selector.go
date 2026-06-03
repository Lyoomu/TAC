package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type selectorModel struct {
	active       bool
	title        string
	items        []string
	selected     []bool
	cursor       int
	singleSelect bool
	width        int
	height       int
}

func newSelectorModel() *selectorModel {
	return &selectorModel{}
}

func (s *selectorModel) IsActive() bool {
	return s.active
}

func (s *selectorModel) start(title string, items []string) {
	s.active = true
	s.title = title
	s.items = items
	s.selected = make([]bool, len(items))
	s.cursor = 0
	s.singleSelect = false
}

func (s *selectorModel) startSingle(title string, items []string) {
	s.active = true
	s.title = title
	s.items = items
	s.selected = nil
	s.cursor = 0
	s.singleSelect = true
}

func (s *selectorModel) startWithPreselected(title string, items []string, preselected []int) {
	s.start(title, items)
	for _, idx := range preselected {
		if idx >= 0 && idx < len(s.selected) {
			s.selected[idx] = true
		}
	}
}

func (s *selectorModel) cancel() {
	s.active = false
	s.items = nil
	s.selected = nil
}

func (s *selectorModel) Update(msg tea.KeyMsg) (done bool, cancelled bool) {
	if !s.active {
		return false, false
	}
	switch msg.String() {
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.items)-1 {
			s.cursor++
		}
	case "x", "X", " ":
		if !s.singleSelect && s.cursor >= 0 && s.cursor < len(s.selected) {
			s.selected[s.cursor] = !s.selected[s.cursor]
		}
	case "a", "A":
		if !s.singleSelect {
			allSelected := true
			for _, v := range s.selected {
				if !v {
					allSelected = false
					break
				}
			}
			for i := range s.selected {
				s.selected[i] = !allSelected
			}
		}
	case "enter":
		s.active = false
		return true, false
	case "esc":
		s.cancel()
		return false, true
	}
	return false, false
}

func (s *selectorModel) SelectedIndices() []int {
	if s.singleSelect {
		if s.cursor >= 0 && s.cursor < len(s.items) {
			return []int{s.cursor}
		}
		return nil
	}
	var result []int
	for i, v := range s.selected {
		if v {
			result = append(result, i)
		}
	}
	return result
}

func (s *selectorModel) View() string {
	if !s.active {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("  %s", s.title)))
	b.WriteString("\n\n")

	if len(s.items) == 0 {
		b.WriteString(hintStyle.Render("  No items available."))
		b.WriteString("\n\n")
		b.WriteString(hintStyle.Render("  [Esc] Back"))
		return b.String()
	}

	maxVisible := s.height - 8
	if maxVisible < 5 {
		maxVisible = 5
	}
	start := 0
	if s.cursor >= maxVisible {
		start = s.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(s.items) {
		end = len(s.items)
	}

	for i := start; i < end; i++ {
		var line string
		if s.singleSelect {
			line = fmt.Sprintf("  %s", s.items[i])
		} else {
			checkbox := "[ ]"
			if s.selected[i] {
				checkbox = "[x]"
			}
			line = fmt.Sprintf("  %s %s", checkbox, s.items[i])
		}
		if i == s.cursor {
			b.WriteString(tableSelectedStyle.Render(line))
		} else {
			b.WriteString(tableRowStyle.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if s.singleSelect {
		b.WriteString(hintStyle.Render("  [↑/↓] Navigate  [Enter] Confirm  [Esc] Cancel"))
	} else {
		selectedCount := 0
		for _, v := range s.selected {
			if v {
				selectedCount++
			}
		}
		b.WriteString(hintStyle.Render(fmt.Sprintf("  Selected: %d/%d", selectedCount, len(s.items))))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  [↑/↓] Navigate  [x] Toggle  [a] Toggle All  [Enter] Confirm  [Esc] Cancel"))
	}

	return b.String()
}
