package trigger

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
)

type periodicRunner struct {
	trigger *model.Trigger
	mu      sync.RWMutex
	running bool
	stopCh  chan struct{}
	timer   *time.Timer
	cron    *cron.Cron
	cronID  cron.EntryID
}

func newPeriodicRunner(t *model.Trigger) Runner {
	return &periodicRunner{trigger: t}
}

func (r *periodicRunner) Start(events []*model.Event, executor *Executor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return nil
	}
	r.running = true
	r.stopCh = make(chan struct{})

	if r.trigger.CronExpr != "" {

		r.cron = cron.New()
		id, err := r.cron.AddFunc(r.trigger.CronExpr, func() {
			r.executeEvents(events, executor)
		})
		if err != nil {
			r.running = false
			return fmt.Errorf("invalid cron expression: %w", err)
		}
		r.cronID = id
		r.cron.Start()
	} else if r.trigger.Interval != "" {

		interval, err := time.ParseDuration(r.trigger.Interval)
		if err != nil {
			r.running = false
			return fmt.Errorf("invalid interval: %w", err)
		}

		var firstDelay time.Duration
		if r.trigger.FirstRunTime != nil {
			firstDelay = time.Until(*r.trigger.FirstRunTime)
			if firstDelay < 0 {
				firstDelay = 0
			}
		}

		go r.runInterval(interval, firstDelay, events, executor)
	} else {
		r.running = false
		return fmt.Errorf("no interval or cron expression set")
	}

	return nil
}

func (r *periodicRunner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}
	r.running = false

	close(r.stopCh)

	if r.cron != nil {
		r.cron.Stop()
	}
	if r.timer != nil {
		r.timer.Stop()
	}

	return nil
}

func (r *periodicRunner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

func (r *periodicRunner) runInterval(interval, firstDelay time.Duration, events []*model.Event, executor *Executor) {

	if firstDelay > 0 {
		select {
		case <-time.After(firstDelay):
			r.executeEvents(events, executor)
		case <-r.stopCh:
			return
		}
	} else {
		r.executeEvents(events, executor)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.executeEvents(events, executor)
		case <-r.stopCh:
			return
		}
	}
}

func (r *periodicRunner) executeEvents(events []*model.Event, executor *Executor) {
	executor.ExecuteConcurrently(events, r.trigger.ID)
}
