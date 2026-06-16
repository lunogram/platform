import * as z from "zod"

export const listCreateFormSchema = z.object({
    name: z.string().trim().min(1, "Name is required"),
    type: z.enum(["dynamic", "static"]),
})

export type ListCreateFormValues = z.infer<typeof listCreateFormSchema>
