package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query <tool> [args...]",
	Short: "Run a security query against the live Security Property Graph",
	Long: `Execute a deterministic security query against the SPG.

Available query tools (map directly to MCP tools):
  taint-paths [sink]         — unsanitized paths to a specific sink
  attack-surface [module]    — all source nodes and reachable sinks for a module
  missing-sanitizers         — all unprotected source-to-sink paths, ranked by severity
  trace [variable] [file]    — full data flow provenance for a variable
  blast-radius [function]    — security properties affected if this function changes
  trust-violations           — code paths crossing trust boundaries without auth checks
  auth-coverage [endpoint]   — whether an endpoint enforces auth before sensitive ops
  security-context [file]    — full security posture of a file

All queries return deterministic JSON. Same code + same query = same result always.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runQuery,
}

func init() {
	queryCmd.Flags().StringP("output", "o", "json", "Output format: json or table")
	queryCmd.Flags().StringP("repo", "r", ".", "Repository path (uses current dir SPG by default)")
}

func runQuery(_ *cobra.Command, args []string) error {
	tool := args[0]

	// TODO(phase1): proxy to running SPG daemon via unix socket
	fmt.Printf("vgx query %s: querying Security Property Graph\n", tool)
	fmt.Println("SPG query engine implementation lands in Phase 1.")
	return nil
}
