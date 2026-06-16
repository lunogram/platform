import * as z from "zod"

export const campaignVariableSchema = z.object({
    name: z.string(),
    default: z.string().optional(),
})

export const campaignSchema = z.object({
    name: z.string().min(1, "Name is required"),
    variables: z.array(campaignVariableSchema),
})

export type CampaignReviewFormData = z.infer<typeof campaignSchema>
