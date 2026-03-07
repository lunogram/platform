import { useTranslation } from "react-i18next"
import { CheckCircle2, XCircle } from "lucide-react"

import type { TestActionResult } from "@/oapi/client"

import { cn } from "@/utils"

export function TestResultPanel({ result }: { result: TestActionResult }) {
    const { t } = useTranslation()
    // status_code === 0 is a client-side sentinel indicating the request
    // failed before reaching the action (e.g. network error).
    const isError = result.status_code >= 400 || result.status_code === 0

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
                <span className="text-sm font-medium">{result.status_code}</span>
            </div>

            {/* Validation message */}
            {result.message && (
                <div
                    className={cn(
                        "rounded-lg border px-4 py-3 text-sm",
                        isError
                            ? "border-destructive/50 bg-destructive/5 text-destructive"
                            : "border-emerald-500/30 bg-emerald-500/5 text-emerald-600 dark:text-emerald-400",
                    )}
                >
                    {result.message}
                </div>
            )}

            {!result.message && (
                <p className="text-sm text-muted-foreground">
                    {t("no_validation_message", "No validation message returned.")}
                </p>
            )}
        </div>
    )
}
