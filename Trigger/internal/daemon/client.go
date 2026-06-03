package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Lyoomu/TAC/proto"
)

type Client struct {
	conn pb.DaemonServiceClient
}

func defaultPortPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".tac", "daemon", "port")
}

func readDaemonPort() (string, error) {
	path := defaultPortPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("daemon not running (port file not found at %s)", path)
		}
		return "", fmt.Errorf("read port file: %w", err)
	}
	port := string(data)
	if port == "" {
		return "", fmt.Errorf("daemon port file is empty")
	}
	return port, nil
}

func NewClient() (*Client, error) {
	port, err := readDaemonPort()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	addr := "127.0.0.1:" + port
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("connect to daemon at %s: %w", addr, err)
	}

	return &Client{conn: pb.NewDaemonServiceClient(conn)}, nil
}

func IsDaemonRunning() bool {
	port, err := readDaemonPort()
	if err != nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	addr := "127.0.0.1:" + port
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return false
	}
	defer conn.Close()

	client := pb.NewDaemonServiceClient(conn)
	_, err = client.Ping(ctx, &pb.DaemonPingRequest{})
	return err == nil
}

func (c *Client) Ping() (*pb.DaemonPingResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.conn.Ping(ctx, &pb.DaemonPingRequest{})
}

func (c *Client) StartTrigger(id string) (*pb.TriggerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.StartTrigger(ctx, &pb.TriggerIDRequest{TriggerId: id})
}

func (c *Client) StopTrigger(id string) (*pb.TriggerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.StopTrigger(ctx, &pb.TriggerIDRequest{TriggerId: id})
}

func (c *Client) RunTrigger(id string) (*pb.TriggerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	return c.conn.RunTrigger(ctx, &pb.TriggerIDRequest{TriggerId: id})
}

func (c *Client) ListTriggers() (*pb.TriggerListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.conn.ListTriggers(ctx, &pb.Empty{})
}

func (c *Client) ListEvents() (*pb.EventListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.conn.ListEvents(ctx, &pb.Empty{})
}

func (c *Client) GetTriggerEvents(id string) (*pb.EventListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.conn.GetTriggerEvents(ctx, &pb.TriggerIDRequest{TriggerId: id})
}

func (c *Client) GetDaemonStatus() (*pb.DaemonStatusResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.conn.GetDaemonStatus(ctx, &pb.Empty{})
}

func (c *Client) CreateTrigger(req *pb.CreateTriggerRequest) (*pb.TriggerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.CreateTrigger(ctx, req)
}

func (c *Client) UpdateTrigger(req *pb.UpdateTriggerRequest) (*pb.TriggerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.UpdateTrigger(ctx, req)
}

func (c *Client) DeleteTrigger(id string) (*pb.TriggerResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.DeleteTrigger(ctx, &pb.TriggerIDRequest{TriggerId: id})
}

func (c *Client) CreateEvent(req *pb.CreateEventRequest) (*pb.EventResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.CreateEvent(ctx, req)
}

func (c *Client) UpdateEvent(req *pb.UpdateEventRequest) (*pb.EventResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.UpdateEvent(ctx, req)
}

func (c *Client) DeleteEvent(id string) (*pb.EventResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.DeleteEvent(ctx, &pb.EventIDRequest{EventId: id})
}

func (c *Client) ListEnvPresets() (*pb.EnvPresetListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.conn.ListEnvPresets(ctx, &pb.Empty{})
}

func (c *Client) CreateEnvPreset(req *pb.CreateEnvPresetRequest) (*pb.EnvPresetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.CreateEnvPreset(ctx, req)
}

func (c *Client) UpdateEnvPreset(req *pb.UpdateEnvPresetRequest) (*pb.EnvPresetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.UpdateEnvPreset(ctx, req)
}

func (c *Client) DeleteEnvPreset(name string) (*pb.EnvPresetResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.conn.DeleteEnvPreset(ctx, &pb.EnvPresetIDRequest{Name: name})
}

func (c *Client) Shutdown() (*pb.Empty, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.conn.Shutdown(ctx, &pb.Empty{})
}

func (c *Client) ListActiveSessions() (*pb.ActiveSessionListResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.conn.ListActiveSessions(ctx, &pb.Empty{})
}

func (c *Client) WatchSession(ctx context.Context, serverName, roleName, sessionID string) (pb.DaemonService_WatchSessionClient, error) {
	return c.conn.WatchSession(ctx, &pb.WatchSessionRequest{
		ServerName: serverName,
		RoleName:   roleName,
		SessionId:  sessionID,
	})
}
