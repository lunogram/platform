import { useContext, useEffect, useState } from "react"
import { Link, useBlocker, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import {
    ListFilter,
    ChevronRight,
    MoreHorizontal,
    Send,
    Upload,
    RefreshCw,
    Archive,
    AlertCircle,
    Radio,
} from "lucide-react"
import { ListContext, ProjectContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { isEnterprise } from "@/config/enterprise"
import type { DynamicList, List, ListUpdateParams, WrapperRule } from "../../types"
import { formatDate, problemDetail, snakeToTitle } from "../../utils"
import { getRandomColor } from "@/lib/colors"
import { ListRuleSection } from "./ListRuleSection"
import { ListUploadDialog } from "./ListUploadDialog"
import { ListUsersTable } from "./ListUsersTable"
import { useArchiveList, useUpdateList } from "./listQueries"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { InlineEdit } from "@/components/ui/inline-edit"
import { CreateBroadcastDialog } from "@/views/broadcast/CreateBroadcastDialog"

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

export default function ListDetail() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [project] = useContext(ProjectContext)
    const [preferences] = useContext(PreferencesContext)
    const [list] = useContext(ListContext)
    const updateList = useUpdateList()
    const archiveList = useArchiveList()
    const [isUploadOpen, setIsUploadOpen] = useState(false)
    const [isBroadcastOpen, setIsBroadcastOpen] = useState(false)
    const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)
    const isSaving = updateList.isPending
    const [savingAction, setSavingAction] = useState<"rule" | "publish" | "other" | null>(null)
    const [error, setError] = useState<string | undefined>()

    const listColor = getRandomColor(list.name ?? list.id)

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

    /**
     * Saves the list and shows a failure in the page's error alert (as well as
     * the mutation cache's toast).
     * @returns the saved list, or null when the save failed.
     */
    const saveList = async (
        { name, rule, published, tags }: ListUpdateParams,
        action: "rule" | "publish" | "other" = "other",
    ): Promise<List | null> => {
        setSavingAction(action)
        try {
            const value = await updateList.mutateAsync({ name, rule, published, tags })
            setError(undefined)
            return value
        } catch (error: unknown) {
            setError(
                problemDetail(error) ??
                    (error instanceof Error ? error.message : "An unexpected error occurred"),
            )
            return null
        } finally {
            setSavingAction(null)
        }
    }

    const handleArchiveList = () => {
        archiveList.mutate(list.id, {
            onSuccess: () => navigate(`/projects/${project.id}/lists`),
        })
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
                            {isEnterprise && (
                                <Tooltip>
                                    <TooltipTrigger asChild>
                                        <div>
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={() => setIsBroadcastOpen(true)}
                                                disabled={list.state !== "ready"}
                                            >
                                                <Radio className="mr-2 h-3.5 w-3.5" />
                                                {t("send_broadcast", "Send Broadcast")}
                                            </Button>
                                        </div>
                                    </TooltipTrigger>
                                    {list.state !== "ready" && (
                                        <TooltipContent>
                                            {t(
                                                "broadcast_requires_published_list",
                                                "Publish the list before sending a broadcast",
                                            )}
                                        </TooltipContent>
                                    )}
                                </Tooltip>
                            )}
                            <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                    <Button variant="ghost" size="icon" className="h-8 w-8">
                                        <MoreHorizontal className="h-4 w-4" />
                                    </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end">
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
                    <ListRuleSection
                        list={list}
                        isSaving={savingAction === "rule"}
                        onRuleSave={async (rule) =>
                            (await saveList(
                                { name: list.name, rule: rule as WrapperRule },
                                "rule",
                            )) as DynamicList | null
                        }
                        onDirtyChange={setHasUnsavedChanges}
                    />
                )}

                {/* Users Table */}
                <ListUsersTable list={list} />
            </div>

            <ListUploadDialog open={isUploadOpen} onOpenChange={setIsUploadOpen} />

            <CreateBroadcastDialog
                open={isBroadcastOpen}
                onOpenChange={setIsBroadcastOpen}
                listId={list.id}
                list={list}
            />
        </div>
    )
}
