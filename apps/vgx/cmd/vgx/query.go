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

var queryCmd = &cobra.Command{
	Use:   "query <tool> [args...]",
	Short: "Run a security query against the live Security Property Graph",
	Long: `Execute a deterministic security query against the SPG.

Available query tools (map directly to MCP tools):
  taint-paths [sink]         — unsanitized paths to a specific sink
  attack-surface [module]    — all source nodes and reachable sinks for a module
  missing-sanitizers         — all unprotected source-to-sink paths, ranked by severity
  trace [node-id]            — full data flow provenance for a node
  blast-radius [node-id]     — security properties affected if this node changes

All queries return deterministic JSON. Same code + same query = same result always.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runQuery,
}

func init() {
	queryCmd.Flags().StringP("repo", "r", ".", "Repository path")
	queryCmd.Flags().StringP("store", "s", "", "Override SPG store path")
}

func runQuery(cmd *cobra.Command, args []string) error {
	tool := args[0]
	repoPath, _ := cmd.Flags().GetString("repo")
	storeOverride, _ := cmd.Flags().GetString("store")

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

	if g.Stats()["total_nodes"] == 0 {
		fmt.Fprintln(os.Stderr, "SPG is empty — run 'vgx init' first")
		os.Exit(2)
	}

	eng := taint.New(g)

	var result any

	switch tool {
	case "taint-paths":
		pattern := ""
		if len(args) > 1 {
			pattern = args[1]
		}
		var paths []graph.TaintPath
		if pattern == "" {
			paths = eng.FindAllPaths()
		} else {
			paths = eng.FindPathsToSink(pattern)
		}
		result = map[string]any{
			"query":      pattern,
			"path_count": len(paths),
			"paths":      formatPaths(paths),
		}

	case "attack-surface":
		module := ""
		if len(args) > 1 {
			module = args[1]
		}
		surface := eng.GetAttackSurface(module)
		result = map[string]any{
			"module":       module,
			"source_count": len(surface.Sources),
			"sources":      formatNodes(surface.Sources),
			"sink_count":   len(surface.ReachableSinks),
			"sinks":        formatNodes(surface.ReachableSinks),
			"taint_paths":  surface.TaintPathCount,
		}

	case "missing-sanitizers":
		paths := eng.FindMissingSanitizers()
		result = map[string]any{
			"count": len(paths),
			"paths": formatPaths(paths),
		}

	case "trace":
		if len(args) < 2 {
			return fmt.Errorf("usage: vgx query trace <node-id>")
		}
		nodeID := args[1]
		nodes := eng.TraceDataFlow(nodeID)
		start := g.NodeByID(nodeID)
		startInfo := map[string]any{}
		if start != nil {
			startInfo = map[string]any{"id": start.ID, "symbol": start.Symbol, "file": start.FilePath, "line": start.LineStart, "type": start.Type}
		}
		result = map[string]any{
			"start_node":    startInfo,
			"reachable":     formatNodes(nodes),
			"reach_count":   len(nodes),
		}

	case "blast-radius":
		if len(args) < 2 {
			return fmt.Errorf("usage: vgx query blast-radius <node-id>")
		}
		nodeID := args[1]
		br := eng.CalcBlastRadius(nodeID)
		result = map[string]any{
			"node_id":        nodeID,
			"affected_nodes": len(br.AffectedNodes),
			"affected_paths": len(br.AffectedPaths),
			"nodes":          formatNodes(br.AffectedNodes),
		}

	default:
		return fmt.Errorf("unknown query tool %q — available: taint-paths, attack-surface, missing-sanitizers, trace, blast-radius", tool)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func formatPaths(paths []graph.TaintPath) []map[string]any {
	out := make([]map[string]any, 0, len(paths))
	for _, tp := range paths {
		out = append(out, map[string]any{
			"severity":    tp.Severity,
			"vuln_class":  tp.VulnClass,
			"source":      tp.Source.Symbol,
			"source_file": tp.Source.FilePath,
			"source_line": tp.Source.LineStart,
			"sink":        tp.Sink.Symbol,
			"sink_file":   tp.Sink.FilePath,
			"sink_line":   tp.Sink.LineStart,
			"path_length": len(tp.Path),
			"confidence":  tp.Confidence,
		})
	}
	return out
}

func formatNodes(nodes []*graph.Node) []map[string]any {
	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		m := map[string]any{
			"id":     n.ID,
			"type":   n.Type,
			"symbol": n.Symbol,
			"file":   n.FilePath,
			"line":   n.LineStart,
		}
		if n.VulnClass != "" {
			m["vuln_class"] = n.VulnClass
		}
		if n.Framework != "" {
			m["framework"] = n.Framework
		}
		out = append(out, m)
	}
	return out
}
