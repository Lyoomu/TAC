package trigger

import (
	"sync"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
)

type directRunner struct {
	trigger *model.Trigger
	mu      sync.RWMutex
	running bool
}

func newDirectRunner(t *model.Trigger) Runner {
	return &directRunner{trigger: t}
}

func (r *directRunner) Start(events []*model.Event, executor *Executor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return nil
	}
	r.running = true
	return nil
}

func (r *directRunner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.running = false
	return nil
}

func (r *directRunner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

func (r *directRunner) RunDirectly(executor *Executor) []error {

	return nil
}
