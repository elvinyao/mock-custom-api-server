package config

import (
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches config directories, including editors' atomic replacements.
type Watcher struct {
	configPath  string
	manager     *ConfigManager
	mu          sync.Mutex
	stopCh      chan struct{}
	doneCh      chan struct{}
	stopOnce    sync.Once
	started     bool
	intervalSec int
	logger      *log.Logger
	// OnReload runs after a valid candidate has been activated.
	OnReload func()
}

func NewWatcher(configPath string, manager *ConfigManager, logger *log.Logger) *Watcher {
	return &Watcher{configPath: configPath, manager: manager, stopCh: make(chan struct{}), doneCh: make(chan struct{}), logger: logger, intervalSec: 5}
}

// Start starts one watcher. intervalSec controls polling and failed-load retries.
func (w *Watcher) Start(intervalSec int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started || w.stopped() {
		return
	}
	if intervalSec > 0 {
		w.intervalSec = intervalSec
	}
	w.started = true
	go func() { defer close(w.doneCh); w.watchWithFsnotify() }()
}

// Stop is idempotent and waits for any in-flight reload to finish.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() { close(w.stopCh) })
	w.mu.Lock()
	started := w.started
	w.mu.Unlock()
	if started {
		<-w.doneCh
	}
}

func (w *Watcher) stopped() bool {
	select {
	case <-w.stopCh:
		return true
	default:
		return false
	}
}

func (w *Watcher) watchWithFsnotify() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.logger.Printf("[WARN] Failed to create fsnotify watcher, falling back to polling: %v", err)
		w.watchWithPolling(w.intervalSec)
		return
	}
	defer watcher.Close()
	watchedPaths := make(map[string]struct{})
	if err := w.addWatchPath(watcher, watchedPaths, w.configPath); err != nil {
		w.logger.Printf("[WARN] Failed to watch config directory, falling back to polling: %v", err)
		w.watchWithPolling(w.intervalSec)
		return
	}
	w.watchEndpointConfigFiles(watcher, watchedPaths, w.manager.GetConfig())
	w.logger.Printf("[INFO] Started watching configuration: %s", w.configPath)

	// Keep debounce handling on this goroutine; no timer callback can outlive Stop.
	var timer *time.Timer
	var timerCh <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case <-w.stopCh:
			return
		case <-timerCh:
			timerCh = nil
			if !w.reloadConfig(watcher, watchedPaths) && !w.stopped() {
				// A rejected candidate may refer to a new file or directory that
				// the active config does not watch yet. Retry until it becomes valid.
				timer.Reset(time.Duration(w.intervalSec) * time.Second)
				timerCh = timer.C
			}
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			path, err := filepath.Abs(event.Name)
			if err != nil {
				continue
			}
			if _, relevant := watchedPaths[path]; !relevant {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(500 * time.Millisecond)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(500 * time.Millisecond)
			}
			timerCh = timer.C
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			w.logger.Printf("[ERROR] Watcher error: %v", err)
		}
	}
}

func (w *Watcher) watchWithPolling(intervalSec int) {
	if intervalSec <= 0 {
		intervalSec = 5
	}
	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()
	w.logger.Printf("[INFO] Started polling config every %d seconds: %s", intervalSec, w.configPath)
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.reloadConfig(nil, nil)
		}
	}
}

func (w *Watcher) reloadConfig(watcher *fsnotify.Watcher, watchedPaths map[string]struct{}) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped() {
		return false
	}
	newCfg, err := LoadValidatedConfig(w.configPath)
	if err != nil {
		w.logger.Printf("[ERROR] Failed to reload config: %v (keeping old config)", err)
		return false
	}
	if w.stopped() {
		return false
	}
	w.manager.SetConfig(newCfg)
	if watcher != nil && watchedPaths != nil {
		clear(watchedPaths)
		if err := w.addWatchPath(watcher, watchedPaths, w.configPath); err != nil {
			w.logger.Printf("[WARN] Failed to watch config directory: %v", err)
		}
	}
	w.watchEndpointConfigFiles(watcher, watchedPaths, newCfg)
	if w.OnReload != nil {
		w.OnReload()
	}
	w.logger.Printf("[INFO] Configuration reloaded successfully at %s", time.Now().Format(time.RFC3339))
	return true
}

func (w *Watcher) watchEndpointConfigFiles(watcher *fsnotify.Watcher, watchedPaths map[string]struct{}, cfg *Config) {
	if watcher == nil || watchedPaths == nil || cfg == nil {
		return
	}
	for _, path := range cfg.EndpointConfigPaths {
		if err := w.addWatchPath(watcher, watchedPaths, path); err != nil {
			w.logger.Printf("[WARN] Failed to watch endpoint config %q: %v", path, err)
		}
	}
}

func (w *Watcher) addWatchPath(watcher *fsnotify.Watcher, watchedPaths map[string]struct{}, path string) error {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, exists := watchedPaths[cleanPath]; exists {
		return nil
	}
	if err := watcher.Add(filepath.Dir(cleanPath)); err != nil {
		return err
	}
	watchedPaths[cleanPath] = struct{}{}
	return nil
}
