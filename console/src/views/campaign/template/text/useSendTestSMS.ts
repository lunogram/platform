import { useCallback, useContext, useState } from "react"
import { toast } from "sonner"
import { AxiosError } from "axios"
import api from "@/api"
import type { User } from "@/types"
import { TemplateWorkflowContext } from "../contexts"

interface UseSendTestSMSOptions {
    projectId: string
    campaignId: string
    templateId: string
}

export interface UseSendTestSMSResult {
    sending: boolean
    handleSendTest: (phoneNumber: string, user?: User | null) => Promise<void>
}

/**
 * Manages sending a test SMS with the current template content.
 *
 * Before dispatching the test, the current editor state is persisted
 * through the template workflow so the backend always has the latest
 * message body.
 */
export function useSendTestSMS({
    projectId,
    campaignId,
    templateId,
}: UseSendTestSMSOptions): UseSendTestSMSResult {
    const [sending, setSending] = useState(false)
    const { save } = useContext(TemplateWorkflowContext)

    const handleSendTest = useCallback(
        async (phoneNumber: string, user?: User | null) => {
            if (!phoneNumber) {
                toast.error(
                    "Selected user has no phone number. Please select a user with a phone number.",
                )
                return
            }

            setSending(true)
            try {
                // Persist the current editor state so the backend has up-to-date
                // template data (message body, sender identity, etc.).
                const saved = await save()
                if (!saved) {
                    toast.error("Failed to save template before sending test SMS")
                    return
                }

                await api.campaigns.templates.sendTest(projectId, campaignId, templateId, {
                    to: phoneNumber,
                    ...(user ? { props: { user } } : {}),
                })
                toast.success(`Test SMS sent to ${phoneNumber}`)
            } catch (err) {
                const detail =
                    err instanceof AxiosError && typeof err.response?.data?.detail === "string"
                        ? err.response.data.detail
                        : null
                toast.error(detail || "Failed to send test SMS")
            } finally {
                setSending(false)
            }
        },
        [projectId, campaignId, templateId, save],
    )

    return { sending, handleSendTest }
}
