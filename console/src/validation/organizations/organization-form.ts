import * as z from "zod"

export const organizationFormSchema = z.object({
    external_id: z.string().min(1, "Identifier is required"),
    name: z.string().optional(),
})

export type OrganizationFormValues = z.infer<typeof organizationFormSchema>
