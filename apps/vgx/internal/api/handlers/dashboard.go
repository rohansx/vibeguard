package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DashboardHandler struct {
	Pool *pgxpool.Pool
}

type DashboardSummary struct {
	TotalRules       int              `json:"total_rules"`
	TotalDocuments   int              `json:"total_documents"`
	TotalScans       int              `json:"total_scans"`
	ComplianceScore  *float64         `json:"compliance_score"`
	CriticalFindings int              `json:"critical_findings"`
	HighFindings     int              `json:"high_findings"`
	MediumFindings   int              `json:"medium_findings"`
	LowFindings      int              `json:"low_findings"`
	RecentActivity   []ActivityItem   `json:"recent_activity"`
}

type ActivityItem struct {
	EventType    string `json:"event_type"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
	ActorName    string `json:"actor_name"`
	CreatedAt    string `json:"created_at"`
}

func (h *DashboardHandler) Summary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var summary DashboardSummary

	h.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM compliance_rules WHERE status = 'active'`,
	).Scan(&summary.TotalRules)

	h.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM documents`,
	).Scan(&summary.TotalDocuments)

	h.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM scan_results`,
	).Scan(&summary.TotalScans)

	h.Pool.QueryRow(ctx,
		`SELECT compliance_score, critical_findings, high_findings, medium_findings, low_findings
		 FROM scan_results WHERE status = 'completed'
		 ORDER BY completed_at DESC LIMIT 1`,
	).Scan(&summary.ComplianceScore, &summary.CriticalFindings,
		&summary.HighFindings, &summary.MediumFindings, &summary.LowFindings)

	rows, err := h.Pool.Query(ctx,
		`SELECT a.event_type, a.resource_type, a.resource_id,
		        COALESCE(u.name, 'system'), a.created_at::text
		 FROM audit_log a
		 LEFT JOIN users u ON u.id = a.actor_id
		 ORDER BY a.created_at DESC LIMIT 10`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var item ActivityItem
			if err := rows.Scan(&item.EventType, &item.ResourceType,
				&item.ResourceID, &item.ActorName, &item.CreatedAt); err == nil {
				summary.RecentActivity = append(summary.RecentActivity, item)
			}
		}
	}
	if summary.RecentActivity == nil {
		summary.RecentActivity = []ActivityItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}
