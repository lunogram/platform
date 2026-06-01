import * as z from "zod"

export const subscriptionSchema = z.object({
    name: z.string().min(1, "Name is required"),
    is_public: z.boolean().optional(),
    channel: z.enum(["email", "push", "sms"]).optional(),
})

export type SubscriptionFormValues = z.infer<typeof subscriptionSchema>
