import { useCallback, useContext, useMemo, useRef, useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { Plus, Search, Puzzle, MoreHorizontal } from "lucide-react"
import oapiClient from "@/oapi/client"
import type { Action, ActionMeta, Provider, ProviderMeta } from "@/oapi/client"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { snakeToTitle } from "../../utils"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"
import { StaggeredMosaic } from "@/components/icon-mosaic"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

type IntegrationRow =
    | {
          id: string
          kind: "provider"
          module: string
          name: string
          channels?: string[]
      }
    | {
          id: string
          kind: "action"
          module: string
          name: string
      }

type IntegrationMeta = {
    kind: "provider" | "action"
    type: string
    name: string
    icon?: string
    color?: string
    locked?: boolean
}

export default function Integrations() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
    const [hoveredIntegration, setHoveredIntegration] = useState<IntegrationRow | null>(null)

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const [{ data: providers }, { data: actions }] = await Promise.all([
                oapiClient.GET("/api/admin/projects/{projectID}/providers", {
                    params: {
                        path: { projectID: project.id },
                        query: { limit: 50 },
                    },
                }),
                oapiClient.GET("/api/admin/projects/{projectID}/actions", {
                    params: {
                        path: { projectID: project.id },
                        query: { limit: 50, offset: 0 },
                    },
                }),
            ])

            return {
                providers: providers?.results ?? [],
                actions: actions?.results ?? [],
            }
            // eslint-disable-next-line react-hooks/exhaustive-deps
        }, [project.id, debouncedQuery]),
    )

    const [metas] = useResolver(
        useCallback(async () => {
            const [{ data: providerMetas }, { data: actionMetas }] = await Promise.all([
                oapiClient.GET("/api/admin/projects/{projectID}/providers/meta", {
                    params: { path: { projectID: project.id } },
                }),
                oapiClient.GET("/api/admin/projects/{projectID}/actions/meta", {
                    params: { path: { projectID: project.id } },
                }),
            ])

            const providerOptions: IntegrationMeta[] = (providerMetas ?? []).map(
                (m: ProviderMeta) => ({
                    kind: "provider",
                    type: m.type,
                    name: m.name,
                    icon: m.icon,
                    color: m.color,
                    locked: m.locked,
                }),
            )

            const actionOptions: IntegrationMeta[] = (actionMetas ?? []).map((m: ActionMeta) => ({
                kind: "action",
                type: m.type,
                name: m.name,
                icon: m.icon,
                color: m.color,
                locked: false,
            }))

            return [...providerOptions, ...actionOptions]
        }, [project.id]),
    )

    const integrations = useMemo<IntegrationRow[]>(() => {
        if (!result) return []

        const providerRows: IntegrationRow[] = result.providers.map((p: Provider) => ({
            id: p.id,
            kind: "provider",
            module: p.module,
            name: p.name,
            channels: p.channels,
        }))

        const actionRows: IntegrationRow[] = result.actions.map((a: Action) => ({
            id: a.id,
            kind: "action",
            module: a.type,
            name: a.name,
        }))

        const rows = [...providerRows, ...actionRows]
        if (!debouncedQuery) return rows

        const q = debouncedQuery.toLowerCase()
        return rows.filter((row) => {
            const byName = row.name.toLowerCase().includes(q)
            const byType = row.module.toLowerCase().includes(q)
            const byKind = row.kind.toLowerCase().includes(q)
            const byChannel =
                row.kind === "provider" && row.channels?.some((ch) => ch.toLowerCase().includes(q))
            return byName || byType || byKind || !!byChannel
        })
    }, [result, debouncedQuery])

    const findMeta = useCallback(
        (integration: IntegrationRow) => {
            if (!metas) return undefined
            return metas.find((m) => m.kind === integration.kind && m.type === integration.module)
        },
        [metas],
    )

    const mosaicProvider = useMemo(() => {
        if (!hoveredIntegration) return undefined
        const meta = findMeta(hoveredIntegration)
        return {
            id: `${hoveredIntegration.kind}:${hoveredIntegration.module}`,
            name: meta?.name ?? hoveredIntegration.module,
            icon: meta?.icon,
            color: meta?.color,
        }
    }, [hoveredIntegration, findMeta])

    const isProviderLocked = useCallback(
        (provider: IntegrationRow): boolean => {
            if (provider.kind !== "provider") return false
            const meta = findMeta(provider)
            return meta?.locked === true
        },
        [findMeta],
    )

    const handleArchive = async (integration: IntegrationRow) => {
        if (!confirm(t("delete_integration_confirmation"))) return

        if (integration.kind === "action") {
            await oapiClient.DELETE("/api/admin/projects/{projectID}/actions/{actionID}", {
                params: { path: { projectID: project.id, actionID: integration.id } },
            })
        } else {
            await oapiClient.DELETE(
                "/api/admin/projects/{projectID}/providers/{type}/{providerID}",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            type: integration.module,
                            providerID: integration.id,
                        },
                    },
                },
            )
        }

        await reload()
    }

    return (
        <div className="flex flex-col min-h-full">
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
                    <div className="flex items-start gap-4">
                        <div className="space-y-1">
                            <h1 className="text-2xl font-semibold tracking-tight">
                                {t("integrations")}
                            </h1>
                            <p className="text-sm text-muted-foreground">
                                {t(
                                    "integrations_description",
                                    "Connect providers and actions to power your messaging workflows.",
                                )}
                            </p>
                        </div>
                    </div>
                </div>
            </div>

            <div className="flex-1 overflow-y-auto p-4 sm:p-6 flex flex-col gap-4 sm:gap-6">
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                    <div className="relative sm:max-w-sm flex-1">
                        <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                        <Input
                            placeholder={t("search")}
                            value={searchQuery}
                            onChange={(e) => handleSearch(e.target.value)}
                            className="pl-9"
                        />
                    </div>
                    <Button
                        onClick={() => navigate(`/projects/${project.id}/integrations/new`)}
                        className="flex-1 sm:flex-initial"
                        aria-label={t("add_integration_from_header", "Add Integration from header")}
                    >
                        <Plus className="mr-2 h-4 w-4" />
                        {t("add_integration")}
                    </Button>
                </div>

                <div className="rounded-lg border bg-card">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>{t("name")}</TableHead>
                                <TableHead className="hidden sm:table-cell">{t("type")}</TableHead>
                                <TableHead className="hidden sm:table-cell">
                                    {t("kind", "Kind")}
                                </TableHead>
                                <TableHead className="hidden sm:table-cell">
                                    {t("channel_list", "Channels")}
                                </TableHead>
                                <TableHead className="w-[70px]" />
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {!result ? (
                                Array.from({ length: 3 }).map((_, i) => (
                                    <TableRow key={i}>
                                        <TableCell>
                                            <Skeleton className="h-4 w-28" />
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell">
                                            <Skeleton className="h-4 w-20" />
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell">
                                            <Skeleton className="h-4 w-20" />
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell">
                                            <Skeleton className="h-4 w-16" />
                                        </TableCell>
                                        <TableCell>
                                            <Skeleton className="h-4 w-8" />
                                        </TableCell>
                                    </TableRow>
                                ))
                            ) : integrations.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={5} className="h-32 text-center">
                                        <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                            <Puzzle className="h-8 w-8" />
                                            <p>
                                                {debouncedQuery
                                                    ? t("no_results")
                                                    : t(
                                                          "no_integrations_yet",
                                                          "No integrations yet",
                                                      )}
                                            </p>
                                            {!debouncedQuery && (
                                                <Button
                                                    variant="outline"
                                                    size="sm"
                                                    onClick={() =>
                                                        navigate(
                                                            `/projects/${project.id}/integrations/new`,
                                                        )
                                                    }
                                                    className="mt-2"
                                                    aria-label={t(
                                                        "add_integration_from_empty",
                                                        "Add Integration from empty state",
                                                    )}
                                                >
                                                    <Plus className="mr-2 h-4 w-4" />
                                                    {t("add_integration")}
                                                </Button>
                                            )}
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ) : (
                                integrations.map((integration) => (
                                    <TableRow
                                        key={`${integration.kind}:${integration.id}`}
                                        className="cursor-pointer"
                                        onClick={() =>
                                            navigate(
                                                `/projects/${project.id}/integrations/${integration.kind}/${integration.id}`,
                                            )
                                        }
                                        onMouseEnter={() => setHoveredIntegration(integration)}
                                        onMouseLeave={() => setHoveredIntegration(null)}
                                    >
                                        <TableCell className="font-medium">
                                            {integration.name}
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {integration.module}
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell">
                                            <Badge variant="secondary">
                                                {integration.kind === "provider"
                                                    ? t("provider_label", "Provider")
                                                    : t("action")}
                                            </Badge>
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell">
                                            {integration.kind === "provider" ? (
                                                <div className="flex gap-1">
                                                    {integration.channels?.map((ch) => (
                                                        <Badge key={ch} variant="secondary">
                                                            {snakeToTitle(ch)}
                                                        </Badge>
                                                    ))}
                                                </div>
                                            ) : (
                                                <span className="text-muted-foreground text-sm">
                                                    -
                                                </span>
                                            )}
                                        </TableCell>
                                        <TableCell>
                                            <DropdownMenu>
                                                <DropdownMenuTrigger asChild>
                                                    <Button
                                                        variant="ghost"
                                                        className="h-8 w-8 p-0"
                                                        onClick={(e) => e.stopPropagation()}
                                                        aria-label={t("options")}
                                                    >
                                                        <MoreHorizontal className="h-4 w-4" />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    {integration.kind === "provider" &&
                                                    isProviderLocked(integration) ? (
                                                        <Tooltip>
                                                            <TooltipTrigger asChild>
                                                                <DropdownMenuItem
                                                                    className="text-muted-foreground"
                                                                    disabled
                                                                    onClick={(e) =>
                                                                        e.stopPropagation()
                                                                    }
                                                                >
                                                                    {t("archive")}
                                                                </DropdownMenuItem>
                                                            </TooltipTrigger>
                                                            <TooltipContent side="left">
                                                                {t(
                                                                    "provider_locked",
                                                                    "This provider is locked and cannot be deleted",
                                                                )}
                                                            </TooltipContent>
                                                        </Tooltip>
                                                    ) : (
                                                        <DropdownMenuItem
                                                            className="text-destructive"
                                                            onClick={async (e) => {
                                                                e.stopPropagation()
                                                                await handleArchive(integration)
                                                            }}
                                                        >
                                                            {t("archive")}
                                                        </DropdownMenuItem>
                                                    )}
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>

                    {integrations.length > 0 && (
                        <div className="flex items-center justify-between border-t px-4 py-3">
                            <p className="text-sm text-muted-foreground">
                                {integrations.length} {t("integrations")}
                            </p>
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}
