import * as z from "zod"

export const createBroadcastSchema = z
    .object({
        campaign_id: z.string().min(1, "Campaign is required"),
        list_id: z.string().min(1, "List is required"),
        is_scheduled: z.boolean(),
        scheduled_at: z.string().optional(),
        variant: z.string().optional(),
    })
    .refine((data) => !data.is_scheduled || data.scheduled_at, {
        message: "Scheduled time is required",
        path: ["scheduled_at"],
    })

export type CreateBroadcastFormValues = z.infer<typeof createBroadcastSchema>
