import * as z from "zod"

export const createUserSchema = z
    .object({
        external_id: z.string().optional(),
        email: z.email("Invalid email address").optional().or(z.literal("")),
        phone: z
            .e164("Invalid phone number, must be in E.164 format (e.g., +1234567890)")
            .optional()
            .or(z.literal("")),
        timezone: z.string().optional(),
        locale: z.string().optional(),
    })
    .refine((data) => data.external_id?.trim() || data.email, {
        message: "Either identifier or email is required",
        path: ["external_id"],
    })

export type CreateUserFormValues = z.infer<typeof createUserSchema>
