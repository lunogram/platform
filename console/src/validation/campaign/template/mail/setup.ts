import * as z from "zod"

export const emailSetupFormSchema = z.object({
    subject: z.string("Subject is required").min(1, "Subject is required"),
    sender_identity_id: z.string("From address is required").min(1, "From address is required"),
    from: z.object({
        name: z.string().optional(),
    }),
    reply_to: z.email("Invalid reply-to email address").optional().or(z.literal("")),
})

export type EmailFormValues = z.infer<typeof emailSetupFormSchema>
