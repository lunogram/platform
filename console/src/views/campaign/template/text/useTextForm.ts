import { useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { Campaign, Template } from "@/types"
import type { TemplateChanges } from "@/validation/campaign/template/data"
import { textSetupFormSchema, type TextFormValues } from "@/validation/campaign/template/text/setup"

export function useTextForm(
    _campaign: Campaign,
    template?: Template,
): UseFormReturn<TextFormValues> {
    const sms = template?.type === "sms" ? template : undefined

    return useForm<TextFormValues>({
        resolver: zodResolver(textSetupFormSchema),
        defaultValues: {
            sender_identity_id: sms?.sender_identity_id ?? "",
            body: sms?.data.body ?? "",
        },
    })
}

export function textTemplateChanges(values: TextFormValues): TemplateChanges {
    return {
        data: { body: values.body },
        sender_identity_id: values.sender_identity_id || null,
    }
}
