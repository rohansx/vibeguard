export interface HealthResponse {
  status: string;
  service: string;
  version: string;
}

export interface ComplianceRule {
  id: string;
  version: string;
  regulation: string;
  article: string;
  paragraph: string;
  title: string;
  requirement: string;
  source_text: string;
  check_type: string;
  severity: "critical" | "high" | "medium" | "low";
  languages: string[];
  status: string;
}

export interface Document {
  id: string;
  name: string;
  description: string;
  file_name: string;
  status: "uploaded" | "processing" | "review_ready" | "active" | "extraction_failed";
  rule_count: number;
  approved_count: number;
  created_at: string;
}

export interface ScanResult {
  id: string;
  repository_url: string;
  branch: string;
  status: "running" | "completed" | "failed";
  total_findings: number;
  critical_findings: number;
  high_findings: number;
  medium_findings: number;
  low_findings: number;
  compliance_score: number | null;
  started_at: string;
  completed_at: string | null;
}

export interface ActivityItem {
  event_type: string;
  resource_type: string;
  resource_id: string;
  actor_name: string;
  created_at: string;
}

export interface DashboardSummary {
  total_rules: number;
  total_documents: number;
  total_scans: number;
  compliance_score: number | null;
  critical_findings: number;
  high_findings: number;
  medium_findings: number;
  low_findings: number;
  recent_activity: ActivityItem[];
}
