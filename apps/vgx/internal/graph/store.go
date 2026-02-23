package graph

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var (
	bucketNodes = []byte("nodes")
	bucketEdges = []byte("edges")
	bucketMeta  = []byte("meta")
)

// SPG is the in-memory Security Property Graph with bbolt persistence.
type SPG struct {
	mu       sync.RWMutex
	nodes    map[string]*Node         // nodeID → Node
	outEdges map[string][]*Edge       // nodeID → outgoing edges
	inEdges  map[string][]*Edge       // nodeID → incoming edges (backward traversal)
	db       *bolt.DB
	repoRoot string
	version  string // content hash — changes when graph is modified
}

// Open opens or creates the persistent graph store at storePath.
func Open(storePath, repoRoot string) (*SPG, error) {
	if err := os.MkdirAll(storePath, 0o700); err != nil {
		return nil, fmt.Errorf("creating store dir %s: %w", storePath, err)
	}

	dbPath := filepath.Join(storePath, "spg.db")
	db, err := bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("opening bolt db %s: %w", dbPath, err)
	}

	// Ensure buckets exist
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketNodes, bucketEdges, bucketMeta} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing buckets: %w", err)
	}

	g := &SPG{
		nodes:    make(map[string]*Node),
		outEdges: make(map[string][]*Edge),
		inEdges:  make(map[string][]*Edge),
		db:       db,
		repoRoot: repoRoot,
	}

	// Load existing data from disk into memory
	if err := g.loadFromDisk(); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading graph: %w", err)
	}

	return g, nil
}

// Close flushes and closes the backing store.
func (g *SPG) Close() error {
	if g.db != nil {
		return g.db.Close()
	}
	return nil
}

// AddNode upserts a node into the graph.
func (g *SPG) AddNode(n *Node) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nodes[n.ID] = n
	g.invalidateVersion()
	return g.persistNode(n)
}

// AddEdge adds a directed edge to the graph.
func (g *SPG) AddEdge(e *Edge) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.outEdges[e.From] = append(g.outEdges[e.From], e)
	g.inEdges[e.To] = append(g.inEdges[e.To], e)
	g.invalidateVersion()
	return g.persistEdge(e)
}

// RemoveFile removes all nodes and edges for a given file path.
// Used during incremental updates when a file changes.
func (g *SPG) RemoveFile(filePath string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	var toRemove []string
	for id, n := range g.nodes {
		if n.FilePath == filePath {
			toRemove = append(toRemove, id)
		}
	}
	for _, id := range toRemove {
		delete(g.nodes, id)
		// Remove associated edges
		for _, e := range g.outEdges[id] {
			g.removeFromInEdges(e.To, id)
		}
		delete(g.outEdges, id)
		// Also remove as edge target
		for _, e := range g.inEdges[id] {
			g.removeFromOutEdges(e.From, id)
		}
		delete(g.inEdges, id)
	}

	g.invalidateVersion()
	return g.persistRemoveFile(filePath)
}

// NodeByID returns a node by ID (nil if not found).
func (g *SPG) NodeByID(id string) *Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes[id]
}

// AllNodes returns a snapshot of all nodes.
func (g *SPG) AllNodes() []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make([]*Node, 0, len(g.nodes))
	for _, n := range g.nodes {
		out = append(out, n)
	}
	return out
}

// NodesByType returns all nodes of a given type.
func (g *SPG) NodesByType(t NodeType) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []*Node
	for _, n := range g.nodes {
		if n.Type == t {
			out = append(out, n)
		}
	}
	return out
}

// NodesByFile returns all nodes in a given file.
func (g *SPG) NodesByFile(filePath string) []*Node {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var out []*Node
	for _, n := range g.nodes {
		if n.FilePath == filePath {
			out = append(out, n)
		}
	}
	return out
}

// OutEdges returns all outgoing edges from a node.
func (g *SPG) OutEdges(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return append([]*Edge{}, g.outEdges[nodeID]...)
}

// InEdges returns all incoming edges to a node.
func (g *SPG) InEdges(nodeID string) []*Edge {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return append([]*Edge{}, g.inEdges[nodeID]...)
}

// Version returns the current content hash of the graph.
func (g *SPG) Version() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.version
}

// Stats returns counts of nodes and edges by type.
func (g *SPG) Stats() map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := map[string]int{
		"total_nodes": len(g.nodes),
	}
	edgeCount := 0
	for _, edges := range g.outEdges {
		edgeCount += len(edges)
	}
	out["total_edges"] = edgeCount
	for _, n := range g.nodes {
		out[string(n.Type)]++
	}
	return out
}

// ---- persistence helpers ----

func (g *SPG) persistNode(n *Node) error {
	data, err := json.Marshal(n)
	if err != nil {
		return err
	}
	return g.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketNodes).Put([]byte(n.ID), data)
	})
}

func (g *SPG) persistEdge(e *Edge) error {
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return g.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketEdges).Put([]byte(e.ID), data)
	})
}

func (g *SPG) persistRemoveFile(filePath string) error {
	return g.db.Update(func(tx *bolt.Tx) error {
		nb := tx.Bucket(bucketNodes)
		var nodeKeys [][]byte
		nb.ForEach(func(k, v []byte) error {
			var n Node
			if json.Unmarshal(v, &n) == nil && n.FilePath == filePath {
				nodeKeys = append(nodeKeys, append([]byte{}, k...))
			}
			return nil
		})
		for _, k := range nodeKeys {
			if err := nb.Delete(k); err != nil {
				return err
			}
		}
		// Clean up edges referencing deleted nodes (best effort)
		return nil
	})
}

func (g *SPG) loadFromDisk() error {
	return g.db.View(func(tx *bolt.Tx) error {
		// Load nodes
		tx.Bucket(bucketNodes).ForEach(func(k, v []byte) error {
			var n Node
			if err := json.Unmarshal(v, &n); err == nil {
				g.nodes[n.ID] = &n
			}
			return nil
		})
		// Load edges
		tx.Bucket(bucketEdges).ForEach(func(k, v []byte) error {
			var e Edge
			if err := json.Unmarshal(v, &e); err == nil {
				g.outEdges[e.From] = append(g.outEdges[e.From], &e)
				g.inEdges[e.To] = append(g.inEdges[e.To], &e)
			}
			return nil
		})
		return nil
	})
}

func (g *SPG) invalidateVersion() {
	// Simple version: hash of node count + current time (good enough for incremental detection)
	raw := fmt.Sprintf("%d:%d", len(g.nodes), time.Now().UnixNano())
	sum := sha256.Sum256([]byte(raw))
	g.version = fmt.Sprintf("%x", sum[:8])
}

func (g *SPG) removeFromInEdges(nodeID, fromID string) {
	edges := g.inEdges[nodeID]
	filtered := edges[:0]
	for _, e := range edges {
		if e.From != fromID {
			filtered = append(filtered, e)
		}
	}
	g.inEdges[nodeID] = filtered
}

func (g *SPG) removeFromOutEdges(nodeID, toID string) {
	edges := g.outEdges[nodeID]
	filtered := edges[:0]
	for _, e := range edges {
		if e.To != toID {
			filtered = append(filtered, e)
		}
	}
	g.outEdges[nodeID] = filtered
}
