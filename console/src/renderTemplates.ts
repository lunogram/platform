import Handlebars from "handlebars"
import type { User } from "@/types"
import type { UUID } from "@/types/common"

export type RenderContext = {
    template_id: UUID
    campaign_id: UUID
    subscription_id: UUID
    reference_type?: string
    reference_id?: UUID
} & Record<string, unknown>

export interface Variables {
    user: User
}

export const compileTemplate = <T = unknown>(template: string) => {
    return Handlebars.compile<T>(template, {
        strict: false,
        noEscape: true,
    })
}

export const Render = (template: string, { user }: Variables) => {
    if (!template) return template

    try {
        return compileTemplate(template)({ user })
    } catch (err) {
        console.warn("Template render error:", err)
        return template
    }
}
