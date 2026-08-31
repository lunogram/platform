import * as z from "zod"

export const passwordSchema = z
    .string()
    .min(1, "Your password is required")
    .max(1024, "Your password is too long")

export const registerSchema = z.object({
    email: z.email("Please enter a valid email address"),
    password: passwordSchema,
    first_name: z.string().max(255).optional(),
    last_name: z.string().max(255).optional(),
})

export type RegisterFormValues = z.infer<typeof registerSchema>

export const forgotPasswordSchema = z.object({
    email: z.email("Please enter a valid email address"),
})

export type ForgotPasswordFormValues = z.infer<typeof forgotPasswordSchema>

export const resetPasswordSchema = z
    .object({
        password: passwordSchema,
        confirm: z.string(),
    })
    .refine((values) => values.password === values.confirm, {
        message: "The two passwords do not match",
        path: ["confirm"],
    })

export type ResetPasswordFormValues = z.infer<typeof resetPasswordSchema>

export const changePasswordSchema = z
    .object({
        current_password: z.string().min(1, "Your current password is required"),
        password: passwordSchema,
        confirm: z.string(),
    })
    .refine((values) => values.password === values.confirm, {
        message: "The two passwords do not match",
        path: ["confirm"],
    })

export type ChangePasswordFormValues = z.infer<typeof changePasswordSchema>
