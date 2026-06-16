import * as z from "zod"

export const eventFormSchema = z.object({
    event_name: z.string().min(1, "Event name is required"),
})

export type EventFormValues = z.infer<typeof eventFormSchema>
