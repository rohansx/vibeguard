package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vibeguard/vgx/internal/config"
	"github.com/vibeguard/vgx/internal/daemon"
	"github.com/vibeguard/vgx/internal/graph"
	"github.com/vibeguard/vgx/internal/taint"
)

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "CI/CD mode: exit non-zero if unsanitized taint paths exist at or above --min-severity",
	Long: `Run VibeGuard in CI/CD mode for pull request security gating.

Builds the SPG for the current working tree and checks for unsanitized taint
paths at or above the given severity threshold. Exits non-zero if any are found.

Exit codes:
  0 — no taint paths at or above --min-severity
  1 — taint paths found (build should fail)
  2 — SPG error (e.g. not initialized)

Example GitHub Actions step:
  - name: VibeGuard security check
    run: |
      vgx init .
      vgx ci --min-severity high --output json`,
	RunE: runCI,
}

func init() {
	ciCmd.Flags().String("min-severity", "high", "Minimum severity to fail on: critical, high, medium, low")
	ciCmd.Flags().StringP("repo", "r", ".", "Repository path")
	ciCmd.Flags().StringP("store", "s", "", "Override SPG store path")
	ciCmd.Flags().StringP("output", "o", "table", "Output format: table or json")
}

var severityRank = map[string]int{
	"critical": 0,
	"high":     1,
	"medium":   2,
	"low":      3,
}

func runCI(cmd *cobra.Command, _ []string) error {
	minSev, _ := cmd.Flags().GetString("min-severity")
	repoPath, _ := cmd.Flags().GetString("repo")
	storeOverride, _ := cmd.Flags().GetString("store")
	outputFmt, _ := cmd.Flags().GetString("output")

	threshold, ok := severityRank[minSev]
	if !ok {
		return fmt.Errorf("invalid severity %q — use critical, high, medium, or low", minSev)
	}

	cfg := config.Load()
	storePath := cfg.SPGStorePath
	if storeOverride != "" {
		storePath = storeOverride
	}
	if storePath == "" {
		storePath = daemon.DefaultStorePath(repoPath)
	}

	g, err := graph.Open(storePath, repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v (did you run 'vgx init'?)\n", err)
		os.Exit(2)
	}
	defer g.Close()

	if g.Stats()["total_nodes"] == 0 {
		fmt.Fprintln(os.Stderr, "SPG is empty — run 'vgx init' first")
		os.Exit(2)
	}

	eng := taint.New(g)
	allPaths := eng.FindAllPaths()

	// Filter paths at or above threshold
	var failing []graph.TaintPath
	for _, tp := range allPaths {
		if severityRank[tp.Severity] <= threshold {
			failing = append(failing, tp)
		}
	}

	if outputFmt == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(map[string]any{
			"min_severity":  minSev,
			"total_paths":   len(allPaths),
			"failing_paths": len(failing),
			"passed":        len(failing) == 0,
			"paths":         formatPaths(failing),
		})
	} else {
		fmt.Printf("\n  VibeGuard CI Security Gate\n")
		fmt.Printf("  ══════════════════════════\n\n")
		fmt.Printf("  Threshold:   %s and above\n", minSev)
		fmt.Printf("  Total paths: %d\n", len(allPaths))
		fmt.Printf("  Failing:     %d\n\n", len(failing))

		if len(failing) > 0 {
			for i, tp := range failing {
				fmt.Printf("  [%d] %s %s\n", i+1, tp.Severity, tp.VulnClass)
				fmt.Printf("      %s (%s:%d) → %s (%s:%d)\n\n",
					tp.Source.Symbol, tp.Source.FilePath, tp.Source.LineStart,
					tp.Sink.Symbol, tp.Sink.FilePath, tp.Sink.LineStart)
			}
		}
	}

	if len(failing) > 0 {
		if outputFmt != "json" {
			fmt.Printf("  FAILED: %d unsanitized taint path(s) at %s severity or above\n\n", len(failing), minSev)
		}
		os.Exit(1)
	}

	if outputFmt != "json" {
		fmt.Printf("  PASSED: no unsanitized taint paths at %s severity or above\n\n", minSev)
	}
	return nil
}
