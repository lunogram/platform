import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import * as z from "zod"
import type { Campaign, Template } from "@/types"
import { emailSetupFormSchema } from "@/validation/campaign/template/mail/setup"

export function useEmailForm(_campaign: Campaign, template?: Template) {
    const formSchema = emailSetupFormSchema.extend({
        sender_identity_id: z.string("From address is required").min(1),
    })
    const email = template?.type === "email" ? template : undefined

    const form = useForm({
        resolver: zodResolver(formSchema),
        defaultValues: {
            sender_identity_id: email?.sender_identity_id ?? "",
            from: {
                name: email?.data.from?.name ?? "",
            },
            subject: email?.data.subject ?? "",
            reply_to: email?.data.reply_to ?? "",
        },
    })

    return form
}
