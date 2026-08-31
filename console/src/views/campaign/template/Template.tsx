import { ChevronRight, Loader2 } from "lucide-react"
import { Link, Outlet, useLocation, useNavigate, useParams } from "react-router"
import { useCallback, useContext, useMemo, memo, useState, useEffect, useRef } from "react"
import { CampaignContext, LocaleContext, ProjectContext, type LocaleSelection } from "@/contexts"
import { CampaignVariableProvider } from "../CampaignVariableContext"
import api from "@/api"

import { Pagination, PaginationContent, PaginationItem } from "@/components/ui/pagination"

import { LocaleSelect } from "@/components/locale/select"
import { VariantSwitcher } from "./VariantSwitcher"
import { isEnterprise } from "@/config/enterprise"
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

export default function Template() {
    const [campaign, setCampaign] = useContext(CampaignContext)
    const [project] = useContext(ProjectContext)
    const { templateId } = useParams()
    const location = useLocation()
    const navigate = useNavigate()

    const [localeState, setLocaleSelection] = useState<LocaleSelection>({
        allLocales: [],
    })
    const [pageLoading, setPageLoading] = useState(true)
    const [isNextLoading, setIsNextLoading] = useState(false)
    const [canProceed, setCanProceed] = useState(true)
    const handler = useRef<(() => Promise<boolean> | boolean) | null>(null)

    const steps = useMemo(() => {
        const templates = campaign.templates || []
        const selectedTemplateId =
            templateId ??
            templates.find((t) => t.locale === project.locale && !t.variant)?.id ??
            templates.find((t) => !t.variant)?.id ??
            templates[0]?.id

        const basePath = `/projects/${project.id}/campaigns/${campaign.id}/templates/${selectedTemplateId}`

        const navSteps = [{ name: "Content", href: basePath }]

        if (campaign.channel === "email") {
            navSteps.push({ name: "Editor", href: `${basePath}/email/editor` })
        }

        navSteps.push({ name: "Review", href: `${basePath}/review` })

        return navSteps
    }, [project.id, campaign.id, campaign.channel, campaign.templates, project.locale, templateId])

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
        return !!(await handler.current())
    }, [])

    const submit = useCallback(async () => {
        if (!handler.current) return

        setIsNextLoading(true)
        try {
            const next = await handler.current()
            if (next && nextStep) navigate(nextStep.href)
        } finally {
            setIsNextLoading(false)
        }
    }, [navigate, nextStep])

    const currentTemplate = useMemo(
        () => campaign.templates?.find((t) => t.id === templateId),
        [campaign.templates, templateId],
    )
    const workflowContextValue = useMemo(
        () => ({ onSubmit, submit, save, canProceed, setCanProceed }),
        [onSubmit, submit, save, canProceed],
    )

    const publish = useCallback(async () => {
        if (!handler.current) return

        setIsNextLoading(true)
        try {
            await handler.current()
            navigate(`/projects/${project.id}/campaigns`)
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

    const currentVariant = currentTemplate?.variant ?? ""

    // Switching either axis of the (locale, variant) pair keeps the other
    // fixed, and creates the template when that combination has none yet -
    // the same on-the-fly creation the locale switcher has always done.
    const openTemplate = useCallback(
        async (localeKey: string, variantKey: string) => {
            if (handler.current) {
                const next = await handler.current()
                if (!next) {
                    return
                }
            }

            const selectedTemplate = campaign.templates.find(
                (template) => template.locale === localeKey && template.variant === variantKey,
            )
            if (selectedTemplate) {
                navigateToTemplate(selectedTemplate.id)
                return
            }

            setPageLoading(true)
            try {
                const template = await api.campaigns.templates.create(project.id, campaign.id, {
                    locale: localeKey,
                    variant: variantKey || undefined,
                    data: {},
                })

                setCampaign({
                    ...campaign,
                    templates: [...campaign.templates, template],
                })

                navigateToTemplate(template.id)
            } finally {
                setPageLoading(false)
            }
        },
        [campaign, project?.id, setCampaign, navigateToTemplate],
    )

    const handleLocaleChange = useCallback(
        (localeKey: string) => openTemplate(localeKey, currentVariant),
        [openTemplate, currentVariant],
    )

    const handleVariantChange = useCallback(
        (variantKey: string) => openTemplate(currentTemplate?.locale ?? project.locale, variantKey),
        [openTemplate, currentTemplate?.locale, project.locale],
    )

    // Fetch locales when template changes
    useEffect(() => {
        const fetchLocales = async () => {
            if (!project?.id) return

            setPageLoading(true)

            const allLocalesResult = await api.locales.search(project.id, {
                limit: 5,
            })
            if (currentTemplate) {
                try {
                    const selectedLocale = await api.locales.getByKey(
                        project.id,
                        currentTemplate.locale,
                    )
                    setLocaleSelection({
                        currentLocale: selectedLocale,
                        allLocales: allLocalesResult.results,
                    })
                } catch {
                    // Locale not found, use default or first available locale
                    console.warn(`Locale ${currentTemplate.locale} not found, using default`)
                    setLocaleSelection({
                        currentLocale: allLocalesResult.results[0],
                        allLocales: allLocalesResult.results,
                    })
                }
            } else {
                setLocaleSelection({
                    currentLocale: undefined,
                    allLocales: allLocalesResult.results,
                })
            }

            setPageLoading(false)
        }

        fetchLocales()
    }, [project?.id, currentTemplate])

    return (
        <TemplateWorkflowContext.Provider value={workflowContextValue}>
            <LocaleContext.Provider value={[localeState, setLocaleSelection]}>
                <div className="flex flex-col h-screen">
                    <div className="flex flex-1 bg-muted/20 overflow-hidden">
                        {pageLoading ? (
                            <div className="flex items-center justify-center w-full">
                                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                            </div>
                        ) : (
                            <CampaignVariableProvider>
                                <Outlet />
                            </CampaignVariableProvider>
                        )}
                    </div>
                    <div className="border-t bg-background flex items-center justify-between px-6 py-4 min-w-0 gap-4">
                        {templateId && (
                            <div className="flex shrink-0 items-center gap-2">
                                <LocaleSelect onChange={handleLocaleChange} />
                                {isEnterprise && (campaign.variants?.options?.length ?? 0) > 0 && (
                                    <VariantSwitcher
                                        variants={campaign.variants?.options ?? []}
                                        value={currentVariant}
                                        onChange={handleVariantChange}
                                    />
                                )}
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
