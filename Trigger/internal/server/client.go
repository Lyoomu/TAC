package server

import (
	"context"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	"github.com/Lyoomu/TAC/Trigger/internal/config"
	"github.com/Lyoomu/TAC/Trigger/internal/security"
	pb "github.com/Lyoomu/TAC/proto"
)

type Client struct {
	conn    *grpc.ClientConn
	address string
	token   string

	Discovery pb.DiscoveryServiceClient
	Chat      pb.ChatServiceClient
	Tool      pb.ToolServiceClient
}

func Connect(address, token, trustedFingerprint string) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tlsConfig, err := security.NewTLSConfig(trustedFingerprint)
	if err != nil {
		return nil, fmt.Errorf("tls config: %w", err)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	}

	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", address, err)
	}

	return &Client{
		conn:      conn,
		address:   address,
		token:     token,
		Discovery: pb.NewDiscoveryServiceClient(conn),
		Chat:      pb.NewChatServiceClient(conn),
		Tool:      pb.NewToolServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) AuthContext(ctx context.Context) context.Context {
	if c.token == "" {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+c.token))
}

func (c *Client) Ping(ctx context.Context) (*pb.PingResponse, error) {
	return c.Discovery.Ping(ctx, &pb.PingRequest{ClientVersion: config.Version})
}

func (c *Client) GetServerInfo(ctx context.Context) (*pb.ServerInfoResponse, error) {
	return c.Discovery.GetServerInfo(c.AuthContext(ctx), &pb.ServerInfoRequest{})
}

func (c *Client) ListRoles(ctx context.Context) (*pb.ListRolesResponse, error) {
	return c.Discovery.ListRoles(c.AuthContext(ctx), &pb.ListRolesRequest{})
}

func (c *Client) GetRole(ctx context.Context, name string) (*pb.Role, error) {
	return c.Discovery.GetRole(c.AuthContext(ctx), &pb.GetRoleRequest{Name: name})
}

func (c *Client) GetRoleTools(ctx context.Context, roleName string) (*pb.GetRoleToolsResponse, error) {
	return c.Discovery.GetRoleTools(c.AuthContext(ctx), &pb.GetRoleToolsRequest{RoleName: roleName})
}

func (c *Client) DownloadTool(ctx context.Context, toolName string, downloadSource, downloadBinary bool) (map[string][]byte, error) {
	stream, err := c.Tool.DownloadTool(c.AuthContext(ctx), &pb.DownloadToolRequest{
		ToolName:       toolName,
		DownloadSource: downloadSource,
		DownloadBinary: downloadBinary,
	})
	if err != nil {
		return nil, fmt.Errorf("start download: %w", err)
	}

	result := make(map[string][]byte)
	var currentFile string
	var lastFile bool

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("recv chunk: %w", err)
		}

		if chunk.FileName != currentFile {
			currentFile = chunk.FileName
		}
		result[currentFile] = append(result[currentFile], chunk.Data...)

		if chunk.WarningMessage != "" {
			fmt.Println(chunk.WarningMessage)
		}

		if chunk.IsLastFile {
			lastFile = true
		}
		if lastFile {
			break
		}
	}

	return result, nil
}

func (c *Client) Address() string {
	return c.address
}
