package server

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/Lyoomu/TAC/Agent/internal/agent"
	"github.com/Lyoomu/TAC/Agent/internal/config"
	"github.com/Lyoomu/TAC/Agent/internal/role"
	"github.com/Lyoomu/TAC/Agent/internal/tool"
	pb "github.com/Lyoomu/TAC/proto"
)

type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	config     *config.Config
	mu         sync.RWMutex

	roleEngine *role.Engine
	toolEngine *tool.Engine
	agentMgr   *agent.Manager

	authToken              string
	tlsConfig              *tls.Config
	certificateFingerprint string
}

func New(cfg *config.Config, roleEngine *role.Engine, toolEngine *tool.Engine, agentMgr *agent.Manager) *Server {
	return &Server{
		config:     cfg,
		roleEngine: roleEngine,
		toolEngine: toolEngine,
		agentMgr:   agentMgr,
	}
}

func (s *Server) SetAuthToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authToken = token
}

func (s *Server) GetAuthToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.authToken
}

func (s *Server) CertificateFingerprint() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.certificateFingerprint
}

func defaultCertPath() (certPath, keyPath string) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	dir := filepath.Join(home, ".tac")
	return filepath.Join(dir, "server.crt"), filepath.Join(dir, "server.key")
}

func (s *Server) SetupTLS(certFile, keyFile string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var cert tls.Certificate
	var err error

	if certFile != "" && keyFile != "" {

		cert, err = tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("load tls certificate: %w", err)
		}
	} else {

		defaultCert, defaultKey := defaultCertPath()
		if _, err := os.Stat(defaultCert); err == nil {
			if _, err := os.Stat(defaultKey); err == nil {
				cert, err = tls.LoadX509KeyPair(defaultCert, defaultKey)
				if err != nil {
					return fmt.Errorf("load default tls certificate: %w", err)
				}
				fmt.Printf("loaded existing certificate from %s\n", defaultCert)
			}
		}

		if cert.Certificate == nil {
			cert, err = generateSelfSignedCert()
			if err != nil {
				return fmt.Errorf("generate self-signed certificate: %w", err)
			}

			if err := saveSelfSignedCert(cert, defaultCert, defaultKey); err != nil {
				fmt.Printf("warning: failed to save certificate: %v\n", err)
			} else {
				fmt.Printf("saved new certificate to %s\n", defaultCert)
			}
		}
	}

	certDER := cert.Certificate[0]
	hash := sha256.Sum256(certDER)
	s.certificateFingerprint = hex.EncodeToString(hash[:])

	s.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	}

	return nil
}

func (s *Server) Start(addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	s.listener = lis

	var opts []grpc.ServerOption

	authInterceptor := newAuthInterceptor(s)
	opts = append(opts, grpc.UnaryInterceptor(authInterceptor.unaryInterceptor))
	opts = append(opts, grpc.StreamInterceptor(authInterceptor.streamInterceptor))

	if s.tlsConfig != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(s.tlsConfig)))
	}

	s.grpcServer = grpc.NewServer(opts...)

	pb.RegisterDiscoveryServiceServer(s.grpcServer, &discoveryServer{server: s})
	pb.RegisterChatServiceServer(s.grpcServer, &chatServer{server: s})
	pb.RegisterToolServiceServer(s.grpcServer, &toolServer{server: s})

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			fmt.Printf("gRPC server error: %v\n", err)
		}
	}()

	return nil
}

func (s *Server) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
}

func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}
