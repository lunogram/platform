import * as z from "zod"

export const identifierFormSchema = z.object({
    source: z.string().min(1, "Source is required"),
    external_id: z.string().min(1, "External ID is required"),
})

export type IdentifierFormValues = z.infer<typeof identifierFormSchema>
