package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Build the initial Security Property Graph for a repository",
	Long: `Initialize VibeGuard for a repository by building its Security Property Graph.

Runs tree-sitter parsing across all supported source files, classifies security
nodes (sources, sinks, sanitizers, trust boundaries), and persists the SPG to
~/.vibeguard/graph/<repo-hash>/ for incremental updates.

Initial build runs in background — use 'vgx report' to check progress.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringSliceP("languages", "l", []string{}, "Languages to analyze (python, typescript, go). Auto-detected if empty.")
	initCmd.Flags().BoolP("background", "b", true, "Run initial graph build in background")
	initCmd.Flags().StringP("store", "s", "", "Override SPG store path (default: ~/.vibeguard/graph)")
}

func runInit(_ *cobra.Command, args []string) error {
	repoPath := "."
	if len(args) > 0 {
		repoPath = args[0]
	}

	// TODO(phase1): implement tree-sitter parsing + SPG construction
	fmt.Printf("vgx init: building Security Property Graph for %s\n", repoPath)
	fmt.Println("  → Auto-detecting languages and frameworks...")
	fmt.Println("  → Initial graph build will run in background (5–15 min for medium repos)")
	fmt.Println("  → Run 'vgx report' to check progress")
	fmt.Println("  → Run 'vgx serve' to start the MCP server once build completes")
	fmt.Println()
	fmt.Println("SPG daemon implementation lands in Phase 1.")
	return nil
}
