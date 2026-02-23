// Package daemon orchestrates the full VibeGuard SPG lifecycle:
//   1. Initial graph build (vgx init)
//   2. File watching + incremental updates
//   3. MCP server serving security queries
package daemon

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/vibeguard/vgx/internal/graph"
	mcpsrv "github.com/vibeguard/vgx/internal/mcp"
	"github.com/vibeguard/vgx/internal/parser"
	"github.com/vibeguard/vgx/internal/watcher"
)

// Config holds runtime configuration for the SPG daemon.
type Config struct {
	RepoPath    string // absolute path to the repository root
	StorePath   string // path to persist the SPG on disk
	MCPMode     bool   // run MCP stdio server
	WatchMode   bool   // watch for file changes
	Verbose     bool
}

// DefaultStorePath returns ~/.vibeguard/graph/<repo-hash>/
func DefaultStorePath(repoPath string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	// Use a hash of the abs path to namespace per-repo graphs
	absPath, _ := filepath.Abs(repoPath)
	sum := fmt.Sprintf("%x", hashBytes([]byte(absPath)))[:12]
	return filepath.Join(home, ".vibeguard", "graph", sum)
}

// Init builds the initial Security Property Graph for a repository.
// This is a full-repo scan — runs in the foreground and reports progress.
func Init(cfg Config) error {
	repoPath, err := filepath.Abs(cfg.RepoPath)
	if err != nil {
		return fmt.Errorf("resolving repo path: %w", err)
	}

	storePath := cfg.StorePath
	if storePath == "" {
		storePath = DefaultStorePath(repoPath)
	}

	log.Printf("[vgx] Initializing SPG for %s", repoPath)
	log.Printf("[vgx] Graph store: %s", storePath)

	g, err := graph.Open(storePath, repoPath)
	if err != nil {
		return fmt.Errorf("opening graph store: %w", err)
	}
	defer g.Close()

	start := time.Now()
	fileCount, nodeCount, edgeCount := 0, 0, 0

	err = walkRepo(repoPath, func(filePath string) error {
		pf, parseErr := parser.ParseFile(filePath)
		if parseErr != nil {
			log.Printf("[vgx] parse error %s: %v", filePath, parseErr)
			return nil // non-fatal
		}
		if pf == nil {
			return nil
		}

		prevNodes := len(g.AllNodes())
		if buildErr := graph.BuildFromFile(g, pf); buildErr != nil {
			log.Printf("[vgx] build error %s: %v", filePath, buildErr)
			return nil
		}

		fileCount++
		delta := len(g.AllNodes()) - prevNodes
		if delta > 0 {
			nodeCount += delta
			if cfg.Verbose {
				log.Printf("[vgx]   %s → +%d nodes", filePath, delta)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Count edges
	for _, n := range g.AllNodes() {
		edgeCount += len(g.OutEdges(n.ID))
	}

	stats := g.Stats()
	elapsed := time.Since(start)

	fmt.Printf("\n  VibeGuard SPG initialized\n")
	fmt.Printf("  ═══════════════════════════\n\n")
	fmt.Printf("  Repository:  %s\n", repoPath)
	fmt.Printf("  Files:       %d analyzed\n", fileCount)
	fmt.Printf("  Nodes:       %d security nodes\n", nodeCount)
	fmt.Printf("    Sources:   %d\n", stats["SourceNode"])
	fmt.Printf("    Sinks:     %d\n", stats["SinkNode"])
	fmt.Printf("    Sanitizers:%d\n", stats["SanitizerNode"])
	fmt.Printf("  Edges:       %d data-flow edges\n", edgeCount)
	fmt.Printf("  Time:        %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  Graph:       %s\n\n", storePath)
	fmt.Printf("  Next steps:\n")
	fmt.Printf("    vgx serve          — start MCP server for Cursor / Claude Code\n")
	fmt.Printf("    vgx report         — view security posture\n")
	fmt.Printf("    vgx query taint-paths sqli  — query for SQL injection paths\n\n")

	return nil
}

// Serve starts the SPG daemon: file watcher + MCP server.
// Blocks until interrupted.
func Serve(cfg Config) error {
	repoPath, err := filepath.Abs(cfg.RepoPath)
	if err != nil {
		return fmt.Errorf("resolving repo path: %w", err)
	}

	storePath := cfg.StorePath
	if storePath == "" {
		storePath = DefaultStorePath(repoPath)
	}

	g, err := graph.Open(storePath, repoPath)
	if err != nil {
		return fmt.Errorf("opening graph store: %w", err)
	}
	defer g.Close()

	stats := g.Stats()
	log.Printf("[vgx] SPG loaded: %d nodes (%d sources, %d sinks, %d sanitizers)",
		stats["total_nodes"], stats["SourceNode"], stats["SinkNode"], stats["SanitizerNode"])

	if stats["total_nodes"] == 0 {
		log.Printf("[vgx] WARNING: graph is empty — run 'vgx init' first to build the SPG")
	}

	// Start file watcher for incremental updates
	if cfg.WatchMode {
		w, watchErr := watcher.New(func(ev watcher.ChangeEvent) {
			handleFileChange(g, repoPath, ev)
		})
		if watchErr != nil {
			return fmt.Errorf("starting watcher: %w", watchErr)
		}
		if addErr := w.AddRepo(repoPath); addErr != nil {
			return fmt.Errorf("watching repo: %w", addErr)
		}
		go w.Start()
		log.Printf("[vgx] File watcher active on %s", repoPath)
	}

	if cfg.MCPMode {
		srv := mcpsrv.New(g)
		log.Printf("[vgx] MCP server starting on stdio")
		return srv.ServeStdio()
	}

	// If not MCP mode, just block (team sync API is handled by the HTTP server)
	log.Printf("[vgx] SPG daemon running (no MCP mode — use --mcp for Cursor/Claude Code)")
	select {}
}

// handleFileChange re-parses a changed file and updates the SPG incrementally.
func handleFileChange(g *graph.SPG, repoPath string, ev watcher.ChangeEvent) {
	start := time.Now()

	if ev.IsDelete {
		if err := g.RemoveFile(ev.Path); err != nil {
			log.Printf("[vgx] error removing %s from SPG: %v", ev.Path, err)
		}
		log.Printf("[vgx] SPG updated: removed %s (%s)", ev.Path, time.Since(start).Round(time.Millisecond))
		return
	}

	pf, err := parser.ParseFile(ev.Path)
	if err != nil {
		log.Printf("[vgx] parse error %s: %v", ev.Path, err)
		return
	}
	if pf == nil {
		return
	}

	prevCount := len(g.AllNodes())
	if err := graph.BuildFromFile(g, pf); err != nil {
		log.Printf("[vgx] build error %s: %v", ev.Path, err)
		return
	}

	newCount := len(g.AllNodes())
	delta := newCount - prevCount
	elapsed := time.Since(start)

	log.Printf("[vgx] SPG updated: %s (Δ%+d nodes, %s)", ev.Path, delta, elapsed.Round(time.Millisecond))
}

// walkRepo walks all source files in a repository, skipping ignored directories.
func walkRepo(root string, fn func(path string) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if isIgnoredDir(filepath.Base(path)) {
				return filepath.SkipDir
			}
			return nil
		}
		if parser.DetectLanguage(path) == parser.Unknown {
			return nil
		}
		return fn(path)
	})
}

func isIgnoredDir(name string) bool {
	switch name {
	case "node_modules", ".git", "__pycache__", ".venv", "venv", "env",
		"vendor", "dist", "build", ".next", ".turbo", "coverage",
		".pytest_cache", ".mypy_cache", ".ruff_cache", "testdata":
		return true
	}
	return false
}

func hashBytes(b []byte) uint64 {
	var h uint64 = 14695981039346656037
	for _, c := range b {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}
