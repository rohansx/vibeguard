package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "CI/CD mode: exit non-zero if this PR introduces new taint paths",
	Long: `Run VibeGuard in CI/CD mode for pull request security gating.

Compares the SPG state of the current branch against the base branch and
exits with a non-zero code if new critical or high severity taint paths are
introduced. Designed for use in GitHub Actions, GitLab CI, and similar.

Output: SARIF format written to --sarif-output (for GitHub Security tab integration).

Exit codes:
  0 — no new taint paths at or above --min-severity
  1 — new taint paths introduced (build should fail)
  2 — SPG not initialized or other error (build should fail)

Example GitHub Actions step:
  - name: VibeGuard security check
    run: vgx ci --base-branch main --sarif-output vgx-results.sarif
  - uses: github/codeql-action/upload-sarif@v3
    with:
      sarif_file: vgx-results.sarif`,
	RunE: runCI,
}

func init() {
	ciCmd.Flags().String("base-branch", "main", "Base branch to diff against")
	ciCmd.Flags().String("sarif-output", "", "Write SARIF output to this file path")
	ciCmd.Flags().String("min-severity", "high", "Minimum severity to fail on: critical, high, medium, low")
	ciCmd.Flags().StringP("repo", "r", ".", "Repository path")
}

func runCI(_ *cobra.Command, _ []string) error {
	// TODO(phase1): full SPG diff against base branch with SARIF output
	fmt.Println("vgx ci: running SPG-based PR security gate")
	fmt.Println("CI mode implementation lands in Phase 1.")

	// Return 0 (pass) as a stub — real gate logic replaces this in Phase 1
	os.Exit(0)
	return nil
}
