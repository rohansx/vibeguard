package taint

import (
	"testing"

	"github.com/vibeguard/vgx/internal/graph"
)

func TestFindAllPaths(t *testing.T) {
	g := buildTestGraph(t)

	// source → sink with no sanitizer = 1 taint path
	addNode(g, "src1", graph.SourceNode, "a.py", 1, "request.args")
	addNode(g, "snk1", graph.SinkNode, "a.py", 10, "cursor.execute")
	g.AddEdge(&graph.Edge{ID: "e1", From: "src1", To: "snk1", Type: graph.DataFlowEdge})

	eng := New(g)
	paths := eng.FindAllPaths()

	if len(paths) != 1 {
		t.Fatalf("expected 1 taint path, got %d", len(paths))
	}
	if paths[0].Source.ID != "src1" {
		t.Errorf("expected source src1, got %s", paths[0].Source.ID)
	}
	if paths[0].Sink.ID != "snk1" {
		t.Errorf("expected sink snk1, got %s", paths[0].Sink.ID)
	}
}

func TestSanitizerBlocksPath(t *testing.T) {
	g := buildTestGraph(t)

	addNode(g, "src1", graph.SourceNode, "a.py", 1, "request.args")
	addNode(g, "san1", graph.SanitizerNode, "a.py", 5, "escape")
	addNode(g, "snk1", graph.SinkNode, "a.py", 10, "cursor.execute")
	g.AddEdge(&graph.Edge{ID: "e1", From: "src1", To: "san1", Type: graph.DataFlowEdge})
	g.AddEdge(&graph.Edge{ID: "e2", From: "san1", To: "snk1", Type: graph.DataFlowEdge})

	eng := New(g)
	paths := eng.FindAllPaths()

	if len(paths) != 0 {
		t.Errorf("expected 0 taint paths (sanitizer blocks), got %d", len(paths))
	}
}

func TestFindPathsToSink(t *testing.T) {
	g := buildTestGraph(t)

	addNodeWithVuln(g, "src1", graph.SourceNode, "a.py", 1, "request.args", "")
	addNodeWithVuln(g, "snk1", graph.SinkNode, "a.py", 10, "cursor.execute", "sqli")
	addNodeWithVuln(g, "snk2", graph.SinkNode, "a.py", 20, "exec", "rce")
	g.AddEdge(&graph.Edge{ID: "e1", From: "src1", To: "snk1", Type: graph.DataFlowEdge})
	g.AddEdge(&graph.Edge{ID: "e2", From: "src1", To: "snk2", Type: graph.DataFlowEdge})

	eng := New(g)

	sqliPaths := eng.FindPathsToSink("sqli")
	if len(sqliPaths) != 1 {
		t.Errorf("expected 1 sqli path, got %d", len(sqliPaths))
	}

	rcePaths := eng.FindPathsToSink("rce")
	if len(rcePaths) != 1 {
		t.Errorf("expected 1 rce path, got %d", len(rcePaths))
	}
}

func TestFindMissingSanitizers(t *testing.T) {
	g := buildTestGraph(t)

	addNode(g, "src1", graph.SourceNode, "a.py", 1, "input")
	addNode(g, "snk1", graph.SinkNode, "a.py", 10, "eval")
	g.AddEdge(&graph.Edge{ID: "e1", From: "src1", To: "snk1", Type: graph.DataFlowEdge})

	eng := New(g)
	paths := eng.FindMissingSanitizers()

	if len(paths) != 1 {
		t.Errorf("expected 1 missing sanitizer path, got %d", len(paths))
	}
}

func TestTraceDataFlow(t *testing.T) {
	g := buildTestGraph(t)

	addNode(g, "n1", graph.SourceNode, "a.py", 1, "input")
	addNode(g, "n2", graph.SinkNode, "a.py", 5, "process")
	addNode(g, "n3", graph.SinkNode, "a.py", 10, "output")
	g.AddEdge(&graph.Edge{ID: "e1", From: "n1", To: "n2", Type: graph.DataFlowEdge})
	g.AddEdge(&graph.Edge{ID: "e2", From: "n2", To: "n3", Type: graph.DataFlowEdge})

	eng := New(g)
	reachable := eng.TraceDataFlow("n1")

	if len(reachable) != 2 {
		t.Errorf("expected 2 reachable nodes from n1, got %d", len(reachable))
	}
}

func TestTraceDataFlowNonexistent(t *testing.T) {
	g := buildTestGraph(t)
	eng := New(g)

	result := eng.TraceDataFlow("doesnotexist")
	if result != nil {
		t.Errorf("expected nil for nonexistent node, got %d nodes", len(result))
	}
}

func TestGetAttackSurface(t *testing.T) {
	g := buildTestGraph(t)

	addNode(g, "src1", graph.SourceNode, "api/handler.py", 1, "request.body")
	addNode(g, "src2", graph.SourceNode, "lib/utils.py", 1, "input")
	addNode(g, "snk1", graph.SinkNode, "api/handler.py", 10, "db.query")
	g.AddEdge(&graph.Edge{ID: "e1", From: "src1", To: "snk1", Type: graph.DataFlowEdge})

	eng := New(g)

	// All sources
	surface := eng.GetAttackSurface("")
	if len(surface.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(surface.Sources))
	}

	// Filtered by module
	apiSurface := eng.GetAttackSurface("api/")
	if len(apiSurface.Sources) != 1 {
		t.Errorf("expected 1 api source, got %d", len(apiSurface.Sources))
	}
}

func TestCalcBlastRadius(t *testing.T) {
	g := buildTestGraph(t)

	addNode(g, "src1", graph.SourceNode, "a.py", 1, "input")
	addNode(g, "mid1", graph.SinkNode, "a.py", 5, "transform")
	addNode(g, "snk1", graph.SinkNode, "a.py", 10, "execute")
	g.AddEdge(&graph.Edge{ID: "e1", From: "src1", To: "mid1", Type: graph.DataFlowEdge})
	g.AddEdge(&graph.Edge{ID: "e2", From: "mid1", To: "snk1", Type: graph.DataFlowEdge})

	eng := New(g)
	br := eng.CalcBlastRadius("mid1")

	if len(br.AffectedNodes) == 0 {
		t.Error("expected affected nodes for mid1")
	}
}

func TestMultiplePathsSameSink(t *testing.T) {
	g := buildTestGraph(t)

	addNode(g, "src1", graph.SourceNode, "a.py", 1, "input1")
	addNode(g, "src2", graph.SourceNode, "a.py", 2, "input2")
	addNode(g, "snk1", graph.SinkNode, "a.py", 10, "execute")
	g.AddEdge(&graph.Edge{ID: "e1", From: "src1", To: "snk1", Type: graph.DataFlowEdge})
	g.AddEdge(&graph.Edge{ID: "e2", From: "src2", To: "snk1", Type: graph.DataFlowEdge})

	eng := New(g)
	paths := eng.FindAllPaths()

	if len(paths) != 2 {
		t.Errorf("expected 2 taint paths from 2 sources, got %d", len(paths))
	}
}

func TestSeverityOrder(t *testing.T) {
	g := buildTestGraph(t)

	addNodeWithVuln(g, "src1", graph.SourceNode, "a.py", 1, "input", "")
	addNodeWithVuln(g, "snk_xss", graph.SinkNode, "a.py", 10, "innerHTML", "xss")
	addNodeWithVuln(g, "snk_rce", graph.SinkNode, "a.py", 20, "eval", "rce")
	g.AddEdge(&graph.Edge{ID: "e1", From: "src1", To: "snk_xss", Type: graph.DataFlowEdge})
	g.AddEdge(&graph.Edge{ID: "e2", From: "src1", To: "snk_rce", Type: graph.DataFlowEdge})

	eng := New(g)
	paths := eng.FindAllPaths()

	if len(paths) < 2 {
		t.Fatalf("expected at least 2 paths, got %d", len(paths))
	}
	// Critical (rce) should come before medium (xss)
	if paths[0].Severity != "critical" {
		t.Errorf("expected first path severity critical, got %q", paths[0].Severity)
	}
}

func TestInterproceduralTaintPath(t *testing.T) {
	g := buildTestGraph(t)

	// src (SourceNode, tainted_vars=name) → DataFlow → caller FunctionNode
	// caller → CallEdge (args=name) → callee FunctionNode (params=q)
	// callee → DataFlow → sink (SinkNode, sqli)
	g.AddNode(&graph.Node{
		ID: "src", Type: graph.SourceNode, FilePath: "app.py", LineStart: 1,
		Symbol: "name", Metadata: map[string]string{"tainted_vars": "name"},
	})
	g.AddNode(&graph.Node{
		ID: "caller", Type: graph.FunctionNode, FilePath: "app.py", LineStart: 5,
		Symbol: "handle", Metadata: map[string]string{"params": "name"},
	})
	g.AddNode(&graph.Node{
		ID: "callee", Type: graph.FunctionNode, FilePath: "db.py", LineStart: 3,
		Symbol: "run_query", Metadata: map[string]string{"params": "q"},
	})
	g.AddNode(&graph.Node{
		ID: "sink", Type: graph.SinkNode, FilePath: "db.py", LineStart: 10,
		Symbol: "execute", VulnClass: "sqli",
	})

	g.AddEdge(&graph.Edge{ID: "e1", From: "src", To: "caller", Type: graph.DataFlowEdge,
		Metadata: map[string]string{"tainted_var": "name"}})
	g.AddEdge(&graph.Edge{ID: "e2", From: "caller", To: "callee", Type: graph.CallEdge,
		Metadata: map[string]string{"args": "name", "call_line": "7"}})
	g.AddEdge(&graph.Edge{ID: "e3", From: "callee", To: "sink", Type: graph.DataFlowEdge})

	eng := New(g)
	paths := eng.FindAllPaths()

	if len(paths) == 0 {
		t.Fatal("expected at least 1 inter-procedural taint path")
	}
	if paths[0].Sink.ID != "sink" {
		t.Errorf("expected sink node, got %q", paths[0].Sink.ID)
	}
	if paths[0].VulnClass != "sqli" {
		t.Errorf("expected sqli, got %q", paths[0].VulnClass)
	}
}

func TestCallEdgeNotCrossedWithoutTaintedArgs(t *testing.T) {
	g := buildTestGraph(t)

	// Source taints "user_input", but call passes "other_var" — taint must not cross
	g.AddNode(&graph.Node{
		ID: "src", Type: graph.SourceNode, FilePath: "app.py", LineStart: 1,
		Symbol: "user_input", Metadata: map[string]string{"tainted_vars": "user_input"},
	})
	g.AddNode(&graph.Node{
		ID: "caller", Type: graph.FunctionNode, FilePath: "app.py", LineStart: 5,
		Symbol: "handle", Metadata: map[string]string{"params": "user_input"},
	})
	g.AddNode(&graph.Node{
		ID: "callee", Type: graph.FunctionNode, FilePath: "db.py", LineStart: 3,
		Symbol: "run_query", Metadata: map[string]string{"params": "q"},
	})
	g.AddNode(&graph.Node{
		ID: "sink", Type: graph.SinkNode, FilePath: "db.py", LineStart: 10,
		Symbol: "execute", VulnClass: "sqli",
	})

	g.AddEdge(&graph.Edge{ID: "e1", From: "src", To: "caller", Type: graph.DataFlowEdge})
	g.AddEdge(&graph.Edge{ID: "e2", From: "caller", To: "callee", Type: graph.CallEdge,
		Metadata: map[string]string{"args": "other_var"}}) // NOT "user_input"
	g.AddEdge(&graph.Edge{ID: "e3", From: "callee", To: "sink", Type: graph.DataFlowEdge})

	eng := New(g)
	paths := eng.FindAllPaths()

	for _, p := range paths {
		if p.Sink.ID == "sink" {
			t.Error("taint should not cross CallEdge when no tainted args match")
		}
	}
}

func TestAnyTainted(t *testing.T) {
	tests := []struct {
		tainted []string
		args    []string
		want    bool
	}{
		{[]string{"name", "id"}, []string{"name"}, true},
		{[]string{"name"}, []string{"other"}, false},
		{[]string{}, []string{"name"}, false},
		{[]string{"x"}, []string{}, false},
		{[]string{"a", "b"}, []string{"c", "b"}, true},
	}
	for _, tt := range tests {
		got := anyTainted(tt.tainted, tt.args)
		if got != tt.want {
			t.Errorf("anyTainted(%v, %v) = %v, want %v", tt.tainted, tt.args, got, tt.want)
		}
	}
}

// ---- helpers ----

func buildTestGraph(t *testing.T) *graph.SPG {
	t.Helper()
	dir := t.TempDir()
	g, err := graph.Open(dir, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

func addNode(g *graph.SPG, id string, typ graph.NodeType, file string, line int, symbol string) {
	g.AddNode(&graph.Node{ID: id, Type: typ, FilePath: file, LineStart: line, Symbol: symbol})
}

func addNodeWithVuln(g *graph.SPG, id string, typ graph.NodeType, file string, line int, symbol, vulnClass string) {
	g.AddNode(&graph.Node{ID: id, Type: typ, FilePath: file, LineStart: line, Symbol: symbol, VulnClass: vulnClass})
}
