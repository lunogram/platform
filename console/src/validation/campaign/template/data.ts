import { z } from "zod"
import type { ChannelType, Template } from "@/types"

// These schemas list every key a template's data may hold. Keep them in step
// with what the backend reads (internal/providers/channels): a key only the
// console knows about is never sent.

export const emailTemplateDataSchema = z.object({
    from: z
        .object({
            name: z.string().default(""),
            address: z.string().default(""),
        })
        .default({ name: "", address: "" }),
    subject: z.string().default(""),
    cc: z.string().optional(),
    bcc: z.string().optional(),
    reply_to: z.string().optional(),
    preheader: z.string().optional(),
    type: z.literal("react-email").optional(),
    editorMode: z.enum(["code", "builder", "blocks"]).optional(),
    code: z
        .object({
            source: z.string(),
            bundle: z.string().optional(),
            bundle_hash: z.string().optional(),
        })
        .optional(),
    blocks: z.record(z.string(), z.unknown()).optional(),
    plaintext: z
        .object({
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
    /** Sent to the device alongside the notification. */
    data: z.record(z.string(), z.unknown()).optional(),
})

export const inboxTemplateDataSchema = z.object({
    title: z.string().default(""),
    body: z.string().default(""),
})

export const templateDataSchemas = {
    email: emailTemplateDataSchema,
    sms: textTemplateDataSchema,
    push: pushTemplateDataSchema,
    inbox: inboxTemplateDataSchema,
} as const satisfies Record<ChannelType, z.ZodObject>

// defaultTemplateData is the data a newly created template of a channel starts with.
export function defaultTemplateData(channel: ChannelType) {
    return templateDataSchemas[channel].parse({})
}

/** What a channel's Content form changes on its template when saved. */
export interface TemplateChanges {
    /** Merged over the template's current data. */
    data: Record<string, unknown>
    sender_identity_id: string | null
}

// The keys a channel's data may hold, or undefined for a channel the console
// doesn't know (the server's type is not checked at runtime).
function knownKeys(channel: ChannelType): Record<string, unknown> | undefined {
    return (templateDataSchemas as Partial<typeof templateDataSchemas>)[channel]?.shape
}

/** @returns the keys of a template's data that its channel's schema doesn't know. */
export function unknownTemplateDataKeys(channel: ChannelType, data: object): string[] {
    const known = knownKeys(channel)
    if (!known) return []
    return Object.keys(data).filter((key) => !(key in known))
}

/**
 * @returns the template data with only the keys its channel knows. Saves send
 * this, so a key written by an older console (such as `replyTo` or `editor`) is
 * dropped instead of being carried forward on every save.
 */
export function knownTemplateData(channel: ChannelType, data: object): Template["data"] {
    const known = knownKeys(channel)
    // Without a schema there is nothing to filter by, so send the data as it is.
    if (!known) return data as Template["data"]
    return Object.fromEntries(
        Object.entries(data).filter(([key]) => key in known),
    ) as Template["data"]
}

/** Logs a warning for each key a loaded template holds that its schema doesn't list. */
export function warnUnknownTemplateData(template: Template) {
    if (!knownKeys(template.type)) {
        console.warn(
            `Template ${template.id} has a channel the console does not know: ${template.type}`,
        )
        return
    }

    const unknown = unknownTemplateDataKeys(template.type, template.data)
    if (unknown.length > 0) {
        console.warn(
            `Template ${template.id} (${template.type}) has data keys the console does not know: ${unknown.join(", ")}`,
        )
    }
}
