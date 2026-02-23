package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/vibeguard/vgx/internal/config"
)

type DocumentsHandler struct {
	Pool *pgxpool.Pool
	Cfg  *config.Config
}

type DocumentResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	FileName      string `json:"file_name"`
	Status        string `json:"status"`
	RuleCount     int    `json:"rule_count"`
	ApprovedCount int    `json:"approved_count"`
	CreatedAt     string `json:"created_at"`
}

func (h *DocumentsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx,
		`SELECT id, name, description, file_name, status,
		        rule_count, approved_count, created_at::text
		 FROM documents
		 ORDER BY created_at DESC`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	docs := []DocumentResponse{}
	for rows.Next() {
		var doc DocumentResponse
		if err := rows.Scan(&doc.ID, &doc.Name, &doc.Description,
			&doc.FileName, &doc.Status, &doc.RuleCount,
			&doc.ApprovedCount, &doc.CreatedAt); err != nil {
			continue
		}
		docs = append(docs, doc)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(docs)
}

// Upload handles POST /documents — proxies the multipart upload to the ingestion service.
func (h *DocumentsHandler) Upload(w http.ResponseWriter, r *http.Request) {
	ingestionURL := h.Cfg.IngestionServiceURL + "/ingest"

	// Forward the entire multipart request to the ingestion service
	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, ingestionURL, r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create proxy request"})
		return
	}

	// Copy relevant headers (Content-Type includes the multipart boundary)
	proxyReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		log.Printf("Ingestion proxy failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{"error": "ingestion service unavailable"})
		return
	}
	defer resp.Body.Close()

	// Forward the response from the ingestion service
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
