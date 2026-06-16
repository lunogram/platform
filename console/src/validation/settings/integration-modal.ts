import * as z from "zod"

export const providerFormSchema = z.object({
    name: z.string().min(1, "Name is required"),
    data: z.record(z.string(), z.unknown()).optional(),
    module: z.string().min(1, "Module is required"),
    link_wrap: z.boolean().default(false),
})

export type ProviderFormValues = z.infer<typeof providerFormSchema>
