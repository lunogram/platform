import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { z } from "zod"
import type { Campaign, Template } from "@/types"
import { inboxSetupFormSchema } from "@/validation/campaign/template/inbox/setup"

export function useInboxForm(_campaign: Campaign, template?: Template) {
    const formSchema = inboxSetupFormSchema.extend({})

    const defaultValues: z.infer<typeof inboxSetupFormSchema> = {
        title: "",
        body: "",
    }

    if (template && template.type == "inbox") {
        defaultValues.title = template.data.title
        defaultValues.body = template.data.body
    }

    return useForm({
        resolver: zodResolver(formSchema),
        defaultValues,
    })
}
