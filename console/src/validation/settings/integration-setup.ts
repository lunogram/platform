import * as z from "zod"

export const integrationSetupSchema = z.object({
    kind: z.enum(["provider", "action"]),
    module: z.string().min(1, "Module is required"),
    name: z.string().min(1, "Name is required"),
    data: z.record(z.string(), z.unknown()).optional(),
    config: z.record(z.string(), z.unknown()).optional(),
    link_wrap: z.boolean().optional(),
    rate_limit: z.number().int().min(0).nullable().optional(),
    rate_interval: z.string().nullable().optional(),
})

export type IntegrationSetupFormValues = z.infer<typeof integrationSetupSchema>
