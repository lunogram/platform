import { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { formatDistanceToNow } from "date-fns"
import { JourneyContext, ProjectContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { useSearchTableState } from "@/components/search-table"
import api from "../../api"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip"
import { camelToTitle, formatDate } from "../../utils"
import { typeVariants } from "./EntranceDetails"
import { UserCell } from "./components/UserCell"
import { getUserDisplayName } from "./components/userUtils"
import { useDebounceControl } from "@/hooks"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import type { User } from "../../types"
import type { UUID } from "@/types/common"
import Menu, { MenuItem } from "@/components/menu"
import {
    ChevronLeft,
    ChevronRight,
    FastForward,
    Loader2,
    Search,
    Trash2,
    Users,
} from "lucide-react"

interface StepUsersProps {
    open: boolean
    onClose: (open: boolean) => void
    stepId: UUID
    stepType: string
    stepName: string
}

function RelativeTime({ date }: { date: string }) {
    const [preferences] = useContext(PreferencesContext)
    const relative = formatDistanceToNow(new Date(date), { addSuffix: true })
    const absolute = formatDate(preferences, date)

    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <span className="cursor-default text-sm text-muted-foreground">{relative}</span>
            </TooltipTrigger>
            <TooltipContent>{absolute}</TooltipContent>
        </Tooltip>
    )
}

export function JourneyStepUsers({ open, onClose, stepType, stepId, stepName }: StepUsersProps) {
    const { t } = useTranslation()
    const [{ id: projectId }] = useContext(ProjectContext)
    const [{ id: journeyId }] = useContext(JourneyContext)

    const [confirmRemove, setConfirmRemove] = useState<{
        stepId: UUID
        user: User
    } | null>(null)

    const state = useSearchTableState(
        useCallback(
            async (params) =>
                await api.journeys.steps.searchUsers(projectId, journeyId, stepId, params),
            [projectId, journeyId, stepId],
        ),
        {
            limit: 25,
            sort: "created_at",
            direction: "desc",
        },
    )

    const [searchInput, setSearchInput] = useDebounceControl(state.params.search ?? "", (search) =>
        state.setParams({ ...state.params, search, cursor: undefined }),
    )

    const handleSkipDelay = async (stepId: UUID, user: User) => {
        await api.journeys.users.skipDelay(projectId, journeyId, user.id, stepId)
        await state.reload()
    }

    const handleRemoveFromJourney = async (stepId: UUID, user: User) => {
        await api.journeys.users.removeFromJourney(projectId, journeyId, user.id, stepId)
        await state.reload()
    }

    const onConfirmRemove = async () => {
        if (!confirmRemove) return
        await handleRemoveFromJourney(confirmRemove.stepId, confirmRemove.user)
        setConfirmRemove(null)
    }

    const items = state.results?.results
    const isLoading = !state.results
    const hasPrev = !!state.results?.prevCursor
    const hasNext = !!state.results?.nextCursor

    return (
        <>
            <Dialog open={open} onOpenChange={onClose}>
                <DialogContent className="w-3/4 max-w-3xl max-h-[85vh] flex flex-col gap-0 p-0">
                    <DialogHeader className="px-4 pt-4 pb-3 sm:px-6 sm:pt-6 sm:pb-4">
                        <DialogTitle>{stepName || t("users")}</DialogTitle>
                        <DialogDescription>
                            {t("users_in_step", {
                                defaultValue:
                                    "Users currently in or that have passed through this step.",
                            })}
                        </DialogDescription>
                        <div className="relative mt-3">
                            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                            <Input
                                placeholder={t("search_users", "Search users...")}
                                value={searchInput}
                                onChange={(e) => setSearchInput(e.target.value)}
                                className="pl-9"
                            />
                        </div>
                    </DialogHeader>

                    <div className="flex-1 min-h-0 overflow-auto border-t">
                        {items && items.length > 0 ? (
                            <div className="divide-y">
                                {items.map((item) => (
                                    <div
                                        key={item.id}
                                        className="flex items-center gap-3 px-4 sm:px-6 py-2 hover:bg-muted/50 transition-colors"
                                    >
                                        <div className="flex-1 min-w-0">
                                            <UserCell user={item.user} />
                                        </div>

                                        <div className="flex items-center gap-3 shrink-0">
                                            {item.created_at && (
                                                <RelativeTime date={item.created_at} />
                                            )}

                                            {stepType === "delay" && item.delay_until && (
                                                <Tooltip>
                                                    <TooltipTrigger asChild>
                                                        <span className="text-xs text-muted-foreground/70 cursor-default">
                                                            {t("delay_until", "until")}{" "}
                                                            {formatDistanceToNow(
                                                                new Date(item.delay_until),
                                                                { addSuffix: true },
                                                            )}
                                                        </span>
                                                    </TooltipTrigger>
                                                    <TooltipContent>
                                                        {new Date(
                                                            item.delay_until,
                                                        ).toLocaleString()}
                                                    </TooltipContent>
                                                </Tooltip>
                                            )}

                                            <Badge variant={typeVariants[item.type]}>
                                                {camelToTitle(item.type)}
                                            </Badge>

                                            {stepType === "delay" &&
                                                item.user &&
                                                item.type !== "completed" && (
                                                    <Menu size="min">
                                                        <MenuItem
                                                            onClick={async () =>
                                                                await handleSkipDelay(
                                                                    item.id,
                                                                    item.user!,
                                                                )
                                                            }
                                                        >
                                                            <FastForward className="h-4 w-4" />
                                                            {t("skip_delay")}
                                                        </MenuItem>
                                                        <MenuItem
                                                            onClick={() =>
                                                                setConfirmRemove({
                                                                    stepId: item.id,
                                                                    user: item.user!,
                                                                })
                                                            }
                                                        >
                                                            <Trash2 className="h-4 w-4 text-destructive" />
                                                            <span className="text-destructive">
                                                                {t("remove_from_journey")}
                                                            </span>
                                                        </MenuItem>
                                                    </Menu>
                                                )}
                                        </div>
                                    </div>
                                ))}
                            </div>
                        ) : isLoading ? (
                            <div className="flex justify-center py-16">
                                <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                            </div>
                        ) : (
                            <div className="flex flex-col items-center gap-2 py-16 text-muted-foreground">
                                <Users className="h-8 w-8" />
                                <p>
                                    {searchInput
                                        ? t("no_users_found", "No users found")
                                        : t("no_users_in_step", {
                                              defaultValue: "No users have reached this step yet",
                                          })}
                                </p>
                            </div>
                        )}
                    </div>

                    {(hasPrev || hasNext) && (
                        <div className="border-t px-4 py-3 sm:px-6 flex items-center justify-between">
                            <p className="text-sm text-muted-foreground">
                                {state.results?.total != null &&
                                    `${state.results.total} ${state.results.total === 1 ? t("user", "user") : t("users", "users")}`}
                            </p>
                            <div className="flex items-center gap-1">
                                <Button
                                    variant="outline"
                                    size="sm"
                                    disabled={!hasPrev}
                                    onClick={() =>
                                        state.setParams({
                                            ...state.params,
                                            cursor: state.results?.prevCursor,
                                            page: "prev",
                                        })
                                    }
                                >
                                    <ChevronLeft className="h-4 w-4" />
                                    {t("previous", "Previous")}
                                </Button>
                                <Button
                                    variant="outline"
                                    size="sm"
                                    disabled={!hasNext}
                                    onClick={() =>
                                        state.setParams({
                                            ...state.params,
                                            cursor: state.results?.nextCursor,
                                            page: "next",
                                        })
                                    }
                                >
                                    {t("next", "Next")}
                                    <ChevronRight className="h-4 w-4" />
                                </Button>
                            </div>
                        </div>
                    )}
                </DialogContent>
            </Dialog>

            <Dialog
                open={!!confirmRemove}
                onOpenChange={(open) => {
                    if (!open) setConfirmRemove(null)
                }}
            >
                <DialogContent className="sm:max-w-md">
                    <DialogHeader>
                        <DialogTitle>{t("remove_from_journey")}</DialogTitle>
                        <DialogDescription>
                            {t("confirm_remove_user", {
                                defaultValue:
                                    "Are you sure you want to remove this user from the journey? This action cannot be undone.",
                                user: confirmRemove ? getUserDisplayName(confirmRemove.user) : "",
                            })}
                        </DialogDescription>
                    </DialogHeader>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setConfirmRemove(null)}>
                            {t("cancel")}
                        </Button>
                        <Button variant="destructive" onClick={onConfirmRemove}>
                            {t("remove")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </>
    )
}
