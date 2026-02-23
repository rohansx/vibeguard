package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Finding represents a single compliance violation found during scanning.
type Finding struct {
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// ScanResult holds the aggregated scan output.
type ScanResult struct {
	ScanID            string    `json:"scan_id"`
	Status            string    `json:"status"`
	TotalRulesChecked int       `json:"total_rules_checked"`
	TotalFindings     int       `json:"total_findings"`
	CriticalFindings  int       `json:"critical_findings"`
	HighFindings      int       `json:"high_findings"`
	MediumFindings    int       `json:"medium_findings"`
	LowFindings       int       `json:"low_findings"`
	ComplianceScore   float64   `json:"compliance_score"`
	Findings          []Finding `json:"findings"`
	Duration          float64   `json:"duration_seconds"`
}

// complianceRule represents a rule loaded from the database.
type complianceRule struct {
	ID              string
	Regulation      string
	Article         string
	Title           string
	Severity        string
	CheckType       string
	Languages       []string
	TechnicalChecks json.RawMessage
}

// astGrepRule represents an ast-grep rule for writing to disk.
type astGrepRule struct {
	ID       string      `json:"id"`
	Language string      `json:"language"`
	Rule     interface{} `json:"rule"`
	Message  string      `json:"message"`
	Severity string      `json:"severity"`
}

// Scanner orchestrates compliance rule execution against codebases.
type Scanner struct {
	Pool *pgxpool.Pool
}

// New creates a new Scanner with a database pool.
func New(pool *pgxpool.Pool) *Scanner {
	return &Scanner{Pool: pool}
}

// Scan runs all active compliance rules against the target directory.
func (s *Scanner) Scan(ctx context.Context, targetDir string, orgID string, repoURL string, branch string) (*ScanResult, error) {
	start := time.Now()

	rules, err := s.loadRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}

	log.Printf("Loaded %d active compliance rules", len(rules))

	var allFindings []Finding

	// Run ast-grep rules
	astGrepFindings, astRuleCount, err := s.runAstGrep(ctx, rules, targetDir)
	if err != nil {
		log.Printf("ast-grep scan error (non-fatal): %v", err)
	} else {
		allFindings = append(allFindings, astGrepFindings...)
	}

	// Run pattern-based checks for rules without ast-grep patterns
	patternFindings, patternRuleCount, err := s.runPatternChecks(rules, targetDir)
	if err != nil {
		log.Printf("pattern check error (non-fatal): %v", err)
	} else {
		allFindings = append(allFindings, patternFindings...)
	}

	totalRulesChecked := astRuleCount + patternRuleCount

	var critical, high, medium, low int
	for _, f := range allFindings {
		switch f.Severity {
		case "critical":
			critical++
		case "high":
			high++
		case "medium":
			medium++
		case "low":
			low++
		}
	}

	score := computeScore(totalRulesChecked, critical, high, medium, low)
	duration := time.Since(start).Seconds()

	result := &ScanResult{
		Status:            "completed",
		TotalRulesChecked: totalRulesChecked,
		TotalFindings:     len(allFindings),
		CriticalFindings:  critical,
		HighFindings:      high,
		MediumFindings:    medium,
		LowFindings:       low,
		ComplianceScore:   score,
		Findings:          allFindings,
		Duration:          duration,
	}

	// Store scan result in DB
	if s.Pool != nil {
		scanID, storeErr := s.storeScanResult(ctx, result, orgID, repoURL, branch)
		if storeErr != nil {
			log.Printf("Failed to store scan result: %v", storeErr)
		} else {
			result.ScanID = scanID
		}
	}

	log.Printf("Scan complete: %d findings (%d critical, %d high, %d medium, %d low), score: %.1f%%",
		len(allFindings), critical, high, medium, low, score)

	return result, nil
}

func (s *Scanner) loadRules(ctx context.Context) ([]complianceRule, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, regulation, article, title, severity, check_type, languages, technical_checks
		FROM compliance_rules
		WHERE status = 'active'
		ORDER BY severity DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []complianceRule
	for rows.Next() {
		var r complianceRule
		if err := rows.Scan(&r.ID, &r.Regulation, &r.Article, &r.Title, &r.Severity, &r.CheckType, &r.Languages, &r.TechnicalChecks); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *Scanner) runAstGrep(ctx context.Context, rules []complianceRule, targetDir string) ([]Finding, int, error) {
	tmpDir, err := os.MkdirTemp("", "vgx-rules-*")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	rulesDir := filepath.Join(tmpDir, "rules")
	if err := os.MkdirAll(rulesDir, 0755); err != nil {
		return nil, 0, fmt.Errorf("failed to create rules subdir: %w", err)
	}

	ruleCount := 0
	for _, rule := range rules {
		var tc map[string]json.RawMessage
		if err := json.Unmarshal(rule.TechnicalChecks, &tc); err != nil {
			continue
		}

		rawRules, ok := tc["ast_grep_rules"]
		if !ok {
			continue
		}

		var agRules []astGrepRule
		if err := json.Unmarshal(rawRules, &agRules); err != nil {
			continue
		}

		for i, agRule := range agRules {
			if agRule.ID == "" {
				agRule.ID = fmt.Sprintf("%s-%d", rule.ID, i)
			}

			// Write as YAML (ast-grep's native format), quoting strings properly
			yaml := fmt.Sprintf("id: %s\nlanguage: %s\nmessage: %q\nseverity: %s\nrule:\n",
				agRule.ID, agRule.Language, agRule.Message, agRule.Severity)
			yaml += ruleToYAML(agRule.Rule, 2)

			ruleFile := filepath.Join(rulesDir, fmt.Sprintf("%s-%d.yml", rule.ID, i))
			if err := os.WriteFile(ruleFile, []byte(yaml), 0644); err != nil {
				continue
			}
			ruleCount++
		}
	}

	if ruleCount == 0 {
		return nil, 0, nil
	}

	// Write sgconfig.yml pointing to rules directory
	sgConfig := "ruleDirs:\n  - rules\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "sgconfig.yml"), []byte(sgConfig), 0644); err != nil {
		return nil, ruleCount, fmt.Errorf("failed to write sgconfig.yml: %w", err)
	}

	// Run ast-grep scan using the config file
	cmd := exec.CommandContext(ctx, "sg", "scan", "--config", filepath.Join(tmpDir, "sgconfig.yml"), targetDir, "--json=stream")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// ast-grep returns exit code 1 when findings exist — that's normal
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Findings exist, output contains the results
		} else {
			return nil, ruleCount, fmt.Errorf("ast-grep exec failed: %w\noutput: %s", err, string(output))
		}
	}

	// Parse newline-delimited JSON
	var findings []Finding
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] != '{' {
			continue
		}

		var match struct {
			RuleID   string `json:"ruleId"`
			Message  string `json:"message"`
			Severity string `json:"severity"`
			File     string `json:"file"`
			Range    struct {
				Start struct {
					Line int `json:"line"`
				} `json:"start"`
			} `json:"range"`
		}
		if err := json.Unmarshal([]byte(line), &match); err != nil {
			continue
		}

		relFile := match.File
		if strings.HasPrefix(relFile, targetDir) {
			relFile = strings.TrimPrefix(relFile, targetDir)
			relFile = strings.TrimPrefix(relFile, "/")
		}

		severity := match.Severity
		if severity == "error" {
			severity = "critical"
		} else if severity == "warning" {
			severity = "high"
		}

		findings = append(findings, Finding{
			RuleID:   match.RuleID,
			Severity: severity,
			Status:   "fail",
			Message:  match.Message,
			File:     relFile,
			Line:     match.Range.Start.Line,
		})
	}

	return findings, ruleCount, nil
}

// ruleToYAML converts a rule interface (from JSON) to YAML string with the given indent level.
func ruleToYAML(v interface{}, indent int) string {
	prefix := strings.Repeat(" ", indent)
	switch val := v.(type) {
	case map[string]interface{}:
		var sb strings.Builder
		for k, vv := range val {
			switch inner := vv.(type) {
			case string:
				sb.WriteString(fmt.Sprintf("%s%s: %q\n", prefix, k, inner))
			case []interface{}:
				sb.WriteString(fmt.Sprintf("%s%s:\n", prefix, k))
				for _, item := range inner {
					if s, ok := item.(string); ok {
						sb.WriteString(fmt.Sprintf("%s  - %q\n", prefix, s))
					} else {
						sb.WriteString(fmt.Sprintf("%s  -\n", prefix))
						sb.WriteString(ruleToYAML(item, indent+4))
					}
				}
			case map[string]interface{}:
				sb.WriteString(fmt.Sprintf("%s%s:\n", prefix, k))
				sb.WriteString(ruleToYAML(inner, indent+2))
			default:
				sb.WriteString(fmt.Sprintf("%s%s: %v\n", prefix, k, vv))
			}
		}
		return sb.String()
	case string:
		return fmt.Sprintf("%s%q\n", prefix, val)
	default:
		return fmt.Sprintf("%s%v\n", prefix, v)
	}
}

func (s *Scanner) runPatternChecks(rules []complianceRule, targetDir string) ([]Finding, int, error) {
	var findings []Finding
	ruleCount := 0

	for _, rule := range rules {
		var tc map[string]json.RawMessage
		if err := json.Unmarshal(rule.TechnicalChecks, &tc); err != nil {
			continue
		}

		// Skip rules that have ast-grep rules (handled separately)
		if _, ok := tc["ast_grep_rules"]; ok {
			continue
		}

		var checks struct {
			Checks []string `json:"checks"`
		}
		if err := json.Unmarshal(rule.TechnicalChecks, &checks); err != nil || len(checks.Checks) == 0 {
			continue
		}

		ruleCount++

		for _, check := range checks.Checks {
			pattern := checkToPattern(check)
			if pattern == "" {
				continue
			}

			matches := grepDirectory(targetDir, pattern)
			if len(matches) == 0 {
				findings = append(findings, Finding{
					RuleID:   rule.ID,
					Severity: rule.Severity,
					Status:   "fail",
					Message:  fmt.Sprintf("%s: pattern '%s' not found in codebase", rule.Title, check),
					File:     ".",
					Line:     0,
				})
			}
		}
	}

	return findings, ruleCount, nil
}

func checkToPattern(check string) string {
	patterns := map[string]string{
		"tls_version_check":            "TLS|tls|ssl|SSL",
		"certificate_validation":       "certificate|cert|x509",
		"hsts_header":                  "Strict-Transport-Security|HSTS|hsts",
		"consent_banner_present":       "consent|Consent|cookie.*banner",
		"consent_record_stored":        "consent.*record|consent.*log|consent.*store",
		"consent_withdrawal_mechanism": "withdraw.*consent|revoke.*consent|opt.out",
		"retention_policy_defined":     "retention|TTL|ttl|expir",
		"auto_deletion_enabled":        "delete.*schedule|purge|cleanup.*cron|auto.*delete",
		"access_log_enabled":           "audit.*log|access.*log|event.*log",
		"log_tamper_protection":        "append.only|immutable|tamper",
		"db_column_encryption":         "encrypt|AES|aes.*256|pgcrypto",
		"cipher_suite_validation":      "cipher|TLS_AES|TLS_CHACHA",
		"certificate_pinning":          "pinning|pin.*cert",
		"log_retention_90_days":        "retention.*90|90.*day.*log",
	}

	if p, ok := patterns[check]; ok {
		return p
	}
	return ""
}

func grepDirectory(dir string, pattern string) []string {
	cmd := exec.Command("grep", "-rl", "-E", pattern, dir)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var matches []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			matches = append(matches, line)
		}
	}
	return matches
}

func computeScore(totalRules, critical, high, medium, low int) float64 {
	if totalRules == 0 {
		return 100.0
	}
	penalty := float64(critical*10+high*5+medium*2+low*1) / float64(totalRules) * 10
	score := 100.0 - penalty
	return math.Max(0, math.Min(100, math.Round(score*100)/100))
}

func (s *Scanner) storeScanResult(ctx context.Context, result *ScanResult, orgID string, repoURL string, branch string) (string, error) {
	findingsJSON, err := json.Marshal(result.Findings)
	if err != nil {
		return "", err
	}
	metaJSON, err := json.Marshal(map[string]interface{}{
		"scanner_version":  "0.1.0",
		"duration_seconds": result.Duration,
	})
	if err != nil {
		return "", err
	}

	var scanID string
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO scan_results (org_id, repository_url, branch, scan_type, status,
			completed_at, total_rules_checked, total_findings,
			critical_findings, high_findings, medium_findings, low_findings,
			compliance_score, findings, scan_meta)
		VALUES ($1, $2, $3, 'full', 'completed', now(), $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id::text
	`, orgID, repoURL, branch,
		result.TotalRulesChecked, result.TotalFindings,
		result.CriticalFindings, result.HighFindings, result.MediumFindings, result.LowFindings,
		result.ComplianceScore, findingsJSON, metaJSON,
	).Scan(&scanID)

	return scanID, err
}
