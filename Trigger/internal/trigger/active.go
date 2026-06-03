package trigger

import (
	"sync"
	"time"

	pb "github.com/Lyoomu/TAC/proto"
)

type ActiveSession struct {
	SessionID   string
	ServerName  string
	RoleName    string
	TriggerID   string
	EventID     string
	StartTime   time.Time
	Subscribers map[string]chan *pb.ChatMessage
	History     []*pb.ChatMessage
	SubMu       sync.Mutex
}

func (s *ActiveSession) Broadcast(msg *pb.ChatMessage) {
	s.SubMu.Lock()
	defer s.SubMu.Unlock()
	s.History = append(s.History, msg)
	for _, ch := range s.Subscribers {
		select {
		case ch <- msg:
		default:
		}
	}
}
