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

export const compileTemplate = <T = any>(template: string) => {
    return Handlebars.compile<T>(template, {
        strict: true,
    })
}

function createSafeProxy(obj: any): any {
    return new Proxy(obj ?? {}, {
        get(target, prop) {
            if (prop in target) return target[prop]
            return `{{${prop.toString()}}}`
        },
    })
}

export const Render = (template: string, { user }: Variables) => {
    if (!template) return template

    const safeUser = createSafeProxy(user)

    try {
        return compileTemplate(template)({
            user: safeUser,
        })
    } catch (err) {
        console.warn("Template render error:", err)
        return template // fallback: show raw template
    }
}
