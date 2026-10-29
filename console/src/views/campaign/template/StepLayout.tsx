import { memo, useContext, useMemo, useState, type PropsWithChildren } from "react"
import { Link, useLocation, useNavigate } from "react-router"
import { useQuery } from "@tanstack/react-query"
import { ChevronRight } from "lucide-react"
import { t } from "i18next"
import { CampaignContext, ProjectContext, TemplateContext } from "@/contexts"
import { projectLocalesQuery } from "@/hooks/use-project-locales"
import { defaultTemplateData } from "@/validation/campaign/template/data"
import { LocaleSelect } from "@/components/locale/select"
import { Button } from "@/components/ui/button"
import { Pagination, PaginationContent, PaginationItem } from "@/components/ui/pagination"
import { useCreateTemplate } from "../queries"
import { nothingToSave, StepSaveContext, type SaveResult, type SaveStep } from "./contexts"

interface StepLayoutProps extends PropsWithChildren {
    /** Persists the step's edits; Next, a locale switch and a test send run it first. */
    save?: SaveStep
    /** Runs when Publish is clicked on the last step, after save succeeded. */
    onPublish?: () => Promise<SaveResult>
}

/**
 * StepLayout lays out one step of the template editor: the step's content above
 * the footer that moves between steps. The step hands in its own save, so the
 * footer always saves exactly the step on screen.
 */
export function StepLayout({ save = nothingToSave, onPublish, children }: StepLayoutProps) {
    return (
        <StepSaveContext.Provider value={save}>
            <div className="flex flex-1 bg-muted/20 overflow-hidden">{children}</div>
            <StepFooter save={save} onPublish={onPublish} />
        </StepSaveContext.Provider>
    )
}

interface StepLinksProps {
    steps: Array<{ name: string; href: string }>
}

const StepLinks = memo(function StepLinks({ steps }: StepLinksProps) {
    const location = useLocation()
    const isStepActive = (stepHref: string) => {
        return location.pathname === stepHref
    }

    return (
        <Pagination className="w-auto min-w-0">
            <PaginationContent>
                {steps.map((step, index) => (
                    <span key={step.name} className="flex items-center">
                        <PaginationItem>
                            <Link
                                to={`${step.href}${location.search}`}
                                className={`inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 hover:bg-accent hover:text-accent-foreground h-9 px-4 py-2 ${isStepActive(step.href) ? "bg-accent text-accent-foreground" : ""}`}
                                aria-label={`Go to ${step.name} step`}
                            >
                                {step.name}
                            </Link>
                        </PaginationItem>
                        {index < steps.length - 1 && <ChevronRight strokeWidth={1} />}
                    </span>
                ))}
            </PaginationContent>
        </Pagination>
    )
})

function StepFooter({
    save,
    onPublish,
}: Required<Pick<StepLayoutProps, "save">> & StepLayoutProps) {
    const [campaign] = useContext(CampaignContext)
    const [project] = useContext(ProjectContext)
    const [template] = useContext(TemplateContext)
    const location = useLocation()
    const navigate = useNavigate()
    const createTemplate = useCreateTemplate()
    const locales = useQuery(projectLocalesQuery(project.id))
    const [busy, setBusy] = useState(false)

    const steps = useMemo(() => {
        const basePath = `/projects/${project.id}/campaigns/${campaign.id}/templates/${template.id}`

        const navSteps = [{ name: "Content", href: basePath }]

        if (campaign.channel === "email") {
            navSteps.push({ name: "Editor", href: `${basePath}/email/editor` })
        }

        navSteps.push({ name: "Review", href: `${basePath}/review` })

        return navSteps
    }, [project.id, campaign.id, campaign.channel, template.id])

    const currentIndex = steps.findIndex((step) => step.href === location.pathname)
    const nextStep =
        currentIndex !== -1 && currentIndex < steps.length - 1 ? steps[currentIndex + 1] : undefined

    // Every footer action saves the step first, and stops when the save didn't
    // succeed. Resolves to whether the action went through.
    const run = async (action: () => Promise<boolean>) => {
        setBusy(true)
        try {
            return (await save()).ok && (await action())
        } finally {
            setBusy(false)
        }
    }

    const next = () =>
        run(async () => {
            if (nextStep) navigate(nextStep.href)
            return true
        })

    const publish = () =>
        run(async () => {
            const published = onPublish ? await onPublish() : { ok: true }
            if (published.ok) navigate(`/projects/${project.id}/campaigns`)
            return published.ok
        })

    const navigateToTemplate = (templateId: string) => {
        const basePath = `/projects/${project.id}/campaigns/${campaign.id}/templates/${templateId}`
        const suffix = location.pathname.split(/\/templates\/[^/]+/)[1] ?? ""
        navigate(basePath + suffix)
    }

    // Tells the locale picker whether it switched, so it never shows a locale
    // whose template isn't on screen.
    const switchLocale = (localeKey: string) =>
        run(async () => {
            const existing = campaign.templates.find((t) => t.locale === localeKey)
            if (existing) {
                navigateToTemplate(existing.id)
                return true
            }

            // The mutation adds the new template to the cached campaign, so the
            // template route can find it once we navigate there.
            try {
                const created = await createTemplate.mutateAsync({
                    locale: localeKey,
                    data: defaultTemplateData(campaign.channel),
                })
                navigateToTemplate(created.id)
                return true
            } catch {
                // The mutation cache already told the user why.
                return false
            }
        })

    const disabled = busy || createTemplate.isPending

    return (
        <div className="border-t bg-background flex items-center justify-between px-6 py-4 min-w-0 gap-4">
            {locales.isError ? (
                <div className="shrink-0 flex items-center gap-2">
                    <span className="text-sm text-muted-foreground">
                        {t("locale.load_failed", "Couldn't load locales.")}
                    </span>
                    <Button variant="outline" size="sm" onClick={() => locales.refetch()}>
                        {t("retry", "Retry")}
                    </Button>
                </div>
            ) : (
                <div className="shrink-0">
                    <LocaleSelect onChange={switchLocale} />
                </div>
            )}
            <StepLinks steps={steps} />
            <div className="shrink-0">
                {!nextStep ? (
                    <Button onClick={publish} isLoading={disabled} disabled={disabled}>
                        {t("publish", "Publish")}
                    </Button>
                ) : (
                    <Button onClick={next} isLoading={disabled} disabled={disabled}>
                        {t("next")}
                    </Button>
                )}
            </div>
        </div>
    )
}
