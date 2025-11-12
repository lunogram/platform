import { ChevronRight, Loader2 } from "lucide-react"
import { Link, Outlet, useLocation, useNavigate, useParams } from "react-router"
import { useCallback, useContext, useMemo, memo, useState, useEffect, useRef } from "react"
import { CampaignContext, LocaleContext, ProjectContext, type LocaleSelection } from "@/contexts"
import api from "@/api"

import {
    Pagination,
    PaginationContent,
    PaginationItem,
} from "@/components/ui/pagination"

import { LocaleSelect } from "@/components/locale/select"
import { Button } from "@/components/ui/button"
import { CampaignWorkflowContext } from './contexts';
import { t } from "i18next"

interface CampaignStepProps {
    steps: Array<{ name: string; href: string }>
}

const CampaignSteps = memo(function CampaignSteps({ steps }: CampaignStepProps) {
    const location = useLocation()
    const isStepActive = (stepHref: string) => {
        return location.pathname === stepHref
    }

    return (
        <Pagination>
            <PaginationContent>
                {steps.map((step, index) => (
                    <span key={step.name} className="flex items-center">
                        <PaginationItem>
                            <Link
                                to={`${step.href}${location.search}`}
                                className={`inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 hover:bg-accent hover:text-accent-foreground h-9 px-4 py-2 ${isStepActive(step.href) ? 'bg-accent text-accent-foreground' : ''}`}
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

export default function Campaign() {
    const [campaign, setCampaign] = useContext(CampaignContext)
    const [project] = useContext(ProjectContext)
    const { templateId } = useParams()
    const location = useLocation()
    const navigate = useNavigate()

    const [localeState, setLocaleSelection] = useState<LocaleSelection>({ allLocales: [] })
    const [pageLoading, setPageLoading] = useState(false)
    const [isNextLoading, setIsNextLoading] = useState(false)

    const handler = useRef<(() => Promise<boolean> | boolean) | null>(null);

    const steps = useMemo(() => {
        const campaignSteps = [
            { name: "Setup", href: `/projects/${project.id}/campaigns/${campaign.id}/setup` },
        ]

        // NOTE: always show template steps, even if no template is selected yet
        const selectedTemplateId = templateId ?? campaign.templates?.find(t => t.locale === project.locale)?.id ?? campaign.templates?.[0]?.id;
        const templateSteps = campaign.channel === 'email'
            ? [
                { name: "Content", href: `/projects/${project.id}/campaigns/${campaign.id}/templates/${selectedTemplateId}` },
                { name: "Template", href: `/projects/${project.id}/campaigns/${campaign.id}/templates/${selectedTemplateId}/email/editor` },
                { name: "Review", href: `/projects/${project.id}/campaigns/${campaign.id}/templates/${selectedTemplateId}/review` },
            ]
            : [
                { name: "Content", href: `/projects/${project.id}/campaigns/${campaign.id}/templates/${selectedTemplateId}` },
                { name: "Review", href: `/projects/${project.id}/campaigns/${campaign.id}/templates/${selectedTemplateId}/review` },
            ]

        return [...campaignSteps, ...templateSteps]
    }, [project.id, campaign.id, campaign.channel, campaign.templates, project.locale, templateId])

    const nextStep = useMemo(() => {
        const currentIndex = steps.findIndex(step => step.href === location.pathname)
        return currentIndex !== -1 && currentIndex < steps.length - 1 ? steps[currentIndex + 1] : undefined
    }, [steps, location.pathname])

    const onSubmit = useCallback((fn: () => Promise<boolean> | boolean) => {
        handler.current = fn;
        return () => {
            if (handler.current === fn) handler.current = null;
        };
    }, []);

    const submit = useCallback(async () => {
        if (!handler.current) return;

        setIsNextLoading(true);
        try {
            const next = await handler.current();
            if (next && nextStep) navigate(nextStep.href);
        } finally {
            setIsNextLoading(false);
        }
    }, [navigate, nextStep]);

    const currentTemplate = useMemo(() => campaign.templates?.find(t => t.id === templateId), [campaign.templates, templateId])
    const workflowContextValue = useMemo(() => ({ onSubmit, submit }), [onSubmit, submit]);

    const publish = useCallback(async () => {
        if (!handler.current) return;

        setIsNextLoading(true);
        try {
            await handler.current();
            navigate(`/projects/${project.id}/campaigns`);
        } finally {
            setIsNextLoading(false);
        }
    }, [project.id, navigate]);

    const navigateToTemplate = useCallback((templateId: string) => {
        const basePath = `/projects/${project.id}/campaigns/${campaign.id}/templates/${templateId}`;
        const suffix = location.pathname.split(/\/templates\/[^/]+/)[1] ?? "";
        navigate(basePath + suffix);
    }, [project.id, campaign.id, location.pathname, navigate]);

    const handleLocaleChange = useCallback(async (localeKey: string) => {
        if (handler.current) {
            const next = await handler.current();
            if (!next) {
                return;
            }
        }

        const selectedTemplate = campaign.templates.find((template) => template.locale === localeKey);
        if (selectedTemplate) {
            navigateToTemplate(selectedTemplate.id);
            return;
        }

        setPageLoading(true);
        const template = await api.templates.create(project.id, {
            campaign_id: campaign.id,
            locale: localeKey,
            type: campaign.channel,
            data: {}
        });

        setCampaign({
            ...campaign,
            templates: [...campaign.templates, template]
        });

        navigateToTemplate(template.id);
        setPageLoading(false);
    }, [campaign, project?.id, setCampaign, navigateToTemplate]);

    // Fetch locales when template changes
    useEffect(() => {
        const fetchLocales = async () => {
            if (!project?.id) return

            setPageLoading(true)

            try {
                const allLocalesResult = await api.locales.search(project.id, { limit: 100 })

                // If we have a current template, fetch its locale
                if (currentTemplate) {
                    const selectedLocale = await api.locales.getByKey(project.id, currentTemplate.locale)
                    setLocaleSelection({
                        currentLocale: selectedLocale,
                        allLocales: allLocalesResult.results,
                    })
                } else {
                    setLocaleSelection({
                        currentLocale: undefined,
                        allLocales: allLocalesResult.results,
                    })
                }
            } catch (error) {
                console.error('Failed to fetch locales:', error)
            }

            setPageLoading(false)
        }

        fetchLocales()
    }, [project?.id, currentTemplate])

    return (
        <CampaignWorkflowContext.Provider value={workflowContextValue}>
            <LocaleContext.Provider value={[localeState, setLocaleSelection]}>
                <div className="flex flex-col h-screen">
                    <div className="flex flex-1 bg-muted/20 overflow-hidden">
                        {pageLoading ? (
                            <div className="flex items-center justify-center w-full">
                                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                            </div>
                        ) : (
                            <Outlet />
                        )}
                    </div>
                    <div className="border-t bg-background flex items-center justify-center px-6 py-4">
                        {templateId && (
                            <div className="mr-auto">
                                <LocaleSelect onChange={handleLocaleChange} />
                            </div>
                        )}
                        <CampaignSteps steps={steps} />
                        <div className="ml-auto">
                            {!nextStep ? (
                                <Button onClick={publish} isLoading={isNextLoading} disabled={isNextLoading}>
                                    {t('publish', 'Publish')}
                                </Button>
                            ) : (
                                <Button onClick={submit} isLoading={isNextLoading} disabled={isNextLoading}>
                                    {t('next')}
                                </Button>
                            )}
                        </div>
                    </div>
                </div>
            </LocaleContext.Provider>
        </CampaignWorkflowContext.Provider>
    )
}
