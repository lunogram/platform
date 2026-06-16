import * as z from "zod"

export const actionFormSchema = z.object({
    name: z.string().min(1, "Name is required"),
    config: z.record(z.string(), z.unknown()).default({}),
    payload: z.record(z.string(), z.unknown()).default({}),
})

export type ActionFormValues = z.infer<typeof actionFormSchema>
