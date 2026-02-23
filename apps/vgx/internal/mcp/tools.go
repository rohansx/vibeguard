package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/vibeguard/vgx/internal/graph"
)

// args extracts the Arguments map from a CallToolRequest (mcp-go v0.44+ stores it as any).
func args(req mcp.CallToolRequest) map[string]any {
	m, _ := req.Params.Arguments.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}

// ---- Phase 1 core tools (fully implemented) ----

func (s *Server) handleQueryTaintPaths(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sink, _ := args(req)["sink"].(string)
	if sink == "" {
		return mcp.NewToolResultError("sink parameter is required"), nil
	}

	paths := s.engine.FindPathsToSink(sink)

	type pathResult struct {
		ID          string       `json:"id"`
		VulnClass   string       `json:"vuln_class"`
		Severity    string       `json:"severity"`
		Confidence  float64      `json:"confidence"`
		Description string       `json:"description"`
		Source      nodeResult   `json:"source"`
		Sink        nodeResult   `json:"sink"`
		PathLength  int          `json:"path_length"`
		Path        []nodeResult `json:"path"`
	}

	type response struct {
		Query     string       `json:"query"`
		PathCount int          `json:"path_count"`
		Paths     []pathResult `json:"paths"`
		Proof     string       `json:"proof"`
	}

	var results []pathResult
	for _, tp := range paths {
		var pathNodes []nodeResult
		for _, n := range tp.Path {
			pathNodes = append(pathNodes, toNodeResult(n))
		}
		results = append(results, pathResult{
			ID:          tp.ID,
			VulnClass:   tp.VulnClass,
			Severity:    tp.Severity,
			Confidence:  tp.Confidence,
			Description: tp.Description,
			Source:      toNodeResult(tp.Source),
			Sink:        toNodeResult(tp.Sink),
			PathLength:  len(tp.Path),
			Path:        pathNodes,
		})
	}

	proof := "NO_UNSANITIZED_PATH_EXISTS"
	if len(results) > 0 {
		proof = fmt.Sprintf("UNSANITIZED_PATHS_FOUND: %d path(s) detected", len(results))
	}

	resp := response{
		Query:     sink,
		PathCount: len(results),
		Paths:     results,
		Proof:     proof,
	}

	return jsonResult(resp)
}

func (s *Server) handleGetAttackSurface(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	module, _ := args(req)["module"].(string)

	surface := s.engine.GetAttackSurface(module)

	type response struct {
		Module         string       `json:"module"`
		SourceCount    int          `json:"source_count"`
		SinkCount      int          `json:"reachable_sink_count"`
		TaintPathCount int          `json:"taint_path_count"`
		Sources        []nodeResult `json:"sources"`
		ReachableSinks []nodeResult `json:"reachable_sinks"`
	}

	var srcs, sinks []nodeResult
	for _, n := range surface.Sources {
		srcs = append(srcs, toNodeResult(n))
	}
	for _, n := range surface.ReachableSinks {
		sinks = append(sinks, toNodeResult(n))
	}

	return jsonResult(response{
		Module:         module,
		SourceCount:    len(surface.Sources),
		SinkCount:      len(surface.ReachableSinks),
		TaintPathCount: surface.TaintPathCount,
		Sources:        srcs,
		ReachableSinks: sinks,
	})
}

func (s *Server) handleFindMissingSanitizers(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	paths := s.engine.FindMissingSanitizers()

	type finding struct {
		ID          string     `json:"id"`
		VulnClass   string     `json:"vuln_class"`
		Severity    string     `json:"severity"`
		Confidence  float64    `json:"confidence"`
		Description string     `json:"description"`
		SourceFile  string     `json:"source_file"`
		SourceLine  int        `json:"source_line"`
		SinkFile    string     `json:"sink_file"`
		SinkLine    int        `json:"sink_line"`
		SinkSymbol  string     `json:"sink_symbol"`
		Remediation string     `json:"remediation"`
	}

	type response struct {
		TotalFindings int                       `json:"total_findings"`
		BySeverity    map[string]int            `json:"by_severity"`
		Findings      []finding                 `json:"findings"`
		GraphVersion  string                    `json:"graph_version"`
	}

	bySeverity := map[string]int{}
	var findings []finding
	for _, tp := range paths {
		bySeverity[tp.Severity]++
		findings = append(findings, finding{
			ID:          tp.ID,
			VulnClass:   tp.VulnClass,
			Severity:    tp.Severity,
			Confidence:  tp.Confidence,
			Description: tp.Description,
			SourceFile:  tp.Source.FilePath,
			SourceLine:  tp.Source.LineStart,
			SinkFile:    tp.Sink.FilePath,
			SinkLine:    tp.Sink.LineStart,
			SinkSymbol:  tp.Sink.Symbol,
			Remediation: remediationFor(tp.VulnClass),
		})
	}

	return jsonResult(response{
		TotalFindings: len(findings),
		BySeverity:    bySeverity,
		Findings:      findings,
		GraphVersion:  s.g.Version(),
	})
}

func (s *Server) handleTraceDataFlow(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	nodeID, _ := args(req)["node_id"].(string)
	symbol, _ := args(req)["symbol"].(string)
	file, _ := args(req)["file"].(string)

	// Find the starting node
	var startNode *graph.Node
	if nodeID != "" {
		startNode = s.g.NodeByID(nodeID)
	} else if symbol != "" {
		// Find by symbol + optional file
		for _, n := range s.g.AllNodes() {
			if n.Symbol == symbol && (file == "" || n.FilePath == file) {
				startNode = n
				break
			}
		}
	}

	if startNode == nil {
		return mcp.NewToolResultError(fmt.Sprintf("node not found: node_id=%q symbol=%q file=%q", nodeID, symbol, file)), nil
	}

	reachable := s.engine.TraceDataFlow(startNode.ID)

	type traceResponse struct {
		Origin       nodeResult   `json:"origin"`
		ReachesCount int          `json:"reaches_count"`
		SinkCount    int          `json:"sink_count"`
		Reaches      []nodeResult `json:"reaches"`
	}

	var reachNodes []nodeResult
	sinkCount := 0
	for _, n := range reachable {
		reachNodes = append(reachNodes, toNodeResult(n))
		if n.Type == graph.SinkNode {
			sinkCount++
		}
	}

	return jsonResult(traceResponse{
		Origin:       toNodeResult(startNode),
		ReachesCount: len(reachable),
		SinkCount:    sinkCount,
		Reaches:      reachNodes,
	})
}

// ---- Phase 2 stubs ----

func (s *Server) handleCalculateBlastRadius(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	fn, _ := args(req)["function"].(string)
	_ = fn

	// Find node by function name
	var targetNode *graph.Node
	for _, n := range s.g.AllNodes() {
		if n.Symbol == fn {
			targetNode = n
			break
		}
	}
	if targetNode == nil {
		return mcp.NewToolResultError(fmt.Sprintf("function %q not found in SPG", fn)), nil
	}

	br := s.engine.CalcBlastRadius(targetNode.ID)

	type response struct {
		Function       string       `json:"function"`
		AffectedNodes  []nodeResult `json:"affected_nodes"`
		AffectedPaths  int          `json:"affected_path_count"`
	}

	var affected []nodeResult
	for _, n := range br.AffectedNodes {
		affected = append(affected, toNodeResult(n))
	}

	return jsonResult(response{
		Function:      fn,
		AffectedNodes: affected,
		AffectedPaths: len(br.AffectedPaths),
	})
}

func (s *Server) handleGetTrustBoundaryViolations(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return jsonResult(map[string]any{
		"violations": []any{},
		"note":       "trust boundary detection lands in Phase 2 — requires inter-procedural call graph with auth annotation",
		"graph_version": s.g.Version(),
	})
}

func (s *Server) handleCheckAuthCoverage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	endpoint, _ := args(req)["endpoint"].(string)
	return jsonResult(map[string]any{
		"endpoint":      endpoint,
		"covered":       nil,
		"note":          "auth coverage analysis lands in Phase 2 — requires AuthCriticalNode classification",
		"graph_version": s.g.Version(),
	})
}

func (s *Server) handleGetSecurityContext(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	file, _ := args(req)["file"].(string)

	fileNodes := s.g.NodesByFile(file)
	var nodes []nodeResult
	for _, n := range fileNodes {
		nodes = append(nodes, toNodeResult(n))
	}

	sourceCount, sinkCount, sanitizerCount := 0, 0, 0
	for _, n := range fileNodes {
		switch n.Type {
		case graph.SourceNode:
			sourceCount++
		case graph.SinkNode:
			sinkCount++
		case graph.SanitizerNode:
			sanitizerCount++
		}
	}

	riskScore := 0.0
	if sinkCount > 0 && sanitizerCount == 0 {
		riskScore = float64(sinkCount) / float64(sinkCount+sanitizerCount+1)
	}

	return jsonResult(map[string]any{
		"file":            file,
		"total_nodes":     len(fileNodes),
		"source_count":    sourceCount,
		"sink_count":      sinkCount,
		"sanitizer_count": sanitizerCount,
		"risk_score":      riskScore,
		"nodes":           nodes,
		"graph_version":   s.g.Version(),
	})
}

// ---- shared helpers ----

type nodeResult struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Symbol    string `json:"symbol"`
	FilePath  string `json:"file_path"`
	Line      int    `json:"line"`
	Framework string `json:"framework,omitempty"`
	VulnClass string `json:"vuln_class,omitempty"`
}

func toNodeResult(n *graph.Node) nodeResult {
	if n == nil {
		return nodeResult{}
	}
	return nodeResult{
		ID:        n.ID,
		Type:      string(n.Type),
		Symbol:    n.Symbol,
		FilePath:  n.FilePath,
		Line:      n.LineStart,
		Framework: n.Framework,
		VulnClass: n.VulnClass,
	}
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("serialization error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func remediationFor(vulnClass string) string {
	switch vulnClass {
	case "sqli":
		return "Use parameterized queries / prepared statements. Never interpolate user input into SQL strings."
	case "xss":
		return "Escape output with a context-appropriate sanitizer (DOMPurify, html.EscapeString). Avoid innerHTML with untrusted data."
	case "rce":
		return "Avoid passing user input to exec/eval. Validate against an allowlist. Use language-native APIs instead of shell commands."
	case "path_traversal":
		return "Validate and normalize file paths. Use filepath.Clean and verify the result is within an allowed base directory."
	case "ssrf":
		return "Validate URLs against an allowlist of permitted hosts. Block internal/private IP ranges. Use a proxy with egress filtering."
	case "deserialization":
		return "Avoid deserializing untrusted data. Use JSON instead of pickle/YAML. If unavoidable, use HMAC signatures to verify data integrity before deserialization."
	default:
		return "Validate and sanitize all user-controlled input before use in sensitive operations."
	}
}
