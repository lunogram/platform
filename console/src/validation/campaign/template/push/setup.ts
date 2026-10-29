import * as z from "zod"

export const pushSetupFormSchema = z.object({
    title: z.string("Title is required").min(1, "Title is required"),
    body: z.string("Body is required").min(1, "Body is required"),
})

export type PushFormValues = z.infer<typeof pushSetupFormSchema>
