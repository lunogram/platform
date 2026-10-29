import { useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { Campaign, Template } from "@/types"
import type { TemplateChanges } from "@/validation/campaign/template/data"
import {
    emailSetupFormSchema,
    type EmailFormValues,
} from "@/validation/campaign/template/mail/setup"

export function useEmailForm(
    _campaign: Campaign,
    template?: Template,
): UseFormReturn<EmailFormValues> {
    const email = template?.type === "email" ? template : undefined

    return useForm<EmailFormValues>({
        resolver: zodResolver(emailSetupFormSchema),
        defaultValues: {
            sender_identity_id: email?.sender_identity_id ?? "",
            from: {
                name: email?.data.from?.name ?? "",
            },
            subject: email?.data.subject ?? "",
            reply_to: email?.data.reply_to ?? "",
        },
    })
}

export function emailTemplateChanges(values: EmailFormValues): TemplateChanges {
    const { sender_identity_id, subject, from, reply_to } = values
    return {
        data: { subject, from, reply_to },
        sender_identity_id: sender_identity_id || null,
    }
}
