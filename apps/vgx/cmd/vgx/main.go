package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.2.0"

var rootCmd = &cobra.Command{
	Use:     "vgx",
	Short:   "VibeGuard — Security Property Graph oracle for AI coding agents",
	Version: version,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	// Phase 1 commands — stubs wired up, implementations land in subsequent PRs
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(ciCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
