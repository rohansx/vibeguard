package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vibeguard/vgx/internal/scanner"
)

type ScansHandler struct {
	Pool *pgxpool.Pool
}

type ScanResponse struct {
	ID               string           `json:"id"`
	RepositoryURL    string           `json:"repository_url"`
	Branch           string           `json:"branch"`
	Status           string           `json:"status"`
	TotalFindings    int              `json:"total_findings"`
	CriticalFindings int              `json:"critical_findings"`
	HighFindings     int              `json:"high_findings"`
	MediumFindings   int              `json:"medium_findings"`
	LowFindings      int              `json:"low_findings"`
	ComplianceScore  *float64         `json:"compliance_score"`
	StartedAt        string           `json:"started_at"`
	CompletedAt      *string          `json:"completed_at"`
	Findings         *json.RawMessage `json:"findings,omitempty"`
}

func (h *ScansHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx,
		`SELECT id::text, COALESCE(repository_url, ''), COALESCE(branch, ''),
		        status, total_findings, critical_findings, high_findings,
		        medium_findings, low_findings, compliance_score,
		        started_at::text, completed_at::text
		 FROM scan_results
		 ORDER BY started_at DESC`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	scans := []ScanResponse{}
	for rows.Next() {
		var scan ScanResponse
		if err := rows.Scan(&scan.ID, &scan.RepositoryURL, &scan.Branch,
			&scan.Status, &scan.TotalFindings, &scan.CriticalFindings,
			&scan.HighFindings, &scan.MediumFindings, &scan.LowFindings,
			&scan.ComplianceScore, &scan.StartedAt, &scan.CompletedAt); err != nil {
			continue
		}
		scans = append(scans, scan)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scans)
}

// Create handles POST /scans — triggers a scan against a repository.
func (h *ScansHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		RepositoryURL string `json:"repository_url"`
		Branch        string `json:"branch"`
		OrgID         string `json:"org_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	if req.RepositoryURL == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "repository_url is required"})
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.OrgID == "" {
		req.OrgID = "org_demo_001"
	}

	// Create a "running" scan record
	var scanID string
	err := h.Pool.QueryRow(ctx, `
		INSERT INTO scan_results (org_id, repository_url, branch, scan_type, status)
		VALUES ($1, $2, $3, 'full', 'running')
		RETURNING id::text
	`, req.OrgID, req.RepositoryURL, req.Branch).Scan(&scanID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Run scan in background goroutine
	go func() {
		tmpDir, err := os.MkdirTemp("", "vgx-scan-*")
		if err != nil {
			log.Printf("Failed to create temp dir for scan %s: %v", scanID, err)
			h.failScan(scanID, err.Error())
			return
		}
		defer os.RemoveAll(tmpDir)

		// Clone repository
		cloneCmd := exec.Command("git", "clone", "--depth", "1", "--branch", req.Branch, req.RepositoryURL, tmpDir)
		if output, err := cloneCmd.CombinedOutput(); err != nil {
			log.Printf("Git clone failed for scan %s: %v\n%s", scanID, err, string(output))
			h.failScan(scanID, fmt.Sprintf("git clone failed: %v", err))
			return
		}

		s := scanner.New(h.Pool)
		result, err := s.Scan(ctx, tmpDir, req.OrgID, req.RepositoryURL, req.Branch)
		if err != nil {
			log.Printf("Scan %s failed: %v", scanID, err)
			h.failScan(scanID, err.Error())
			return
		}

		// Update the running scan record with results
		findingsJSON, _ := json.Marshal(result.Findings)
		metaJSON, _ := json.Marshal(map[string]interface{}{
			"scanner_version":  "0.1.0",
			"duration_seconds": result.Duration,
		})

		_, err = h.Pool.Exec(ctx, `
			UPDATE scan_results
			SET status = 'completed', completed_at = now(),
				total_rules_checked = $2, total_findings = $3,
				critical_findings = $4, high_findings = $5,
				medium_findings = $6, low_findings = $7,
				compliance_score = $8, findings = $9, scan_meta = $10
			WHERE id = $1::uuid
		`, scanID,
			result.TotalRulesChecked, result.TotalFindings,
			result.CriticalFindings, result.HighFindings,
			result.MediumFindings, result.LowFindings,
			result.ComplianceScore, findingsJSON, metaJSON,
		)
		if err != nil {
			log.Printf("Failed to update scan %s: %v", scanID, err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"scan_id": scanID,
		"status":  "running",
	})
}

func (h *ScansHandler) failScan(scanID string, errMsg string) {
	metaJSON, _ := json.Marshal(map[string]string{"error": errMsg})
	_, _ = h.Pool.Exec(
		context.Background(),
		`UPDATE scan_results SET status = 'failed', completed_at = now(), scan_meta = $2 WHERE id = $1::uuid`,
		scanID, metaJSON,
	)
}

// Get handles GET /scans/{scanID} — returns scan details with findings.
func (h *ScansHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	scanID := chi.URLParam(r, "scanID")

	var scan ScanResponse
	var findings json.RawMessage
	err := h.Pool.QueryRow(ctx, `
		SELECT id::text, COALESCE(repository_url, ''), COALESCE(branch, ''),
		       status, total_findings, critical_findings, high_findings,
		       medium_findings, low_findings, compliance_score,
		       started_at::text, completed_at::text, findings
		FROM scan_results
		WHERE id = $1::uuid
	`, scanID).Scan(&scan.ID, &scan.RepositoryURL, &scan.Branch,
		&scan.Status, &scan.TotalFindings, &scan.CriticalFindings,
		&scan.HighFindings, &scan.MediumFindings, &scan.LowFindings,
		&scan.ComplianceScore, &scan.StartedAt, &scan.CompletedAt, &findings)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "scan not found"})
		return
	}

	scan.Findings = &findings
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scan)
}
