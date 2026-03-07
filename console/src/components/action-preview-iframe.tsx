import { useCallback, useEffect, useRef, useState } from "react"
import oapiClient from "@/oapi/client"

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
    const iframeRef = useRef<HTMLIFrameElement>(null)
    const [iframeHeight, setIframeHeight] = useState(0)
    const [previewHtml, setPreviewHtml] = useState<string | null>(null)
    const iframeLoadedRef = useRef(false)

    // Fetch preview HTML when action type changes
    useEffect(() => {
        if (!actionType) {
            setPreviewHtml(null)
            return
        }
        iframeLoadedRef.current = false
        oapiClient
            .GET("/api/admin/projects/{projectID}/actions/meta/{actionType}/preview", {
                params: { path: { projectID: projectId, actionType } },
                parseAs: "text",
            })
            .then(({ data }) => setPreviewHtml((data as unknown as string) ?? null))
            .catch(() => setPreviewHtml(null))
    }, [actionType, projectId])

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
