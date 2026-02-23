package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RulesHandler struct {
	Pool *pgxpool.Pool
}

type RuleResponse struct {
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Regulation  string   `json:"regulation"`
	Article     string   `json:"article"`
	Paragraph   string   `json:"paragraph"`
	Title       string   `json:"title"`
	Requirement string   `json:"requirement"`
	SourceText  string   `json:"source_text"`
	CheckType   string   `json:"check_type"`
	Severity    string   `json:"severity"`
	Languages   []string `json:"languages"`
	Status      string   `json:"status"`
}

func (h *RulesHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.Pool.Query(ctx,
		`SELECT id, version, regulation, COALESCE(article, ''), COALESCE(paragraph, ''),
		        title, requirement, source_text, check_type, severity, languages, status
		 FROM compliance_rules
		 WHERE status = 'active'
		 ORDER BY regulation, id`)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	rules := []RuleResponse{}
	for rows.Next() {
		var rule RuleResponse
		if err := rows.Scan(&rule.ID, &rule.Version, &rule.Regulation,
			&rule.Article, &rule.Paragraph, &rule.Title, &rule.Requirement,
			&rule.SourceText, &rule.CheckType, &rule.Severity,
			&rule.Languages, &rule.Status); err != nil {
			continue
		}
		rules = append(rules, rule)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}
