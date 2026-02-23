import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { HealthResponse, DashboardSummary } from "@/types";

export function DashboardPage() {
  const health = useQuery({
    queryKey: ["health"],
    queryFn: () => apiFetch<HealthResponse>("/health"),
  });

  const summary = useQuery({
    queryKey: ["dashboard-summary"],
    queryFn: () => apiFetch<DashboardSummary>("/dashboard/summary"),
  });

  const score = summary.data?.compliance_score;

  return (
    <div>
      <h2 className="text-2xl font-bold text-foreground mb-6">
        Compliance Dashboard
      </h2>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
        <div className="rounded-lg border border-border bg-card p-6">
          <p className="text-sm text-muted-foreground">Compliance Score</p>
          <p className="text-3xl font-bold text-foreground mt-2">
            {score != null ? `${score}%` : "--"}
          </p>
        </div>
        <div className="rounded-lg border border-border bg-card p-6">
          <p className="text-sm text-muted-foreground">Active Rules</p>
          <p className="text-3xl font-bold text-foreground mt-2">
            {summary.data?.total_rules ?? 0}
          </p>
        </div>
        <div className="rounded-lg border border-border bg-card p-6">
          <p className="text-sm text-muted-foreground">Documents</p>
          <p className="text-3xl font-bold text-foreground mt-2">
            {summary.data?.total_documents ?? 0}
          </p>
        </div>
        <div className="rounded-lg border border-border bg-card p-6">
          <p className="text-sm text-muted-foreground">Total Scans</p>
          <p className="text-3xl font-bold text-foreground mt-2">
            {summary.data?.total_scans ?? 0}
          </p>
        </div>
      </div>

      {summary.data && (
        <div className="rounded-lg border border-border bg-card p-6 mb-8">
          <h3 className="text-lg font-semibold text-foreground mb-4">
            Latest Scan Findings
          </h3>
          <div className="grid grid-cols-4 gap-4">
            {(["critical", "high", "medium", "low"] as const).map((level) => (
              <div key={level} className="text-center">
                <p className="text-xs uppercase tracking-wider text-muted-foreground">
                  {level}
                </p>
                <p className="text-2xl font-bold text-foreground mt-1">
                  {summary.data[`${level}_findings`]}
                </p>
              </div>
            ))}
          </div>
        </div>
      )}

      {summary.data && summary.data.recent_activity.length > 0 && (
        <div className="rounded-lg border border-border bg-card p-6 mb-8">
          <h3 className="text-lg font-semibold text-foreground mb-4">
            Recent Activity
          </h3>
          <div className="space-y-3">
            {summary.data.recent_activity.map((item, i) => (
              <div
                key={i}
                className="flex items-center justify-between py-2 border-b border-border last:border-b-0"
              >
                <div>
                  <span className="text-sm font-medium text-foreground">
                    {item.event_type}
                  </span>
                  <span className="text-sm text-muted-foreground ml-2">
                    {item.resource_type}/{item.resource_id}
                  </span>
                </div>
                <span className="text-xs text-muted-foreground">
                  {item.actor_name}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="rounded-lg border border-border bg-card p-6">
        <h3 className="text-lg font-semibold text-foreground mb-4">
          API Status
        </h3>
        {health.isLoading && (
          <p className="text-muted-foreground">Checking API...</p>
        )}
        {health.isError && (
          <p className="text-destructive">
            API unreachable. Start the VGX server on :8080.
          </p>
        )}
        {health.data && (
          <div className="flex items-center gap-2">
            <span className="h-2 w-2 rounded-full bg-foreground" />
            <span className="text-sm text-foreground">
              {health.data.service} v{health.data.version} —{" "}
              {health.data.status}
            </span>
          </div>
        )}
      </div>
    </div>
  );
}
