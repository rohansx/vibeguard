package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vibeguard/vgx/internal/config"
	"github.com/vibeguard/vgx/internal/db"
	"github.com/vibeguard/vgx/internal/scanner"
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan a codebase for compliance violations",
	Long: `Scan a directory against active compliance rules.

Loads rules from the database, runs ast-grep pattern matching and
grep-based checks, then reports findings with a compliance score.`,
	Args: cobra.ExactArgs(1),
	RunE: runScan,
}

func init() {
	scanCmd.Flags().String("branch", "main", "Branch name for scan metadata")
	scanCmd.Flags().String("org-id", "org_demo_001", "Organization ID")
	scanCmd.Flags().String("repo-url", "", "Repository URL for scan metadata")
	scanCmd.Flags().StringP("output", "o", "table", "Output format: table or json")
}

func runScan(cmd *cobra.Command, args []string) error {
	targetDir := args[0]

	// Verify directory exists
	info, err := os.Stat(targetDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("target path is not a valid directory: %s", targetDir)
	}

	branch, _ := cmd.Flags().GetString("branch")
	orgID, _ := cmd.Flags().GetString("org-id")
	repoURL, _ := cmd.Flags().GetString("repo-url")
	output, _ := cmd.Flags().GetString("output")

	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer pool.Close()

	s := scanner.New(pool)
	result, err := s.Scan(ctx, targetDir, orgID, repoURL, branch)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if output == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Table output
	fmt.Printf("\n  VibeGuard Compliance Scan\n")
	fmt.Printf("  ========================\n\n")
	fmt.Printf("  Target:     %s\n", targetDir)
	if repoURL != "" {
		fmt.Printf("  Repository: %s\n", repoURL)
	}
	fmt.Printf("  Branch:     %s\n", branch)
	fmt.Printf("  Rules:      %d checked\n", result.TotalRulesChecked)
	fmt.Printf("  Score:      %.1f%%\n\n", result.ComplianceScore)

	if len(result.Findings) == 0 {
		fmt.Printf("  No compliance violations found.\n\n")
		return nil
	}

	fmt.Printf("  Findings (%d total):\n", result.TotalFindings)
	fmt.Printf("  %-10s %-10s %-40s %s\n", "SEVERITY", "RULE", "MESSAGE", "LOCATION")
	fmt.Printf("  %-10s %-10s %-40s %s\n", "--------", "----", "-------", "--------")

	for _, f := range result.Findings {
		msg := f.Message
		if len(msg) > 40 {
			msg = msg[:37] + "..."
		}
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Printf("  %-10s %-10s %-40s %s\n", f.Severity, f.RuleID, msg, loc)
	}

	fmt.Printf("\n  Summary: %d critical, %d high, %d medium, %d low\n\n",
		result.CriticalFindings, result.HighFindings, result.MediumFindings, result.LowFindings)

	if result.ScanID != "" {
		fmt.Printf("  Scan ID: %s\n\n", result.ScanID)
	}

	return nil
}
