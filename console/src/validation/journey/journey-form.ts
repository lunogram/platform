import * as z from "zod"

export const journeyFormSchema = z.object({
    name: z.string().trim().min(1, "Name is required"),
    description: z.string().optional(),
    status: z.enum(["draft", "published", "archived"]).default("draft"),
})

export type JourneyFormValues = z.infer<typeof journeyFormSchema>
