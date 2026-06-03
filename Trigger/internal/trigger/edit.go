package trigger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
)

type editRunner struct {
	trigger  *model.Trigger
	basePath string
	mu       sync.RWMutex
	running  bool
	watcher  *fsnotify.Watcher
	stopCh   chan struct{}
}

func newEditRunner(t *model.Trigger, basePath string) Runner {
	return &editRunner{trigger: t, basePath: basePath}
}

func (r *editRunner) Start(events []*model.Event, executor *Executor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	watchPath := r.trigger.WatchPath
	if !filepath.IsAbs(watchPath) {
		watchPath = filepath.Join(r.basePath, watchPath)
	}

	info, err := os.Stat(watchPath)
	if err != nil {
		watcher.Close()
		return fmt.Errorf("stat watch path: %w", err)
	}

	if !info.IsDir() {
		if err := watcher.Add(watchPath); err != nil {
			watcher.Close()
			return fmt.Errorf("watch file: %w", err)
		}
	} else {

		if err := r.addDirRecursive(watcher, watchPath); err != nil {
			watcher.Close()
			return fmt.Errorf("watch directory: %w", err)
		}
	}

	r.watcher = watcher
	r.running = true
	r.stopCh = make(chan struct{})

	go r.run(events, executor)
	return nil
}

func (r *editRunner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}
	r.running = false

	close(r.stopCh)
	if r.watcher != nil {
		r.watcher.Close()
	}
	return nil
}

func (r *editRunner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

func (r *editRunner) addDirRecursive(watcher *fsnotify.Watcher, path string) error {

	if filepath.Base(path) == ".tac" {
		return nil
	}

	if r.isBlacklisted(path) {
		return nil
	}

	if err := watcher.Add(path); err != nil {
		return err
	}

	if !r.trigger.Recursive {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subPath := filepath.Join(path, entry.Name())
		if err := r.addDirRecursive(watcher, subPath); err != nil {
			return err
		}
	}

	return nil
}

func (r *editRunner) isBlacklisted(path string) bool {
	for _, pattern := range r.trigger.Blacklist {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}

		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (r *editRunner) run(events []*model.Event, executor *Executor) {
	for {
		select {
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}

			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				if r.shouldTrigger(event.Name) {
					r.executeEvents(events, executor, event.Name)
				}
			}

			if event.Op&fsnotify.Create == fsnotify.Create && r.trigger.Recursive {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					r.addDirRecursive(r.watcher, event.Name)
				}
			}

		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("[Edit %s] Watcher error: %v\n", r.trigger.ID, err)

		case <-r.stopCh:
			return
		}
	}
}

func (r *editRunner) shouldTrigger(path string) bool {

	if strings.Contains(path, string(filepath.Separator)+".tac") {
		return false
	}

	if r.isBlacklisted(path) {
		return false
	}

	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return false
	}
	return true
}

func (r *editRunner) executeEvents(events []*model.Event, executor *Executor, filePath string) {

	copies := make([]*model.Event, len(events))
	for i, ev := range events {
		env := make(map[string]string, len(ev.Env))
		for k, v := range ev.Env {
			env[k] = v
		}
		env["TRIGGER_FILE"] = filePath
		evCopy := *ev
		evCopy.Env = env
		copies[i] = &evCopy
	}
	executor.ExecuteConcurrently(copies, r.trigger.ID)
}
