import { Outlet } from "react-router"
import { useContext, useEffect, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { LocaleContext, ProjectContext, TemplateContext, type LocaleSelection } from "@/contexts"
import { projectLocalesQuery } from "@/hooks/use-project-locales"
import { CampaignVariableProvider } from "../CampaignVariableContext"

/**
 * Template is the frame of the template editor. Each step renders its own
 * content and footer through StepLayout, so the footer saves that step.
 */
export default function Template() {
    const [project] = useContext(ProjectContext)
    const [template] = useContext(TemplateContext)
    const locales = useQuery(projectLocalesQuery(project.id))

    const [localeState, setLocaleSelection] = useState<LocaleSelection>({
        allLocales: [],
    })

    // LocaleSelect keeps its own copy in LocaleContext (it adds locales it
    // creates), so seed it from the query whenever the query or template changes.
    useEffect(() => {
        if (!locales.data) return
        setLocaleSelection({
            // Fall back to the first locale when the template's locale was deleted.
            currentLocale:
                locales.data.find((locale) => locale.key === template.locale) ?? locales.data[0],
            allLocales: locales.data,
        })
    }, [locales.data, template.locale])

    return (
        <LocaleContext.Provider value={[localeState, setLocaleSelection]}>
            <div className="flex flex-col h-screen">
                {/* Keyed so every step's forms and editor state are seeded again
                    when switching to another template. */}
                <CampaignVariableProvider key={template.id}>
                    <Outlet />
                </CampaignVariableProvider>
            </div>
        </LocaleContext.Provider>
    )
}
