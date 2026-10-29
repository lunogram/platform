import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type * as z from "zod"
import type { Campaign, Template } from "@/types"
import { textSetupFormSchema } from "@/validation/campaign/template/text/setup"

export function useTextForm(_campaign: Campaign, template?: Template) {
    const formSchema = textSetupFormSchema

    const defaultValues: z.infer<typeof textSetupFormSchema> = {
        sender_identity_id: "",
        body: "",
    }

    if (template && template.type === "sms") {
        defaultValues.sender_identity_id = template.sender_identity_id ?? ""
        defaultValues.body = template.data.body
    }

    return useForm({
        resolver: zodResolver(formSchema),
        defaultValues: defaultValues,
    })
}
