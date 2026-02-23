package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/vibeguard/vgx/internal/api"
	"github.com/vibeguard/vgx/internal/config"
	"github.com/vibeguard/vgx/internal/daemon"
	"github.com/vibeguard/vgx/internal/db"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the VibeGuard SPG daemon: file watcher + MCP server",
	Long: `Start the VibeGuard Security Property Graph daemon.

Default mode: watches the repository for file changes and serves the SPG to AI
coding agents (Cursor, Claude Code) via MCP on stdio. No network. Code stays local.

Add --api to also start the team sync REST API on --port (Pro/Team tier).`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().IntP("port", "p", 8080, "Port for the team sync REST API")
	serveCmd.Flags().BoolP("mcp", "m", true, "Serve MCP on stdio (default: true)")
	serveCmd.Flags().BoolP("api", "a", false, "Also run team sync REST API on --port")
	serveCmd.Flags().StringP("repo", "r", ".", "Repository path to watch and serve")
	serveCmd.Flags().BoolP("watch", "w", true, "Watch for file changes and update SPG incrementally")
}

func runServe(cmd *cobra.Command, _ []string) error {
	port, _ := cmd.Flags().GetInt("port")
	mcpMode, _ := cmd.Flags().GetBool("mcp")
	apiMode, _ := cmd.Flags().GetBool("api")
	repoPath, _ := cmd.Flags().GetString("repo")
	watchMode, _ := cmd.Flags().GetBool("watch")

	cfg := config.Load()

	storePath := cfg.SPGStorePath
	if storePath == "" {
		storePath = daemon.DefaultStorePath(repoPath)
	}

	// Team sync REST API runs in a goroutine if requested
	if apiMode {
		go func() {
			ctx := context.Background()
			pool, err := db.NewPool(ctx, cfg.DatabaseURL)
			if err != nil {
				log.Printf("[vgx] WARNING: database connection failed (%v) — team sync API disabled", err)
				return
			}
			router := api.NewRouter(cfg, pool)
			addr := fmt.Sprintf(":%d", port)
			log.Printf("[vgx] Team sync API listening on %s", addr)
			if err := http.ListenAndServe(addr, router); err != nil {
				log.Printf("[vgx] Team sync API error: %v", err)
			}
		}()
	}

	return daemon.Serve(daemon.Config{
		RepoPath:  repoPath,
		StorePath: storePath,
		MCPMode:   mcpMode,
		WatchMode: watchMode,
	})
}
