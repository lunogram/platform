import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { ProjectContext } from "@/contexts"
import { useCallback, useContext, useMemo, useState } from "react"

import oapiClient, { type ActionMeta } from "@/oapi/client"
import { useResolver } from "@/hooks"

import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { ArrowLeft, ArrowRight, Search, Zap } from "lucide-react"
import { StaggeredMosaic } from "@/components/icon-mosaic"

export default function CreateAction() {
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()
    const [hoveredMeta, setHoveredMeta] = useState<ActionMeta | null>(null)
    const [searchQuery, setSearchQuery] = useState("")

    const [actionMetas] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/actions/meta", {
                params: { path: { projectID: project.id } },
            })
            return data ?? null
        }, [project.id]),
    )

    const filteredMetas = useMemo(() => {
        if (!actionMetas) return null
        if (!searchQuery) return actionMetas
        const q = searchQuery.toLowerCase()
        return actionMetas.filter(
            (m: ActionMeta) =>
                m.name.toLowerCase().includes(q) ||
                m.type.toLowerCase().includes(q) ||
                m.description?.toLowerCase().includes(q),
        )
    }, [actionMetas, searchQuery])

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
                {/* Ambient mosaic — faded right-side background */}
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
                            onClick={() => navigate(`/projects/${project.id}/actions`)}
                        >
                            <ArrowLeft className="h-4 w-4" />
                        </Button>
                        <div className="space-y-0.5">
                            <h1 className="text-2xl font-semibold tracking-tight">
                                {t("action.create.title", "Create Action")}
                            </h1>
                            <p className="text-sm text-muted-foreground">
                                {t(
                                    "action.create.description",
                                    "Select the type of action you want to create.",
                                )}
                            </p>
                        </div>
                    </div>
                </div>
            </div>

            {/* Action type cards — below header */}
            <div className="flex-1 overflow-y-auto p-4 sm:p-6 flex flex-col gap-4 sm:gap-6">
                {/* Search */}
                <div className="relative sm:max-w-sm">
                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        placeholder={t("search_actions", "Search actions...")}
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                    />
                </div>

                {!filteredMetas ? (
                    <div className="flex items-center justify-center h-32 text-muted-foreground">
                        <p className="text-sm">{t("loading", "Loading...")}</p>
                    </div>
                ) : filteredMetas.length === 0 ? (
                    <div className="flex flex-col items-center gap-2 text-muted-foreground py-12">
                        <Zap className="h-8 w-8" />
                        <p>
                            {searchQuery
                                ? t("no_results")
                                : t("no_actions", "No action types available")}
                        </p>
                    </div>
                ) : (
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                        {filteredMetas.map((meta: ActionMeta) => (
                            <Card
                                key={meta.type}
                                role="button"
                                className="group flex items-center gap-4 p-4 cursor-pointer transition-colors hover:border-primary hover:bg-accent/50"
                                onClick={() =>
                                    navigate(`/projects/${project.id}/actions/new/${meta.type}`)
                                }
                                onMouseEnter={() => setHoveredMeta(meta)}
                                onMouseLeave={() => setHoveredMeta(null)}
                            >
                                {/* Icon */}
                                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border bg-background">
                                    {meta.icon ? (
                                        <img
                                            src={meta.icon}
                                            alt={meta.name}
                                            className="h-6 w-6 object-contain"
                                        />
                                    ) : (
                                        <Zap className="h-4 w-4 text-muted-foreground" />
                                    )}
                                </div>

                                {/* Content */}
                                <div className="flex-1 min-w-0">
                                    <span className="text-sm font-medium">{meta.name}</span>
                                    {meta.description && (
                                        <p className="text-sm text-muted-foreground mt-0.5 line-clamp-1">
                                            {meta.description}
                                        </p>
                                    )}
                                </div>

                                {/* Arrow */}
                                <ArrowRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity shrink-0" />
                            </Card>
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}
