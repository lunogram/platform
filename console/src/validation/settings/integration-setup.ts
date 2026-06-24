import * as z from "zod"

export const integrationSetupSchema = z.object({
    kind: z.enum(["provider", "action"]),
    module: z.string().min(1, "Module is required"),
    name: z.string().min(1, "Name is required"),
    data: z.record(z.string(), z.unknown()).optional(),
    config: z.record(z.string(), z.unknown()).optional(),
    link_wrap: z.boolean().default(false),
    // rate_limit is an override object ({ limit, interval }), matching the form
    // fields registered as rate_limit.limit / rate_limit.interval. Declaring it as
    // a scalar made zod reject every submit (the create button silently no-op'd).
    rate_limit: z
        .object({
            limit: z.number().int().min(0),
            interval: z.string(),
        })
        .optional(),
})

export type IntegrationSetupFormValues = z.infer<typeof integrationSetupSchema>
