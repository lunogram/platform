import { useCallback, useContext, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Plus, Search, Puzzle, MoreHorizontal } from "lucide-react"
import api from "../../api"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { snakeToTitle } from "../../utils"
import type { Provider } from "../../types"
import IntegrationModal from "./IntegrationModal"
import type { UUID } from "@/types/common"

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

export default function Integrations() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
    const [isModalOpen, setIsModalOpen] = useState(false)
    const [provider, setProvider] = useState<Provider>()

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            return await api.providers.search(project.id, {
                limit: 50,
                search: debouncedQuery || undefined,
            } as any)
        }, [project.id, debouncedQuery]),
    )

    const providers = result?.results ?? []

    const handleArchive = async (id: UUID) => {
        if (!confirm(t("delete_integration_confirmation"))) return
        await api.providers.delete(project.id, id)
        await reload()
    }

    return (
        <div className="flex flex-col gap-6">
            {/* Header */}
            <h2 className="text-2xl font-semibold tracking-tight">{t("integrations")}</h2>

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
                    onClick={() => {
                        setProvider(undefined)
                        setIsModalOpen(true)
                    }}
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
                            <TableHead className="hidden sm:table-cell">{t("group")}</TableHead>
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
                                                : t("no_integrations_yet", "No integrations yet")}
                                        </p>
                                        {!debouncedQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => {
                                                    setProvider(undefined)
                                                    setIsModalOpen(true)
                                                }}
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
                                    onClick={() => {
                                        setProvider(p)
                                        setIsModalOpen(true)
                                    }}
                                >
                                    <TableCell className="font-medium">{p.name}</TableCell>
                                    <TableCell className="hidden sm:table-cell text-muted-foreground">
                                        {p.module}
                                    </TableCell>
                                    <TableCell className="hidden sm:table-cell">
                                        <Badge variant="secondary">{snakeToTitle(p.channel)}</Badge>
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
                                                <DropdownMenuItem
                                                    className="text-destructive"
                                                    onClick={async (e) => {
                                                        e.stopPropagation()
                                                        await handleArchive(p.id)
                                                    }}
                                                >
                                                    {t("archive")}
                                                </DropdownMenuItem>
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

            <IntegrationModal
                open={isModalOpen}
                onClose={setIsModalOpen}
                provider={provider}
                onChange={async () => await reload()}
            />
        </div>
    )
}
