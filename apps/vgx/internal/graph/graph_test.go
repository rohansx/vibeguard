package graph

import (
	"testing"

	"github.com/vibeguard/vgx/internal/parser"
)

func TestSPGAddAndRetrieveNodes(t *testing.T) {
	g := openTestGraph(t)

	n := &Node{ID: "n1", Type: SourceNode, FilePath: "app.py", LineStart: 10, Symbol: "request.args"}
	if err := g.AddNode(n); err != nil {
		t.Fatal(err)
	}

	got := g.NodeByID("n1")
	if got == nil {
		t.Fatal("expected node n1")
	}
	if got.Symbol != "request.args" {
		t.Errorf("got symbol %q, want %q", got.Symbol, "request.args")
	}
}

func TestSPGAddEdge(t *testing.T) {
	g := openTestGraph(t)

	g.AddNode(&Node{ID: "src", Type: SourceNode, FilePath: "a.py", LineStart: 1, Symbol: "input"})
	g.AddNode(&Node{ID: "snk", Type: SinkNode, FilePath: "a.py", LineStart: 5, Symbol: "execute"})

	e := &Edge{ID: "e1", From: "src", To: "snk", Type: DataFlowEdge}
	if err := g.AddEdge(e); err != nil {
		t.Fatal(err)
	}

	out := g.OutEdges("src")
	if len(out) != 1 {
		t.Fatalf("expected 1 outgoing edge, got %d", len(out))
	}
	if out[0].To != "snk" {
		t.Errorf("edge.To = %q, want %q", out[0].To, "snk")
	}

	in := g.InEdges("snk")
	if len(in) != 1 {
		t.Fatalf("expected 1 incoming edge, got %d", len(in))
	}
}

func TestSPGNodesByType(t *testing.T) {
	g := openTestGraph(t)

	g.AddNode(&Node{ID: "s1", Type: SourceNode, FilePath: "a.py", LineStart: 1})
	g.AddNode(&Node{ID: "s2", Type: SourceNode, FilePath: "b.py", LineStart: 1})
	g.AddNode(&Node{ID: "k1", Type: SinkNode, FilePath: "a.py", LineStart: 5})

	sources := g.NodesByType(SourceNode)
	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}

	sinks := g.NodesByType(SinkNode)
	if len(sinks) != 1 {
		t.Errorf("expected 1 sink, got %d", len(sinks))
	}
}

func TestSPGNodesByFile(t *testing.T) {
	g := openTestGraph(t)

	g.AddNode(&Node{ID: "n1", Type: SourceNode, FilePath: "a.py", LineStart: 1})
	g.AddNode(&Node{ID: "n2", Type: SinkNode, FilePath: "a.py", LineStart: 5})
	g.AddNode(&Node{ID: "n3", Type: SourceNode, FilePath: "b.py", LineStart: 1})

	nodes := g.NodesByFile("a.py")
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes in a.py, got %d", len(nodes))
	}
}

func TestSPGRemoveFile(t *testing.T) {
	g := openTestGraph(t)

	g.AddNode(&Node{ID: "n1", Type: SourceNode, FilePath: "a.py", LineStart: 1})
	g.AddNode(&Node{ID: "n2", Type: SinkNode, FilePath: "a.py", LineStart: 5})
	g.AddNode(&Node{ID: "n3", Type: SourceNode, FilePath: "b.py", LineStart: 1})

	g.AddEdge(&Edge{ID: "e1", From: "n1", To: "n2", Type: DataFlowEdge})

	if err := g.RemoveFile("a.py"); err != nil {
		t.Fatal(err)
	}

	if g.NodeByID("n1") != nil {
		t.Error("n1 should be removed")
	}
	if g.NodeByID("n2") != nil {
		t.Error("n2 should be removed")
	}
	if g.NodeByID("n3") == nil {
		t.Error("n3 should still exist")
	}

	stats := g.Stats()
	if stats["total_nodes"] != 1 {
		t.Errorf("expected 1 node remaining, got %d", stats["total_nodes"])
	}
}

func TestSPGStats(t *testing.T) {
	g := openTestGraph(t)

	g.AddNode(&Node{ID: "s1", Type: SourceNode, FilePath: "a.py", LineStart: 1})
	g.AddNode(&Node{ID: "k1", Type: SinkNode, FilePath: "a.py", LineStart: 5})
	g.AddNode(&Node{ID: "z1", Type: SanitizerNode, FilePath: "a.py", LineStart: 3})
	g.AddEdge(&Edge{ID: "e1", From: "s1", To: "k1", Type: DataFlowEdge})

	stats := g.Stats()
	if stats["total_nodes"] != 3 {
		t.Errorf("total_nodes = %d, want 3", stats["total_nodes"])
	}
	if stats["total_edges"] != 1 {
		t.Errorf("total_edges = %d, want 1", stats["total_edges"])
	}
	if stats["SourceNode"] != 1 {
		t.Errorf("SourceNode = %d, want 1", stats["SourceNode"])
	}
}

func TestSPGPersistence(t *testing.T) {
	dir := t.TempDir()

	// Write data
	g1, err := Open(dir, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	g1.AddNode(&Node{ID: "p1", Type: SourceNode, FilePath: "x.py", LineStart: 1, Symbol: "req.args"})
	g1.Close()

	// Re-open and verify
	g2, err := Open(dir, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	defer g2.Close()

	got := g2.NodeByID("p1")
	if got == nil {
		t.Fatal("expected node p1 after re-open")
	}
	if got.Symbol != "req.args" {
		t.Errorf("got symbol %q, want %q", got.Symbol, "req.args")
	}
}

func TestBuildFromFile(t *testing.T) {
	g := openTestGraph(t)

	pf := &parser.ParsedFile{
		Path:     "/repo/app.py",
		Language: parser.Python,
		Hash:     "abc123",
		Nodes: []parser.ParsedNode{
			{Kind: parser.KindSource, Symbol: "request.args.get", VulnClass: "", Framework: "flask", Line: 5, Text: `name = request.args.get("name")`, TaintedVars: []string{"name"}},
			{Kind: parser.KindSink, Symbol: "cursor.execute", VulnClass: "sqli", Framework: "", Line: 10, Text: `cursor.execute("SELECT * FROM users WHERE name = '" + name + "'")`, TaintedVars: nil},
		},
	}

	if err := BuildFromFile(g, pf); err != nil {
		t.Fatal(err)
	}

	sources := g.NodesByType(SourceNode)
	sinks := g.NodesByType(SinkNode)

	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
	if len(sinks) != 1 {
		t.Errorf("expected 1 sink, got %d", len(sinks))
	}

	// Should have a data-flow edge because "name" appears in the sink text
	srcID := sources[0].ID
	edges := g.OutEdges(srcID)
	if len(edges) != 1 {
		t.Errorf("expected 1 data-flow edge from source, got %d", len(edges))
	}
}

func TestBuildFromFileIdempotent(t *testing.T) {
	g := openTestGraph(t)

	pf := &parser.ParsedFile{
		Path:     "/repo/app.py",
		Language: parser.Python,
		Hash:     "abc123",
		Nodes: []parser.ParsedNode{
			{Kind: parser.KindSource, Symbol: "input", Line: 1, TaintedVars: []string{"x"}},
		},
	}

	BuildFromFile(g, pf)
	BuildFromFile(g, pf) // rebuild same file

	sources := g.NodesByType(SourceNode)
	if len(sources) != 1 {
		t.Errorf("expected 1 source after rebuild, got %d (not idempotent)", len(sources))
	}
}

func TestMakeNodeID(t *testing.T) {
	id1 := MakeNodeID("/repo", "/repo/app.py", 10, "func")
	id2 := MakeNodeID("/repo", "/repo/app.py", 10, "func")
	id3 := MakeNodeID("/repo", "/repo/app.py", 11, "func")

	if id1 != id2 {
		t.Error("same inputs should produce same ID")
	}
	if id1 == id3 {
		t.Error("different line should produce different ID")
	}
}

func TestSeverityForVulnClass(t *testing.T) {
	tests := []struct {
		cls  string
		want string
	}{
		{"rce", "critical"},
		{"deserialization", "critical"},
		{"sqli", "high"},
		{"ssrf", "high"},
		{"xss", "medium"},
		{"path_traversal", "medium"},
		{"other", "low"},
	}
	for _, tt := range tests {
		got := SeverityForVulnClass(tt.cls)
		if got != tt.want {
			t.Errorf("SeverityForVulnClass(%q) = %q, want %q", tt.cls, got, tt.want)
		}
	}
}

func TestFunctionNodeRegistration(t *testing.T) {
	g := openTestGraph(t)

	pf := &parser.ParsedFile{
		Path:     "/repo/service.py",
		Language: parser.Python,
		Hash:     "abc",
		Functions: []parser.FuncDef{
			{Name: "handle_request", Line: 5, Params: []string{"user_id", "data"}},
			{Name: "validate_input", Line: 20, Params: []string{"val"}},
		},
	}

	if err := BuildFromFile(g, pf); err != nil {
		t.Fatal(err)
	}

	reg := g.FuncRegistry()
	if _, ok := reg["handle_request"]; !ok {
		t.Error("expected handle_request in funcRegistry")
	}
	if _, ok := reg["validate_input"]; !ok {
		t.Error("expected validate_input in funcRegistry")
	}

	fnodes := g.NodesByType(FunctionNode)
	if len(fnodes) != 2 {
		t.Errorf("expected 2 FunctionNodes, got %d", len(fnodes))
	}
}

func TestCrossFileCallEdge(t *testing.T) {
	g := openTestGraph(t)

	// Parse file A — defines callee function
	pfA := &parser.ParsedFile{
		Path:     "/repo/db.py",
		Language: parser.Python,
		Hash:     "aaa",
		Functions: []parser.FuncDef{
			{Name: "run_query", Line: 3, Params: []string{"q"}},
		},
	}
	if err := BuildFromFile(g, pfA); err != nil {
		t.Fatal(err)
	}

	// Parse file B — calls the function from file A
	pfB := &parser.ParsedFile{
		Path:     "/repo/app.py",
		Language: parser.Python,
		Hash:     "bbb",
		Functions: []parser.FuncDef{
			{Name: "handle", Line: 1, Params: []string{"request"}},
		},
		CallSites: []parser.CallSite{
			{CallerFunc: "handle", Callee: "run_query", ArgNames: []string{"name"}, Line: 5},
		},
	}
	if err := BuildFromFile(g, pfB); err != nil {
		t.Fatal(err)
	}

	// handle FunctionNode should have a CallEdge → run_query FunctionNode
	reg := g.FuncRegistry()
	handleID, ok := reg["handle"]
	if !ok {
		t.Fatal("handle not in funcRegistry")
	}
	edges := g.OutEdges(handleID)
	callEdges := 0
	for _, e := range edges {
		if e.Type == CallEdge {
			callEdges++
		}
	}
	if callEdges == 0 {
		t.Error("expected at least one CallEdge from handle → run_query")
	}
}

func TestFuncRegistryCleanupOnRemoveFile(t *testing.T) {
	g := openTestGraph(t)

	pf := &parser.ParsedFile{
		Path:     "/repo/utils.py",
		Language: parser.Python,
		Hash:     "abc",
		Functions: []parser.FuncDef{
			{Name: "helper_func", Line: 10, Params: []string{"x"}},
		},
	}
	if err := BuildFromFile(g, pf); err != nil {
		t.Fatal(err)
	}

	if _, ok := g.FuncRegistry()["helper_func"]; !ok {
		t.Fatal("helper_func should be registered before removal")
	}

	if err := g.RemoveFile("/repo/utils.py"); err != nil {
		t.Fatal(err)
	}

	if _, ok := g.FuncRegistry()["helper_func"]; ok {
		t.Error("helper_func should be removed from funcRegistry after RemoveFile")
	}
}

func openTestGraph(t *testing.T) *SPG {
	t.Helper()
	dir := t.TempDir()
	g, err := Open(dir, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}
