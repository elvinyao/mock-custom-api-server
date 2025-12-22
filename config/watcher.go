package config

import (
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches config file for changes and reloads configuration
type Watcher struct {
	configPath string
	manager    *ConfigManager
	mu         sync.RWMutex
	stopCh     chan struct{}
	logger     *log.Logger
}

// NewWatcher creates a new config watcher
func NewWatcher(configPath string, manager *ConfigManager, logger *log.Logger) *Watcher {
	return &Watcher{
		configPath: configPath,
		manager:    manager,
		stopCh:     make(chan struct{}),
		logger:     logger,
	}
}

// Start starts watching the config file for changes
func (w *Watcher) Start(intervalSec int) {
	go w.watchWithFsnotify()
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	close(w.stopCh)
}

// watchWithFsnotify uses fsnotify to watch for file changes
func (w *Watcher) watchWithFsnotify() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.logger.Printf("[WARN] Failed to create fsnotify watcher, falling back to polling: %v", err)
		w.watchWithPolling(5) // fallback to polling
		return
	}
	defer watcher.Close()

	if err := watcher.Add(w.configPath); err != nil {
		w.logger.Printf("[WARN] Failed to watch config file, falling back to polling: %v", err)
		w.watchWithPolling(5) // fallback to polling
		return
	}

	w.logger.Printf("[INFO] Started watching config file: %s", w.configPath)

	// Debounce timer to avoid rapid reloads
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case <-w.stopCh:
			w.logger.Println("[INFO] Config watcher stopped")
			return

		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Only reload on write or create events
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// Debounce: cancel previous timer and set a new one
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					w.reloadConfig()
				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			w.logger.Printf("[ERROR] Watcher error: %v", err)
		}
	}
}

// watchWithPolling polls for file changes at regular intervals
func (w *Watcher) watchWithPolling(intervalSec int) {
	ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer ticker.Stop()

	w.logger.Printf("[INFO] Started polling config file every %d seconds: %s", intervalSec, w.configPath)

	for {
		select {
		case <-w.stopCh:
			w.logger.Println("[INFO] Config watcher stopped")
			return
		case <-ticker.C:
			w.reloadConfig()
		}
	}
}

// reloadConfig reloads the configuration from file
func (w *Watcher) reloadConfig() {
	w.mu.Lock()
	defer w.mu.Unlock()

	newCfg, err := LoadConfig(w.configPath)
	if err != nil {
		w.logger.Printf("[ERROR] Failed to reload config: %v (keeping old config)", err)
		return
	}

	// Validate new config
	warnings := ValidateConfig(newCfg)
	for _, warn := range warnings {
		w.logger.Printf("[WARN] Config validation: %s", warn)
	}

	// Update config
	w.manager.SetConfig(newCfg)
	w.logger.Printf("[INFO] Configuration reloaded successfully at %s", time.Now().Format(time.RFC3339))
}
