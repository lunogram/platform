import { useTranslation } from "react-i18next"
import { CheckCircle2, XCircle } from "lucide-react"

import type { TestActionResult } from "@/oapi/client"

import { CodeEditor } from "@/components/ui/code-editor"
import { cn } from "@/utils"

export function TestResultPanel({ result }: { result: TestActionResult }) {
    const { t } = useTranslation()
    const isError = result.status === "error"
    const hasMetadata = result.metadata && Object.keys(result.metadata).length > 0
    const statusLabel = result.status || "unknown"

    const metadataJson = hasMetadata ? JSON.stringify(result.metadata, null, 2) : ""

    return (
        <div className="space-y-4">
            {/* Status banner */}
            <div
                className={cn(
                    "flex items-center gap-3 rounded-lg border px-4 py-3",
                    isError
                        ? "border-destructive/30 bg-destructive/5 text-destructive"
                        : "border-emerald-500/30 bg-emerald-500/5 text-emerald-600 dark:text-emerald-400",
                )}
            >
                {isError ? (
                    <XCircle className="h-5 w-5 shrink-0" />
                ) : (
                    <CheckCircle2 className="h-5 w-5 shrink-0" />
                )}
                <span className="text-sm font-medium">
                    {statusLabel}
                    {result.status_code != null && ` · ${result.status_code}`}
                </span>
            </div>

            {/* Error message */}
            {result.error && (
                <div className="rounded-lg border border-destructive/50 bg-destructive/5 px-4 py-3 text-sm text-destructive">
                    {result.error}
                </div>
            )}

            {/* Metadata */}
            {metadataJson ? (
                <CodeEditor
                    value={metadataJson}
                    onChange={() => {}}
                    readOnly
                    minHeight={80}
                    maxHeight={400}
                />
            ) : (
                <p className="text-sm text-muted-foreground">
                    {t("no_response_data", "No response data returned.")}
                </p>
            )}
        </div>
    )
}
