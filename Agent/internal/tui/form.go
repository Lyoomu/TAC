package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type formField struct {
	label            string
	placeholder      string
	value            string
	required         bool
	options          []string // if non-empty, field becomes a choice selector
	multiSelect      bool     // if true, field is a multi-select checklist
	multiSelectItems []string // available items for multi-select
}

type formMode int

const (
	formModeCreate formMode = iota
	formModeEdit
)

type formModel struct {
	active bool
	title  string
	mode   formMode
	fields []formField
	step   int // current field index (create mode) or selected field (edit mode)
	input  textinput.Model

	choiceCursor int

	msAllItems      []string // all available items
	msSelectedOrder []int    // indices of selected items in selection order
	msCursor        int      // cursor in displayed (filtered) list
	msSearching     bool     // search mode active
	msSearchInput   textinput.Model
	msFilteredIdx   []int // indices into msAllItems matching filter

	editSelecting bool
	editCursor    int

	width  int
	height int
}

func newFormModel() *formModel {
	ti := textinput.New()
	ti.CharLimit = 512
	si := textinput.New()
	si.CharLimit = 128
	si.Placeholder = "type to filter..."
	return &formModel{
		input:         ti,
		msSearchInput: si,
	}
}

func (f *formModel) startCreate(title string, fields []formField) {
	f.active = true
	f.title = title
	f.mode = formModeCreate
	f.fields = fields
	f.step = 0
	f.editSelecting = false
	f.editCursor = 0
	f.setupInputForStep(0)
}

func (f *formModel) startEdit(title string, fields []formField) {
	f.active = true
	f.title = title
	f.mode = formModeEdit
	f.fields = fields
	f.step = 0
	f.editSelecting = true
	f.editCursor = 0
	f.input.Blur()
}

func (f *formModel) isChoiceStep() bool {
	if f.step >= 0 && f.step < len(f.fields) {
		return len(f.fields[f.step].options) > 0
	}
	return false
}

func (f *formModel) isMultiSelectStep() bool {
	if f.step >= 0 && f.step < len(f.fields) {
		return f.fields[f.step].multiSelect
	}
	return false
}

func (f *formModel) toggleMsSelection(idx int) {
	for i, selIdx := range f.msSelectedOrder {
		if selIdx == idx {
			f.msSelectedOrder = append(f.msSelectedOrder[:i], f.msSelectedOrder[i+1:]...)
			return
		}
	}
	f.msSelectedOrder = append(f.msSelectedOrder, idx)
}

func (f *formModel) msSelectionOrder(idx int) int {
	for i, selIdx := range f.msSelectedOrder {
		if selIdx == idx {
			return i + 1
		}
	}
	return 0
}

func (f *formModel) getMsValue() string {
	var selected []string
	for _, idx := range f.msSelectedOrder {
		if idx < len(f.msAllItems) {
			selected = append(selected, f.msAllItems[idx])
		}
	}
	return strings.Join(selected, ",")
}

func (f *formModel) updateMsFilter(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	f.msFilteredIdx = nil
	for i, item := range f.msAllItems {
		if query == "" || strings.Contains(strings.ToLower(item), query) {
			f.msFilteredIdx = append(f.msFilteredIdx, i)
		}
	}
	if f.msCursor >= len(f.msFilteredIdx) {
		f.msCursor = len(f.msFilteredIdx) - 1
		if f.msCursor < 0 {
			f.msCursor = 0
		}
	}
}

func (f *formModel) setupInputForStep(step int) {
	if step >= 0 && step < len(f.fields) {
		field := f.fields[step]
		if field.multiSelect {

			f.msAllItems = field.multiSelectItems
			f.msCursor = 0
			f.msSearching = false
			f.msSearchInput.SetValue("")
			f.msSearchInput.Blur()
			f.msFilteredIdx = make([]int, len(f.msAllItems))
			for i := range f.msAllItems {
				f.msFilteredIdx[i] = i
			}

			f.msSelectedOrder = nil
			if field.value != "" {
				for _, name := range strings.Split(field.value, ",") {
					name = strings.TrimSpace(name)
					if name == "" {
						continue
					}
					for idx, item := range f.msAllItems {
						if item == name {
							f.msSelectedOrder = append(f.msSelectedOrder, idx)
							break
						}
					}
				}
			}
			f.input.Blur()
		} else if len(field.options) > 0 {

			f.choiceCursor = 0
			for i, opt := range field.options {
				if opt == field.value {
					f.choiceCursor = i
					break
				}
			}
			f.input.Blur()
		} else {
			f.input.Placeholder = field.placeholder
			f.input.SetValue(field.value)
			f.input.Focus()
			f.input.CursorEnd()
		}
	}
}

func (f *formModel) cancel() {
	f.active = false
	f.input.Blur()
}

func (f *formModel) IsActive() bool {
	return f.active
}

func (f *formModel) Values() []string {
	vals := make([]string, len(f.fields))
	for i, field := range f.fields {
		vals[i] = field.value
	}
	return vals
}

func (f *formModel) currentValue() string {
	if f.isMultiSelectStep() {
		return f.getMsValue()
	}
	if f.isChoiceStep() {
		return f.fields[f.step].options[f.choiceCursor]
	}
	return strings.TrimSpace(f.input.Value())
}

func (f *formModel) Update(msg tea.Msg) (done bool, cancelled bool) {
	if !f.active {
		return false, false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:

		if f.isMultiSelectStep() && f.msSearching {
			switch msg.String() {
			case "esc":
				f.msSearching = false
				f.msSearchInput.Blur()
				f.updateMsFilter("")
				return false, false
			case "enter":
				f.msSearching = false
				f.msSearchInput.Blur()
				f.updateMsFilter("")
				return false, false
			case "up":
				if f.msCursor > 0 {
					f.msCursor--
				}
				return false, false
			case "down":
				if f.msCursor < len(f.msFilteredIdx)-1 {
					f.msCursor++
				}
				return false, false
			default:
				var cmd tea.Cmd
				f.msSearchInput, cmd = f.msSearchInput.Update(msg)
				_ = cmd
				f.updateMsFilter(f.msSearchInput.Value())
				return false, false
			}
		}

		switch msg.String() {
		case "esc":
			if f.mode == formModeCreate {
				if f.step > 0 {
					f.fields[f.step].value = f.currentValue()
					f.step--
					f.setupInputForStep(f.step)
					return false, false
				}
				f.cancel()
				return false, true
			}

			if !f.editSelecting {

				f.setupInputForStep(f.step)
				f.editSelecting = true
				f.input.Blur()
				return false, false
			}
			f.cancel()
			return false, true

		case "enter":
			if f.mode == formModeCreate {
				val := f.currentValue()
				if f.fields[f.step].required && val == "" {
					return false, false
				}
				f.fields[f.step].value = val

				if f.step < len(f.fields)-1 {
					f.step++
					f.setupInputForStep(f.step)
					return false, false
				}
				f.active = false
				f.input.Blur()
				return true, false
			}

			if f.editSelecting {
				f.editSelecting = false
				f.step = f.editCursor
				f.setupInputForStep(f.step)
				return false, false
			}
			val := f.currentValue()
			if f.fields[f.step].required && val == "" {
				return false, false
			}
			f.fields[f.step].value = val
			f.active = false
			f.input.Blur()
			return true, false

		case "x":
			if f.isMultiSelectStep() && !f.editSelecting {
				if f.msCursor < len(f.msFilteredIdx) {
					f.toggleMsSelection(f.msFilteredIdx[f.msCursor])
				}
				return false, false
			}

		case "s":
			if f.isMultiSelectStep() && !f.editSelecting {
				f.msSearching = true
				f.msSearchInput.SetValue("")
				f.msSearchInput.Focus()
				return false, false
			}

		case "up", "k":
			if f.editSelecting {
				if f.editCursor > 0 {
					f.editCursor--
				}
				return false, false
			}
			if f.isMultiSelectStep() {
				if f.msCursor > 0 {
					f.msCursor--
				}
				return false, false
			}
			if f.isChoiceStep() {
				if f.choiceCursor > 0 {
					f.choiceCursor--
				}
				return false, false
			}

		case "down", "j":
			if f.editSelecting {
				if f.editCursor < len(f.fields)-1 {
					f.editCursor++
				}
				return false, false
			}
			if f.isMultiSelectStep() {
				if f.msCursor < len(f.msFilteredIdx)-1 {
					f.msCursor++
				}
				return false, false
			}
			if f.isChoiceStep() {
				if f.choiceCursor < len(f.fields[f.step].options)-1 {
					f.choiceCursor++
				}
				return false, false
			}

		case "ctrl+d":
			if !f.editSelecting && !f.isChoiceStep() && !f.isMultiSelectStep() {
				f.input.SetValue("")
			}
			return false, false

		}

		if !f.editSelecting && !f.isChoiceStep() && !f.isMultiSelectStep() {
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			_ = cmd
		}
	}

	return false, false
}

func (f *formModel) View() string {
	if !f.active {
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  " + f.title))
	b.WriteString("\n\n")

	if f.mode == formModeCreate {
		return f.viewCreate(&b)
	}
	return f.viewEdit(&b)
}

func (f *formModel) viewCreate(b *strings.Builder) string {

	for i := 0; i < f.step; i++ {
		field := f.fields[i]
		displayVal := field.value
		if displayVal == "" {
			displayVal = "(empty)"
		}
		line := fmt.Sprintf("  %s: %s", field.label, displayVal)
		b.WriteString(successStyle.Render("  ✓"))
		b.WriteString(valueStyle.Render(line))
		b.WriteString("\n")
	}

	if f.step < len(f.fields) {
		field := f.fields[f.step]
		reqMark := ""
		if field.required {
			reqMark = " *"
		}
		b.WriteString("\n")
		b.WriteString(labelStyle.Render(fmt.Sprintf("  %s%s", field.label, reqMark)))
		b.WriteString("\n")
		if field.multiSelect {
			f.renderMultiSelect(b)
		} else if len(field.options) > 0 {

			for i, opt := range field.options {
				if i == f.choiceCursor {
					b.WriteString(tableSelectedStyle.Render("  ▸ " + opt))
					b.WriteString("\n")
				} else {
					b.WriteString(tableRowStyle.Render("    " + opt))
					b.WriteString("\n")
				}
			}
		} else {
			b.WriteString("  ")
			b.WriteString(f.input.View())
			b.WriteString("\n")
		}
	}

	if f.step < len(f.fields)-1 {
		b.WriteString("\n")
		for i := f.step + 1; i < len(f.fields); i++ {
			field := f.fields[i]
			reqMark := ""
			if field.required {
				reqMark = " *"
			}
			b.WriteString(hintStyle.Render(fmt.Sprintf("  ○ %s%s", field.label, reqMark)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	progress := fmt.Sprintf("  Step %d/%d", f.step+1, len(f.fields))
	b.WriteString(hintStyle.Render(progress))
	b.WriteString("\n")
	if f.isMultiSelectStep() {
		if f.msSearching {
			b.WriteString(hintStyle.Render("  [Esc/Enter] Exit search  [↑/↓] Navigate"))
		} else {
			b.WriteString(hintStyle.Render("  [x] Toggle  [s] Search  [↑/↓] Navigate  [Enter] Confirm  [Esc] Back/Cancel"))
		}
	} else if f.isChoiceStep() {
		b.WriteString(hintStyle.Render("  [↑/↓] Select  [Enter] Confirm  [Esc] Back/Cancel"))
	} else {
		b.WriteString(hintStyle.Render("  [Enter] Next  [Esc] Back/Cancel"))
	}

	return b.String()
}

func (f *formModel) renderMultiSelect(b *strings.Builder) {

	if len(f.msSelectedOrder) > 0 {
		var names []string
		for _, idx := range f.msSelectedOrder {
			if idx < len(f.msAllItems) {
				names = append(names, f.msAllItems[idx])
			}
		}
		b.WriteString(valueStyle.Render(fmt.Sprintf("  Selected (%d): %s", len(names), strings.Join(names, ", "))))
		b.WriteString("\n")
	} else {
		b.WriteString(hintStyle.Render("  No items selected"))
		b.WriteString("\n")
	}

	if f.msSearching {
		b.WriteString("\n  ")
		b.WriteString(f.msSearchInput.View())
		b.WriteString("\n")
	}

	b.WriteString("\n")

	if len(f.msFilteredIdx) == 0 {
		b.WriteString(hintStyle.Render("  (no matching items)"))
		b.WriteString("\n")
	} else {
		for i, filteredIdx := range f.msFilteredIdx {
			item := f.msAllItems[filteredIdx]
			order := f.msSelectionOrder(filteredIdx)
			checkbox := "[ ]"
			orderStr := ""
			if order > 0 {
				checkbox = "[✓]"
				orderStr = fmt.Sprintf("  (%d)", order)
			}
			line := fmt.Sprintf("  %s %s%s", checkbox, item, orderStr)
			if i == f.msCursor {
				b.WriteString(tableSelectedStyle.Render(line))
				b.WriteString("\n")
			} else {
				b.WriteString(tableRowStyle.Render(line))
				b.WriteString("\n")
			}
		}
	}
}

func (f *formModel) viewEdit(b *strings.Builder) string {
	if f.editSelecting {

		b.WriteString(subtitleStyle.Render("  Select field to edit:"))
		b.WriteString("\n\n")
		for i, field := range f.fields {
			displayVal := field.value
			if displayVal == "" {
				displayVal = "(empty)"
			}
			line := fmt.Sprintf("  %-16s %s", field.label+":", displayVal)
			if i == f.editCursor {
				b.WriteString(tableSelectedStyle.Render(line))
				b.WriteString("\n")
			} else {
				b.WriteString(tableRowStyle.Render(line))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  [↑/↓] Navigate  [Enter] Select  [Esc] Cancel"))
	} else {

		field := f.fields[f.step]
		b.WriteString(labelStyle.Render(fmt.Sprintf("  Editing: %s", field.label)))
		b.WriteString("\n\n")
		if field.multiSelect {
			f.renderMultiSelect(b)
			b.WriteString("\n")
			if f.msSearching {
				b.WriteString(hintStyle.Render("  [Esc/Enter] Exit search  [↑/↓] Navigate"))
			} else {
				b.WriteString(hintStyle.Render("  [x] Toggle  [s] Search  [↑/↓] Navigate  [Enter] Confirm  [Esc] Back"))
			}
		} else if len(field.options) > 0 {

			for i, opt := range field.options {
				if i == f.choiceCursor {
					b.WriteString(tableSelectedStyle.Render("  ▸ " + opt))
					b.WriteString("\n")
				} else {
					b.WriteString(tableRowStyle.Render("    " + opt))
					b.WriteString("\n")
				}
			}
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("  [↑/↓] Select  [Enter] Confirm  [Esc] Back"))
		} else {
			inputStyle := lipgloss.NewStyle().Padding(0, 2)
			b.WriteString(inputStyle.Render(f.input.View()))
			b.WriteString("\n")
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("  [Ctrl+D] Clear  [Enter] Confirm  [Esc] Back"))
		}
	}

	return b.String()
}
