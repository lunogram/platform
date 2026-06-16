import * as z from "zod"

export const apiKeySchema = z.object({
    name: z.string().min(1, "Name is required"),
    description: z.string().optional(),
    role: z.string().optional(),
})

export type ApiKeyFormValues = z.infer<typeof apiKeySchema>
