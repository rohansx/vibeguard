import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { Document } from "@/types";
import { cn } from "@/lib/cn";

const statusStyles: Record<string, string> = {
  processed: "bg-foreground text-background",
  processing: "bg-muted-foreground text-background",
  uploaded: "bg-secondary text-foreground border border-border",
  review_ready: "bg-foreground text-background",
  extraction_failed: "bg-destructive text-background",
};

export function DocumentsPage() {
  const {
    data: docs,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["documents"],
    queryFn: () => apiFetch<Document[]>("/documents"),
  });

  return (
    <div>
      <h2 className="text-2xl font-bold text-foreground mb-6">
        Regulatory Documents
      </h2>

      {isLoading && (
        <p className="text-muted-foreground">Loading documents...</p>
      )}
      {isError && (
        <p className="text-destructive">Failed to load documents.</p>
      )}

      {docs && docs.length === 0 && (
        <div className="rounded-lg border border-border bg-card p-6">
          <p className="text-muted-foreground">No documents uploaded yet.</p>
        </div>
      )}

      {docs && docs.length > 0 && (
        <div className="grid grid-cols-1 gap-4">
          {docs.map((doc) => (
            <div
              key={doc.id}
              className="rounded-lg border border-border bg-card p-6"
            >
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="text-lg font-semibold text-foreground">
                    {doc.name}
                  </h3>
                  <p className="text-sm text-muted-foreground mt-1">
                    {doc.description}
                  </p>
                </div>
                <span
                  className={cn(
                    "inline-block px-2 py-0.5 rounded text-xs uppercase shrink-0 ml-4",
                    statusStyles[doc.status] || statusStyles.uploaded,
                  )}
                >
                  {doc.status}
                </span>
              </div>
              <div className="flex gap-6 mt-4 text-sm text-muted-foreground">
                <span>File: {doc.file_name}</span>
                <span>Rules: {doc.rule_count}</span>
                <span>Approved: {doc.approved_count}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
