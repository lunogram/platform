import * as z from "zod"

export const projectFormSchema = z.object({
    name: z.string().min(1, "Name is required"),
    description: z.string().optional(),
    locale: z.string().min(1, "Locale is required"),
    timezone: z.string().min(1, "Timezone is required"),
    text_opt_out_message: z.string().optional(),
    text_help_message: z.string().optional(),
})

export type ProjectFormValues = z.infer<typeof projectFormSchema>
