import * as z from "zod"

export const inboxSetupFormSchema = z.object({
    title: z.string("Title is required").min(1, "Title is required"),
    body: z.string().default(""),
})
