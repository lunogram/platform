import { ChevronRight, Loader2 } from "lucide-react"
import { Link, Outlet, useLocation, useNavigate } from "react-router"
import { useCallback, useContext, useMemo, memo, useState, useEffect, useRef } from "react"
import { useQuery } from "@tanstack/react-query"
import {
    CampaignContext,
    LocaleContext,
    ProjectContext,
    TemplateContext,
    type LocaleSelection,
} from "@/contexts"
import { CampaignVariableProvider } from "../CampaignVariableContext"
import { toast } from "sonner"
import { projectLocalesQuery } from "@/hooks/use-project-locales"
import { wasReported } from "@/lib/query"
import { problemDetail } from "@/utils"
import { defaultTemplateData } from "@/validation/campaign/template/data"
import { useCreateTemplate } from "../queries"

import { Pagination, PaginationContent, PaginationItem } from "@/components/ui/pagination"

import { LocaleSelect } from "@/components/locale/select"
import { Button } from "@/components/ui/button"
import { TemplateWorkflowContext } from "./contexts"
import { t } from "i18next"

interface CampaignStepProps {
    steps: Array<{ name: string; href: string }>
}

const TemplateSteps = memo(function CampaignSteps({ steps }: CampaignStepProps) {
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

// reportUnlessShown toasts an error, unless the mutation cache already did.
function reportUnlessShown(err: unknown, fallback: string) {
    if (!wasReported(err)) toast.error(problemDetail(err) ?? fallback)
}

export default function Template() {
    const [campaign] = useContext(CampaignContext)
    const [project] = useContext(ProjectContext)
    const [template] = useContext(TemplateContext)
    const location = useLocation()
    const navigate = useNavigate()
    const createTemplate = useCreateTemplate()
    const locales = useQuery(projectLocalesQuery(project.id))

    const [localeState, setLocaleSelection] = useState<LocaleSelection>({
        allLocales: [],
    })
    const [isNextLoading, setIsNextLoading] = useState(false)
    const [canProceed, setCanProceed] = useState(true)
    const handler = useRef<(() => Promise<boolean> | boolean) | null>(null)

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

    const steps = useMemo(() => {
        const basePath = `/projects/${project.id}/campaigns/${campaign.id}/templates/${template.id}`

        const navSteps = [{ name: "Content", href: basePath }]

        if (campaign.channel === "email") {
            navSteps.push({ name: "Editor", href: `${basePath}/email/editor` })
        }

        navSteps.push({ name: "Review", href: `${basePath}/review` })

        return navSteps
    }, [project.id, campaign.id, campaign.channel, template.id])

    const nextStep = useMemo(() => {
        const currentIndex = steps.findIndex((step) => step.href === location.pathname)
        return currentIndex !== -1 && currentIndex < steps.length - 1
            ? steps[currentIndex + 1]
            : undefined
    }, [steps, location.pathname])

    const onSubmit = useCallback((fn: () => Promise<boolean> | boolean) => {
        handler.current = fn
        return () => {
            if (handler.current === fn) handler.current = null
        }
    }, [])

    const save = useCallback(async () => {
        if (!handler.current) return false
        try {
            return !!(await handler.current())
        } catch (err) {
            // Callers report a failed save themselves.
            console.error(err)
            return false
        }
    }, [])

    const submit = useCallback(async () => {
        if (!handler.current) return

        setIsNextLoading(true)
        try {
            const next = await handler.current()
            if (next && nextStep) navigate(nextStep.href)
        } catch (err) {
            reportUnlessShown(err, t("campaign.template.save_failed", "Couldn't save the template"))
        } finally {
            setIsNextLoading(false)
        }
    }, [navigate, nextStep])

    const workflowContextValue = useMemo(
        () => ({ onSubmit, submit, save, canProceed, setCanProceed }),
        [onSubmit, submit, save, canProceed],
    )

    const publish = useCallback(async () => {
        if (!handler.current) return

        setIsNextLoading(true)
        try {
            const published = await handler.current()
            if (published) navigate(`/projects/${project.id}/campaigns`)
        } catch (err) {
            reportUnlessShown(
                err,
                t("campaign.template.publish_failed", "Couldn't publish the campaign"),
            )
        } finally {
            setIsNextLoading(false)
        }
    }, [project.id, navigate])

    const navigateToTemplate = useCallback(
        (templateId: string) => {
            const basePath = `/projects/${project.id}/campaigns/${campaign.id}/templates/${templateId}`
            const suffix = location.pathname.split(/\/templates\/[^/]+/)[1] ?? ""
            navigate(basePath + suffix)
        },
        [project.id, campaign.id, location.pathname, navigate],
    )

    const handleLocaleChange = useCallback(
        async (localeKey: string) => {
            try {
                if (handler.current) {
                    const next = await handler.current()
                    if (!next) {
                        return
                    }
                }
            } catch (err) {
                reportUnlessShown(
                    err,
                    t("campaign.template.save_failed", "Couldn't save the template"),
                )
                return
            }

            const selectedTemplate = campaign.templates.find(
                (template) => template.locale === localeKey,
            )
            if (selectedTemplate) {
                navigateToTemplate(selectedTemplate.id)
                return
            }

            // The mutation adds the new template to the cached campaign, so the
            // template route can find it once we navigate there.
            createTemplate.mutate(
                { locale: localeKey, data: defaultTemplateData(campaign.channel) },
                { onSuccess: (created) => navigateToTemplate(created.id) },
            )
        },
        [campaign.templates, campaign.channel, createTemplate, navigateToTemplate],
    )

    return (
        <TemplateWorkflowContext.Provider value={workflowContextValue}>
            <LocaleContext.Provider value={[localeState, setLocaleSelection]}>
                <div className="flex flex-col h-screen">
                    <div className="flex flex-1 bg-muted/20 overflow-hidden">
                        {createTemplate.isPending ? (
                            <div className="flex items-center justify-center w-full">
                                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                            </div>
                        ) : (
                            // Keyed so every step's forms and editor state are seeded
                            // again when switching to another template.
                            <CampaignVariableProvider key={template.id}>
                                <Outlet />
                            </CampaignVariableProvider>
                        )}
                    </div>
                    <div className="border-t bg-background flex items-center justify-between px-6 py-4 min-w-0 gap-4">
                        {locales.isError ? (
                            <div className="shrink-0 flex items-center gap-2">
                                <span className="text-sm text-muted-foreground">
                                    {t("locale.load_failed", "Couldn't load locales.")}
                                </span>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => locales.refetch()}
                                >
                                    {t("retry", "Retry")}
                                </Button>
                            </div>
                        ) : (
                            <div className="shrink-0">
                                <LocaleSelect onChange={handleLocaleChange} />
                            </div>
                        )}
                        <TemplateSteps steps={steps} />
                        <div className="shrink-0">
                            {!nextStep ? (
                                <Button
                                    onClick={publish}
                                    isLoading={isNextLoading}
                                    disabled={isNextLoading || !canProceed}
                                >
                                    {t("publish", "Publish")}
                                </Button>
                            ) : (
                                <Button
                                    onClick={submit}
                                    isLoading={isNextLoading}
                                    disabled={isNextLoading || !canProceed}
                                >
                                    {t("next")}
                                </Button>
                            )}
                        </div>
                    </div>
                </div>
            </LocaleContext.Provider>
        </TemplateWorkflowContext.Provider>
    )
}
