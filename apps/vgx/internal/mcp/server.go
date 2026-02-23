// Package mcp implements the VibeGuard MCP server — the interface between
// the Security Property Graph and AI coding agents (Cursor, Claude Code, etc.).
//
// Transport: stdio only. No network exposure. Code never leaves the machine.
package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vibeguard/vgx/internal/graph"
	"github.com/vibeguard/vgx/internal/taint"
)

// Server wraps the MCP server with the SPG and taint engine.
type Server struct {
	s      *server.MCPServer
	g      *graph.SPG
	engine *taint.Engine
}

// New creates and wires up the VibeGuard MCP server with all Phase 1 tools.
func New(g *graph.SPG) *Server {
	engine := taint.New(g)

	s := server.NewMCPServer(
		"vibeguard-spg",
		"0.2.0",
		server.WithToolCapabilities(true),
	)

	srv := &Server{s: s, g: g, engine: engine}

	// Register all 8 MCP tools (Phase 1: 4 core tools; Phase 2: remaining 4)
	srv.registerQueryTaintPaths()
	srv.registerGetAttackSurface()
	srv.registerFindMissingSanitizers()
	srv.registerTraceDataFlow()

	// Phase 2 tools (stubs registered so agents can discover them)
	srv.registerCalculateBlastRadius()
	srv.registerGetTrustBoundaryViolations()
	srv.registerCheckAuthCoverage()
	srv.registerGetSecurityContext()

	return srv
}

// ServeStdio starts the MCP server on stdin/stdout.
// This is the primary mode — called by `vgx serve`.
func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.s)
}

// registerQueryTaintPaths wires the query_taint_paths tool.
func (s *Server) registerQueryTaintPaths() {
	tool := mcp.NewTool("query_taint_paths",
		mcp.WithDescription(`Find unsanitized taint paths from any SourceNode to a given sink.

Returns a deterministic list of source-to-sink paths where attacker-controlled
data reaches a dangerous operation without sanitization. Same code + same query
always returns the same result — no LLM reasoning in the verification path.

Answer: full source-to-sink path with file paths and line numbers, or empty array if no paths exist.`),
		mcp.WithString("sink",
			mcp.Required(),
			mcp.Description("Sink pattern to query: function name (e.g. 'db.execute'), vulnerability class (e.g. 'sqli'), or file path substring"),
		),
	)
	s.s.AddTool(tool, s.handleQueryTaintPaths)
}

// registerGetAttackSurface wires the get_attack_surface tool.
func (s *Server) registerGetAttackSurface() {
	tool := mcp.NewTool("get_attack_surface",
		mcp.WithDescription(`Return the full attack surface for a module: all untrusted entry points (SourceNodes) and the dangerous sinks reachable from them.

Use this before writing new code in a module to understand what attacker-controlled data is already present and where it can travel.`),
		mcp.WithString("module",
			mcp.Description("Module path prefix to scope the query (e.g. 'src/api/', 'handlers/'). Leave empty for the full repository."),
		),
	)
	s.s.AddTool(tool, s.handleGetAttackSurface)
}

// registerFindMissingSanitizers wires the find_missing_sanitizers tool.
func (s *Server) registerFindMissingSanitizers() {
	tool := mcp.NewTool("find_missing_sanitizers",
		mcp.WithDescription(`Find all locations where tainted data reaches a dangerous sink without a sanitizer in the path.

Returns all active taint paths ranked by severity (critical → high → medium → low).
This is the primary security audit query — run it after significant code changes.`),
	)
	s.s.AddTool(tool, s.handleFindMissingSanitizers)
}

// registerTraceDataFlow wires the trace_data_flow tool.
func (s *Server) registerTraceDataFlow() {
	tool := mcp.NewTool("trace_data_flow",
		mcp.WithDescription(`Trace where a variable or expression travels through the codebase.

Returns the full provenance of a value: its origin, all transformations, any sinks it reaches, and external calls it participates in. Use this to understand the security implications of a specific data value.`),
		mcp.WithString("node_id",
			mcp.Description("SPG node ID to trace from (from a previous query result)"),
		),
		mcp.WithString("symbol",
			mcp.Description("Symbol name (variable, function) to find and trace"),
		),
		mcp.WithString("file",
			mcp.Description("File path to scope the symbol search"),
		),
	)
	s.s.AddTool(tool, s.handleTraceDataFlow)
}

func (s *Server) registerCalculateBlastRadius() {
	tool := mcp.NewTool("calculate_blast_radius",
		mcp.WithDescription(`[Phase 2] Calculate what security properties could change if a function is modified.

Returns all taint paths and trust boundaries affected by changes to the given function.`),
		mcp.WithString("function",
			mcp.Required(),
			mcp.Description("Function name or node ID to compute blast radius for"),
		),
	)
	s.s.AddTool(tool, s.handleCalculateBlastRadius)
}

func (s *Server) registerGetTrustBoundaryViolations() {
	tool := mcp.NewTool("get_trust_boundary_violations",
		mcp.WithDescription(`[Phase 2] Find code paths that cross trust boundaries without authentication checks.`),
	)
	s.s.AddTool(tool, s.handleGetTrustBoundaryViolations)
}

func (s *Server) registerCheckAuthCoverage() {
	tool := mcp.NewTool("check_auth_coverage",
		mcp.WithDescription(`[Phase 2] Check whether an endpoint enforces authentication before sensitive operations.`),
		mcp.WithString("endpoint",
			mcp.Required(),
			mcp.Description("Endpoint path or handler function name"),
		),
	)
	s.s.AddTool(tool, s.handleCheckAuthCoverage)
}

func (s *Server) registerGetSecurityContext() {
	tool := mcp.NewTool("get_security_context",
		mcp.WithDescription(`[Phase 2] Get the full security posture of a file: classified nodes, taint status, trust level, risk score.`),
		mcp.WithString("file",
			mcp.Required(),
			mcp.Description("File path to get security context for"),
		),
	)
	s.s.AddTool(tool, s.handleGetSecurityContext)
}
