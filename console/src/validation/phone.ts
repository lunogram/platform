import * as z from "zod"

export const phoneSchema = z.e164(
    "Please enter a valid phone number in E.164 format (e.g., +1234567890)",
)

export const optionalPhoneSchema = z.e164().or(z.literal("")).optional()
