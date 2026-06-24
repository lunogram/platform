import * as z from "zod"

export const onboardingDomainSchema = z.object({
    email_address: z.email("Please enter a valid email address"),
    display_name: z.string().optional(),
})

export type OnboardingDomainFormValues = z.infer<typeof onboardingDomainSchema>
