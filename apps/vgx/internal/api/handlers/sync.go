package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vibeguard/vgx/internal/config"
)

// SyncHandler handles the team sync REST API — mirrors SPG metadata from local
// graph daemons to PostgreSQL for Pro/Team tier shared security posture views.
type SyncHandler struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

func NewSyncHandler(pool *pgxpool.Pool, cfg *config.Config) *SyncHandler {
	return &SyncHandler{Pool: pool, Cfg: cfg}
}

// ListRepositories returns all repositories registered with this org's SPG daemon.
func (h *SyncHandler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	// TODO(phase2): query repositories table with org filter from Clerk JWT
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"repositories": []any{},
		"message":      "team sync API — repository registry implementation lands in Phase 2",
	})
}

// RegisterRepository registers a new repository for SPG tracking.
func (h *SyncHandler) RegisterRepository(w http.ResponseWriter, r *http.Request) {
	// TODO(phase2): insert into repositories table, return repo ID for daemon config
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "team sync API — repository registration implementation lands in Phase 2",
		"code":  "NOT_IMPLEMENTED",
	})
}

// GetRepository returns SPG metadata for a specific repository.
func (h *SyncHandler) GetRepository(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoID")
	_ = repoID
	// TODO(phase2): fetch repository + aggregate spg_nodes + taint_paths counts
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "team sync API — repository detail implementation lands in Phase 2",
		"code":  "NOT_IMPLEMENTED",
	})
}

// ListTaintPaths returns active taint paths for a repository.
func (h *SyncHandler) ListTaintPaths(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoID")
	_ = repoID
	// TODO(phase2): query taint_paths where repo_id = ? AND is_active = true
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"taint_paths": []any{},
		"message":     "team sync API — taint path listing implementation lands in Phase 2",
	})
}

// GetAttackSurface returns the aggregated attack surface for a repository.
func (h *SyncHandler) GetAttackSurface(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoID")
	_ = repoID
	// TODO(phase2): query spg_nodes WHERE node_type = 'SourceNode' AND repo_id = ?
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"sources": []any{},
		"message": "team sync API — attack surface implementation lands in Phase 2",
	})
}

// RecordCalibration records an agent accept/reject signal for the data flywheel.
func (h *SyncHandler) RecordCalibration(w http.ResponseWriter, r *http.Request) {
	repoID := chi.URLParam(r, "repoID")
	_ = repoID

	var body struct {
		TaintPathID string `json:"taint_path_id"`
		Action      string `json:"action"`
		AgentType   string `json:"agent_type"`
		SessionID   string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body", "code": "BAD_REQUEST"})
		return
	}

	// TODO(phase2): INSERT INTO calibration_events ... (append-only)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": "calibration event recording implementation lands in Phase 2",
	})
}
