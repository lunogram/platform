import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"

import { Loader2 } from "lucide-react"

import type { TestActionResult } from "@/oapi/client"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/utils"

import type { ActionFormValues } from "./action-form-types"
import { variablesToMap } from "./action-form-types"
import { TestResultPanel } from "./TestResultPanel"

interface ActionPreviewPanelProps {
    selectedType: string | undefined
    projectId: string
    isTesting: boolean
    testResult: TestActionResult | null
    activeTab: string
    setActiveTab: (tab: string) => void
    /** Watched form values for live preview updates */
    config: ActionFormValues["config"]
    payload: ActionFormValues["payload"]
    variables: ActionFormValues["variables"]
}

export function ActionPreviewPanel({
    selectedType,
    projectId,
    isTesting,
    testResult,
    activeTab,
    setActiveTab,
    config,
    payload,
    variables,
}: ActionPreviewPanelProps) {
    const { t } = useTranslation()

    const iframeRef = useRef<HTMLIFrameElement>(null)
    const [iframeHeight, setIframeHeight] = useState(300)
    const [previewHtml, setPreviewHtml] = useState<string | null>(null)
    const iframeLoadedRef = useRef(false)

    // Fetch preview HTML when action type changes
    useEffect(() => {
        if (!selectedType) {
            setPreviewHtml(null)
            return
        }
        iframeLoadedRef.current = false
        fetch(`/api/admin/projects/${projectId}/actions/meta/${selectedType}/preview`)
            .then((r) => {
                if (r.ok) return r.text()
                return null
            })
            .then((html) => setPreviewHtml(html ?? null))
            .catch(() => setPreviewHtml(null))
    }, [selectedType, projectId])

    // Serialize to JSON so the effect fires on deep changes
    const previewData = JSON.stringify({
        config: { ...(config ?? {}), ...(payload ?? {}) },
        payload: payload ?? {},
        variables: variablesToMap(variables),
    })

    const postToIframe = useCallback(() => {
        if (!iframeRef.current?.contentWindow || !iframeLoadedRef.current) return
        iframeRef.current.contentWindow.postMessage(
            {
                type: "preview-update",
                actionType: selectedType,
                ...JSON.parse(previewData),
            },
            "*",
        )
    }, [selectedType, previewData])

    // Keep a ref to the latest postToIframe so the "preview-ready" handler
    // always calls the most current version (avoids stale closure from
    // the race between form.reset and iframe mount).
    const postToIframeRef = useRef(postToIframe)
    useEffect(() => {
        postToIframeRef.current = postToIframe
    }, [postToIframe])

    // Post data when it changes (only if iframe is loaded)
    useEffect(() => {
        if (!previewHtml) return
        postToIframe()
    }, [previewHtml, postToIframe])

    // Re-post data when iframe finishes loading (DOM ready)
    const handleIframeLoad = useCallback(() => {
        iframeLoadedRef.current = true
    }, [])

    // Listen for messages from iframe: resize and preview-ready
    useEffect(() => {
        const handler = (e: MessageEvent) => {
            if (e.data?.type === "resize" && typeof e.data.height === "number") {
                setIframeHeight(e.data.height)
            }
            // The Preact app inside the iframe signals it has mounted its
            // message listener. Post the current form data in response so
            // the preview is guaranteed to receive it.
            if (e.data?.type === "preview-ready") {
                iframeLoadedRef.current = true
                postToIframeRef.current()
            }
            if (e.data?.type === "copy-to-clipboard" && typeof e.data.text === "string") {
                navigator.clipboard.writeText(e.data.text)
            }
        }
        window.addEventListener("message", handler)
        return () => window.removeEventListener("message", handler)
    }, [])

    return (
        <div className="flex flex-col w-3/5 border-l overflow-hidden">
            <nav className="flex gap-1 px-8 pt-8 border-b bg-card/50">
                {(
                    [
                        { key: "preview", label: t("preview", "Preview") },
                        { key: "results", label: t("results", "Results") },
                    ] as const
                ).map((tab) => {
                    const isDisabled = tab.key === "results" && !testResult && !isTesting
                    const tabButton = (
                        <button
                            key={tab.key}
                            type="button"
                            disabled={isDisabled}
                            onClick={() => setActiveTab(tab.key)}
                            className={cn(
                                "flex items-center gap-2 px-4 py-2.5 text-sm font-medium rounded-t-lg border-b-2 transition-colors -mb-px",
                                activeTab === tab.key
                                    ? "border-primary text-foreground bg-background"
                                    : "border-transparent text-muted-foreground hover:text-foreground hover:bg-muted/50",
                                isDisabled && "opacity-50 pointer-events-none",
                            )}
                        >
                            {tab.label}
                        </button>
                    )

                    if (isDisabled) {
                        return (
                            <Tooltip key={tab.key}>
                                <TooltipTrigger asChild>
                                    <span className="cursor-default">{tabButton}</span>
                                </TooltipTrigger>
                                <TooltipContent side="bottom">
                                    {t("test_to_see_results", "Run a test to see results")}
                                </TooltipContent>
                            </Tooltip>
                        )
                    }

                    return tabButton
                })}
            </nav>

            <div className="flex-1 overflow-y-auto p-8">
                {activeTab === "preview" &&
                    (previewHtml ? (
                        <iframe
                            ref={iframeRef}
                            srcDoc={previewHtml}
                            title="Action Preview"
                            className="w-full rounded-lg bg-background"
                            style={{ height: iframeHeight, border: "none" }}
                            sandbox="allow-scripts"
                            onLoad={handleIframeLoad}
                        />
                    ) : (
                        <div className="flex items-center justify-center h-48 border rounded-lg text-muted-foreground text-sm">
                            {t("no_preview", "No preview available")}
                        </div>
                    ))}

                {activeTab === "results" && isTesting && (
                    <div className="flex flex-col items-center justify-center h-48 text-muted-foreground gap-3">
                        <Loader2 className="h-8 w-8 animate-spin" />
                        <span className="text-sm">{t("executing_test", "Executing test...")}</span>
                    </div>
                )}

                {activeTab === "results" && !isTesting && testResult && (
                    <TestResultPanel result={testResult} />
                )}
            </div>
        </div>
    )
}
