import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { ComplianceRule } from "@/types";
import { cn } from "@/lib/cn";

const severityStyles: Record<string, string> = {
  critical: "border-2 border-foreground font-bold",
  high: "border border-foreground font-semibold",
  medium: "border border-muted-foreground font-medium",
  low: "border border-border font-normal",
};

export function RulesPage() {
  const {
    data: rules,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["rules"],
    queryFn: () => apiFetch<ComplianceRule[]>("/rules"),
  });

  return (
    <div>
      <h2 className="text-2xl font-bold text-foreground mb-6">
        Compliance Rules
      </h2>

      {isLoading && (
        <p className="text-muted-foreground">Loading rules...</p>
      )}
      {isError && <p className="text-destructive">Failed to load rules.</p>}

      {rules && rules.length === 0 && (
        <div className="rounded-lg border border-border bg-card p-6">
          <p className="text-muted-foreground">
            No active compliance rules yet.
          </p>
        </div>
      )}

      {rules && rules.length > 0 && (
        <div className="rounded-lg border border-border bg-card overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-secondary">
                <th className="text-left p-3 font-semibold text-foreground">
                  ID
                </th>
                <th className="text-left p-3 font-semibold text-foreground">
                  Title
                </th>
                <th className="text-left p-3 font-semibold text-foreground">
                  Regulation
                </th>
                <th className="text-left p-3 font-semibold text-foreground">
                  Article
                </th>
                <th className="text-left p-3 font-semibold text-foreground">
                  Type
                </th>
                <th className="text-left p-3 font-semibold text-foreground">
                  Severity
                </th>
                <th className="text-left p-3 font-semibold text-foreground">
                  Languages
                </th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr
                  key={rule.id}
                  className="border-b border-border last:border-b-0"
                >
                  <td className="p-3 font-mono text-xs text-foreground">
                    {rule.id}
                  </td>
                  <td className="p-3 text-foreground">{rule.title}</td>
                  <td className="p-3 text-muted-foreground">
                    {rule.regulation}
                  </td>
                  <td className="p-3 text-muted-foreground">
                    {rule.article} {rule.paragraph}
                  </td>
                  <td className="p-3 text-muted-foreground">
                    {rule.check_type}
                  </td>
                  <td className="p-3">
                    <span
                      className={cn(
                        "inline-block px-2 py-0.5 rounded text-xs uppercase",
                        severityStyles[rule.severity] || severityStyles.low,
                      )}
                    >
                      {rule.severity}
                    </span>
                  </td>
                  <td className="p-3 text-muted-foreground text-xs">
                    {rule.languages.join(", ")}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
