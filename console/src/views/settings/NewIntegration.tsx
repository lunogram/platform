import { useCallback, useContext, useMemo, useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { ArrowLeft, ArrowRight, Puzzle, Search } from "lucide-react"

import oapiClient from "@/oapi/client"
import type { ActionMeta, ProviderMeta } from "@/oapi/client"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { snakeToTitle } from "../../utils"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { StaggeredMosaic } from "@/components/icon-mosaic"

type IntegrationKind = "provider" | "action"

type IntegrationOption = {
    kind: IntegrationKind
    type: string
    name: string
    description?: string
    icon?: string
    color?: string
    channels?: string[]
}

export default function NewIntegration() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const [failedIcons, setFailedIcons] = useState<Set<string>>(new Set())
    const [hoveredMeta, setHoveredMeta] = useState<IntegrationOption | null>(null)
    const [searchQuery, setSearchQuery] = useState("")

    const [options] = useResolver(
        useCallback(async () => {
            const [{ data: providers }, { data: actions }] = await Promise.all([
                oapiClient.GET("/api/admin/projects/{projectID}/providers/meta", {
                    params: { path: { projectID: project.id } },
                }),
                oapiClient.GET("/api/admin/projects/{projectID}/actions/meta", {
                    params: { path: { projectID: project.id } },
                }),
            ])

            const providerOptions: IntegrationOption[] = (providers ?? []).map(
                (provider: ProviderMeta) => ({
                    kind: "provider",
                    type: provider.type,
                    name: provider.name,
                    description: provider.description,
                    icon: provider.icon,
                    color: provider.color,
                    channels: provider.channels,
                }),
            )

            const actionOptions: IntegrationOption[] = (actions ?? []).map(
                (action: ActionMeta) => ({
                    kind: "action",
                    type: action.type,
                    name: action.name,
                    description: action.description,
                    icon: action.icon,
                    color: action.color,
                }),
            )

            return [...providerOptions, ...actionOptions]
        }, [project]),
    )

    const filteredOptions = useMemo(() => {
        if (!options) return null
        if (!searchQuery) return options
        const q = searchQuery.toLowerCase()
        return options.filter(
            (o: IntegrationOption) =>
                o.name.toLowerCase().includes(q) ||
                o.type.toLowerCase().includes(q) ||
                o.kind.toLowerCase().includes(q) ||
                o.channels?.some((c) => c.toLowerCase().includes(q)) ||
                o.description?.toLowerCase().includes(q),
        )
    }, [options, searchQuery])

    const handleIconError = useCallback((type: string) => {
        setFailedIcons((prev) => {
            if (prev.has(type)) return prev
            const next = new Set(prev)
            next.add(type)
            return next
        })
    }, [])

    const mosaicProvider = useMemo(() => {
        if (!hoveredMeta) return undefined
        return {
            id: hoveredMeta.type,
            name: hoveredMeta.name,
            icon: hoveredMeta.icon,
            color: hoveredMeta.color,
        }
    }, [hoveredMeta])

    return (
        <div className="flex flex-col min-h-full">
            {/* Header — ambient mosaic background */}
            <div className="border-b bg-card/50 relative overflow-hidden">
                <div
                    className="absolute top-1/2 -translate-y-1/2 left-[50%] xl:left-[30%] right-0 hidden lg:block pointer-events-none opacity-[0.8]"
                    style={{
                        maskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                        WebkitMaskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                    }}
                >
                    <StaggeredMosaic provider={mosaicProvider} cols={12} rows={4} />
                </div>

                <div className="p-4 sm:p-6 py-8 sm:py-10 relative z-20">
                    <div className="flex items-center gap-3">
                        <Button
                            variant="ghost"
                            size="icon"
                            type="button"
                            onClick={() => navigate(`/projects/${project.id}/integrations`)}
                        >
                            <ArrowLeft className="h-4 w-4" />
                        </Button>
                        <div className="space-y-0.5">
                            <h1 className="text-2xl font-semibold tracking-tight">
                                {t("add_integration", "Add Integration")}
                            </h1>
                            <p className="text-sm text-muted-foreground">
                                {t(
                                    "pick_integration_hint",
                                    "Choose a provider or action to connect to your project.",
                                )}
                            </p>
                        </div>
                    </div>
                </div>
            </div>

            {/* Provider cards — below header */}
            <div className="flex-1 overflow-y-auto p-4 sm:p-6 flex flex-col gap-4 sm:gap-6">
                {/* Search */}
                <div className="relative sm:max-w-sm">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_integrations", "Search integrations...")}
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                    />
                </div>

                {!filteredOptions ? (
                    <div className="flex items-center justify-center h-32 text-muted-foreground">
                        <p className="text-sm">{t("loading", "Loading...")}</p>
                    </div>
                ) : (
                    <div>
                        {filteredOptions.length === 0 ? (
                            <div className="flex flex-col items-center gap-2 text-muted-foreground py-12">
                                <Puzzle className="h-8 w-8" />
                                <p>
                                    {searchQuery
                                        ? t("no_results")
                                        : t("no_integrations_available", "No integrations available")}
                                </p>
                            </div>
                        ) : (
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                {filteredOptions.map((option: IntegrationOption) => {
                                    const key = `${option.kind}:${option.type}`
                                    const showIcon = option.icon && !failedIcons.has(key)
                                    return (
                                        <Card
                                            key={key}
                                            role="button"
                                            aria-label={`Select integration ${option.name}`}
                                            className="group flex items-center gap-4 p-4 cursor-pointer transition-colors hover:border-primary hover:bg-accent/50"
                                            onClick={() =>
                                                navigate(
                                                    `/projects/${project.id}/integrations/new/${option.kind}/${option.type}`,
                                                )
                                            }
                                            onMouseEnter={() => setHoveredMeta(option)}
                                            onMouseLeave={() => setHoveredMeta(null)}
                                        >
                                            {/* Icon */}
                                            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border bg-background">
                                                {showIcon ? (
                                                    <img
                                                        src={option.icon}
                                                        alt={option.name}
                                                        className="h-6 w-6 object-contain"
                                                        onError={() => handleIconError(key)}
                                                    />
                                                ) : (
                                                    <Puzzle className="h-4 w-4 text-muted-foreground" />
                                                )}
                                            </div>

                                            {/* Content */}
                                            <div className="flex-1 min-w-0">
                                                <div className="flex items-center gap-2">
                                                    <span className="text-sm font-medium">
                                                        {option.name}
                                                    </span>
                                                    {option.kind === "provider" &&
                                                        option.channels?.map((ch) => (
                                                            <Badge
                                                                key={ch}
                                                                variant="secondary"
                                                                className="text-[10px] px-1.5 py-0"
                                                            >
                                                                {snakeToTitle(ch)}
                                                            </Badge>
                                                        ))}
                                                    {option.kind === "action" && (
                                                        <Badge
                                                            variant="secondary"
                                                            className="text-[10px] px-1.5 py-0"
                                                        >
                                                            {t("action.singular", "Action")}
                                                        </Badge>
                                                    )}
                                                </div>
                                                {option.description && (
                                                    <p className="text-sm text-muted-foreground mt-0.5 line-clamp-1">
                                                        {option.description}
                                                    </p>
                                                )}
                                            </div>

                                            {/* Arrow */}
                                            <ArrowRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
                                        </Card>
                                    )
                                })}
                            </div>
                        )}
                    </div>
                )}
            </div>
        </div>
    )
}
