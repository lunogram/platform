import type { MouseEvent } from "react"
import { useCallback, useContext, useState, useRef } from "react"
import { useTranslation } from "react-i18next"
import { Plus, Search, Key, MoreHorizontal, Copy } from "lucide-react"
import { toast } from "sonner"
import api from "../../api"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import { snakeToTitle } from "../../utils"
import type { ProjectApiKey } from "../../types"
import type { UUID } from "@/types/common"
import ApiKeyDialog from "./ApiKeyDialog"

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

export default function ProjectApiKeys() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
    const [editing, setEditing] = useState<null | Partial<ProjectApiKey>>(null)
    const [isSaving, setIsSaving] = useState(false)

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const [result, , reload] = useResolver(
        useCallback(async () => {
            return await api.apiKeys.search(project.id, {
                limit: 50,
                search: debouncedQuery || undefined,
            })
        }, [project.id, debouncedQuery]),
    )

    const apiKeys = result?.results ?? []

    const handleArchive = async (id: UUID) => {
        if (confirm(t("delete_key_confirmation"))) {
            await api.apiKeys.delete(project.id, id)
            await reload()
        }
    }

    const handleCopy = async (event: MouseEvent<HTMLButtonElement>, value: string) => {
        event.preventDefault()
        event.stopPropagation()
        await navigator.clipboard.writeText(value)
        toast.success(t("copied_api_key", "Copied API Key"))
    }

    return (
        <div className="flex flex-col gap-6">
            {/* Header */}
            <h2 className="text-2xl font-semibold tracking-tight">{t("api_keys")}</h2>

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
                    onClick={() => setEditing({ role: "support" })}
                    className="flex-1 sm:flex-initial"
                    aria-label={t("create_key_from_header", "Create Key from header")}
                >
                    <Plus className="mr-2 h-4 w-4" />
                    {t("create_key")}
                </Button>
            </div>

            {/* Table */}
            <div className="rounded-lg border bg-card">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>{t("name")}</TableHead>
                            <TableHead>{t("role")}</TableHead>
                            <TableHead className="hidden md:table-cell">{t("value")}</TableHead>
                            <TableHead className="hidden lg:table-cell">
                                {t("description")}
                            </TableHead>
                            <TableHead className="w-[70px]" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {!result ? (
                            Array.from({ length: 3 }).map((_, i) => (
                                <TableRow key={i}>
                                    <TableCell>
                                        <Skeleton className="h-4 w-24" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-16" />
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <Skeleton className="h-4 w-40" />
                                    </TableCell>
                                    <TableCell className="hidden lg:table-cell">
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-8" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : apiKeys.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={5} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                        <Key className="h-8 w-8" />
                                        <p>
                                            {debouncedQuery
                                                ? t("no_results")
                                                : t("no_api_keys_yet", "No API keys yet")}
                                        </p>
                                        {!debouncedQuery && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setEditing({ role: "support" })}
                                                className="mt-2"
                                                aria-label={t(
                                                    "create_key_from_empty",
                                                    "Create Key from empty state",
                                                )}
                                            >
                                                <Plus className="mr-2 h-4 w-4" />
                                                {t("create_key")}
                                            </Button>
                                        )}
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            apiKeys.map((apiKey) => (
                                <TableRow
                                    key={apiKey.id}
                                    className="cursor-pointer"
                                    onClick={() => setEditing(apiKey)}
                                >
                                    <TableCell className="font-medium">{apiKey.name}</TableCell>
                                    <TableCell className="text-muted-foreground">
                                        {snakeToTitle(apiKey.role ?? "support")}
                                    </TableCell>
                                    <TableCell className="hidden md:table-cell">
                                        <div className="flex items-center gap-2">
                                            <code className="text-sm text-muted-foreground truncate max-w-[200px]">
                                                {apiKey.value}
                                            </code>
                                            <Button
                                                size="icon"
                                                variant="ghost"
                                                className="h-7 w-7 shrink-0"
                                                onClick={async (e) =>
                                                    await handleCopy(e, apiKey.value)
                                                }
                                            >
                                                <Copy className="h-3.5 w-3.5" />
                                            </Button>
                                        </div>
                                    </TableCell>
                                    <TableCell className="hidden lg:table-cell text-muted-foreground">
                                        {apiKey.description ?? "—"}
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
                                                        await handleArchive(apiKey.id)
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

                {apiKeys.length > 0 && (
                    <div className="flex items-center justify-between border-t px-4 py-3">
                        <p className="text-sm text-muted-foreground">
                            {apiKeys.length}{" "}
                            {apiKeys.length === 1 ? t("key", "key") : t("api_keys")}
                        </p>
                    </div>
                )}
            </div>

            {/* Create/Edit API Key Dialog */}
            <ApiKeyDialog
                editing={editing}
                onClose={() => setEditing(null)}
                onSave={async (data) => {
                    setIsSaving(true)
                    try {
                        const { id, name, description, role } = data
                        if (id) {
                            await api.apiKeys.update(project.id, id, {
                                name: name as string,
                                description,
                                role,
                            })
                        } else {
                            await api.apiKeys.create(project.id, {
                                name: name as string,
                                description,
                                role,
                            })
                        }
                        await reload()
                        setEditing(null)
                    } finally {
                        setIsSaving(false)
                    }
                }}
                isSaving={isSaving}
            />
        </div>
    )
}
