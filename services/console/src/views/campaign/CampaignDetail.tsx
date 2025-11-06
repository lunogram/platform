import { ChevronRight, Loader2 } from "lucide-react"
import { Link, Outlet, useLocation, useSearchParams } from "react-router"
import { useCallback, useContext, useMemo, useRef, useState, useEffect, memo } from "react"
import { CampaignContext, LocaleContext, ProjectContext, type LocaleSelection } from "@/contexts"
import { CampaignDetailContext } from "./contexts"
import { useTranslation } from "react-i18next"
import api from "@/api"

import { Button } from "@/components/ui/button"
import { LocaleSelect } from "@/components/locale-select"

import {
    Pagination,
    PaginationContent,
    PaginationItem,
} from "@/components/ui/pagination"

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

export default function CampaignDetail() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [campaign, setCampaign] = useContext(CampaignContext)
    const [searchParams, setSearchParams] = useSearchParams();

    const [localeState, setLocaleSelection] = useState<LocaleSelection>({ allLocales: [] })
    const [loading, setLoading] = useState(false)

    const handleLocaleChange = useCallback(async (localeKey: string) => {
        setSearchParams((params) => {
            params.set('locale', localeKey);
            return params;
        }, { replace: true })

        const hasTemplate = campaign.templates.find((template) => template.locale === localeKey);
        if (hasTemplate) {
            return;
        }

        setLoading(true);
        const template = await api.templates.create(project.id, {
            campaign_id: campaign.id,
            locale: localeKey,
            type: campaign.channel,
            name: undefined,
            data: {}
        });

        setCampaign({
            ...campaign,
            templates: [...campaign.templates, template]
        });

        setLoading(false);
    }, [campaign, project?.id, setCampaign, setSearchParams]);

    // Fetch locales on mount and set default based on project default locale or URL param
    useEffect(() => {
        const fetchInitialLocales = async () => {
            if (!project?.id) return

            const localeKey = searchParams.get('locale') ?? project.locale

            // Fetch all locales and the selected locale
            const [allLocalesResult, selectedLocale] = await Promise.all([
                api.locales.search(project.id, { limit: 100 }),
                api.locales.getByKey(project.id, localeKey)
            ])

            setSearchParams((params) => {
                params.set('locale', selectedLocale.key);
                return params;
            }, { replace: true })

            setLocaleSelection({
                currentLocale: selectedLocale,
                allLocales: allLocalesResult.results,
            })

            setLoading(false)
        }

        fetchInitialLocales()
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [project?.id])

    const steps = useMemo(() => [
        { name: "Setup", href: `/projects/${project.id}/campaigns/${campaign.id}` },
        { name: "Content", href: `/projects/${project.id}/campaigns/${campaign.id}/content` },
        { name: "Review", href: `/projects/${project.id}/campaigns/${campaign.id}/review` },
    ], [project.id, campaign.id])

    const [isLoading, setIsLoading] = useState(false)

    async function handleNext() {
        try {
            setIsLoading(true)
            await next()
            // if (nextStep) navigate(nextStep.href)
        } finally {
            setIsLoading(false)
        }
    }

    const handler = useRef<(() => Promise<boolean> | boolean) | null>(null);

    const onNext = useCallback((fn: () => Promise<boolean> | boolean) => {
        handler.current = fn;
        return () => {
            if (handler.current === fn) handler.current = null;
        };
    }, []);

    const next = useCallback(async () => {
        if (!handler.current) return

        const next = await handler.current();
        if (!next) {
            return;
        }

        // TODO: proceed
    }, []);

    const contextValue = useMemo(() => ({ onNext, next }), [onNext, next]);

    return (
        <LocaleContext.Provider value={[localeState, setLocaleSelection]}>
            <CampaignDetailContext.Provider value={contextValue}>
                <div className="flex flex-col h-screen">
                    <div className="flex flex-1 bg-muted/20 overflow-hidden flex-row bg-muted/20">
                        {loading ? (
                            <div className="flex items-center justify-center w-full">
                                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                            </div>
                        ) : (
                            <Outlet />
                        )}
                    </div >
                    <div className="border-t bg-background flex items-center justify-between px-6 py-4">
                        <div>
                            <LocaleSelect onChange={handleLocaleChange} />
                        </div>
                        <div className="mx-auto w-full flex items-center gap-4">
                            <CampaignSteps steps={steps} />
                        </div>
                        <div>
                            <Button variant="default" onClick={handleNext} isLoading={isLoading}>
                                {t('next')}
                            </Button>
                        </div>
                    </div>
                </div >
            </CampaignDetailContext.Provider>
        </LocaleContext.Provider>
    )
}
