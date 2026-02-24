package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vibeguard/vgx/internal/config"
	"github.com/vibeguard/vgx/internal/daemon"
	"github.com/vibeguard/vgx/internal/graph"
	"github.com/vibeguard/vgx/internal/parser"
	"github.com/vibeguard/vgx/internal/taint"
)

var diffCmd = &cobra.Command{
	Use:   "diff [base-ref]",
	Short: "Show security property changes introduced since a git ref",
	Long: `Diff the Security Property Graph between the current working tree and a git ref.

Shows:
  • Files changed (with security relevance)
  • New taint paths introduced
  • Taint paths resolved
  • Net security posture delta

Examples:
  vgx diff HEAD~1          — changes since last commit
  vgx diff main            — changes since main branch
  vgx diff v1.2.0          — changes since a tagged release`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDiff,
}

func init() {
	diffCmd.Flags().StringP("output", "o", "table", "Output format: table or json")
	diffCmd.Flags().StringP("repo", "r", ".", "Repository path")
	diffCmd.Flags().StringP("store", "s", "", "Override SPG store path")
	diffCmd.Flags().BoolP("exit-code", "e", false, "Exit non-zero if new critical/high paths are introduced")
}

type diffResult struct {
	BaseRef      string          `json:"base_ref"`
	ChangedFiles []string        `json:"changed_files"`
	Before       diffSnapshot    `json:"before"`
	After        diffSnapshot    `json:"after"`
	NewPaths     []taintPathInfo `json:"new_paths"`
	ResolvedPaths []taintPathInfo `json:"resolved_paths"`
	Delta        string          `json:"delta"` // improved | degraded | unchanged
}

type diffSnapshot struct {
	Sources    int `json:"sources"`
	Sinks      int `json:"sinks"`
	Sanitizers int `json:"sanitizers"`
	TaintPaths int `json:"taint_paths"`
}

func runDiff(cmd *cobra.Command, args []string) error {
	ref := "HEAD~1"
	if len(args) > 0 {
		ref = args[0]
	}

	outputFmt, _ := cmd.Flags().GetString("output")
	repoPath, _ := cmd.Flags().GetString("repo")
	storeOverride, _ := cmd.Flags().GetString("store")
	exitCode, _ := cmd.Flags().GetBool("exit-code")

	absRepo, _ := filepath.Abs(repoPath)

	cfg := config.Load()
	storePath := cfg.SPGStorePath
	if storeOverride != "" {
		storePath = storeOverride
	}
	if storePath == "" {
		storePath = daemon.DefaultStorePath(repoPath)
	}

	// Get list of changed files from git
	changedFiles, err := gitChangedFiles(absRepo, ref)
	if err != nil {
		return fmt.Errorf("git diff: %w", err)
	}

	if len(changedFiles) == 0 {
		if outputFmt == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(diffResult{BaseRef: ref, Delta: "unchanged"})
		}
		fmt.Printf("\n  No source files changed since %s\n\n", ref)
		return nil
	}

	// Load current SPG (after state)
	g, err := graph.Open(storePath, absRepo)
	if err != nil {
		return fmt.Errorf("opening graph: %w (did you run 'vgx init'?)", err)
	}
	defer g.Close()

	afterStats := g.Stats()
	afterEng := taint.New(g)
	afterPaths := afterEng.FindAllPaths()
	afterPathSet := pathIDSet(afterPaths)

	// Build a temporary SPG from the base ref versions of changed files
	// We simulate "before" by removing changed files from current SPG and re-parsing base versions
	beforePaths := buildBaseRefPaths(g, absRepo, ref, changedFiles)
	beforePathSet := pathIDSet(beforePaths)

	// Compute new and resolved paths
	var newPaths, resolvedPaths []taintPathInfo
	for _, tp := range afterPaths {
		if !beforePathSet[tp.ID] {
			newPaths = append(newPaths, taintPathInfo{
				Severity: tp.Severity, VulnClass: tp.VulnClass,
				Source: tp.Source.Symbol, SourceFile: tp.Source.FilePath, SourceLine: tp.Source.LineStart,
				Sink: tp.Sink.Symbol, SinkFile: tp.Sink.FilePath, SinkLine: tp.Sink.LineStart,
				PathLen: len(tp.Path),
			})
		}
	}
	for _, tp := range beforePaths {
		if !afterPathSet[tp.ID] {
			resolvedPaths = append(resolvedPaths, taintPathInfo{
				Severity: tp.Severity, VulnClass: tp.VulnClass,
				Source: tp.Source.Symbol, SourceFile: tp.Source.FilePath, SourceLine: tp.Source.LineStart,
				Sink: tp.Sink.Symbol, SinkFile: tp.Sink.FilePath, SinkLine: tp.Sink.LineStart,
				PathLen: len(tp.Path),
			})
		}
	}

	delta := "unchanged"
	if len(newPaths) > len(resolvedPaths) {
		delta = "degraded"
	} else if len(resolvedPaths) > len(newPaths) {
		delta = "improved"
	}

	result := diffResult{
		BaseRef:       ref,
		ChangedFiles:  changedFiles,
		Before:        diffSnapshot{TaintPaths: len(beforePaths)},
		After:         diffSnapshot{Sources: afterStats["SourceNode"], Sinks: afterStats["SinkNode"], Sanitizers: afterStats["SanitizerNode"], TaintPaths: len(afterPaths)},
		NewPaths:      newPaths,
		ResolvedPaths: resolvedPaths,
		Delta:         delta,
	}

	if outputFmt == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printDiffTable(result)

	if exitCode && len(newPaths) > 0 {
		os.Exit(1)
	}
	return nil
}

func gitChangedFiles(repoPath, ref string) ([]string, error) {
	out, err := exec.Command("git", "-C", repoPath, "diff", "--name-only", ref).Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		// Only include source files we can parse
		full := filepath.Join(repoPath, line)
		if parser.DetectLanguage(full) != parser.Unknown {
			files = append(files, line)
		}
	}
	return files, nil
}

func buildBaseRefPaths(g *graph.SPG, repoPath, ref string, changedFiles []string) []graph.TaintPath {
	// Build a temporary in-memory graph with base ref versions of changed files
	// For simplicity: get the base ref content via git show, parse it, build a temp graph
	tmpStore, err := os.MkdirTemp("", "vgx-diff-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpStore)

	tmpG, err := graph.Open(tmpStore, repoPath)
	if err != nil {
		return nil
	}
	defer tmpG.Close()

	// Copy all current nodes/edges to temp graph
	for _, n := range g.AllNodes() {
		tmpG.AddNode(n)
	}

	// For changed files, replace with base ref versions
	for _, relFile := range changedFiles {
		fullPath := filepath.Join(repoPath, relFile)
		tmpG.RemoveFile(fullPath)

		// Get base ref version of the file
		content, err := exec.Command("git", "-C", repoPath, "show", ref+":"+relFile).Output()
		if err != nil {
			continue // file didn't exist at base ref
		}

		// Write to temp file, parse, build
		tmpFile, err := os.CreateTemp("", "vgx-base-*"+filepath.Ext(relFile))
		if err != nil {
			continue
		}
		tmpFile.Write(content)
		tmpFile.Close()

		// Rename temp to match original path for correct language detection
		renamedPath := tmpFile.Name() + filepath.Ext(relFile)
		os.Rename(tmpFile.Name(), renamedPath)

		pf, err := parser.ParseFile(renamedPath)
		os.Remove(renamedPath)
		if err != nil || pf == nil {
			continue
		}

		// Fix the file path so BuildFromFile uses the original path
		pf.Path = fullPath
		graph.BuildFromFile(tmpG, pf)
	}

	eng := taint.New(tmpG)
	return eng.FindAllPaths()
}

func pathIDSet(paths []graph.TaintPath) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, tp := range paths {
		set[tp.ID] = true
	}
	return set
}

func printDiffTable(r diffResult) {
	fmt.Printf("\n  VibeGuard SPG Diff\n")
	fmt.Printf("  ═══════════════════\n\n")
	fmt.Printf("  Base ref:      %s\n", r.BaseRef)
	fmt.Printf("  Files changed: %d\n", len(r.ChangedFiles))
	fmt.Printf("  Posture:       %s\n\n", strings.ToUpper(r.Delta))

	if len(r.NewPaths) > 0 {
		fmt.Printf("  New Taint Paths (+%d)\n", len(r.NewPaths))
		fmt.Printf("  ─────────────────────\n")
		for i, tp := range r.NewPaths {
			fmt.Printf("  [+%d] %s %s\n", i+1, strings.ToUpper(tp.Severity), tp.VulnClass)
			fmt.Printf("       %s (%s:%d) → %s (%s:%d)\n\n",
				tp.Source, tp.SourceFile, tp.SourceLine,
				tp.Sink, tp.SinkFile, tp.SinkLine)
		}
	}

	if len(r.ResolvedPaths) > 0 {
		fmt.Printf("  Resolved Taint Paths (-%d)\n", len(r.ResolvedPaths))
		fmt.Printf("  ──────────────────────────\n")
		for i, tp := range r.ResolvedPaths {
			fmt.Printf("  [-%d] %s %s\n", i+1, strings.ToUpper(tp.Severity), tp.VulnClass)
			fmt.Printf("       %s (%s:%d) → %s (%s:%d)\n\n",
				tp.Source, tp.SourceFile, tp.SourceLine,
				tp.Sink, tp.SinkFile, tp.SinkLine)
		}
	}

	if len(r.NewPaths) == 0 && len(r.ResolvedPaths) == 0 {
		fmt.Printf("  No security posture changes detected.\n\n")
	}
}
