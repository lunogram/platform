import { useCallback, useState } from "react"
import { toast } from "sonner"
import type { User } from "@/types"
import { oapiClient } from "@/oapi/client"

interface UseSendTestInboxOptions {
    projectId: string
}

export interface UseSendTestInboxResult {
    sending: boolean
    handleSendTest: (user: User, title: string, body: string) => Promise<void>
}

/**
 * Sends a test inbox message into the selected user's inbox.
 *
 * Inbox messages have no external provider dispatch, so a test is simply a real
 * inbox message created through the user inbox endpoint — the same path the user
 * detail inbox tab uses. The title and body should already have their template
 * variables substituted for the chosen user.
 */
export function useSendTestInbox({ projectId }: UseSendTestInboxOptions): UseSendTestInboxResult {
    const [sending, setSending] = useState(false)

    const handleSendTest = useCallback(
        async (user: User, title: string, body: string) => {
            setSending(true)
            try {
                const { error } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox",
                    {
                        params: { path: { projectID: projectId, userID: user.id } },
                        body: {
                            channel: "inbox",
                            content: {
                                title: title.trim(),
                                body: body.trim() || undefined,
                            },
                            tags: ["test"],
                            priority: 3,
                        },
                    },
                )

                if (error) {
                    const detail =
                        typeof (error as Record<string, unknown>)?.detail === "string"
                            ? ((error as Record<string, unknown>).detail as string)
                            : null
                    toast.error(detail || "Failed to send test inbox message")
                    return
                }

                toast.success(`Test inbox message sent to ${user.email}`)
            } catch {
                toast.error("Failed to send test inbox message")
            } finally {
                setSending(false)
            }
        },
        [projectId],
    )

    return { sending, handleSendTest }
}
