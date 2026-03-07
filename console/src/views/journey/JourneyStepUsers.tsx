import { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { formatDistanceToNow } from "date-fns"
import clsx from "clsx"
import { JourneyContext, ProjectContext } from "../../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { SearchTable, useSearchTableState } from "@/components/search-table"
import api from "../../api"
import { Badge } from "@/components/ui/badge"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip"
import { camelToTitle, formatDate } from "../../utils"
import { typeVariants } from "./EntranceDetails"
import { getStepType } from "./editor/JourneyEditor.utils"
import { stepCategoryColors } from "./hooks/JourneyEditor.constants"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import type { JourneyUserStep, User } from "../../types"
import type { DataTableCol } from "@/components/search-table"
import type { UUID } from "@/types/common"
import Menu, { MenuItem } from "@/components/menu"
import { FastForward, Trash2, Users } from "lucide-react"

interface StepUsersProps {
    open: boolean
    onClose: (open: boolean) => void
    stepId: UUID
    stepType: string
    stepName: string
}

function getUserDisplayName(user?: User) {
    if (!user) return "Unknown"
    return user.full_name || user.email || user.external_id || "Unknown"
}

function getUserInitials(user?: User) {
    const name = getUserDisplayName(user)
    const parts = name.trim().split(/\s+/)
    if (parts.length >= 2) {
        return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
    }
    return name.substring(0, 2).toUpperCase()
}

function getUserSubtext(user?: User) {
    if (!user) return null
    if (user.full_name && user.email) return user.email
    if (user.full_name && user.external_id) return user.external_id
    if (user.email && user.external_id) return user.external_id
    return null
}

function RelativeTime({ date }: { date: string }) {
    const [preferences] = useContext(PreferencesContext)
    const relative = formatDistanceToNow(new Date(date), { addSuffix: true })
    const absolute = formatDate(preferences, date)

    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <span className="cursor-default">{relative}</span>
            </TooltipTrigger>
            <TooltipContent>{absolute}</TooltipContent>
        </Tooltip>
    )
}

export function JourneyStepUsers({ open, onClose, stepType, stepId, stepName }: StepUsersProps) {
    const { t } = useTranslation()
    const [{ id: projectId }] = useContext(ProjectContext)
    const [{ id: journeyId }] = useContext(JourneyContext)
    const isEntrance = stepType === "entrance"

    const [confirmRemove, setConfirmRemove] = useState<{
        stepId: UUID
        user: User
    } | null>(null)

    const stepTypeInfo = getStepType(stepType)
    const category = stepTypeInfo?.category ?? "action"

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

    const options: Array<DataTableCol<JourneyUserStep>> =
        stepType === "delay"
            ? [
                  {
                      key: "options",
                      title: "",
                      cell: ({ item: { id, user, type } }) => {
                          if (user && type !== "completed") {
                              return (
                                  <Menu size="min">
                                      <MenuItem
                                          onClick={async () => await handleSkipDelay(id, user)}
                                      >
                                          <FastForward className="h-4 w-4" />
                                          {t("skip_delay")}
                                      </MenuItem>
                                      <MenuItem
                                          onClick={() => setConfirmRemove({ stepId: id, user })}
                                      >
                                          <Trash2 className="h-4 w-4 text-destructive" />
                                          <span className="text-destructive">
                                              {t("remove_from_journey")}
                                          </span>
                                      </MenuItem>
                                  </Menu>
                              )
                          }
                          return null
                      },
                  },
              ]
            : []

    return (
        <>
            <Dialog open={open} onOpenChange={onClose}>
                <DialogContent className="sm:max-w-3xl max-h-[85vh] min-h-[520px] flex flex-col gap-0 p-0">
                    <DialogHeader className="px-6 pt-6 pb-4">
                        <div className="flex items-center gap-2.5">
                            {stepTypeInfo?.icon && (
                                <span
                                    className={clsx(
                                        "flex h-8 w-8 shrink-0 items-center justify-center rounded-md [&_svg]:h-4 [&_svg]:w-4",
                                        stepCategoryColors[category],
                                    )}
                                >
                                    {stepTypeInfo.icon}
                                </span>
                            )}
                            <div className="min-w-0">
                                <DialogTitle className="truncate">
                                    {stepName || stepTypeInfo?.name || t("users")}
                                </DialogTitle>
                                <DialogDescription>
                                    {t("users_in_step", {
                                        defaultValue:
                                            "Users currently in or that have passed through this step.",
                                    })}
                                </DialogDescription>
                            </div>
                        </div>
                    </DialogHeader>

                    <div className="flex-1 overflow-auto px-6 pb-6 pt-1 -mt-1">
                        <SearchTable
                            {...state}
                            enableSearch
                            emptyMessage={
                                <div className="flex flex-col items-center gap-2 py-16 text-muted-foreground">
                                    <Users className="h-8 w-8" />
                                    <p>
                                        {state.params.search
                                            ? t("no_users_found", {
                                                  defaultValue: "No users found",
                                              })
                                            : t("no_users_in_step", {
                                                  defaultValue:
                                                      "No users have reached this step yet",
                                              })}
                                    </p>
                                </div>
                            }
                            columns={[
                                {
                                    key: "user",
                                    title: t("user"),
                                    cell: ({ item }) => {
                                        const subtext = getUserSubtext(item.user)
                                        return (
                                            <div className="flex items-center gap-3 py-0.5">
                                                <Avatar className="h-8 w-8">
                                                    <AvatarFallback className="bg-primary/10 text-primary text-xs font-medium">
                                                        {getUserInitials(item.user)}
                                                    </AvatarFallback>
                                                </Avatar>
                                                <div className="min-w-0">
                                                    <div className="font-medium text-sm truncate">
                                                        {getUserDisplayName(item.user)}
                                                    </div>
                                                    {subtext && (
                                                        <div className="text-xs text-muted-foreground truncate">
                                                            {subtext}
                                                        </div>
                                                    )}
                                                </div>
                                            </div>
                                        )
                                    },
                                    minWidth: 200,
                                },
                                {
                                    key: "type",
                                    title: t("status"),
                                    cell: ({ item }) => (
                                        <Badge variant={typeVariants[item.type]}>
                                            {camelToTitle(item.type)}
                                        </Badge>
                                    ),
                                },
                                {
                                    key: "created_at",
                                    title: t("step_date"),
                                    cell: ({ item }) =>
                                        item.created_at ? (
                                            <RelativeTime date={item.created_at} />
                                        ) : null,
                                },
                                ...(stepType === "delay"
                                    ? [
                                          {
                                              key: "delay_until",
                                              title: t("delay_until"),
                                              cell: ({ item }) =>
                                                  item.delay_until ? (
                                                      <RelativeTime date={item.delay_until} />
                                                  ) : null,
                                          } as DataTableCol<JourneyUserStep>,
                                      ]
                                    : []),
                                ...options,
                            ]}
                            onSelectRow={
                                isEntrance
                                    ? ({ id }) =>
                                          window.open(
                                              `/projects/${projectId}/entrances/${id}`,
                                              "_blank",
                                          )
                                    : undefined
                            }
                        />
                    </div>
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
