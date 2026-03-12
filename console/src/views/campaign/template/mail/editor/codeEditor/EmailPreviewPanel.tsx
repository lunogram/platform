import { useTranslation } from "react-i18next"
import { AlertCircle } from "lucide-react"

interface EmailPreviewPanelProps {
    html: string
    error: string | null
    viewport: string
    viewportWidth: number
}

export function EmailPreviewPanel({ html, error, viewportWidth }: EmailPreviewPanelProps) {
    const { t } = useTranslation()

    if (error) {
        return (
            <div className="flex-1 min-h-0 overflow-hidden h-full flex items-start justify-center p-6">
                <div className="max-w-md w-full rounded-lg border border-destructive/30 bg-destructive/5 p-4">
                    <div className="flex items-start gap-3">
                        <AlertCircle className="h-4 w-4 text-destructive mt-0.5 shrink-0" />
                        <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium text-destructive">
                                {t(
                                    "campaign.template.email.editor.compileError",
                                    "Compilation Error",
                                )}
                            </p>
                            <pre className="mt-2 text-xs text-destructive/80 whitespace-pre-wrap break-words font-mono leading-relaxed">
                                {error}
                            </pre>
                        </div>
                    </div>
                </div>
            </div>
        )
    }

    if (!html) {
        return (
            <div className="flex-1 min-h-0 overflow-hidden h-full flex items-center justify-center">
                <p className="text-sm text-muted-foreground">
                    {t(
                        "campaign.template.email.editor.noPreview",
                        "Start typing to see a preview...",
                    )}
                </p>
            </div>
        )
    }

    return (
        <div className="flex-1 min-h-0 min-w-0 overflow-hidden h-full px-6 py-4 flex justify-center">
            <iframe
                srcDoc={html}
                title="Email preview"
                className="border rounded-md bg-white shadow-sm h-full min-w-0"
                style={{
                    width: "100%",
                    maxWidth: viewportWidth,
                }}
                sandbox="allow-same-origin allow-scripts"
            />
        </div>
    )
}
