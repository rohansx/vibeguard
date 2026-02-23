import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { ScanResult } from "@/types";
import { cn } from "@/lib/cn";

const statusStyles: Record<string, string> = {
  completed: "bg-foreground text-background",
  running: "bg-muted-foreground text-background",
  failed: "bg-destructive text-background",
};

export function ScansPage() {
  const {
    data: scans,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["scans"],
    queryFn: () => apiFetch<ScanResult[]>("/scans"),
  });

  return (
    <div>
      <h2 className="text-2xl font-bold text-foreground mb-6">Scan Results</h2>

      {isLoading && (
        <p className="text-muted-foreground">Loading scans...</p>
      )}
      {isError && <p className="text-destructive">Failed to load scans.</p>}

      {scans && scans.length === 0 && (
        <div className="rounded-lg border border-border bg-card p-6">
          <p className="text-muted-foreground">
            No scans yet. Run{" "}
            <code className="bg-muted px-1 rounded text-sm">
              vgx scan ./your-project
            </code>{" "}
            to start.
          </p>
        </div>
      )}

      {scans && scans.length > 0 && (
        <div className="space-y-4">
          {scans.map((scan) => (
            <div
              key={scan.id}
              className="rounded-lg border border-border bg-card p-6"
            >
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="text-sm font-mono text-foreground">
                    {scan.repository_url}
                  </h3>
                  <p className="text-sm text-muted-foreground mt-1">
                    Branch: {scan.branch} &middot; Started: {scan.started_at}
                  </p>
                </div>
                <span
                  className={cn(
                    "inline-block px-2 py-0.5 rounded text-xs uppercase shrink-0 ml-4",
                    statusStyles[scan.status] || statusStyles.running,
                  )}
                >
                  {scan.status}
                </span>
              </div>

              {scan.status === "completed" && (
                <div className="mt-4 grid grid-cols-5 gap-4 text-center">
                  <div>
                    <p className="text-xs uppercase text-muted-foreground">
                      Score
                    </p>
                    <p className="text-xl font-bold text-foreground">
                      {scan.compliance_score}%
                    </p>
                  </div>
                  <div>
                    <p className="text-xs uppercase text-muted-foreground">
                      Critical
                    </p>
                    <p className="text-xl font-bold text-foreground">
                      {scan.critical_findings}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs uppercase text-muted-foreground">
                      High
                    </p>
                    <p className="text-xl font-bold text-foreground">
                      {scan.high_findings}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs uppercase text-muted-foreground">
                      Medium
                    </p>
                    <p className="text-xl font-bold text-foreground">
                      {scan.medium_findings}
                    </p>
                  </div>
                  <div>
                    <p className="text-xs uppercase text-muted-foreground">
                      Low
                    </p>
                    <p className="text-xl font-bold text-foreground">
                      {scan.low_findings}
                    </p>
                  </div>
                </div>
              )}

              {scan.status === "running" && (
                <p className="mt-4 text-sm text-muted-foreground">
                  Scan in progress...
                </p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
