import { useCallback, useEffect, useRef, useState } from "react"
import { fetchEventSource } from "@microsoft/fetch-event-source"
import { apiUrl } from "@/api"
import type { BroadcastState } from "@/types"
import type { UUID } from "@/types/common"

/** Shape of a progress SSE event payload from the backend. */
interface ProgressEvent {
    state: BroadcastState
    sent: number
    total: number
    terminal: boolean
}

interface UseBroadcastProgressOptions {
    projectId: UUID
    broadcastId: UUID
    /** Only connect when the broadcast is actively sending. */
    enabled: boolean
    /** Called when the broadcast reaches a terminal state. */
    onTerminal: () => void
}

interface UseBroadcastProgressResult {
    /** Number of messages sent so far (from the SSE stream). Null until first event. */
    sent: number | null
    /** Total number of messages queued. Null until first event. */
    total: number | null
}

/**
 * Opens an SSE connection to the broadcast progress endpoint while
 * `enabled` is true. The backend sends `{state, sent, total, terminal}`
 * events every ~2s based on a DB query. When a terminal event arrives,
 * `onTerminal` is called so the parent can re-fetch the broadcast.
 */
export function useBroadcastProgress({
    projectId,
    broadcastId,
    enabled,
    onTerminal,
}: UseBroadcastProgressOptions): UseBroadcastProgressResult {
    const [sent, setSent] = useState<number | null>(null)
    const [total, setTotal] = useState<number | null>(null)
    const onTerminalRef = useRef(onTerminal)
    onTerminalRef.current = onTerminal

    const disconnect = useCallback(() => {
        // abort is handled by the cleanup return
    }, [])

    useEffect(() => {
        if (!enabled) {
            setSent(null)
            setTotal(null)
            return
        }

        const abortController = new AbortController()
        const url = apiUrl(projectId, `broadcasts/${broadcastId}/progress`)

        fetchEventSource(url, {
            signal: abortController.signal,
            credentials: "include",
            onopen: async (response) => {
                if (!response.ok) {
                    console.error("Broadcast SSE connection failed:", response.status)
                    throw new Error(`SSE open failed: ${response.status}`)
                }
            },
            onmessage: (event) => {
                if (event.event !== "progress") return

                let data: ProgressEvent
                try {
                    data = JSON.parse(event.data)
                } catch {
                    return
                }

                setSent(data.sent)
                setTotal(data.total)

                if (data.terminal) {
                    onTerminalRef.current()
                }
            },
            onerror: (err) => {
                console.error("Broadcast SSE error:", err)
                throw err
            },
        })

        return () => {
            abortController.abort()
        }
    }, [projectId, broadcastId, enabled, disconnect])

    return { sent, total }
}
