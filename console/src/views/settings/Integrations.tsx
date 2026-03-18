import { useCallback, useContext, useMemo, useRef, useState } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { Plus, Search, Puzzle, MoreHorizontal } from "lucide-react"
import oapiClient from "@/oapi/client"
import type { Provider, ProviderMeta } from "@/oapi/client"
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

export default function Integrations() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
    const [hoveredProvider, setHoveredProvider] = useState<Provider | null>(null)

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/providers", {
                params: {
                    path: { projectID: project.id },
                    query: {
                        limit: 50,
                    },
                },
            })
            return data
        }, [project.id, debouncedQuery]),
    )

    // Load provider metas so we can resolve icons for the mosaic
    const [metas] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/providers/meta",
                {
                    params: { path: { projectID: project.id } },
                },
            )
            return data
        }, [project.id]),
    )

    const providers = result?.results ?? []

    const mosaicProvider = useMemo(() => {
        if (!hoveredProvider || !metas) return undefined
        const meta = metas.find(
            (m: ProviderMeta) =>
                m.type === hoveredProvider.module,
        )
        return {
            id: hoveredProvider.module,
            name: meta?.name ?? hoveredProvider.module,
            icon: meta?.icon,
            color: meta?.color,
        }
    }, [hoveredProvider, metas])

    const isProviderLocked = useCallback(
        (provider: Provider): boolean => {
            if (!metas) return false
            const meta = metas.find(
                (m: ProviderMeta) => m.type === provider.module,
            )
            return meta?.locked === true
        },
        [metas],
    )

    const handleArchive = async (id: string) => {
        if (!confirm(t("delete_integration_confirmation"))) return
        await oapiClient.DELETE("/api/admin/projects/{projectID}/providers/{providerID}", {
            params: { path: { projectID: project.id, providerID: id } },
        })
        await reload()
    }

    return (
        <div className="flex flex-col min-h-full">
            {/* Header with ambient mosaic */}
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
                                    "Connect email, SMS, and push notification providers to send messages.",
                                )}
                            </p>
                        </div>
                    </div>
                </div>
            </div>

            {/* Content */}
            <div className="flex-1 overflow-y-auto p-4 sm:p-6 flex flex-col gap-4 sm:gap-6">
                {/* Search and Actions */}
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
                    >
                        <Plus className="mr-2 h-4 w-4" />
                        {t("add_integration")}
                    </Button>
                </div>

                {/* Table */}
                <div className="rounded-lg border bg-card">
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>{t("name")}</TableHead>
                                <TableHead className="hidden sm:table-cell">{t("type")}</TableHead>
                                <TableHead className="hidden sm:table-cell">{t("channel_list", "Channels")}</TableHead>
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
                                            <Skeleton className="h-4 w-16" />
                                        </TableCell>
                                        <TableCell>
                                            <Skeleton className="h-4 w-8" />
                                        </TableCell>
                                    </TableRow>
                                ))
                            ) : providers.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={4} className="h-32 text-center">
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
                                                >
                                                    <Plus className="mr-2 h-4 w-4" />
                                                    {t("add_integration")}
                                                </Button>
                                            )}
                                        </div>
                                    </TableCell>
                                </TableRow>
                            ) : (
                                providers.map((p) => (
                                    <TableRow
                                        key={p.id}
                                        className="cursor-pointer"
                                        onClick={() =>
                                            navigate(`/projects/${project.id}/integrations/${p.id}`)
                                        }
                                        onMouseEnter={() => setHoveredProvider(p)}
                                        onMouseLeave={() => setHoveredProvider(null)}
                                    >
                                        <TableCell className="font-medium">{p.name}</TableCell>
                                        <TableCell className="hidden sm:table-cell text-muted-foreground">
                                            {p.module}
                                        </TableCell>
                                        <TableCell className="hidden sm:table-cell">
                                            <div className="flex gap-1">
                                                {p.channels?.map((ch) => (
                                                    <Badge key={ch} variant="secondary">
                                                        {snakeToTitle(ch)}
                                                    </Badge>
                                                ))}
                                            </div>
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
                                                    {isProviderLocked(p) ? (
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
                                                                await handleArchive(p.id)
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

                    {providers.length > 0 && (
                        <div className="flex items-center justify-between border-t px-4 py-3">
                            <p className="text-sm text-muted-foreground">
                                {providers.length}{" "}
                                {providers.length === 1
                                    ? t("integration", "integration")
                                    : t("integrations")}
                            </p>
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}
