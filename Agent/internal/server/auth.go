package server

import (
	"context"
	"crypto/subtle"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const authMetadataKey = "authorization"

type authInterceptor struct {
	server *Server
}

func newAuthInterceptor(s *Server) *authInterceptor {
	return &authInterceptor{server: s}
}

var publicMethods = map[string]bool{
	"/tac.DiscoveryService/Ping": true,
}

func (a *authInterceptor) validateToken(ctx context.Context) error {

	method, _ := grpc.Method(ctx)
	if publicMethods[method] {
		return nil
	}

	token := a.server.GetAuthToken()
	if token == "" {

		return nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	authHeader := md.Get(authMetadataKey)
	if len(authHeader) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization token")
	}

	parts := strings.SplitN(authHeader[0], " ", 2)
	var providedToken string
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		providedToken = parts[1]
	} else {
		providedToken = authHeader[0]
	}

	if subtle.ConstantTimeCompare([]byte(providedToken), []byte(token)) != 1 {
		return status.Error(codes.PermissionDenied, "invalid authorization token")
	}

	return nil
}

func (a *authInterceptor) unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if err := a.validateToken(ctx); err != nil {
		return nil, err
	}
	return handler(ctx, req)
}

func (a *authInterceptor) streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	if err := a.validateToken(ss.Context()); err != nil {
		return err
	}
	return handler(srv, ss)
}

func BuildAuthMetadata(token string) metadata.MD {
	return metadata.Pairs(authMetadataKey, "Bearer "+token)
}

func BuildInsecureMetadata() metadata.MD {
	return metadata.MD{}
}
