// Package taint implements the taint propagation engine for the SPG.
// It finds unsanitized source-to-sink data flow paths using BFS traversal.
package taint

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"github.com/vibeguard/vgx/internal/graph"
)

// Engine runs taint analysis over the Security Property Graph.
type Engine struct {
	g *graph.SPG
}

// New creates a taint engine bound to a graph.
func New(g *graph.SPG) *Engine {
	return &Engine{g: g}
}

// FindAllPaths returns all unsanitized source-to-sink taint paths in the graph.
func (e *Engine) FindAllPaths() []graph.TaintPath {
	sources := e.g.NodesByType(graph.SourceNode)
	var results []graph.TaintPath

	for _, src := range sources {
		paths := e.traceForward(src, nil)
		results = append(results, paths...)
	}

	// De-duplicate and sort by severity
	results = deduplicatePaths(results)
	sortBySeverity(results)
	return results
}

// FindPathsToSink returns unsanitized taint paths leading to a specific sink pattern.
func (e *Engine) FindPathsToSink(sinkPattern string) []graph.TaintPath {
	sinks := e.g.NodesByType(graph.SinkNode)
	var matchedSinks []*graph.Node
	for _, sn := range sinks {
		if matchesPattern(sn.Symbol, sinkPattern) || matchesPattern(sn.VulnClass, sinkPattern) || matchesPattern(sn.FilePath, sinkPattern) {
			matchedSinks = append(matchedSinks, sn)
		}
	}

	var results []graph.TaintPath
	sources := e.g.NodesByType(graph.SourceNode)

	for _, src := range sources {
		for _, snk := range matchedSinks {
			if path := e.findPath(src, snk.ID, nil); path != nil {
				results = append(results, buildTaintPath(src, snk, path))
			}
		}
	}

	return deduplicatePaths(results)
}

// FindMissingSanitizers returns all taint paths where no sanitizer is in the path.
// This is the core "find_missing_sanitizers" MCP query.
func (e *Engine) FindMissingSanitizers() []graph.TaintPath {
	return e.FindAllPaths() // FindAllPaths already excludes sanitized paths
}

// TraceDataFlow returns all nodes reachable from a given node via data flow edges.
func (e *Engine) TraceDataFlow(nodeID string) []*graph.Node {
	start := e.g.NodeByID(nodeID)
	if start == nil {
		return nil
	}

	visited := map[string]bool{nodeID: true}
	queue := []*graph.Node{start}
	var result []*graph.Node

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, edge := range e.g.OutEdges(cur.ID) {
			if visited[edge.To] {
				continue
			}
			visited[edge.To] = true
			next := e.g.NodeByID(edge.To)
			if next != nil {
				result = append(result, next)
				queue = append(queue, next)
			}
		}
	}

	return result
}

// AttackSurface returns all source nodes and the sinks reachable from them.
type AttackSurface struct {
	Sources        []*graph.Node
	ReachableSinks []*graph.Node
	TaintPathCount int
}

// GetAttackSurface returns the full attack surface for a module path prefix.
func (e *Engine) GetAttackSurface(modulePrefix string) AttackSurface {
	var surface AttackSurface
	sinkSet := make(map[string]*graph.Node)

	sources := e.g.NodesByType(graph.SourceNode)
	for _, src := range sources {
		if modulePrefix != "" && !matchesPattern(src.FilePath, modulePrefix) {
			continue
		}
		surface.Sources = append(surface.Sources, src)
		paths := e.traceForward(src, nil)
		for _, tp := range paths {
			sinkSet[tp.Sink.ID] = tp.Sink
			surface.TaintPathCount++
		}
	}

	for _, s := range sinkSet {
		surface.ReachableSinks = append(surface.ReachableSinks, s)
	}
	return surface
}

// BlastRadius returns the taint paths and nodes affected if a given node changes.
type BlastRadius struct {
	AffectedPaths []*graph.TaintPath
	AffectedNodes []*graph.Node
}

// CalcBlastRadius computes what security properties would change if nodeID is modified.
func (e *Engine) CalcBlastRadius(nodeID string) BlastRadius {
	var br BlastRadius
	visited := map[string]bool{}

	// Walk backward from the node to find what sources feed into it
	var backward func(id string)
	backward = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		n := e.g.NodeByID(id)
		if n != nil {
			br.AffectedNodes = append(br.AffectedNodes, n)
		}
		for _, e := range e.g.InEdges(id) {
			backward(e.From)
		}
	}
	backward(nodeID)

	// Find all taint paths that pass through this node
	allPaths := e.FindAllPaths()
	for i, tp := range allPaths {
		for _, n := range tp.Path {
			if n.ID == nodeID {
				br.AffectedPaths = append(br.AffectedPaths, &allPaths[i])
				break
			}
		}
	}

	return br
}

// ---- internal BFS ----

// traceForward performs BFS from a source node, following DataFlow and Call edges.
// Returns all taint paths (source → sink) without an intervening sanitizer.
// For CallEdge traversal, it uses conservative propagation: if any argument name
// matches a currently tainted variable, all callee parameters are treated as tainted.
func (e *Engine) traceForward(src *graph.Node, visited map[string]bool) []graph.TaintPath {
	if visited == nil {
		visited = make(map[string]bool)
	}

	// Seed tainted vars from the source node's metadata
	var srcTaintedVars []string
	if tv, ok := src.Metadata["tainted_vars"]; ok && tv != "" {
		srcTaintedVars = strings.Split(tv, ",")
	}
	if len(srcTaintedVars) == 0 {
		srcTaintedVars = []string{src.Symbol}
	}

	type state struct {
		node        *graph.Node
		path        []*graph.Node
		sanitized   bool
		taintedVars []string // variable names currently tainted at this point in the path
	}

	queue := []state{{node: src, path: []*graph.Node{src}, taintedVars: srcTaintedVars}}
	var results []graph.TaintPath
	const maxDepth = 20 // prevent runaway BFS on large graphs

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if len(cur.path) > maxDepth {
			continue
		}

		nodeKey := fmt.Sprintf("%s:%d", cur.node.ID, len(cur.path))
		if visited[nodeKey] {
			continue
		}
		visited[nodeKey] = true

		// If we hit a sink and there's no sanitizer in the path, record a taint path
		if cur.node.Type == graph.SinkNode && !cur.sanitized {
			tp := buildTaintPath(src, cur.node, cur.path)
			results = append(results, tp)
			continue // don't explore further from a sink
		}

		// Propagate along outgoing edges
		for _, edge := range e.g.OutEdges(cur.node.ID) {
			next := e.g.NodeByID(edge.To)
			if next == nil {
				continue
			}

			newPath := append(append([]*graph.Node{}, cur.path...), next)
			newSanitized := cur.sanitized || next.Type == graph.SanitizerNode

			if edge.Type == graph.CallEdge {
				// Inter-procedural: only cross the call if a tainted variable is
				// passed as an argument. Conservative: any tainted arg → all params tainted.
				args := splitNonEmpty(edge.Metadata["args"])
				if !anyTainted(cur.taintedVars, args) {
					continue // no tainted args — don't cross this call boundary
				}
				// Propagate into callee: treat its declared params as tainted
				calleeParams := splitNonEmpty(next.Metadata["params"])
				if len(calleeParams) == 0 {
					calleeParams = cur.taintedVars // fallback: keep current set
				}
				queue = append(queue, state{
					node:        next,
					path:        newPath,
					sanitized:   newSanitized,
					taintedVars: calleeParams,
				})
				continue
			}

			// DataFlowEdge or other edges: propagate tainted vars unchanged
			queue = append(queue, state{
				node:        next,
				path:        newPath,
				sanitized:   newSanitized,
				taintedVars: cur.taintedVars,
			})
		}
	}

	return results
}

// anyTainted returns true if any element of args appears in the tainted vars set.
func anyTainted(taintedVars, args []string) bool {
	if len(taintedVars) == 0 || len(args) == 0 {
		return false
	}
	taintSet := make(map[string]bool, len(taintedVars))
	for _, v := range taintedVars {
		taintSet[v] = true
	}
	for _, a := range args {
		if taintSet[a] {
			return true
		}
	}
	return false
}

// splitNonEmpty splits a comma-separated string and drops empty entries.
func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// findPath finds a path from src to a specific sink ID using BFS.
// Returns the path (inclusive of start and end) or nil if no path exists.
func (e *Engine) findPath(src *graph.Node, sinkID string, visited map[string]bool) []*graph.Node {
	if visited == nil {
		visited = make(map[string]bool)
	}

	type state struct {
		node      *graph.Node
		path      []*graph.Node
		sanitized bool
	}

	queue := []state{{node: src, path: []*graph.Node{src}}}
	const maxDepth = 20

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if len(cur.path) > maxDepth {
			continue
		}
		if visited[cur.node.ID] {
			continue
		}
		visited[cur.node.ID] = true

		if cur.node.ID == sinkID && !cur.sanitized {
			return cur.path
		}

		for _, edge := range e.g.OutEdges(cur.node.ID) {
			next := e.g.NodeByID(edge.To)
			if next == nil {
				continue
			}
			newSanitized := cur.sanitized || next.Type == graph.SanitizerNode
			newPath := append(append([]*graph.Node{}, cur.path...), next)
			queue = append(queue, state{node: next, path: newPath, sanitized: newSanitized})
		}
	}

	return nil
}

func buildTaintPath(src, sink *graph.Node, path []*graph.Node) graph.TaintPath {
	vulnClass := sink.VulnClass
	if vulnClass == "" {
		vulnClass = "unknown"
	}
	severity := graph.SeverityForVulnClass(vulnClass)

	pathIDs := make([]string, len(path))
	for i, n := range path {
		pathIDs[i] = n.ID
	}

	confidence := 0.85
	if len(path) <= 2 {
		// Direct source → sink with no intermediate nodes: higher confidence
		confidence = 0.95
	}

	desc := fmt.Sprintf("%s (%s) flows to %s (%s) without sanitization",
		src.Symbol, src.FilePath, sink.Symbol, sink.FilePath)

	sum := sha256.Sum256([]byte(src.ID + sink.ID))
	id := fmt.Sprintf("%x", sum[:8])

	return graph.TaintPath{
		ID:          id,
		Source:      src,
		Sink:        sink,
		Path:        path,
		VulnClass:   vulnClass,
		Severity:    severity,
		Confidence:  confidence,
		FilePath:    sink.FilePath,
		Description: desc,
	}
}

func deduplicatePaths(paths []graph.TaintPath) []graph.TaintPath {
	seen := make(map[string]bool)
	var out []graph.TaintPath
	for _, tp := range paths {
		if !seen[tp.ID] {
			seen[tp.ID] = true
			out = append(out, tp)
		}
	}
	return out
}

func sortBySeverity(paths []graph.TaintPath) {
	order := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	sort.Slice(paths, func(i, j int) bool {
		oi := order[paths[i].Severity]
		oj := order[paths[j].Severity]
		return oi < oj
	})
}

func matchesPattern(value, pattern string) bool {
	if pattern == "" {
		return true
	}
	// Simple substring match for Phase 1
	return findSubstr(value, pattern)
}

func findSubstr(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
