import { useCallback, useContext, useState } from "react"
import { toast } from "sonner"
import { AxiosError } from "axios"
import api from "@/api"
import { TemplateWorkflowContext } from "../../../contexts"

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
 *
 * Before dispatching the test, the current editor state is persisted
 * through the template workflow so the backend always has the latest
 * content (subject, html, code, etc.).
 */
export function useSendTestEmail({
    previewProps,
    projectId,
    campaignId,
    templateId,
}: UseSendTestEmailOptions): UseSendTestEmailResult {
    const [sending, setSending] = useState(false)
    const { save } = useContext(TemplateWorkflowContext)

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
            const saved = await save()
            if (!saved) {
                toast.error("Failed to save template before sending test email")
                return
            }

            await api.campaigns.templates.sendTest(projectId, campaignId, templateId, {
                to: email,
                props: previewProps,
            })
            toast.success(`Test email sent to ${email}`)
        } catch (err) {
            const detail =
                err instanceof AxiosError && typeof err.response?.data?.detail === "string"
                    ? err.response.data.detail
                    : null
            toast.error(detail || "Failed to send test email")
        } finally {
            setSending(false)
        }
    }, [previewProps, projectId, campaignId, templateId, save])

    return { sending, handleSendTest }
}
