package trigger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
	srv "github.com/Lyoomu/TAC/Trigger/internal/server"
	sess "github.com/Lyoomu/TAC/Trigger/internal/session"
	"github.com/Lyoomu/TAC/Trigger/internal/tool"
	pb "github.com/Lyoomu/TAC/proto"
)

type Executor struct {
	serverEngine  *srv.Engine
	sessionMgr    *sess.Manager
	triggerEngine *Engine

	activeMu sync.RWMutex
	active   map[string]*ActiveSession // key = ServerName-RoleName-SessionID
}

func NewExecutor(serverEngine *srv.Engine, sessionMgr *sess.Manager, triggerEngine *Engine) *Executor {
	return &Executor{
		serverEngine:  serverEngine,
		sessionMgr:    sessionMgr,
		triggerEngine: triggerEngine,
		active:        make(map[string]*ActiveSession),
	}
}

func (ex *Executor) GetActiveSessions() []*ActiveSession {
	ex.activeMu.RLock()
	defer ex.activeMu.RUnlock()
	var list []*ActiveSession
	for _, s := range ex.active {
		list = append(list, s)
	}
	return list
}

func (ex *Executor) GetActiveSession(serverName, roleName, sessionID string) *ActiveSession {
	ex.activeMu.RLock()
	defer ex.activeMu.RUnlock()
	key := serverName + "-" + roleName + "-" + sessionID
	return ex.active[key]
}

func (ex *Executor) Execute(ev *model.Event, triggerID string) error {

	_ = ex.serverEngine.Load()

	roleKey := ev.RoleKey

	var serverDisplayName, roleName string
	for _, s := range ex.serverEngine.List() {
		for _, r := range s.Roles {
			key := r.ServerName + "-" + r.RoleName
			if key == roleKey {
				serverDisplayName = r.ServerName
				roleName = r.RoleName
				break
			}
		}
		if serverDisplayName != "" {
			break
		}
	}
	if serverDisplayName == "" {
		return fmt.Errorf("role '%s' not loaded", roleKey)
	}

	serverConn, err := ex.serverEngine.GetByDisplayName(serverDisplayName)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	client, err := ex.serverEngine.GetClient(serverConn.Address)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	mergedEnv, err := ex.triggerEngine.ResolveEventEnv(ev)
	if err != nil {
		return fmt.Errorf("env preset: %w", err)
	}

	env, err := ResolveEnv(mergedEnv, ex.triggerEngine.BasePath())
	if err != nil {
		return fmt.Errorf("resolve env: %w", err)
	}

	initialMsg := ev.InitialMsg
	for key, val := range env {
		initialMsg = strings.ReplaceAll(initialMsg, "{"+key+"}", val)
	}

	var sessionID string
	if ev.SessionMode == model.SessionModeShared {
		sessionID = ev.SharedSessionID
		if sessionID == "" {

			sessionID = fmt.Sprintf("%d", time.Now().Unix())
			_ = ex.updateEventSessionID(ev.ID, sessionID)
		}
	}

	triggerName := "default"
	if trigger, err := ex.triggerEngine.GetTrigger(triggerID); err == nil && trigger != nil {
		triggerName = trigger.Name
	}

	var session *model.Session
	if sessionID != "" {
		session, _ = ex.sessionMgr.GetOrCreate(triggerName, serverDisplayName, roleName, sessionID)
	} else {
		session = ex.sessionMgr.Create(triggerName, serverDisplayName, roleName)
	}

	activeSess := &ActiveSession{
		SessionID:   session.ID,
		ServerName:  serverDisplayName,
		RoleName:    roleName,
		TriggerID:   triggerID,
		EventID:     ev.ID,
		StartTime:   time.Now(),
		Subscribers: make(map[string]chan *pb.ChatMessage),
	}
	sessKey := serverDisplayName + "-" + roleName + "-" + session.ID
	ex.activeMu.Lock()
	ex.active[sessKey] = activeSess
	ex.activeMu.Unlock()

	defer func() {
		ex.activeMu.Lock()
		delete(ex.active, sessKey)
		ex.activeMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	stream, err := client.Chat.Chat(ctx)
	if err != nil {
		return fmt.Errorf("start chat: %w", err)
	}

	if len(session.Messages) > 0 {
		for _, msg := range session.Messages {
			_ = stream.Send(&pb.ChatMessage{
				Role:        msg.Role,
				Content:     msg.Content,
				MessageType: "history",
				IsHistory:   true,
				RoleName:    roleName,
				SessionId:   session.ID,
			})
		}
	}

	initialMsgProto := &pb.ChatMessage{
		MessageType: "text",
		Content:     initialMsg,
		Role:        "user",
		RoleName:    roleName,
		SessionId:   session.ID,
	}
	if ev.MessageMode != "" {
		initialMsgProto.MessageMode = ev.MessageMode
	}
	_ = stream.Send(initialMsgProto)
	activeSess.Broadcast(initialMsgProto)

	_ = ex.sessionMgr.AppendMessage(session, "user", initialMsg)

	var assistantContent strings.Builder
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}

		activeSess.Broadcast(resp)

		switch resp.MessageType {
		case "text":
			if resp.EndOfTurn {
				if assistantContent.Len() > 0 {
					_ = ex.sessionMgr.AppendMessage(session, "assistant", assistantContent.String())
				}
				_ = ex.sessionMgr.Save(session)
				return nil
			}
			assistantContent.WriteString(resp.Content)

		case "tool_call":
			fmt.Printf("[Event %s] Tool call: %s(%s)\n", ev.ID, resp.ToolName, resp.ToolArguments)

			var resultStr string
			toolInfo, ok := tool.FindLoadedTool(ex.serverEngine.GetLoadedRoles(), serverDisplayName, roleName, resp.ToolName)
			if !ok {
				errJSON, _ := json.Marshal(map[string]string{"error": "tool not loaded: " + resp.ToolName})
				resultStr = string(errJSON)
			} else {
				res, err := tool.Execute(toolInfo, resp.ToolArguments, ex.triggerEngine.BasePath())
				if err != nil {
					errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
					resultStr = string(errJSON)
				} else {
					resultStr = res
				}
			}

			toolResultProto := &pb.ChatMessage{
				MessageType: "tool_result",
				ToolCallId:  resp.ToolCallId,
				ToolResult:  resultStr,
				RoleName:    roleName,
				SessionId:   session.ID,
			}
			_ = stream.Send(toolResultProto)
			activeSess.Broadcast(toolResultProto)

		case "error":
			return fmt.Errorf("agent error: %s", resp.ErrorMessage)
		}
	}

	_ = ex.sessionMgr.Save(session)
	return nil
}

func (ex *Executor) updateEventSessionID(eventID, sessionID string) error {
	if ex.triggerEngine == nil {
		return nil
	}
	return ex.triggerEngine.UpdateEvent(eventID, func(ev *model.Event) {
		ev.SharedSessionID = sessionID
	})
}

func (ex *Executor) ExecuteConcurrently(events []*model.Event, triggerID string) {
	var wg sync.WaitGroup
	for _, ev := range events {
		wg.Add(1)
		go func(ev *model.Event) {
			defer wg.Done()
			if err := ex.Execute(ev, triggerID); err != nil {
				fmt.Printf("[Trigger %s] Event '%s' failed: %v\n", triggerID, ev.ID, err)
			} else {
				fmt.Printf("[Trigger %s] Event '%s' executed\n", triggerID, ev.ID)
			}
		}(ev)
	}
	wg.Wait()
}
