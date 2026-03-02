// Package graph implements the Security Property Graph (SPG) — the core data
// structure that encodes security semantics of a codebase.
package graph

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
)

// NodeType classifies the security role of a graph node.
type NodeType string

const (
	SourceNode    NodeType = "SourceNode"    // untrusted attacker-controlled entry
	SinkNode      NodeType = "SinkNode"      // dangerous operation
	SanitizerNode NodeType = "SanitizerNode" // validation / escaping
	FunctionNode  NodeType = "FunctionNode"  // function definition (inter-procedural call graph)
	TrustBoundary NodeType = "TrustBoundaryEdge"
	AuthCritical  NodeType = "AuthCriticalNode"
	GenericNode   NodeType = "GenericNode"
)

// EdgeType classifies the relationship between two nodes.
type EdgeType string

const (
	CallEdge       EdgeType = "call"       // function A calls function B
	DataFlowEdge   EdgeType = "dataflow"   // data flows from A to B
	ReturnEdge     EdgeType = "return"     // return value flows to caller
	AssignmentEdge EdgeType = "assignment" // variable assigned from expression
)

// Node is a vertex in the SPG.
type Node struct {
	ID         string            // stable hash: sha256(repo+file+line+symbol)
	Type       NodeType
	FilePath   string
	LineStart  int
	LineEnd    int
	Symbol     string // function/variable/method name
	Framework  string // fastapi | django | express | gin | chi | etc.
	VulnClass  string // sqli | xss | rce | path_traversal | ssrf | deserialization
	Confidence float64
	Metadata   map[string]string
}

// Edge is a directed edge in the SPG.
type Edge struct {
	ID       string
	From     string // Node.ID
	To       string // Node.ID
	Type     EdgeType
	Metadata map[string]string
}

// TaintPath is a source-to-sink data flow path without an intervening sanitizer.
type TaintPath struct {
	ID          string
	Source      *Node
	Sink        *Node
	Path        []*Node // ordered path from source to sink (inclusive)
	VulnClass   string
	Severity    string // critical | high | medium | low
	Confidence  float64
	FilePath    string // file where sink is located
	Description string
}

// MakeNodeID creates a stable ID for a node.
func MakeNodeID(repoRoot, filePath string, line int, symbol string) string {
	rel, err := filepath.Rel(repoRoot, filePath)
	if err != nil {
		rel = filePath
	}
	raw := fmt.Sprintf("%s:%d:%s", rel, line, symbol)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:8]) // 16-char hex prefix — collision-resistant enough
}

// MakeEdgeID creates a stable ID for an edge.
func MakeEdgeID(from, to string, edgeType EdgeType) string {
	raw := fmt.Sprintf("%s→%s[%s]", from, to, edgeType)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:8])
}

// SeverityForVulnClass returns the default severity for a vulnerability class.
func SeverityForVulnClass(vulnClass string) string {
	switch vulnClass {
	case "rce", "deserialization":
		return "critical"
	case "sqli", "ssrf":
		return "high"
	case "xss", "path_traversal":
		return "medium"
	default:
		return "low"
	}
}
