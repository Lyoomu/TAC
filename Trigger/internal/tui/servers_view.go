package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
	"github.com/Lyoomu/TAC/Trigger/internal/tool"
	pb "github.com/Lyoomu/TAC/proto"
)

type selectorPurpose int

const (
	purposeLoadRoles selectorPurpose = iota
	purposePickChatRole
	purposeUpdateRoles
)

type serversViewModel struct {
	ctx          *AppContext
	width        int
	height       int
	items        []model.ServerConnection
	cursor       int
	scrollOffset int
	form         *formModel
	selector     *selectorModel

	selectorPurpose selectorPurpose
	pendingAddress  string
	remoteRoles     []*pb.Role
	isUpdateMode    bool
}

func newServersViewModel(ctx *AppContext) *serversViewModel {
	return &serversViewModel{ctx: ctx, form: newFormModel(), selector: newSelectorModel()}
}

func (v *serversViewModel) setSize(w, h int) {
	v.width = w
	v.height = h
}

func (v *serversViewModel) refresh() tea.Cmd {
	return func() tea.Msg {
		return serversDataMsg{}
	}
}

type serversDataMsg struct{}

type remoteRolesFetchedMsg struct {
	address string
	roles   []*pb.Role
	err     error
}

type rolesLoadCompleteMsg struct {
	loaded int
	errors []string
}

func (v *serversViewModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case serversDataMsg:
		if v.ctx.ServerEngine != nil {
			v.items = v.ctx.ServerEngine.List()
		}
	case remoteRolesFetchedMsg:
		if msg.err != nil {
			v.isUpdateMode = false
			return func() tea.Msg {
				return triggerStatusMsg("Fetch roles failed: " + msg.err.Error())
			}
		}
		if len(msg.roles) == 0 {
			v.isUpdateMode = false
			return func() tea.Msg {
				return triggerStatusMsg("No roles available on server")
			}
		}
		v.pendingAddress = msg.address
		v.remoteRoles = msg.roles
		var names []string
		for _, r := range msg.roles {
			desc := r.Name
			if r.ApiType != "" {
				desc += "  [" + r.ApiType + "]"
			}
			if r.Description != "" {
				desc += "  " + r.Description
			}
			names = append(names, desc)
		}
		if v.isUpdateMode {
			v.isUpdateMode = false
			var preselected []int
			if v.cursor < len(v.items) {
				s := v.items[v.cursor]
				loadedNames := make(map[string]bool)
				for _, lr := range s.Roles {
					loadedNames[lr.RoleName] = true
				}
				for i, r := range msg.roles {
					if loadedNames[r.Name] {
						preselected = append(preselected, i)
					}
				}
			}
			v.selectorPurpose = purposeUpdateRoles
			v.selector.startWithPreselected("Update Roles", names, preselected)
		} else {
			v.selectorPurpose = purposeLoadRoles
			v.selector.start("Select Roles to Load", names)
		}
		return nil
	case rolesLoadCompleteMsg:
		if v.ctx.ServerEngine != nil {
			v.items = v.ctx.ServerEngine.List()
		}
		summary := fmt.Sprintf("Loaded %d role(s)", msg.loaded)
		if len(msg.errors) > 0 {
			summary += fmt.Sprintf(", %d error(s): %s", len(msg.errors), strings.Join(msg.errors, "; "))
		}
		return func() tea.Msg { return triggerStatusMsg(summary) }
	case tea.KeyMsg:

		if v.form.IsActive() {
			done, cancelled := v.form.Update(msg)
			if cancelled {
				return nil
			}
			if done {
				return v.handleFormDone()
			}
			return nil
		}

		if v.selector.IsActive() {
			done, cancelled := v.selector.Update(msg)
			if cancelled {
				v.remoteRoles = nil
				v.pendingAddress = ""
				return nil
			}
			if done {
				return v.handleSelectorDone()
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
				s := v.items[v.cursor]
				if len(s.Roles) == 0 {
					return nil
				}
				if len(s.Roles) == 1 {
					return func() tea.Msg {
						roleKey := s.Roles[0].ServerName + "-" + s.Roles[0].RoleName
						return switchToChatMsg{roleKey: roleKey, serverAddr: s.Address, sessionID: ""}
					}
				}
				// Multiple loaded roles: show selector to pick one for chat
				v.pendingAddress = s.Address
				var names []string
				for _, r := range s.Roles {
					desc := r.RoleName
					if r.APIType != "" {
						desc += "  [" + r.APIType + "]"
					}
					names = append(names, desc)
				}
				v.selectorPurpose = purposePickChatRole
				v.selector.startSingle("Select Role to Chat", names)
				return nil
			}
		case "c", "C":
			v.form.startCreate("Connect to Server", []formField{
				{label: "Address", placeholder: "host:port (e.g. 127.0.0.1:50051)", required: true},
				{label: "Display Name", placeholder: "friendly name for this server", required: true},
				{label: "Auth Token", placeholder: "authentication token (optional)", required: false},
				{label: "Fingerprint", placeholder: "TLS certificate fingerprint (optional, 'insecure' to skip)", required: false},
			})
			return nil
		case "l", "L":
			if v.cursor < len(v.items) {
				s := v.items[v.cursor]
				return v.fetchRemoteRoles(s.Address)
			}
			return nil
		case "u", "U":
			if v.cursor < len(v.items) {
				s := v.items[v.cursor]
				v.isUpdateMode = true
				return v.fetchRemoteRoles(s.Address)
			}
			return nil
		case "d", "D":
			if v.cursor < len(v.items) {
				s := v.items[v.cursor]
				if v.ctx.ServerEngine != nil {
					if err := v.ctx.ServerEngine.Remove(s.Address); err != nil {
						return func() tea.Msg { return triggerStatusMsg("Remove failed: " + err.Error()) }
					}
					v.items = v.ctx.ServerEngine.List()
					if v.cursor >= len(v.items) {
						v.cursor = len(v.items) - 1
						if v.cursor < 0 {
							v.cursor = 0
						}
					}
					return func() tea.Msg { return triggerStatusMsg("Removed: " + s.DisplayName) }
				}
				return func() tea.Msg { return triggerStatusMsg("ServerEngine not available") }
			}
		}
	}
	return nil
}

func (v *serversViewModel) handleFormDone() tea.Cmd {
	vals := v.form.Values()

	if v.ctx.ServerEngine == nil {
		return func() tea.Msg { return triggerStatusMsg("ServerEngine not available") }
	}

	address := vals[0]
	displayName := vals[1]
	authToken := vals[2]
	fingerprint := vals[3]

	if err := v.ctx.ServerEngine.Add(address, displayName, authToken, fingerprint); err != nil {
		return func() tea.Msg { return triggerStatusMsg("Connect failed: " + err.Error()) }
	}
	v.items = v.ctx.ServerEngine.List()

	// After connecting, automatically fetch remote roles for selection
	return v.fetchRemoteRoles(address)
}

func (v *serversViewModel) handleSelectorDone() tea.Cmd {
	selected := v.selector.SelectedIndices()
	if len(selected) == 0 {
		v.remoteRoles = nil
		v.pendingAddress = ""
		return func() tea.Msg { return triggerStatusMsg("No roles selected") }
	}

	if v.selectorPurpose == purposePickChatRole {
		// Pick first selected role for chat
		idx := selected[0]
		s, err := v.ctx.ServerEngine.Get(v.pendingAddress)
		if err != nil || idx >= len(s.Roles) {
			v.pendingAddress = ""
			return func() tea.Msg { return triggerStatusMsg("Role not found") }
		}
		role := s.Roles[idx]
		addr := v.pendingAddress
		v.pendingAddress = ""
		return func() tea.Msg {
			roleKey := role.ServerName + "-" + role.RoleName
			return switchToChatMsg{roleKey: roleKey, serverAddr: addr, sessionID: ""}
		}
	}

	// purposeLoadRoles or purposeUpdateRoles: load selected remote roles
	address := v.pendingAddress
	var rolesToLoad []*pb.Role
	for _, idx := range selected {
		if idx < len(v.remoteRoles) {
			rolesToLoad = append(rolesToLoad, v.remoteRoles[idx])
		}
	}
	v.remoteRoles = nil
	v.pendingAddress = ""

	return v.loadSelectedRoles(address, rolesToLoad)
}

func (v *serversViewModel) fetchRemoteRoles(address string) tea.Cmd {
	return func() tea.Msg {
		if v.ctx.ServerEngine == nil {
			return triggerStatusMsg("ServerEngine not available")
		}
		client, err := v.ctx.ServerEngine.GetClient(address)
		if err != nil {
			return remoteRolesFetchedMsg{address: address, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		resp, err := client.ListRoles(ctx)
		if err != nil {
			return remoteRolesFetchedMsg{address: address, err: err}
		}
		return remoteRolesFetchedMsg{address: address, roles: resp.Roles}
	}
}

func (v *serversViewModel) loadSelectedRoles(address string, roles []*pb.Role) tea.Cmd {
	return func() tea.Msg {
		if v.ctx.ServerEngine == nil {
			return rolesLoadCompleteMsg{errors: []string{"ServerEngine not available"}}
		}
		s, err := v.ctx.ServerEngine.Get(address)
		if err != nil {
			return rolesLoadCompleteMsg{errors: []string{err.Error()}}
		}

		client, err := v.ctx.ServerEngine.GetClient(address)
		if err != nil {
			return rolesLoadCompleteMsg{errors: []string{err.Error()}}
		}

		var loaded int
		var errs []string

		for _, role := range roles {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			var toolInfos []model.ToolInfo
			toolsResp, err := client.GetRoleTools(ctx, role.Name)
			if err != nil {
				errs = append(errs, fmt.Sprintf("get tools for %s: %v", role.Name, err))
			} else if toolsResp != nil {
				for _, t := range toolsResp.Tools {
					ti := model.ToolInfo{
						Name:                t.Name,
						Description:         t.Description,
						Language:            t.Language,
						Version:             t.Version,
						Dependencies:        t.Dependencies,
						RequiresCompilation: t.RequiresCompilation,
						IsBinary:            t.IsBinary,
						SourceAvailable:     t.SourceAvailable,
						RuntimeRequirement:  t.RuntimeRequirement,
						Files:               t.Files,
					}

					files, dlErr := client.DownloadTool(ctx, t.Name, true, true)
					if dlErr != nil {
						errs = append(errs, fmt.Sprintf("download %s: %v", t.Name, dlErr))
					} else {
						toolDir, saveErr := tool.SaveFiles(s.DisplayName, t.Name, files)
						if saveErr != nil {
							errs = append(errs, fmt.Sprintf("save %s: %v", t.Name, saveErr))
						} else {
							ti.LocalPath = toolDir
							ti.DownloadedAt = time.Now()
						}
					}
					toolInfos = append(toolInfos, ti)
				}
			}
			cancel()

			loadedRole := model.LoadedRole{
				ServerName:  s.DisplayName,
				RoleName:    role.Name,
				Description: role.Description,
				APIType:     role.ApiType,
				MessageMode: role.MessageMode,
				Tools:       toolInfos,
				LoadedAt:    time.Now(),
			}
			if err := v.ctx.ServerEngine.LoadRole(address, loadedRole); err != nil {
				errs = append(errs, fmt.Sprintf("save role %s: %v", role.Name, err))
			} else {
				loaded++
			}
		}

		return rolesLoadCompleteMsg{loaded: loaded, errors: errs}
	}
}

func (v *serversViewModel) ensureCursorVisible() {
	// Reserve lines: title(2) + header(2) + hint(2) = 6
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

func (v *serversViewModel) View() string {

	if v.selector.IsActive() {
		return v.selector.View()
	}

	if v.form.IsActive() {
		return v.form.View()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("  Servers"))
	b.WriteString("\n\n")

	if len(v.items) == 0 {
		b.WriteString(hintStyle.Render("  No servers connected. Press [c] to connect one."))
		return b.String()
	}

	header := fmt.Sprintf("  %-18s %-22s %-8s %-20s", "NAME", "ADDRESS", "ROLES", "FINGERPRINT")
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
		s := v.items[i]
		fp := ""
		if s.TrustedFingerprint != "" && len(s.TrustedFingerprint) > 16 {
			fp = s.TrustedFingerprint[:16] + "..."
		}
		row := fmt.Sprintf("  %-18s %-22s %-8d %-20s",
			truncate(s.DisplayName, 18), truncate(s.Address, 22), len(s.Roles), fp)
		if i == v.cursor {
			b.WriteString(tableSelectedStyle.Render(row))
		} else {
			b.WriteString(tableRowStyle.Render(row))
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
	b.WriteString(hintStyle.Render("  [\u2191/\u2193] Navigate  [c] Connect  [l] Load Roles  [u] Update Roles  [d] Remove  [Enter] Chat"))

	return b.String()
}
