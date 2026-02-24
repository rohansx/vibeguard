package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vibeguard/vgx/internal/config"
	"github.com/vibeguard/vgx/internal/daemon"
	"github.com/vibeguard/vgx/internal/graph"
	"github.com/vibeguard/vgx/internal/taint"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate a structured security posture report from the current SPG state",
	Long: `Print a full security posture report for the repository.

Report sections:
  • SPG status: graph version, node counts by type
  • Attack surface: all active SourceNodes by module
  • Open taint paths: unsanitized source-to-sink paths, ranked by severity
  • Missing sanitizers: where tainted data reaches sinks unprotected
  • Risk score: weighted severity summary

Output formats: table (default), json.`,
	RunE: runReport,
}

func init() {
	reportCmd.Flags().StringP("output", "o", "table", "Output format: table or json")
	reportCmd.Flags().StringP("repo", "r", ".", "Repository path")
	reportCmd.Flags().StringP("store", "s", "", "Override SPG store path")
	reportCmd.Flags().BoolP("summary", "S", false, "Print summary only (no finding details)")
}

type reportData struct {
	Repository   string              `json:"repository"`
	GraphVersion string              `json:"graph_version"`
	Stats        map[string]int      `json:"stats"`
	Sources      []nodeInfo          `json:"sources"`
	Sinks        []nodeInfo          `json:"sinks"`
	Sanitizers   []nodeInfo          `json:"sanitizers"`
	TaintPaths   []taintPathInfo     `json:"taint_paths"`
	RiskScore    riskScore           `json:"risk_score"`
}

type nodeInfo struct {
	Symbol    string `json:"symbol"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	VulnClass string `json:"vuln_class,omitempty"`
	Framework string `json:"framework,omitempty"`
}

type taintPathInfo struct {
	Severity   string `json:"severity"`
	VulnClass  string `json:"vuln_class"`
	Source     string `json:"source"`
	SourceFile string `json:"source_file"`
	SourceLine int    `json:"source_line"`
	Sink       string `json:"sink"`
	SinkFile   string `json:"sink_file"`
	SinkLine   int    `json:"sink_line"`
	PathLen    int    `json:"path_length"`
}

type riskScore struct {
	Score    float64        `json:"score"`
	Grade    string         `json:"grade"`
	Breakdown map[string]int `json:"breakdown"`
}

func runReport(cmd *cobra.Command, _ []string) error {
	outputFmt, _ := cmd.Flags().GetString("output")
	repoPath, _ := cmd.Flags().GetString("repo")
	storeOverride, _ := cmd.Flags().GetString("store")
	summaryOnly, _ := cmd.Flags().GetBool("summary")

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
		return fmt.Errorf("opening graph: %w (did you run 'vgx init'?)", err)
	}
	defer g.Close()

	stats := g.Stats()
	if stats["total_nodes"] == 0 {
		fmt.Fprintln(os.Stderr, "SPG is empty — run 'vgx init' first")
		os.Exit(2)
	}

	eng := taint.New(g)
	paths := eng.FindAllPaths()

	report := buildReport(g, paths, repoPath)

	if outputFmt == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	printTableReport(report, summaryOnly)
	return nil
}

func buildReport(g *graph.SPG, paths []graph.TaintPath, repoPath string) reportData {
	stats := g.Stats()

	var sources, sinks, sanitizers []nodeInfo
	for _, n := range g.NodesByType(graph.SourceNode) {
		sources = append(sources, nodeInfo{Symbol: n.Symbol, File: n.FilePath, Line: n.LineStart, Framework: n.Framework})
	}
	for _, n := range g.NodesByType(graph.SinkNode) {
		sinks = append(sinks, nodeInfo{Symbol: n.Symbol, File: n.FilePath, Line: n.LineStart, VulnClass: n.VulnClass})
	}
	for _, n := range g.NodesByType(graph.SanitizerNode) {
		sanitizers = append(sanitizers, nodeInfo{Symbol: n.Symbol, File: n.FilePath, Line: n.LineStart})
	}

	sort.Slice(sources, func(i, j int) bool { return sources[i].File < sources[j].File })
	sort.Slice(sinks, func(i, j int) bool { return sinks[i].File < sinks[j].File })

	var taintInfos []taintPathInfo
	breakdown := map[string]int{}
	for _, tp := range paths {
		breakdown[tp.Severity]++
		taintInfos = append(taintInfos, taintPathInfo{
			Severity:   tp.Severity,
			VulnClass:  tp.VulnClass,
			Source:     tp.Source.Symbol,
			SourceFile: tp.Source.FilePath,
			SourceLine: tp.Source.LineStart,
			Sink:       tp.Sink.Symbol,
			SinkFile:   tp.Sink.FilePath,
			SinkLine:   tp.Sink.LineStart,
			PathLen:    len(tp.Path),
		})
	}

	score, grade := calcRiskScore(breakdown, stats)

	return reportData{
		Repository:   repoPath,
		GraphVersion: g.Version(),
		Stats:        stats,
		Sources:      sources,
		Sinks:        sinks,
		Sanitizers:   sanitizers,
		TaintPaths:   taintInfos,
		RiskScore:    riskScore{Score: score, Grade: grade, Breakdown: breakdown},
	}
}

func calcRiskScore(breakdown map[string]int, stats map[string]int) (float64, string) {
	// Score 0-100, where 100 = fully secure (no unmitigated paths)
	// Deductions: critical=-25, high=-15, medium=-5, low=-2
	score := 100.0
	score -= float64(breakdown["critical"]) * 25
	score -= float64(breakdown["high"]) * 15
	score -= float64(breakdown["medium"]) * 5
	score -= float64(breakdown["low"]) * 2

	if score < 0 {
		score = 0
	}

	var grade string
	switch {
	case score >= 90:
		grade = "A"
	case score >= 75:
		grade = "B"
	case score >= 60:
		grade = "C"
	case score >= 40:
		grade = "D"
	default:
		grade = "F"
	}

	return score, grade
}

func printTableReport(r reportData, summaryOnly bool) {
	fmt.Printf("\n  VibeGuard Security Report\n")
	fmt.Printf("  ════════════════════════════\n\n")

	fmt.Printf("  Repository:  %s\n", r.Repository)
	fmt.Printf("  Graph:       %s\n", r.GraphVersion)
	fmt.Printf("  Risk Score:  %.0f/100 (Grade: %s)\n\n", r.RiskScore.Score, r.RiskScore.Grade)

	fmt.Printf("  SPG Stats\n")
	fmt.Printf("  ─────────\n")
	fmt.Printf("  Sources:     %d\n", r.Stats["SourceNode"])
	fmt.Printf("  Sinks:       %d\n", r.Stats["SinkNode"])
	fmt.Printf("  Sanitizers:  %d\n", r.Stats["SanitizerNode"])
	fmt.Printf("  Edges:       %d\n", r.Stats["total_edges"])
	fmt.Printf("  Total Nodes: %d\n\n", r.Stats["total_nodes"])

	if len(r.TaintPaths) == 0 {
		fmt.Printf("  Taint Paths: none found — all paths are sanitized\n\n")
	} else {
		fmt.Printf("  Unsanitized Taint Paths (%d)\n", len(r.TaintPaths))
		fmt.Printf("  ────────────────────────────\n")
		for k, v := range r.RiskScore.Breakdown {
			fmt.Printf("    %s: %d\n", k, v)
		}
		fmt.Println()

		if !summaryOnly {
			for i, tp := range r.TaintPaths {
				sev := strings.ToUpper(tp.Severity)
				fmt.Printf("  [%d] %s %s\n", i+1, sev, tp.VulnClass)
				fmt.Printf("      %s (%s:%d) → %s (%s:%d)  [%d hops]\n\n",
					tp.Source, tp.SourceFile, tp.SourceLine,
					tp.Sink, tp.SinkFile, tp.SinkLine, tp.PathLen)
			}
		}
	}

	if !summaryOnly && len(r.Sources) > 0 {
		fmt.Printf("  Attack Surface (%d sources)\n", len(r.Sources))
		fmt.Printf("  ──────────────────────────\n")
		for _, s := range r.Sources {
			fw := ""
			if s.Framework != "" {
				fw = " [" + s.Framework + "]"
			}
			fmt.Printf("    %s  %s:%d%s\n", s.Symbol, s.File, s.Line, fw)
		}
		fmt.Println()
	}

	if !summaryOnly && len(r.Sinks) > 0 {
		fmt.Printf("  Dangerous Sinks (%d)\n", len(r.Sinks))
		fmt.Printf("  ────────────────────\n")
		for _, s := range r.Sinks {
			vc := ""
			if s.VulnClass != "" {
				vc = " (" + s.VulnClass + ")"
			}
			fmt.Printf("    %s  %s:%d%s\n", s.Symbol, s.File, s.Line, vc)
		}
		fmt.Println()
	}
}
