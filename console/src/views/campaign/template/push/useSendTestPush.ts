import { useCallback, useContext, useState } from "react"
import { toast } from "sonner"
import type { Device, User } from "@/types"
import { oapiClient } from "@/oapi/client"
import { TemplateWorkflowContext } from "../contexts"

interface UseSendTestPushOptions {
    projectId: string
    campaignId: string
    templateId: string
}

export interface UseSendTestPushResult {
    sending: boolean
    handleSendTest: (device: Device, user?: User | null) => Promise<void>
}

/**
 * Manages sending a test push notification with the current template content.
 *
 * Before dispatching the test, the current editor state is persisted
 * through the template workflow so the backend always has the latest
 * title and body.
 */
export function useSendTestPush({
    projectId,
    campaignId,
    templateId,
}: UseSendTestPushOptions): UseSendTestPushResult {
    const [sending, setSending] = useState(false)
    const { save } = useContext(TemplateWorkflowContext)

    const handleSendTest = useCallback(
        async (device: Device, user?: User | null) => {
            if (!device.token) {
                toast.error("Selected device has no push token registered.")
                return
            }

            setSending(true)
            try {
                // Persist the current editor state so the backend has up-to-date
                // template data (title, body, custom data, etc.).
                const saved = await save()
                if (!saved) {
                    toast.error("Failed to save template before sending test push notification")
                    return
                }

                const { error } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/campaigns/{campaignID}/templates/{templateID}/test",
                    {
                        params: {
                            path: {
                                projectID: projectId,
                                campaignID: campaignId,
                                templateID: templateId,
                            },
                        },
                        body: {
                            to: device.token,
                            ...(user ? { props: { user } } : {}),
                        },
                    },
                )

                if (error) {
                    const detail =
                        typeof (error as Record<string, unknown>)?.detail === "string"
                            ? ((error as Record<string, unknown>).detail as string)
                            : null
                    toast.error(detail || "Failed to send test push notification")
                    return
                }

                toast.success(
                    `Test push notification sent to ${device.model || device.device_id}`,
                )
            } catch {
                toast.error("Failed to send test push notification")
            } finally {
                setSending(false)
            }
        },
        [projectId, campaignId, templateId, save],
    )

    return { sending, handleSendTest }
}
