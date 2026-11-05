import { useCallback, useEffect, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import oapiClient from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import { Button } from "@/components/ui/button"

export type PreviewMode = "action-config" | "function-call"

interface ActionPreviewIframeProps {
    /** Action module type, e.g. "webhook" */
    actionType?: string
    /** Project ID used to fetch the preview HTML */
    projectId: string
    /** Preview mode — determines the data shape sent to the iframe */
    mode: PreviewMode
    /** Serialisable data posted to the iframe on every change */
    data: Record<string, unknown>
    /** Optional extra class names for the iframe wrapper */
    className?: string
}

/**
 * Reusable iframe that loads an action module's preview micro-app and
 * keeps it synchronised with the parent form via postMessage.
 *
 * Used both in the action configuration screen and in journey step
 * previews.
 */
export function ActionPreviewIframe({
    actionType,
    projectId,
    mode,
    data,
    className,
}: ActionPreviewIframeProps) {
    const { t } = useTranslation()
    const iframeRef = useRef<HTMLIFrameElement>(null)
    const [iframeHeight, setIframeHeight] = useState(0)
    const iframeLoadedRef = useRef(false)

    // Keyed by action type, so switching types quickly can't show the preview
    // of a type that was left.
    const preview = useQuery({
        queryKey: ["projects", projectId, "actions", "preview", actionType],
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/actions/meta/{actionType}/preview",
                    {
                        params: { path: { projectID: projectId, actionType: actionType ?? "" } },
                        parseAs: "text",
                    },
                ),
            ) as unknown as string,
        enabled: !!actionType,
    })
    const previewHtml = actionType ? (preview.data ?? null) : null

    // A new document has to announce itself again before it is posted to.
    useEffect(() => {
        iframeLoadedRef.current = false
    }, [previewHtml])

    // Serialize to JSON so the effect fires on deep changes
    const previewPayload = JSON.stringify({
        type: "preview-update",
        mode,
        actionType,
        ...data,
    })

    const postToIframe = useCallback(() => {
        if (!iframeRef.current?.contentWindow || !iframeLoadedRef.current) return
        iframeRef.current.contentWindow.postMessage(JSON.parse(previewPayload), "*")
    }, [previewPayload])

    // Keep a ref to the latest postToIframe so the "preview-ready" handler
    // always calls the most current version.
    const postToIframeRef = useRef(postToIframe)
    useEffect(() => {
        postToIframeRef.current = postToIframe
    }, [postToIframe])

    // Post data when it changes (only if iframe is loaded)
    useEffect(() => {
        if (!previewHtml) return
        postToIframe()
    }, [previewHtml, postToIframe])

    // Listen for messages from iframe
    useEffect(() => {
        const handler = (e: MessageEvent) => {
            if (e.data?.type === "resize" && typeof e.data.height === "number") {
                setIframeHeight(e.data.height)
            }
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

    if (actionType && preview.isError) {
        return (
            <div className="flex items-center justify-between gap-3 rounded-md border px-3 py-2">
                <span className="text-sm text-destructive">
                    {t("action_preview_load_failed", "Couldn't load the preview.")}
                </span>
                <Button type="button" variant="outline" size="sm" onClick={() => preview.refetch()}>
                    {t("retry", "Retry")}
                </Button>
            </div>
        )
    }

    if (!previewHtml) return null

    return (
        <iframe
            ref={iframeRef}
            srcDoc={previewHtml}
            title="Action Preview"
            className={className ?? "w-full rounded-lg bg-background"}
            style={{ height: iframeHeight, border: "none" }}
            sandbox="allow-scripts"
            onLoad={() => {
                iframeLoadedRef.current = true
            }}
        />
    )
}
