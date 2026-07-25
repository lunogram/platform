import { z } from "zod"

export const emailTemplateDataSchema = z.object({
    from: z
        .object({
            name: z.string().default(""),
            address: z.string().default(""),
        })
        .default({ name: "", address: "" }),
    subject: z.string().default(""),
    editor: z.enum(["code", "visual"]).default("visual"),
    cc: z.string().optional(),
    bcc: z.string().optional(),
    reply_to: z.string().optional(),
    preheader: z.string().optional(),
    // Which document the backend compiles. Absent means "react-email".
    type: z.enum(["react-email", "templatical"]).optional(),
    code: z
        .object({
            source: z.string(),
            bundle: z.string().optional(),
            bundle_hash: z.string().optional(),
        })
        .optional(),
    // The Templatical document, present only when type is "templatical".
    blocks: z.record(z.string(), z.unknown()).optional(),
    plaintext: z
        .object({
            generated: z.string().optional(),
            custom: z.string().optional(),
        })
        .optional(),
})

export const textTemplateDataSchema = z.object({
    body: z.string().default(""),
})

export const pushTemplateDataSchema = z.object({
    title: z.string().default(""),
    body: z.string().default(""),
    url: z.string().default(""),
    custom: z.record(z.string(), z.unknown()).default({}),
})

export const inboxTemplateDataSchema = z.object({
    title: z.string().default(""),
    body: z.string().default(""),
})
