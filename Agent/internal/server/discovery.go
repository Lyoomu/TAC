package server

import (
	"context"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/config"
	pb "github.com/Lyoomu/TAC/proto"
)

type discoveryServer struct {
	server *Server
	pb.UnimplementedDiscoveryServiceServer
}

func (d *discoveryServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		ServerName:             "TAC-Agent",
		ServerVersion:          config.Version,
		CertificateFingerprint: d.server.CertificateFingerprint(),
	}, nil
}

func (d *discoveryServer) GetServerInfo(ctx context.Context, req *pb.ServerInfoRequest) (*pb.ServerInfoResponse, error) {
	roles, _ := d.server.roleEngine.List()
	tools := d.server.toolEngine.List()

	return &pb.ServerInfoResponse{
		ServerName:    "TAC-Agent",
		ServerVersion: config.Version,
		RoleCount:     int32(len(roles)),
		ToolCount:     int32(len(tools)),
		ApiVersion:    "v1",
	}, nil
}

func (d *discoveryServer) ListRoles(ctx context.Context, req *pb.ListRolesRequest) (*pb.ListRolesResponse, error) {
	roles, err := d.server.roleEngine.List()
	if err != nil {
		return nil, err
	}

	var pbRoles []*pb.Role
	for _, r := range roles {
		pbRoles = append(pbRoles, &pb.Role{
			Name:        r.Name,
			Description: r.Description,
			Components:  r.Components,
			Tools:       r.Tools,
			Env:         r.Env,
			ApiType:     string(r.APIType),
			MessageMode: r.MessageMode,
			Model:       r.Model,
			CreatedAt:   r.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &pb.ListRolesResponse{Roles: pbRoles}, nil
}

func (d *discoveryServer) GetRole(ctx context.Context, req *pb.GetRoleRequest) (*pb.Role, error) {
	role, err := d.server.roleEngine.Get(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.Role{
		Name:        role.Name,
		Description: role.Description,
		Components:  role.Components,
		Tools:       role.Tools,
		Env:         role.Env,
		ApiType:     string(role.APIType),
		MessageMode: role.MessageMode,
		Model:       role.Model,
		CreatedAt:   role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (d *discoveryServer) GetRoleTools(ctx context.Context, req *pb.GetRoleToolsRequest) (*pb.GetRoleToolsResponse, error) {
	role, err := d.server.roleEngine.Get(req.RoleName)
	if err != nil {
		return nil, err
	}

	var pbTools []*pb.ToolInfo
	for _, toolName := range role.Tools {
		tool, err := d.server.toolEngine.GetDetail(toolName)
		if err != nil {
			continue
		}

		pbTools = append(pbTools, &pb.ToolInfo{
			Name:                tool.Name,
			Description:         tool.Config.Description,
			Language:            tool.Language,
			Version:             tool.Version,
			Dependencies:        tool.Dependencies,
			RequiresCompilation: tool.RequiresCompilation,
			IsBinary:            tool.IsBinary,
			SourceAvailable:     tool.SourceAvailable,
			RuntimeRequirement:  tool.RuntimeRequirement,
			Files:               tool.Scripts,
		})
	}

	return &pb.GetRoleToolsResponse{Tools: pbTools}, nil
}
