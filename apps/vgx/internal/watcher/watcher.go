// Package watcher monitors a repository for file changes and triggers
// incremental SPG updates via a callback.
package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vibeguard/vgx/internal/parser"
)

// ChangeEvent describes a file system event that should trigger an SPG update.
type ChangeEvent struct {
	Path      string
	Language  parser.Language
	IsDelete  bool
}

// Watcher wraps fsnotify with debouncing and extension filtering.
type Watcher struct {
	fw       *fsnotify.Watcher
	onChange func(ChangeEvent)
	debounce time.Duration
	pending  map[string]time.Time
}

// New creates a file watcher that calls onChange for every relevant file change.
// Debounce prevents rapid-fire updates on the same file (e.g. editor auto-save).
func New(onChange func(ChangeEvent)) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fw:       fw,
		onChange: onChange,
		debounce: 150 * time.Millisecond,
		pending:  make(map[string]time.Time),
	}, nil
}

// AddRepo recursively watches all directories under root.
func (w *Watcher) AddRepo(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if info.IsDir() {
			// Skip common non-source directories
			base := filepath.Base(path)
			if isIgnoredDir(base) {
				return filepath.SkipDir
			}
			return w.fw.Add(path)
		}
		return nil
	})
}

// Start begins processing fsnotify events. Blocks until Stop is called.
func (w *Watcher) Start() {
	ticker := time.NewTicker(w.debounce)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-w.fw.Events:
			if !ok {
				return
			}
			lang := parser.DetectLanguage(event.Name)
			if lang == parser.Unknown {
				continue // ignore non-source files
			}
			w.pending[event.Name] = time.Now()

		case err, ok := <-w.fw.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)

		case now := <-ticker.C:
			for path, ts := range w.pending {
				if now.Sub(ts) >= w.debounce {
					delete(w.pending, path)
					lang := parser.DetectLanguage(path)

					// Detect deletes
					_, statErr := os.Stat(path)
					w.onChange(ChangeEvent{
						Path:     path,
						Language: lang,
						IsDelete: statErr != nil,
					})
				}
			}
		}
	}
}

// Stop closes the underlying fsnotify watcher.
func (w *Watcher) Stop() error {
	return w.fw.Close()
}

func isIgnoredDir(name string) bool {
	ignored := []string{
		"node_modules", ".git", "__pycache__", ".venv", "venv", "env",
		"vendor", "dist", "build", ".next", ".turbo", "coverage",
		".pytest_cache", ".mypy_cache", ".ruff_cache",
	}
	for _, ig := range ignored {
		if strings.EqualFold(name, ig) {
			return true
		}
	}
	return false
}
