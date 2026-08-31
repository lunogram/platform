import * as z from "zod"

export const broadcastResponseSchema = z.object({
    id: z.string(),
    project_id: z.string(),
    campaign_id: z.string(),
    list_id: z.string(),
    list_name: z.string(),
    list_type: z.enum(["static", "dynamic"]),
    variant: z
        .object({
            type: z.enum(["static", "expression"]),
            key: z.string().optional(),
            expression: z.string().optional(),
        })
        .nullish(),
    state: z.enum(["scheduled", "pending", "sending", "completed", "failed", "cancelled"]),
    total: z.number(),
    sent: z.number().optional().default(0),
    failed: z.number().optional().default(0),
    error: z.string().optional(),
    created_at: z.string(),
    updated_at: z.string(),
    started_at: z.string().optional(),
    completed_at: z.string().optional(),
    scheduled_at: z.string().optional(),
    campaign: z
        .object({
            id: z.string().optional().default(""),
            name: z.string().optional().default(""),
            channel: z.enum(["email", "push", "sms"]).optional().default("email"),
        })
        .optional(),
})

export type BroadcastResponse = z.infer<typeof broadcastResponseSchema>
