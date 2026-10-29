import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { z } from "zod"
import type { Campaign, Template } from "@/types"
import { pushSetupFormSchema } from "@/validation/campaign/template/push/setup"

export function usePushForm(_campaign: Campaign, template?: Template) {
    const formSchema = pushSetupFormSchema.extend({})

    const defaultValues: z.infer<typeof pushSetupFormSchema> = {
        title: "",
        body: "",
        custom: undefined,
    }

    if (template && template.type == "push") {
        defaultValues.title = template.data.title
        defaultValues.body = template.data.body
        defaultValues.custom = template.data.custom
    }

    return useForm({
        resolver: zodResolver(formSchema),
        defaultValues,
    })
}
