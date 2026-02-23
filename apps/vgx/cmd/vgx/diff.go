package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff [base-ref]",
	Short: "Show security property changes introduced since a git ref",
	Long: `Diff the Security Property Graph between the current working tree and a git ref.

Shows:
  • New taint paths introduced (potential vulnerabilities added)
  • Taint paths resolved (potential vulnerabilities fixed)
  • Trust boundary violations added or removed
  • Auth coverage changes (endpoints gaining or losing auth checks)
  • Net security posture delta: improved / degraded / unchanged

Examples:
  vgx diff HEAD~1          — changes since last commit
  vgx diff main            — changes since main branch
  vgx diff v1.2.0          — changes since a tagged release`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().StringP("output", "o", "table", "Output format: table, json, or sarif")
	diffCmd.Flags().StringP("repo", "r", ".", "Repository path")
	diffCmd.Flags().BoolP("exit-code", "e", false, "Exit non-zero if new critical/high paths are introduced")
}

func runDiff(_ *cobra.Command, args []string) error {
	ref := "HEAD~1"
	if len(args) > 0 {
		ref = args[0]
	}

	// TODO(phase1): compare SPG snapshots between refs using git-aware graph reconciliation
	fmt.Printf("vgx diff %s: computing SPG delta\n", ref)
	fmt.Println("SPG diff engine implementation lands in Phase 1.")
	return nil
}
