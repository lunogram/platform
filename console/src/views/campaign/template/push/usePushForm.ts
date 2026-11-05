import { useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { Campaign, Template } from "@/types"
import type { TemplateChanges } from "@/validation/campaign/template/data"
import { pushSetupFormSchema, type PushFormValues } from "@/validation/campaign/template/push/setup"

export function usePushForm(
    _campaign: Campaign,
    template?: Template,
): UseFormReturn<PushFormValues> {
    const push = template?.type === "push" ? template : undefined

    return useForm<PushFormValues>({
        resolver: zodResolver(pushSetupFormSchema),
        defaultValues: {
            title: push?.data.title ?? "",
            body: push?.data.body ?? "",
        },
    })
}

export function pushTemplateChanges(values: PushFormValues): TemplateChanges {
    return {
        data: { title: values.title, body: values.body },
        sender_identity_id: null,
    }
}
