import { useCallback, useContext, useEffect, useState, useRef } from "react"
import { Link, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import {
    ListFilter,
    ChevronRight,
    ChevronLeft,
    MoreHorizontal,
    Send,
    Upload,
    RefreshCw,
    Archive,
    Search,
    AlertCircle,
    Users,
    Eye,
} from "lucide-react"
import { oapiClient } from "@/oapi/client"
import { ListContext, ProjectContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import type { DynamicList, ListUpdateParams, Rule, WrapperRule } from "../../types"
import { formatDate, snakeToTitle } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import RuleBuilder from "./rules/RuleBuilder"
import { useRoute } from "../router"
import { useBlocker } from "react-router"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Label } from "@/components/ui/label"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { InlineEdit } from "@/components/ui/inline-edit"

import type { ListState } from "../../types"

function getStateBadge(state: ListState, t: (key: string) => string) {
    const config: Record<ListState, { label: string; className: string }> = {
        draft: {
            label: t("draft"),
            className: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
        },
        loading: {
            label: t("loading"),
            className: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
        },
        ready: {
            label: t("ready"),
            className: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
        },
    }
    const { label, className } = config[state] ?? config.draft
    return (
        <Badge variant="outline" className={`border-0 ${className}`}>
            {label}
        </Badge>
    )
}

interface RuleSectionProps {
    list: DynamicList
    isSaving: boolean
    onRuleSave: (rule: Rule) => void
    onChange?: (rule: Rule) => void
}

function RuleSection({ list, isSaving, onRuleSave, onChange }: RuleSectionProps) {
    const { t } = useTranslation()
    // Display draft_rule if available, otherwise fall back to published rule
    const displayRule = list.draft_rule ?? list.rule
    const [rule, setRule] = useState<Rule | null>(displayRule)
    const [hasLocalEdits, setHasLocalEdits] = useState(false)
    const onSetRule = (rule: Rule) => {
        setRule(rule)
        setHasLocalEdits(true)
        onChange?.(rule)
    }
    return (
        <div className="space-y-4">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div>
                    <h3 className="text-base font-semibold">{t("rules")}</h3>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "rules_description",
                            "Define conditions to dynamically include users in this list.",
                        )}
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    {hasLocalEdits && (
                        <span className="text-xs text-amber-600 dark:text-amber-500">
                            {t("unsaved_changes", "Unsaved changes")}
                        </span>
                    )}
                    <Button
                        size="sm"
                        onClick={() => {
                            if (rule) {
                                setHasLocalEdits(false)
                                onRuleSave(rule)
                            }
                        }}
                        disabled={isSaving || !rule}
                    >
                        {isSaving ? <RefreshCw className="mr-2 h-3.5 w-3.5 animate-spin" /> : null}
                        {t("rules_save")}
                    </Button>
                </div>
            </div>
            <div className="rounded-lg border bg-card p-4">
                {rule && <RuleBuilder rule={rule} setRule={onSetRule} />}
            </div>
        </div>
    )
}

export default function ListDetail() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [project] = useContext(ProjectContext)
    const [preferences] = useContext(PreferencesContext)
    const [list, setList] = useContext(ListContext)
    const [isUploadOpen, setIsUploadOpen] = useState(false)
    const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)
    const [isSaving, setIsSaving] = useState(false)
    const [savingAction, setSavingAction] = useState<"rule" | "publish" | "other" | null>(null)
    const [error, setError] = useState<string | undefined>()

    // Users table state
    const [users, setUsers] = useState<any[] | null>(null)
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [cursor, setCursor] = useState<string | undefined>()
    const [pageDirection, setPageDirection] = useState<"next" | "prev" | undefined>()
    const [cursorHistory, setCursorHistory] = useState<string[]>([])
    const [nextCursor, setNextCursor] = useState<string | undefined>()
    const [previewTotal, setPreviewTotal] = useState<number | null>(null)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
    const route = useRoute()

    const isPreviewMode = list.type === "dynamic" && !!list.draft_rule

    const listColor = getRandomColor(list.name ?? list.id)

    const loadUsers = useCallback(async () => {
        try {
            if (isPreviewMode) {
                const { data } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/lists/{listID}/users/preview",
                    {
                        params: {
                            path: { projectID: project.id, listID: list.id },
                            query: { limit: 25 },
                        },
                    },
                )
                setUsers(data?.results ?? [])
                setPreviewTotal(data?.total ?? data?.results?.length ?? 0)
                setNextCursor(undefined)
            } else {
                const res = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/lists/{listID}/users",
                    {
                        params: {
                            path: {
                                projectID: project.id,
                                listID: list.id,
                            },
                            query: {
                                limit: 25,
                                cursor,
                                page: pageDirection,
                                search: debouncedQuery || undefined,
                            },
                        },
                    },
                )
                setUsers(res.data?.results ?? [])
                setPreviewTotal(null)
                setNextCursor(res.data?.nextCursor || undefined)
            }
        } catch {
            setUsers([])
            setPreviewTotal(null)
        }
    }, [project.id, list.id, cursor, pageDirection, debouncedQuery, isPreviewMode])

    useEffect(() => {
        loadUsers()
    }, [loadUsers])

    const refreshList = useCallback(() => {
        oapiClient.GET("/api/admin/projects/{projectID}/lists/{listID}", {
            params: {
                path: {
                    projectID: project.id,
                    listID: list.id,
                },
            },
        })
            .then(res => { if (res.data) setList(res.data) })
            .then(() => loadUsers())
            .catch(() => {})
    }, [project.id, list.id, setList, loadUsers])

    useEffect(() => {
        if (list.state !== "loading") return
        const complete = list.progress?.complete ?? 0
        const total = list.progress?.total ?? 0
        const percent = total > 0 ? (complete / total) * 100 : 0
        const refreshRate = percent < 5 ? 1000 : 5000
        const interval = setInterval(refreshList, refreshRate)
        refreshList()

        return () => clearInterval(interval)
    }, [list.state, list.progress?.complete, list.progress?.total, refreshList])

    const blocker = useBlocker(
        ({ currentLocation, nextLocation }) =>
            hasUnsavedChanges && currentLocation.pathname !== nextLocation.pathname,
    )

    useEffect(() => {
        if (blocker.state !== "blocked") return
        if (confirm(t("confirm_unsaved_changes"))) {
            blocker.proceed()
        } else {
            blocker.reset()
        }
    }, [blocker, t])

    const handleSearch = useCallback((value: string) => {
        setSearchQuery(value)
        setCursor(undefined)
        setPageDirection(undefined)
        setCursorHistory([])
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }, [])

    const hasPrevPage = cursorHistory.length > 0
    const hasNextPage = !!nextCursor

    const handleNextPage = () => {
        if (nextCursor) {
            setCursorHistory((prev) => [...prev, cursor ?? ""])
            setCursor(nextCursor)
            setPageDirection("next")
        }
    }

    const handlePrevPage = () => {
        if (cursorHistory.length > 0) {
            const prev = [...cursorHistory]
            const prevCursor = prev.pop()
            setCursorHistory(prev)
            setCursor(prevCursor || undefined)
            setPageDirection(prevCursor ? "next" : undefined)
        }
    }

    const saveList = async (
        { name, rule, published, tags }: ListUpdateParams,
        action: "rule" | "publish" | "other" = "other",
    ) => {
        setIsSaving(true)
        setSavingAction(action)
        try {
            const res = await oapiClient.PATCH(
                "/api/admin/projects/{projectID}/lists/{listID}",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            listID: list.id,
                        },
                    },
                    body: { name, rule, published, tags },
                },
            )
            if (res.error) {
                setError(res.error.detail || res.error.title || "Failed to save list")
            } else if (res.data) {
                setError(undefined)
                setList(res.data)
                setHasUnsavedChanges(false)
                loadUsers()
            }
        } catch (error: unknown) {
            const errorMessage =
                error instanceof Error ? error.message : "An unexpected error occurred"
            setError(errorMessage)
        } finally {
            setIsSaving(false)
            setSavingAction(null)
        }
    }

    const uploadUsers = async (file: File) => {
        const formData = new FormData()
        formData.append("file", file)
        await fetch(`/api/admin/projects/${project.id}/lists/${list.id}/users`, {
            method: "POST",
            body: formData,
        })
        refreshList()
        setIsUploadOpen(false)
    }

    const handleRecountList = async () => {
        await oapiClient.POST('/api/admin/projects/{projectID}/lists/{listID}/recount', {
            params: {
                path: {
                    projectID: project.id,
                    listID: list.id,
                },
            },
        })
        window.location.reload()
    }

    const handleArchiveList = async () => {
        await oapiClient.DELETE("/api/admin/projects/{projectID}/lists/{listID}", {
            params: {
                path: {
                    projectID: project.id,
                    listID: list.id,
                },
            },
        })
        await navigate(`/projects/${project.id}/lists`)
    }

    const progress =
        list.state === "loading" && list.progress
            ? Math.round((list.progress.complete / (list.progress.total || 1)) * 100)
            : null

    return (
        <div className="flex flex-col min-h-full">
            {/* Header Section */}
            <div className="border-b bg-card/50">
                <div className="p-4 sm:p-6">
                    {/* Breadcrumb */}
                    <nav className="flex items-center gap-1.5 text-sm text-muted-foreground mb-4">
                        <Link
                            to={`/projects/${project.id}/lists`}
                            className="hover:text-foreground transition-colors"
                        >
                            {t("lists")}
                        </Link>
                        <ChevronRight className="h-3.5 w-3.5" />
                        <span className="text-foreground font-medium">{list.name}</span>
                    </nav>

                    {/* List Identity */}
                    <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-4 sm:gap-6">
                        <div className="flex items-start gap-4 min-w-0">
                            <div
                                className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0"
                                style={{ backgroundColor: listColor }}
                            >
                                <ListFilter className="h-7 w-7 text-white" />
                            </div>
                            <div className="space-y-1">
                                <div className="flex items-center gap-3">
                                    <InlineEdit
                                        value={list.name}
                                        onSave={async (name) => {
                                            await saveList({ name })
                                        }}
                                        required
                                        triggerClassName="gap-1.5"
                                        pencilSize="h-3.5 w-3.5"
                                    >
                                        <h1 className="text-2xl font-semibold tracking-tight">
                                            {list.name}
                                        </h1>
                                    </InlineEdit>
                                    {getStateBadge(list.state, t)}
                                </div>
                                <p className="text-sm text-muted-foreground flex items-center flex-wrap gap-x-2 gap-y-1">
                                    <span>{snakeToTitle(list.type)}</span>
                                    {list.version_number != null && (
                                        <>
                                            <span>·</span>
                                            <span>v{list.version_number}</span>
                                        </>
                                    )}
                                    <span>·</span>
                                    <span>
                                        {list.state === "loading"
                                            ? t("counting", "Counting...")
                                            : `${list.users_count?.toLocaleString() ?? 0} ${t("users").toLowerCase()}`}
                                    </span>
                                    <span>·</span>
                                    <span>
                                        {t("created")}{" "}
                                        {formatDate(preferences, list.created_at, "PP")}
                                    </span>
                                </p>
                            </div>
                        </div>

                        <div className="flex items-center gap-2">
                            {(list.state === "draft" ||
                                (list.type === "dynamic" && list.draft_rule != null)) && (
                                <Button
                                    size="sm"
                                    onClick={async () =>
                                        await saveList(
                                            { name: list.name, published: true },
                                            "publish",
                                        )
                                    }
                                    disabled={list.state === "loading" || isSaving}
                                >
                                    {savingAction === "publish" ? (
                                        <RefreshCw className="mr-2 h-3.5 w-3.5 animate-spin" />
                                    ) : (
                                        <Send className="mr-2 h-3.5 w-3.5" />
                                    )}
                                    {t("publish")}
                                </Button>
                            )}
                            {list.type === "static" && (
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={() => setIsUploadOpen(true)}
                                >
                                    <Upload className="mr-2 h-3.5 w-3.5" />
                                    {t("upload_list")}
                                </Button>
                            )}
                            <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                    <Button variant="ghost" size="icon" className="h-8 w-8">
                                        <MoreHorizontal className="h-4 w-4" />
                                    </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end">
                                    <DropdownMenuItem onClick={handleRecountList}>
                                        <RefreshCw className="h-4 w-4 mr-2" />
                                        {t("recount")}
                                    </DropdownMenuItem>
                                    <DropdownMenuSeparator />
                                    <DropdownMenuItem
                                        className="text-destructive focus:text-destructive"
                                        onClick={handleArchiveList}
                                    >
                                        <Archive className="h-4 w-4 mr-2" />
                                        {t("archive")}
                                    </DropdownMenuItem>
                                </DropdownMenuContent>
                            </DropdownMenu>
                        </div>
                    </div>
                </div>
            </div>

            {/* Progress Bar for Loading State */}
            {list.state === "loading" && progress !== null && (
                <div className="border-b bg-blue-50/50 dark:bg-blue-950/20 px-4 sm:px-6 py-3">
                    <div className="flex items-center gap-3">
                        <RefreshCw className="h-4 w-4 animate-spin text-blue-600 dark:text-blue-400" />
                        <div className="flex-1">
                            <div className="flex items-center justify-between text-sm mb-1">
                                <span className="text-blue-700 dark:text-blue-300 font-medium">
                                    {t("processing", "Processing...")}
                                </span>
                                <span className="text-blue-600 dark:text-blue-400">
                                    {progress}%
                                </span>
                            </div>
                            <div className="h-1.5 rounded-full bg-blue-200/60 dark:bg-blue-800/40 overflow-hidden">
                                <div
                                    className="h-full rounded-full bg-blue-600 dark:bg-blue-400 transition-all duration-500"
                                    style={{ width: `${progress}%` }}
                                />
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Content Area */}
            <div className="flex-1 p-4 sm:p-6 space-y-6">
                {/* Error Alert */}
                {error && (
                    <Alert variant="destructive">
                        <AlertCircle className="h-4 w-4" />
                        <AlertTitle>{t("error")}</AlertTitle>
                        <AlertDescription>{error}</AlertDescription>
                    </Alert>
                )}

                {/* Rules Section (Dynamic Lists) */}
                {list.type === "dynamic" && (
                    <RuleSection
                        list={list}
                        isSaving={savingAction === "rule"}
                        onRuleSave={async (rule) =>
                            await saveList({ name: list.name, rule: rule as WrapperRule }, "rule")
                        }
                        onChange={() => setHasUnsavedChanges(true)}
                    />
                )}

                {/* Users Table */}
                <div className="space-y-4">
                    <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 sm:gap-4">
                        <h3 className="text-base font-semibold">{t("users")}</h3>
                        <div className="flex items-center gap-3 flex-wrap">
                            {isPreviewMode && (
                                <div className="flex items-center gap-1.5 rounded-md border border-amber-200 bg-amber-50/50 dark:border-amber-800 dark:bg-amber-950/20 px-2.5 py-1.5">
                                    <Eye className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400 shrink-0" />
                                    <span className="text-xs text-amber-700 dark:text-amber-300 whitespace-nowrap">
                                        {t("preview_mode", "Preview mode")}
                                        {previewTotal != null && (
                                            <>
                                                {" · "}
                                                <span className="font-medium">
                                                    {previewTotal.toLocaleString()}{" "}
                                                    {t("matching", "matching")}
                                                </span>
                                            </>
                                        )}
                                    </span>
                                </div>
                            )}
                            {isPreviewMode ? (
                                <Tooltip>
                                    <TooltipTrigger asChild>
                                        <div className="relative max-w-xs flex-1">
                                            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground/50" />
                                            <Input
                                                placeholder={t("search_users", "Search users...")}
                                                disabled
                                                className="pl-9 h-8"
                                            />
                                        </div>
                                    </TooltipTrigger>
                                    <TooltipContent>
                                        {t(
                                            "search_disabled_preview",
                                            "Search is not available in preview mode",
                                        )}
                                    </TooltipContent>
                                </Tooltip>
                            ) : (
                                <div className="relative max-w-xs flex-1">
                                    <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                                    <Input
                                        placeholder={t("search_users", "Search users...")}
                                        value={searchQuery}
                                        onChange={(e) => handleSearch(e.target.value)}
                                        className="pl-9 h-8"
                                    />
                                </div>
                            )}
                        </div>
                    </div>

                    <div className="rounded-lg border bg-card">
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>{t("name")}</TableHead>
                                    <TableHead className="hidden md:table-cell">
                                        {t("external_id")}
                                    </TableHead>
                                    <TableHead>{t("email")}</TableHead>
                                    <TableHead className="hidden sm:table-cell">
                                        {t("phone")}
                                    </TableHead>
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {!users ? (
                                    Array.from({ length: 5 }).map((_, i) => (
                                        <TableRow key={i}>
                                            <TableCell>
                                                <Skeleton className="h-4 w-32" />
                                            </TableCell>
                                            <TableCell className="hidden md:table-cell">
                                                <Skeleton className="h-4 w-24" />
                                            </TableCell>
                                            <TableCell>
                                                <Skeleton className="h-4 w-36" />
                                            </TableCell>
                                            <TableCell className="hidden sm:table-cell">
                                                <Skeleton className="h-4 w-24" />
                                            </TableCell>
                                        </TableRow>
                                    ))
                                ) : users.length === 0 ? (
                                    <TableRow>
                                        <TableCell colSpan={4} className="h-32 text-center">
                                            <div className="flex flex-col items-center gap-2 text-muted-foreground">
                                                <Users className="h-8 w-8" />
                                                <p>
                                                    {debouncedQuery
                                                        ? t("no_users_found", "No users found")
                                                        : t(
                                                              "no_users_yet",
                                                              "No users in this list yet",
                                                          )}
                                                </p>
                                            </div>
                                        </TableCell>
                                    </TableRow>
                                ) : (
                                    users.map((user: any) => (
                                        <TableRow
                                            key={user.id}
                                            className="cursor-pointer"
                                            onClick={() => route(`users/${user.id}`)}
                                        >
                                            <TableCell className="font-medium">
                                                {user.full_name || "—"}
                                            </TableCell>
                                            <TableCell className="text-muted-foreground hidden md:table-cell">
                                                <code className="text-xs bg-muted px-1.5 py-0.5 rounded">
                                                    {user.external_id}
                                                </code>
                                            </TableCell>
                                            <TableCell className="text-muted-foreground">
                                                {user.email || "—"}
                                            </TableCell>
                                            <TableCell className="text-muted-foreground hidden sm:table-cell">
                                                {user.phone || "—"}
                                            </TableCell>
                                        </TableRow>
                                    ))
                                )}
                            </TableBody>
                        </Table>

                        {/* Pagination */}
                        {users && users.length > 0 && !isPreviewMode && (
                            <div className="flex items-center justify-between border-t px-4 py-3">
                                <p className="text-sm text-muted-foreground">
                                    {users.length} {t("users").toLowerCase()}
                                </p>
                                {(hasPrevPage || hasNextPage) && (
                                    <div className="flex items-center gap-2">
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={handlePrevPage}
                                            disabled={!hasPrevPage}
                                            aria-label={t("previous")}
                                        >
                                            <ChevronLeft className="h-4 w-4 sm:mr-1" />
                                            <span className="hidden sm:inline">
                                                {t("previous")}
                                            </span>
                                        </Button>
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={handleNextPage}
                                            disabled={!hasNextPage}
                                            aria-label={t("next")}
                                        >
                                            <span className="hidden sm:inline">{t("next")}</span>
                                            <ChevronRight className="h-4 w-4 sm:ml-1" />
                                        </Button>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </div>
            </div>

            {/* Upload Users Dialog */}
            <Dialog open={isUploadOpen} onOpenChange={setIsUploadOpen}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>{t("import_users")}</DialogTitle>
                        <DialogDescription>{t("upload_instructions")}</DialogDescription>
                    </DialogHeader>
                    <form
                        onSubmit={async (e) => {
                            e.preventDefault()
                            const formData = new FormData(e.currentTarget)
                            const file = formData.get("file") as File
                            if (file) await uploadUsers(file)
                        }}
                    >
                        <div className="grid gap-4 py-4">
                            <div className="grid gap-2">
                                <Label htmlFor="upload-file">{t("file")}</Label>
                                <Input
                                    id="upload-file"
                                    name="file"
                                    type="file"
                                    accept=".csv,.txt"
                                    required
                                    className="cursor-pointer"
                                />
                            </div>
                        </div>
                        <DialogFooter>
                            <Button
                                type="button"
                                variant="outline"
                                onClick={() => setIsUploadOpen(false)}
                            >
                                {t("cancel")}
                            </Button>
                            <Button type="submit">
                                <Upload className="mr-2 h-3.5 w-3.5" />
                                {t("upload")}
                            </Button>
                        </DialogFooter>
                    </form>
                </DialogContent>
            </Dialog>
        </div>
    )
}
