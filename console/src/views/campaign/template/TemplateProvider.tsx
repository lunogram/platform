import { useContext, useMemo, type PropsWithChildren } from "react"
import { useParams } from "react-router"
import { CampaignContext, TemplateContext } from "@/contexts"
import type { Template as TemplateType, UseStateContext } from "@/types"
import type { UUID } from "@/types/common"
import Template from "./Template"

interface TemplateProviderProps extends PropsWithChildren {
    template: TemplateType
    /** Called when a page picks another template of the campaign, e.g. by locale. */
    onSelect?: (templateId: UUID) => void
}

/**
 * TemplateProvider exposes one of the campaign's templates through
 * TemplateContext. The template is read from the cached campaign rather than
 * kept as a separate copy, so setting it writes into the campaign.
 */
export function TemplateProvider({ template, onSelect, children }: TemplateProviderProps) {
    const [, setCampaign] = useContext(CampaignContext)

    const value = useMemo<UseStateContext<TemplateType>>(
        () => [
            template,
            (next) => {
                const resolved = typeof next === "function" ? next(template) : next
                if (resolved.id !== template.id && onSelect) {
                    onSelect(resolved.id)
                    return
                }
                setCampaign((campaign) => ({
                    ...campaign,
                    templates: campaign.templates.map((t) => (t.id === resolved.id ? resolved : t)),
                }))
            },
        ],
        [template, onSelect, setCampaign],
    )

    return <TemplateContext.Provider value={value}>{children}</TemplateContext.Provider>
}

// TemplateRoute shows the template editor for the template in the URL.
export function TemplateRoute() {
    const [campaign] = useContext(CampaignContext)
    const { templateId } = useParams()
    const template = campaign.templates.find((t) => t.id === templateId)

    if (!template) {
        // Handled by the route's errorElement.
        throw new Error("Template not found")
    }

    return (
        <TemplateProvider template={template}>
            <Template />
        </TemplateProvider>
    )
}
