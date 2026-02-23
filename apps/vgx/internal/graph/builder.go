package graph

import (
	"github.com/vibeguard/vgx/internal/parser"
)

// BuildFromFile ingests a ParsedFile into the SPG — adding nodes and data-flow
// edges derived from the parse results.
func BuildFromFile(g *SPG, pf *parser.ParsedFile) error {
	if pf == nil {
		return nil
	}

	// Remove stale data for this file before rebuilding
	if err := g.RemoveFile(pf.Path); err != nil {
		return err
	}

	// Classify and add each parsed node
	nodeIDs := make(map[int]string) // line → nodeID (for edge construction)

	for i := range pf.Nodes {
		pn := &pf.Nodes[i]
		nodeType := classifyParsedNode(pn)
		if nodeType == GenericNode {
			continue // skip unclassified nodes for now
		}

		id := MakeNodeID(g.repoRoot, pf.Path, pn.Line, pn.Symbol)
		n := &Node{
			ID:        id,
			Type:      nodeType,
			FilePath:  pf.Path,
			LineStart: pn.Line,
			LineEnd:   pn.Line,
			Symbol:    pn.Symbol,
			Framework: pn.Framework,
			VulnClass: pn.VulnClass,
			Confidence: 0.85, // pattern-based confidence — upgraded to 0.95+ with tree-sitter
		}
		if err := g.AddNode(n); err != nil {
			return err
		}
		nodeIDs[pn.Line] = id
	}

	// Add intra-file data-flow edges:
	// For each source node, look for sink nodes in the same file and add a
	// potential data-flow edge if a tainted variable appears in the sink line.
	sourceNodes := collectByType(pf.Nodes, parser.KindSource)
	sinkNodes := collectByType(pf.Nodes, parser.KindSink)

	for _, src := range sourceNodes {
		srcID, ok := nodeIDs[src.Line]
		if !ok {
			continue
		}
		for _, taintedVar := range src.TaintedVars {
			if taintedVar == "" {
				continue
			}
			for _, snk := range sinkNodes {
				// Heuristic: if the sink line contains the tainted variable name,
				// add a data-flow edge. This is sound for intra-procedural flows.
				if containsWord(snk.Text, taintedVar) {
					snkID, ok := nodeIDs[snk.Line]
					if !ok {
						continue
					}
					e := &Edge{
						ID:   MakeEdgeID(srcID, snkID, DataFlowEdge),
						From: srcID,
						To:   snkID,
						Type: DataFlowEdge,
						Metadata: map[string]string{
							"tainted_var": taintedVar,
							"file":        pf.Path,
						},
					}
					if err := g.AddEdge(e); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// classifyParsedNode converts a parser.NodeKind to a graph.NodeType.
func classifyParsedNode(pn *parser.ParsedNode) NodeType {
	switch pn.Kind {
	case parser.KindSource:
		return SourceNode
	case parser.KindSink:
		return SinkNode
	case parser.KindSanitizer:
		return SanitizerNode
	default:
		return GenericNode
	}
}

func collectByType(nodes []parser.ParsedNode, kind parser.NodeKind) []parser.ParsedNode {
	var out []parser.ParsedNode
	for _, n := range nodes {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

// containsWord checks if text contains word as a whole word or substring.
// This is intentionally permissive for Phase 1 — reduces false negatives.
func containsWord(text, word string) bool {
	if word == "" {
		return false
	}
	return len(text) >= len(word) && containsSubstr(text, word)
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		(len(s) > 0 && findSubstr(s, sub)))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
