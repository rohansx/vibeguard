package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/vibeguard/vgx/internal/api"
	"github.com/vibeguard/vgx/internal/config"
	"github.com/vibeguard/vgx/internal/db"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the VGX daemon: file watcher + MCP server + team sync API",
	Long: `Start the VibeGuard SPG daemon.

Watches the repository for file changes, maintains the Security Property Graph
incrementally, and exposes it to AI coding agents via an MCP server on stdio.

The optional --api flag also starts the team sync REST API for Pro/Team tiers.`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().IntP("port", "p", 8080, "Port for the team sync REST API")
	serveCmd.Flags().BoolP("mcp", "m", false, "Run MCP server on stdio (for Cursor / Claude Code)")
	serveCmd.Flags().BoolP("api", "a", false, "Run team sync REST API alongside MCP")
	serveCmd.Flags().StringP("repo", "r", ".", "Path to the repository to watch")
}

func runServe(cmd *cobra.Command, _ []string) error {
	port, _ := cmd.Flags().GetInt("port")
	mcpMode, _ := cmd.Flags().GetBool("mcp")
	apiMode, _ := cmd.Flags().GetBool("api")
	_ = mcpMode // MCP server implementation lands in Phase 1

	cfg := config.Load()

	if apiMode {
		ctx := context.Background()
		pool, err := db.NewPool(ctx, cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("database connection failed: %w", err)
		}
		defer pool.Close()

		router := api.NewRouter(cfg, pool)
		addr := fmt.Sprintf(":%d", port)
		log.Printf("VGX team sync API starting on %s", addr)
		return http.ListenAndServe(addr, router)
	}

	// Default: MCP daemon mode (no network, stdio only)
	// TODO(phase1): start file watcher + SPG daemon + MCP stdio server
	log.Println("VGX SPG daemon starting — MCP server implementation coming in Phase 1")
	log.Println("Run with --api to start the team sync REST API")
	select {} // block until interrupted
}
