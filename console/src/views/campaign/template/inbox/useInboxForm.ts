import { useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { Campaign, Template } from "@/types"
import type { TemplateChanges } from "@/validation/campaign/template/data"
import {
    inboxSetupFormSchema,
    type InboxFormValues,
} from "@/validation/campaign/template/inbox/setup"

export function useInboxForm(
    _campaign: Campaign,
    template?: Template,
): UseFormReturn<InboxFormValues> {
    const inbox = template?.type === "inbox" ? template : undefined

    return useForm<InboxFormValues>({
        resolver: zodResolver(inboxSetupFormSchema),
        defaultValues: {
            title: inbox?.data.title ?? "",
            body: inbox?.data.body ?? "",
        },
    })
}

export function inboxTemplateChanges(values: InboxFormValues): TemplateChanges {
    return {
        data: { title: values.title, body: values.body },
        sender_identity_id: null,
    }
}
