import { useCallback, useState } from "react"
import { toast } from "sonner"
import { oapiClient } from "@/oapi/client"
import type { SaveStep } from "../../../contexts"

interface UseSendTestEmailOptions {
    /** The editor step's save, run first so the test uses the latest content. */
    save: SaveStep
    previewProps: Record<string, unknown>
    projectId: string
    campaignId: string
    templateId: string
}

export interface UseSendTestEmailResult {
    sending: boolean
    handleSendTest: () => Promise<void>
}

/**
 * Manages sending a test email with the current preview props.
 *
 * Before dispatching the test, the current editor state is persisted
 * through the template workflow so the backend always has the latest
 * content (subject, html, code, etc.).
 */
export function useSendTestEmail({
    save,
    previewProps,
    projectId,
    campaignId,
    templateId,
}: UseSendTestEmailOptions): UseSendTestEmailResult {
    const [sending, setSending] = useState(false)

    const handleSendTest = useCallback(async () => {
        const userProps = previewProps.user as Record<string, unknown> | undefined
        const email = userProps?.email
        if (typeof email !== "string" || !email) {
            toast.error("No recipient email found. Set user.email in the props panel below.")
            return
        }

        setSending(true)
        try {
            // Persist the current editor state so the backend has up-to-date
            // template data (html, text, code source, subject, etc.).
            // A failed save was already reported; only invalid content needs saying here.
            const saved = await save()
            if (!saved.ok) {
                if (saved.reason === "invalid") {
                    toast.error("Fix the template's errors before sending a test email")
                }
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
                        to: email,
                        props: previewProps,
                    },
                },
            )
            if (error) {
                toast.error(error.detail || "Failed to send test email")
                return
            }
            toast.success(`Test email sent to ${email}`)
        } catch {
            toast.error("Failed to send test email")
        } finally {
            setSending(false)
        }
    }, [previewProps, projectId, campaignId, templateId, save])

    return { sending, handleSendTest }
}
