import * as z from "zod"

export const textSetupFormSchema = z.object({
    sender_identity_id: z.string().optional(),
    body: z.string("Message is required").min(1, "Message is required"),
})
