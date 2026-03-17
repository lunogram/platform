import { useCallback, useState } from "react"
import { toast } from "sonner"
import api from "@/api"

interface UseSendTestEmailOptions {
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
 */
export function useSendTestEmail({
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
            await api.campaigns.templates.sendTest(projectId, campaignId, templateId, {
                to: email,
                props: previewProps,
            })
            toast.success(`Test email sent to ${email}`)
        } catch {
            toast.error("Failed to send test email")
        } finally {
            setSending(false)
        }
    }, [previewProps, projectId, campaignId, templateId])

    return { sending, handleSendTest }
}
