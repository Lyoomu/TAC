package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Lyoomu/TAC/Trigger/internal/daemon"
	pb "github.com/Lyoomu/TAC/proto"
)

type triggerItem struct {
	ID          string
	Name        string
	Type        string
	Description string
	Events      int32
	Running     bool
	Interval    string
	CronExpr    string
	WatchPath   string
}

type triggerSessionItem struct {
	ID         string
	ServerName string
	RoleName   string
	UpdatedAt  string
	Status     string // "RUNNING" or "SILENT"
}

type triggersViewModel struct {
	ctx               *AppContext
	width             int
	height            int
	items             []triggerItem
	cursor            int
	scrollOffset      int
	form              *formModel
	pendingCreateType string
	editingTrigger    *triggerItem
	events            []eventItem

	showingSessions     bool
	sessions            []triggerSessionItem
	sessionCursor       int
	sessionScrollOffset int
}

func newTriggersViewModel(ctx *AppContext) *triggersViewModel {
	return &triggersViewModel{ctx: ctx, form: newFormModel()}
}

func (v *triggersViewModel) loadEvents() []eventItem {
	if !daemon.IsDaemonRunning() {
		return nil
	}
	client, err := daemon.NewClient()
	if err != nil {
		return nil
	}
	resp, err := client.ListEvents()
	if err != nil {
		return nil
	}
	items := make([]eventItem, 0, len(resp.Events))
	for _, e := range resp.Events {
		items = append(items, eventItem{
			ID:   e.Id,
			Name: e.Name,
		})
	}
	return items
}

func (v *triggersViewModel) loadSessions(triggerID string) []triggerSessionItem {
	if !daemon.IsDaemonRunning() {
		return nil
	}
	client, err := daemon.NewClient()
	if err != nil {
		return nil
	}

	var triggerName string
	if tResp, err := client.ListTriggers(); err == nil {
		for _, t := range tResp.Triggers {
			if t.Id == triggerID {
				triggerName = t.Name
				break
			}
		}
	}

	// 1. Get all events of the trigger
	eventsResp, err := client.GetTriggerEvents(triggerID)
	if err != nil {
		return nil
	}

	eventRoleKeys := make(map[string]bool)
	for _, ev := range eventsResp.Events {
		eventRoleKeys[ev.RoleKey] = true
	}

	// 2. Load all local sessions
	allSess, err := v.ctx.SessionManager.LoadAll()
	if err != nil {
		return nil
	}

	// 3. Get active sessions from daemon
	activeResp, err := client.ListActiveSessions()
	var activeMap = make(map[string]bool)
	if err == nil && activeResp != nil {
		for _, as := range activeResp.Sessions {
			key := fmt.Sprintf("%s-%s-%s", as.ServerName, as.RoleName, as.SessionId)
			activeMap[key] = true
		}
	}

	// 4. Filter sessions that match triggerName and any of the trigger's event role keys
	var items []triggerSessionItem
	for _, s := range allSess {
		if s.TriggerName != triggerName {
			continue
		}
		key := s.ServerName + "-" + s.RoleName
		if eventRoleKeys[key] {
			status := "SILENT"
			if activeMap[s.ServerName+"-"+s.RoleName+"-"+s.ID] {
				status = "RUNNING"
			}
			items = append(items, triggerSessionItem{
				ID:         s.ID,
				ServerName: s.ServerName,
				RoleName:   s.RoleName,
				UpdatedAt:  s.UpdatedAt.Format("2006-01-02 15:04:05"),
				Status:     status,
			})
		}
	}

	// Check active sessions and append any that are not in items
	if activeResp != nil {
		for _, as := range activeResp.Sessions {
			key := as.ServerName + "-" + as.RoleName
			if eventRoleKeys[key] {
				found := false
				for _, item := range items {
					if item.ID == as.SessionId && item.ServerName == as.ServerName && item.RoleName == as.RoleName {
						found = true
						break
					}
				}
				if !found {
					items = append([]triggerSessionItem{{
						ID:         as.SessionId,
						ServerName: as.ServerName,
						RoleName:   as.RoleName,
						UpdatedAt:  time.Unix(as.StartTimeUnix, 0).Format("2006-01-02 15:04:05"),
						Status:     "RUNNING",
					}}, items...)
				}
			}
		}
	}

	// Sort sessions by updated at descending so latest is on top
	sort.Slice(items, func(i, j int) bool {
		return items[i].UpdatedAt > items[j].UpdatedAt
	})

	return items
}

func (v *triggersViewModel) parseEventIDs(displayValue string) []string {
	if displayValue == "" {
		return nil
	}
	var ids []string
	for _, part := range strings.Split(displayValue, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, e := range v.events {
			if fmt.Sprintf("%s (%s)", e.Name, e.ID) == part {
				ids = append(ids, e.ID)
				break
			}
		}
	}
	return ids
}

func (v *triggersViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *triggersViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return triggersDataMsg{}
	}
}

type triggersDataMsg struct{}

type loadSessionsDoneMsg struct {
	sessions []triggerSessionItem
}

func (v *triggersViewModel) Update(msg tea.Msg) tea.Cmd {

	if v.form.IsActive() {
		if msg, ok := msg.(tea.KeyMsg); ok {
			done, cancelled := v.form.Update(msg)
			if cancelled {
				return nil
			}
			if done {
				return v.handleFormDone()
			}
		}
		return nil
	}

	switch msg := msg.(type) {
	case triggersDataMsg:
		if !daemon.IsDaemonRunning() {
			v.items = nil
			return nil
		}
		client, err := daemon.NewClient()
		if err != nil {
			return nil
		}
		resp, err := client.ListTriggers()
		if err != nil {
			return nil
		}
		v.items = make([]triggerItem, 0, len(resp.Triggers))
		for _, t := range resp.Triggers {
			v.items = append(v.items, triggerItem{
				ID:          t.Id,
				Name:        t.Name,
				Type:        t.TriggerType,
				Description: t.Description,
				Events:      t.EventCount,
				Running:     t.Running,
				Interval:    t.Interval,
				CronExpr:    t.CronExpr,
				WatchPath:   t.WatchPath,
			})
		}
	case loadSessionsDoneMsg:
		v.sessions = msg.sessions
		return nil

	case tea.KeyMsg:
		if v.showingSessions {
			switch msg.String() {
			case "up", "k":
				if v.sessionCursor > 0 {
					v.sessionCursor--
				}
			case "down", "j":
				if v.sessionCursor < len(v.sessions)-1 {
					v.sessionCursor++
				}
			case "esc":
				v.showingSessions = false
				return v.refresh()
			case "enter":
				if v.sessionCursor < len(v.sessions) {
					s := v.sessions[v.sessionCursor]
					serverAddr := ""
					if srv, err := v.ctx.ServerEngine.GetByDisplayName(s.ServerName); err == nil {
						serverAddr = srv.Address
					}
					return func() tea.Msg {
						return switchToChatMsg{
							roleKey:     s.ServerName + "-" + s.RoleName,
							serverAddr:  serverAddr,
							sessionID:   s.ID,
							triggerName: v.items[v.cursor].Name,
						}
					}
				}
			}
			return nil
		}

		switch msg.String() {
		case "up", "k":
			if v.cursor > 0 {
				v.cursor--
			}
		case "down", "j":
			if v.cursor < len(v.items)-1 {
				v.cursor++
			}
		case "enter":
			if v.cursor < len(v.items) {
				v.showingSessions = true
				v.sessionCursor = 0
				v.sessionScrollOffset = 0
				v.sessions = nil // Reset while loading
				tID := v.items[v.cursor].ID
				return func() tea.Msg {
					return loadSessionsDoneMsg{sessions: v.loadSessions(tID)}
				}
			}
		case "s":

			if v.cursor < len(v.items) {
				id := v.items[v.cursor].ID
				return func() tea.Msg {
					if client, err := daemon.NewClient(); err == nil {
						_, _ = client.StartTrigger(id)
					}
					return triggersDataMsg{}
				}
			}
		case "x":

			if v.cursor < len(v.items) {
				id := v.items[v.cursor].ID
				return func() tea.Msg {
					if client, err := daemon.NewClient(); err == nil {
						_, _ = client.StopTrigger(id)
					}
					return triggersDataMsg{}
				}
			}
		case "r":

			if v.cursor < len(v.items) {
				id := v.items[v.cursor].ID
				return func() tea.Msg {
					if client, err := daemon.NewClient(); err == nil {
						_, _ = client.RunTrigger(id)
					}
					return triggersDataMsg{}
				}
			}
		case "c", "C":
			v.pendingCreateType = ""
			v.editingTrigger = nil
			v.form.startCreate("Create Trigger", []formField{
				{label: "Type", required: true, options: []string{"direct", "periodic", "edit"}},
			})
			return nil
		case "e", "E":
			if v.cursor < len(v.items) {
				t := v.items[v.cursor]
				v.editingTrigger = &t
				v.pendingCreateType = ""
				v.events = v.loadEvents()
				var selectedEventIDs []string
				if client, err := daemon.NewClient(); err == nil {
					if resp, err := client.GetTriggerEvents(t.ID); err == nil {
						for _, e := range resp.Events {
							selectedEventIDs = append(selectedEventIDs, e.Id)
						}
					}
				}
				v.form.startEdit("Edit Trigger: "+t.Name, triggerFieldsForType(t.Type, &t, v.events, selectedEventIDs))
			}
			return nil
		case "d", "D":
			if v.cursor < len(v.items) {
				id := v.items[v.cursor].ID
				return func() tea.Msg {
					if client, err := daemon.NewClient(); err == nil {
						_, _ = client.DeleteTrigger(id)
					}
					return triggersDataMsg{}
				}
			}
		}
	}
	return nil
}

func (v *triggersViewModel) handleFormDone() tea.Cmd {
	vals := v.form.Values()
	mode := v.form.mode

	if mode == formModeCreate {
		if v.pendingCreateType == "" && len(vals) == 1 {
			v.pendingCreateType = vals[0]
			v.events = v.loadEvents()
			v.form.startCreate("Create "+v.pendingCreateType+" Trigger", triggerFieldsForType(v.pendingCreateType, nil, v.events, nil))
			return nil
		}

		triggerType := v.pendingCreateType
		fields := formValuesByLabel(v.form.fields)
		v.pendingCreateType = ""

		return func() tea.Msg {
			if client, err := daemon.NewClient(); err == nil {
				req := &pb.CreateTriggerRequest{
					Name:        fields["Name"],
					TriggerType: triggerType,
					Description: fields["Description"],
					EventIds:    v.parseEventIDs(fields["Event IDs"]),
				}
				applyTypeSpecificTriggerFields(req, triggerType, fields)
				_, _ = client.CreateTrigger(req)
			}
			return triggersDataMsg{}
		}
	}

	if v.cursor < len(v.items) {
		id := v.items[v.cursor].ID
		triggerType := v.items[v.cursor].Type
		fields := formValuesByLabel(v.form.fields)
		v.editingTrigger = nil
		return func() tea.Msg {
			if client, err := daemon.NewClient(); err == nil {
				req := &pb.UpdateTriggerRequest{
					Id:          id,
					Name:        fields["Name"],
					Description: fields["Description"],
					EventIds:    v.parseEventIDs(fields["Event IDs"]),
				}
				applyTypeSpecificUpdateFields(req, triggerType, fields)
				_, _ = client.UpdateTrigger(req)
			}
			return triggersDataMsg{}
		}
	}
	return nil
}

func triggerFieldsForType(triggerType string, t *triggerItem, events []eventItem, selectedEventIDs []string) []formField {
	fields := []formField{
		{label: "Name", placeholder: "trigger name", value: triggerValue(t, "name"), required: true},
		{label: "Description", placeholder: "description", value: triggerValue(t, "description"), required: false},
	}
	switch triggerType {
	case "periodic":
		fields = append(fields,
			formField{label: "Schedule Type", value: scheduleType(t), required: true, options: []string{"interval", "cron"}},
			formField{label: "Schedule Value", placeholder: "5m or cron expression", value: scheduleValue(t), required: true},
		)
	case "edit":
		fields = append(fields,
			formField{label: "Watch Path", placeholder: "workspace-relative path", value: triggerValue(t, "watch_path"), required: true},
		)
		if t == nil {
			fields = append(fields, formField{label: "Recursive", value: "No", required: true, options: []string{"No", "Yes"}})
		}
	}
	if len(events) > 0 {
		items := make([]string, len(events))
		for i, e := range events {
			items[i] = fmt.Sprintf("%s (%s)", e.Name, e.ID)
		}
		selected := ""
		if len(selectedEventIDs) > 0 {
			var sel []string
			for _, id := range selectedEventIDs {
				for _, e := range events {
					if e.ID == id {
						sel = append(sel, fmt.Sprintf("%s (%s)", e.Name, e.ID))
						break
					}
				}
			}
			selected = strings.Join(sel, ",")
		}
		fields = append(fields, formField{label: "Event IDs", required: false, multiSelect: true, multiSelectItems: items, value: selected})
	} else {
		fields = append(fields, formField{label: "Event IDs", placeholder: "comma-separated event IDs", required: false})
	}
	return fields
}

func triggerValue(t *triggerItem, name string) string {
	if t == nil {
		return ""
	}
	switch name {
	case "name":
		return t.Name
	case "description":
		return t.Description
	case "watch_path":
		return t.WatchPath
	default:
		return ""
	}
}

func scheduleType(t *triggerItem) string {
	if t != nil && t.CronExpr != "" {
		return "cron"
	}
	return "interval"
}

func scheduleValue(t *triggerItem) string {
	if t == nil {
		return ""
	}
	if t.CronExpr != "" {
		return t.CronExpr
	}
	return t.Interval
}

func formValuesByLabel(fields []formField) map[string]string {
	values := make(map[string]string, len(fields))
	for _, f := range fields {
		values[f.label] = f.value
	}
	return values
}

func applyTypeSpecificTriggerFields(req *pb.CreateTriggerRequest, triggerType string, fields map[string]string) {
	switch triggerType {
	case "periodic":
		if fields["Schedule Type"] == "cron" {
			req.CronExpr = fields["Schedule Value"]
		} else {
			req.Interval = fields["Schedule Value"]
		}
	case "edit":
		req.WatchPath = fields["Watch Path"]
		req.Recursive = fields["Recursive"] == "Yes"
	}
}

func applyTypeSpecificUpdateFields(req *pb.UpdateTriggerRequest, triggerType string, fields map[string]string) {
	switch triggerType {
	case "periodic":
		if fields["Schedule Type"] == "cron" {
			req.CronExpr = fields["Schedule Value"]
		} else {
			req.Interval = fields["Schedule Value"]
		}
	case "edit":
		req.WatchPath = fields["Watch Path"]
		if recursive, ok := fields["Recursive"]; ok {
			req.Recursive = recursive == "Yes"
		}
	}
}

func (v *triggersViewModel) ensureSessionCursorVisible() {
	maxVisible := v.height - 6
	if maxVisible < 3 {
		maxVisible = 3
	}
	if v.sessionCursor < v.sessionScrollOffset {
		v.sessionScrollOffset = v.sessionCursor
	}
	if v.sessionCursor >= v.sessionScrollOffset+maxVisible {
		v.sessionScrollOffset = v.sessionCursor - maxVisible + 1
	}
}

func (v *triggersViewModel) ensureCursorVisible() {
	maxVisible := v.height - 6
	if maxVisible < 3 {
		maxVisible = 3
	}
	if v.cursor < v.scrollOffset {
		v.scrollOffset = v.cursor
	}
	if v.cursor >= v.scrollOffset+maxVisible {
		v.scrollOffset = v.cursor - maxVisible + 1
	}
}

func (v *triggersViewModel) View() string {
	if v.form.IsActive() {
		return v.form.View()
	}

	var b strings.Builder

	if v.showingSessions {
		b.WriteString(titleStyle.Render("  Trigger Sessions"))
		b.WriteString("\n\n")

		if !daemon.IsDaemonRunning() {
			b.WriteString(errorStyle.Render("  Daemon is not running."))
			b.WriteString("\n")
			b.WriteString(hintStyle.Render("  Use 'daemon start' to launch the background daemon."))
			return b.String()
		}

		if len(v.sessions) == 0 {
			b.WriteString(hintStyle.Render("  No sessions found for this trigger yet. Press [esc] to go back."))
			return b.String()
		}

		header := fmt.Sprintf("  %-25s %-25s %-20s %-12s", "SESSION ID", "ROLE/EVENT", "LAST ACTIVE", "STATUS")
		b.WriteString(tableHeaderStyle.Render(header))
		b.WriteString("\n")

		maxVisible := v.height - 6
		if maxVisible < 3 {
			maxVisible = 3
		}
		v.ensureSessionCursorVisible()
		end := v.sessionScrollOffset + maxVisible
		if end > len(v.sessions) {
			end = len(v.sessions)
		}

		for i := v.sessionScrollOffset; i < end; i++ {
			s := v.sessions[i]
			status := errorStyle.Render("SILENT")
			if s.Status == "RUNNING" {
				status = successStyle.Render("RUNNING")
			}
			row := fmt.Sprintf("  %-25s %-25s %-20s ",
				truncate(s.ID, 25), truncate(s.ServerName+"-"+s.RoleName, 25), s.UpdatedAt)
			if i == v.sessionCursor {
				b.WriteString(tableSelectedStyle.Render(row))
				b.WriteString(status)
			} else {
				b.WriteString(tableRowStyle.Render(row))
				b.WriteString(status)
			}
			b.WriteString("\n")
		}

		if len(v.sessions) > maxVisible {
			scrollInfo := fmt.Sprintf("  (%d-%d of %d)", v.sessionScrollOffset+1, end, len(v.sessions))
			b.WriteString(hintStyle.Render(scrollInfo))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  [esc] Back  [enter] Enter Session"))
		return b.String()
	}

	b.WriteString(titleStyle.Render("  Triggers"))
	b.WriteString("\n\n")

	if !daemon.IsDaemonRunning() {
		b.WriteString(errorStyle.Render("  Daemon is not running."))
		b.WriteString("\n")
		b.WriteString(hintStyle.Render("  Use 'daemon start' to launch the background daemon."))
		return b.String()
	}

	if len(v.items) == 0 {
		b.WriteString(hintStyle.Render("  No triggers defined. Press [c] to create one."))
		return b.String()
	}

	header := fmt.Sprintf("  %-16s %-18s %-10s %-8s %-10s", "ID", "NAME", "TYPE", "EVENTS", "STATUS")
	b.WriteString(tableHeaderStyle.Render(header))
	b.WriteString("\n")

	// Calculate visible window
	maxVisible := v.height - 6
	if maxVisible < 3 {
		maxVisible = 3
	}
	v.ensureCursorVisible()
	end := v.scrollOffset + maxVisible
	if end > len(v.items) {
		end = len(v.items)
	}

	for i := v.scrollOffset; i < end; i++ {
		t := v.items[i]
		status := errorStyle.Render("stopped")
		if t.Running {
			status = successStyle.Render("running")
		}
		row := fmt.Sprintf("  %-16s %-18s %-10s %-8d ",
			truncate(t.ID, 16), truncate(t.Name, 18), t.Type, t.Events)
		if i == v.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
			b.WriteString(status)
		} else {
			b.WriteString(tableRowStyle.Render(row))
			b.WriteString(status)
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(v.items) > maxVisible {
		scrollInfo := fmt.Sprintf("  (%d-%d of %d)", v.scrollOffset+1, end, len(v.items))
		b.WriteString(hintStyle.Render(scrollInfo))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(hintStyle.Render("  [\u2191/\u2193] Navigate  [c] Create  [e] Edit  [d] Delete  [s] Start  [x] Stop  [r] Run"))

	return b.String()
}
