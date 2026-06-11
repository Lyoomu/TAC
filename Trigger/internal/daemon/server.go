package daemon

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
	srv "github.com/Lyoomu/TAC/Trigger/internal/server"
	sess "github.com/Lyoomu/TAC/Trigger/internal/session"
	"github.com/Lyoomu/TAC/Trigger/internal/trigger"
	"github.com/Lyoomu/TAC/Trigger/internal/workspace"
	pb "github.com/Lyoomu/TAC/proto"
)

type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	configPath string
	portPath   string
	workspace  string
	basePath   string
	startTime  time.Time

	triggerEngine *trigger.Engine
	executor      *trigger.Executor
	serverEngine  *srv.Engine

	pb.UnimplementedDaemonServiceServer
}

func NewServer() (*Server, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	configPath := filepath.Join(home, ".tac", "daemon")
	portPath := filepath.Join(configPath, "port")

	wsEngine := workspace.NewEngine()
	if err := wsEngine.Load(); err != nil {
		return nil, fmt.Errorf("load workspaces: %w", err)
	}
	ws, err := wsEngine.GetActive()
	if err != nil {
		return nil, fmt.Errorf("no active workspace: %w", err)
	}

	te := trigger.NewEngine(ws.Name, ws.Path)
	if err := te.Load(); err != nil {
		return nil, fmt.Errorf("load triggers: %w", err)
	}

	serverEngine := srv.NewEngine()
	_ = serverEngine.Load()
	sessionMgr := sess.NewManager()
	executor := trigger.NewExecutor(serverEngine, sessionMgr, te)

	return &Server{
		configPath:    configPath,
		portPath:      portPath,
		workspace:     ws.Name,
		basePath:      ws.Path,
		startTime:     time.Now(),
		triggerEngine: te,
		executor:      executor,
		serverEngine:  serverEngine,
	}, nil
}

func (s *Server) Start() (string, error) {

	if err := os.MkdirAll(s.configPath, 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen: %w", err)
	}
	s.listener = lis

	port := lis.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(s.portPath, []byte(fmt.Sprintf("%d", port)), 0644); err != nil {
		return "", fmt.Errorf("write port file: %w", err)
	}

	s.grpcServer = grpc.NewServer()
	pb.RegisterDaemonServiceServer(s.grpcServer, s)

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			fmt.Printf("daemon server error: %v\n", err)
		}
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	fmt.Printf("TAC Trigger daemon started on %s (workspace: %s)\n", addr, s.workspace)
	return addr, nil
}

func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	_ = os.Remove(s.portPath)

	s.triggerEngine.StopAll()

	if s.serverEngine != nil {
		s.serverEngine.CloseAll()
	}
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) Ping(ctx context.Context, req *pb.DaemonPingRequest) (*pb.DaemonPingResponse, error) {
	runningCount := 0
	for _, t := range s.triggerEngine.ListTriggers() {
		if s.triggerEngine.IsRunning(t.ID) {
			runningCount++
		}
	}
	return &pb.DaemonPingResponse{
		DaemonVersion:   "0.1.0",
		Workspace:       s.workspace,
		RunningTriggers: int32(runningCount),
	}, nil
}

func (s *Server) StartTrigger(ctx context.Context, req *pb.TriggerIDRequest) (*pb.TriggerResponse, error) {
	if err := s.triggerEngine.StartTrigger(req.TriggerId, s.executor); err != nil {
		return &pb.TriggerResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.TriggerResponse{Success: true, Message: fmt.Sprintf("trigger '%s' started", req.TriggerId)}, nil
}

func (s *Server) StopTrigger(ctx context.Context, req *pb.TriggerIDRequest) (*pb.TriggerResponse, error) {
	if err := s.triggerEngine.StopTrigger(req.TriggerId); err != nil {
		return &pb.TriggerResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.TriggerResponse{Success: true, Message: fmt.Sprintf("trigger '%s' stopped", req.TriggerId)}, nil
}

func (s *Server) RunTrigger(ctx context.Context, req *pb.TriggerIDRequest) (*pb.TriggerResponse, error) {
	t, err := s.triggerEngine.GetTrigger(req.TriggerId)
	if err != nil {
		return &pb.TriggerResponse{Success: false, Message: err.Error()}, nil
	}
	if t.Type != model.TriggerTypeDirect {
		return &pb.TriggerResponse{Success: false, Message: "not a direct trigger"}, nil
	}

	events, err := s.triggerEngine.GetTriggerEvents(req.TriggerId)
	if err != nil {
		return &pb.TriggerResponse{Success: false, Message: err.Error()}, nil
	}

	var lastErr error
	for _, ev := range events {
		if err := s.executor.Execute(&ev, req.TriggerId); err != nil {
			lastErr = err
		}
	}

	if lastErr != nil {
		return &pb.TriggerResponse{Success: false, Message: lastErr.Error()}, nil
	}
	return &pb.TriggerResponse{Success: true, Message: "executed"}, nil
}

func (s *Server) ListTriggers(ctx context.Context, req *pb.Empty) (*pb.TriggerListResponse, error) {
	triggers := s.triggerEngine.ListTriggers()
	var pbTriggers []*pb.DaemonTrigger
	for _, t := range triggers {
		pbTriggers = append(pbTriggers, &pb.DaemonTrigger{
			Id:          t.ID,
			Name:        t.Name,
			TriggerType: string(t.Type),
			Description: t.Description,
			Enabled:     t.Enabled,
			Running:     s.triggerEngine.IsRunning(t.ID),
			EventCount:  int32(len(t.EventIDs)),
			Interval:    t.Interval,
			CronExpr:    t.CronExpr,
			WatchPath:   t.WatchPath,
		})
	}
	return &pb.TriggerListResponse{Triggers: pbTriggers}, nil
}

func (s *Server) ListEvents(ctx context.Context, req *pb.Empty) (*pb.EventListResponse, error) {
	events := s.triggerEngine.ListEvents()
	var pbEvents []*pb.DaemonEvent
	for _, ev := range events {
		envCount := int32(len(ev.Env))
		if merged, err := s.triggerEngine.ResolveEventEnv(&ev); err == nil {
			envCount = int32(len(merged))
		}
		pbEvents = append(pbEvents, &pb.DaemonEvent{
			Id:          ev.ID,
			Name:        ev.Name,
			RoleKey:     ev.RoleKey,
			SessionMode: string(ev.SessionMode),
			EnvCount:    envCount,
			EnvPreset:   ev.EnvPreset,
		})
	}
	return &pb.EventListResponse{Events: pbEvents}, nil
}

func (s *Server) GetTriggerEvents(ctx context.Context, req *pb.TriggerIDRequest) (*pb.EventListResponse, error) {
	events, err := s.triggerEngine.GetTriggerEvents(req.TriggerId)
	if err != nil {
		return nil, err
	}
	var pbEvents []*pb.DaemonEvent
	for _, ev := range events {
		envCount := int32(len(ev.Env))
		if merged, err := s.triggerEngine.ResolveEventEnv(&ev); err == nil {
			envCount = int32(len(merged))
		}
		pbEvents = append(pbEvents, &pb.DaemonEvent{
			Id:          ev.ID,
			Name:        ev.Name,
			RoleKey:     ev.RoleKey,
			SessionMode: string(ev.SessionMode),
			EnvCount:    envCount,
			EnvPreset:   ev.EnvPreset,
		})
	}
	return &pb.EventListResponse{Events: pbEvents}, nil
}

func (s *Server) GetDaemonStatus(ctx context.Context, req *pb.Empty) (*pb.DaemonStatusResponse, error) {
	triggers := s.triggerEngine.ListTriggers()
	events := s.triggerEngine.ListEvents()

	runningCount := 0
	for _, t := range triggers {
		if s.triggerEngine.IsRunning(t.ID) {
			runningCount++
		}
	}

	uptime := time.Since(s.startTime).Round(time.Second).String()

	return &pb.DaemonStatusResponse{
		Running:             true,
		Workspace:           s.workspace,
		TriggerCount:        int32(len(triggers)),
		EventCount:          int32(len(events)),
		RunningTriggerCount: int32(runningCount),
		Uptime:              uptime,
	}, nil
}

func (s *Server) CreateTrigger(ctx context.Context, req *pb.CreateTriggerRequest) (*pb.TriggerResponse, error) {
	t := &model.Trigger{
		ID:          req.Id,
		Name:        req.Name,
		Type:        model.TriggerType(req.TriggerType),
		Description: req.Description,
		Interval:    req.Interval,
		CronExpr:    req.CronExpr,
		WatchPath:   req.WatchPath,
		Recursive:   req.Recursive,
		EventIDs:    req.EventIds,
	}
	if err := s.triggerEngine.CreateTrigger(t); err != nil {
		return &pb.TriggerResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.TriggerResponse{Success: true, Message: fmt.Sprintf("trigger '%s' created", t.ID)}, nil
}

func (s *Server) UpdateTrigger(ctx context.Context, req *pb.UpdateTriggerRequest) (*pb.TriggerResponse, error) {
	err := s.triggerEngine.UpdateTrigger(req.Id, func(t *model.Trigger) {
		if req.Name != "" {
			t.Name = req.Name
		}
		if req.Description != "" {
			t.Description = req.Description
		}
		if req.Interval != "" {
			t.Interval = req.Interval
			t.CronExpr = ""
		}
		if req.CronExpr != "" {
			t.CronExpr = req.CronExpr
			t.Interval = ""
		}
		if req.WatchPath != "" {
			t.WatchPath = req.WatchPath
		}
		t.Recursive = req.Recursive
		if req.EventIds != nil {
			t.EventIDs = req.EventIds
		}
	})
	if err != nil {
		return &pb.TriggerResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.TriggerResponse{Success: true, Message: fmt.Sprintf("trigger '%s' updated", req.Id)}, nil
}

func (s *Server) DeleteTrigger(ctx context.Context, req *pb.TriggerIDRequest) (*pb.TriggerResponse, error) {
	if err := s.triggerEngine.DeleteTrigger(req.TriggerId); err != nil {
		return &pb.TriggerResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.TriggerResponse{Success: true, Message: fmt.Sprintf("trigger '%s' deleted", req.TriggerId)}, nil
}

func (s *Server) CreateEvent(ctx context.Context, req *pb.CreateEventRequest) (*pb.EventResponse, error) {
	ev := &model.Event{
		ID:          req.Id,
		Name:        req.Name,
		Description: req.Description,
		RoleKey:     req.RoleKey,
		InitialMsg:  req.InitialMsg,
		SessionMode: model.SessionMode(req.SessionMode),
		MessageMode: req.MessageMode,
		Env:         req.Env,
		EnvPreset:   req.EnvPreset,
	}
	if err := s.triggerEngine.CreateEvent(ev); err != nil {
		return &pb.EventResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.EventResponse{Success: true, Message: fmt.Sprintf("event '%s' created", req.Id)}, nil
}

func (s *Server) UpdateEvent(ctx context.Context, req *pb.UpdateEventRequest) (*pb.EventResponse, error) {
	err := s.triggerEngine.UpdateEvent(req.Id, func(ev *model.Event) {
		if req.Name != "" {
			ev.Name = req.Name
		}
		if req.Description != "" {
			ev.Description = req.Description
		}
		if req.RoleKey != "" {
			ev.RoleKey = req.RoleKey
		}
		if req.InitialMsg != "" {
			ev.InitialMsg = req.InitialMsg
		}
		if req.SessionMode != "" {
			ev.SessionMode = model.SessionMode(req.SessionMode)
		}
		if req.SetMessageMode {
			ev.MessageMode = req.MessageMode
		}
		if req.SetEnvPreset {
			ev.EnvPreset = req.EnvPreset
		}
		if req.SetEnv {
			if req.Env == nil {
				ev.Env = map[string]string{}
			} else {
				ev.Env = req.Env
			}
		}
	})
	if err != nil {
		return &pb.EventResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.EventResponse{Success: true, Message: fmt.Sprintf("event '%s' updated", req.Id)}, nil
}

func (s *Server) DeleteEvent(ctx context.Context, req *pb.EventIDRequest) (*pb.EventResponse, error) {
	if err := s.triggerEngine.DeleteEvent(req.EventId); err != nil {
		return &pb.EventResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.EventResponse{Success: true, Message: fmt.Sprintf("event '%s' deleted", req.EventId)}, nil
}

func (s *Server) ListEnvPresets(ctx context.Context, req *pb.Empty) (*pb.EnvPresetListResponse, error) {
	presets := s.triggerEngine.ListEnvPresets()
	var pbPresets []*pb.EnvPreset
	for _, p := range presets {
		pbPresets = append(pbPresets, &pb.EnvPreset{
			Name:        p.Name,
			Description: p.Description,
			Env:         p.Env,
			CreatedAt:   p.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
		})
	}
	return &pb.EnvPresetListResponse{Presets: pbPresets}, nil
}

func (s *Server) CreateEnvPreset(ctx context.Context, req *pb.CreateEnvPresetRequest) (*pb.EnvPresetResponse, error) {
	p := &model.EnvPreset{
		Name:        req.Name,
		Description: req.Description,
		Env:         req.Env,
	}
	if err := s.triggerEngine.CreateEnvPreset(p); err != nil {
		return &pb.EnvPresetResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.EnvPresetResponse{Success: true, Message: fmt.Sprintf("env preset '%s' created", req.Name)}, nil
}

func (s *Server) UpdateEnvPreset(ctx context.Context, req *pb.UpdateEnvPresetRequest) (*pb.EnvPresetResponse, error) {
	if req.Name == "" {
		return &pb.EnvPresetResponse{Success: false, Message: "name is required"}, nil
	}
	err := s.triggerEngine.UpdateEnvPreset(req.Name, func(p *model.EnvPreset) {
		if req.SetDescription {
			p.Description = req.Description
		}
		if req.SetEnv {
			if req.Env == nil {
				p.Env = map[string]string{}
			} else {
				p.Env = req.Env
			}
		}
	})
	if err != nil {
		return &pb.EnvPresetResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.EnvPresetResponse{Success: true, Message: fmt.Sprintf("env preset '%s' updated", req.Name)}, nil
}

func (s *Server) DeleteEnvPreset(ctx context.Context, req *pb.EnvPresetIDRequest) (*pb.EnvPresetResponse, error) {
	if err := s.triggerEngine.DeleteEnvPreset(req.Name); err != nil {
		return &pb.EnvPresetResponse{Success: false, Message: err.Error()}, nil
	}
	return &pb.EnvPresetResponse{Success: true, Message: fmt.Sprintf("env preset '%s' deleted", req.Name)}, nil
}

func (s *Server) Shutdown(ctx context.Context, req *pb.Empty) (*pb.Empty, error) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.Stop()
	}()
	return &pb.Empty{}, nil
}

func (s *Server) ListActiveSessions(ctx context.Context, req *pb.Empty) (*pb.ActiveSessionListResponse, error) {
	active := s.executor.GetActiveSessions()
	var pbSessions []*pb.ActiveSessionInfo
	for _, as := range active {
		pbSessions = append(pbSessions, &pb.ActiveSessionInfo{
			SessionId:     as.SessionID,
			ServerName:    as.ServerName,
			RoleName:      as.RoleName,
			TriggerId:     as.TriggerID,
			EventId:       as.EventID,
			StartTimeUnix: as.StartTime.Unix(),
		})
	}
	return &pb.ActiveSessionListResponse{Sessions: pbSessions}, nil
}

func (s *Server) WatchSession(req *pb.WatchSessionRequest, stream pb.DaemonService_WatchSessionServer) error {
	as := s.executor.GetActiveSession(req.ServerName, req.RoleName, req.SessionId)
	if as == nil {
		return fmt.Errorf("active session %s-%s-%s not found", req.ServerName, req.RoleName, req.SessionId)
	}

	// Create a subscriber channel
	ch := make(chan *pb.ChatMessage, 128)
	subID := fmt.Sprintf("sub-%d", time.Now().UnixNano())

	as.SubMu.Lock()
	// Send existing history first
	for _, msg := range as.History {
		if err := stream.Send(msg); err != nil {
			as.SubMu.Unlock()
			return err
		}
	}
	as.Subscribers[subID] = ch
	as.SubMu.Unlock()

	defer func() {
		as.SubMu.Lock()
		delete(as.Subscribers, subID)
		as.SubMu.Unlock()
		// Do not close ch while sending is in progress, but since we are deleting sub first, it's safe.
	}()

	// Loop and forward subsequent messages
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(msg); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}
