import * as z from "zod"

export const emailSetupFormSchema = z.object({
    subject: z.string("Subject is required").min(1, "Subject is required"),
    sender_identity_id: z.string().optional(),
    from: z.object({
        name: z.string().optional(),
    }),
    replyTo: z.email("Invalid reply-to email address").optional().or(z.literal("")),
})
