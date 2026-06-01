import * as z from "zod"

export const newCampaignSchema = z.object({
    name: z.string().min(1, "Name is required"),
    channel: z.enum(["email", "sms", "push"]),
    transactional: z.boolean(),
    subscription_id: z.string().optional(),
})

export type NewCampaignFormValues = z.infer<typeof newCampaignSchema>
