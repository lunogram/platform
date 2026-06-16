import * as z from "zod"

export const newCampaignSchema = z
    .object({
        name: z.string().min(1, "Name is required"),
        channel: z.enum(["email", "sms", "push"]),
        transactional: z.boolean(),
        subscription_id: z.string().optional(),
    })
    .refine((data) => data.transactional || (data.subscription_id && data.subscription_id.length > 0), {
        message: "Subscription is required for non-transactional campaigns",
        path: ["subscription_id"],
    })

export type NewCampaignFormValues = z.infer<typeof newCampaignSchema>
