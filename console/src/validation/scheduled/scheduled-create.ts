import * as z from "zod"

export const scheduledCreateSchema = z.object({
    scheduled_name: z.string().min(1, "Schedule name is required"),
    scheduled_at: z.string().optional(),
    start_at: z.string().optional(),
    interval: z.string().optional(),
})

export type ScheduledCreateFormValues = z.infer<typeof scheduledCreateSchema>
