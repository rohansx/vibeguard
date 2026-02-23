package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vibeguard/vgx/internal/config"
	"github.com/vibeguard/vgx/internal/daemon"
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Build the initial Security Property Graph for a repository",
	Long: `Initialize VibeGuard for a repository by building its Security Property Graph.

Walks all source files (Python, TypeScript, JavaScript, Go), classifies security
nodes (sources, sinks, sanitizers), builds data-flow edges, and persists the SPG
to disk at ~/.vibeguard/graph/<repo-hash>/

Subsequent updates via 'vgx serve' are incremental (sub-500ms per file change).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolP("verbose", "v", false, "Print per-file progress")
	initCmd.Flags().StringP("store", "s", "", "Override SPG store path (default: ~/.vibeguard/graph/<hash>)")
}

func runInit(cmd *cobra.Command, args []string) error {
	repoPath := "."
	if len(args) > 0 {
		repoPath = args[0]
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	storeOverride, _ := cmd.Flags().GetString("store")

	cfg := config.Load()
	storePath := cfg.SPGStorePath
	if storeOverride != "" {
		storePath = storeOverride
	}
	if storePath == "" {
		storePath = daemon.DefaultStorePath(repoPath)
	}

	fmt.Printf("\n  VibeGuard — Building Security Property Graph\n")
	fmt.Printf("  ─────────────────────────────────────────────\n\n")

	return daemon.Init(daemon.Config{
		RepoPath:  repoPath,
		StorePath: storePath,
		Verbose:   verbose,
	})
}
