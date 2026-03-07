import { useCallback } from "react"
import { useTranslation } from "react-i18next"
import { Zap, Webhook } from "lucide-react"

import type { JourneyStepType } from "../../../types"
import type { UUID } from "@/types/common"
import { useResolver } from "../../../hooks"
import { Combobox } from "@/components/ui/combobox"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { snakeToTitle } from "@/utils"
import oapiClient, { type Action } from "@/oapi/client"
import { NIL } from "uuid"

interface ActionConfig {
    action_id: UUID
}

const actionTypeIcons: Record<string, JSX.Element> = {
    webhook: <Webhook className="h-4 w-4" />,
}

export const actionStep: JourneyStepType<ActionConfig> = {
    name: "action",
    icon: <Zap className="h-4 w-4" />,
    category: "action",
    description: "action_step_desc",

    Describe({ project: { id: projectId }, value: { action_id } }) {
        const { t } = useTranslation()
        const [action] = useResolver(
            useCallback(async () => {
                if (action_id && action_id !== NIL) {
                    const { data } = await oapiClient.GET(
                        "/api/admin/projects/{projectID}/actions/{actionID}",
                        {
                            params: {
                                path: { projectID: projectId, actionID: action_id },
                            },
                        },
                    )
                    return data ?? null
                }
                return null
            }, [projectId, action_id]),
        )

        if (!action) {
            return (
                <span className="text-sm text-muted-foreground">
                    {t("action_step_empty", "No action selected")}
                </span>
            )
        }

        return (
            <div className="flex items-center gap-2.5">
                <div className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-md bg-muted">
                    {actionTypeIcons[action.type] ?? <Zap className="h-4 w-4" />}
                </div>
                <div className="min-w-0 flex-1">
                    <span className="truncate text-sm font-medium">{action.name}</span>
                    <Badge variant="secondary" className="ml-2 text-[10px]">
                        {snakeToTitle(action.type)}
                    </Badge>
                </div>
            </div>
        )
    },

    newData: async () => ({
        action_id: NIL as UUID,
    }),

    Edit({ project, onChange, value }) {
        const { t } = useTranslation()
        const projectId = project.id

        const [currentAction] = useResolver(
            useCallback(async () => {
                if (value.action_id && value.action_id !== NIL) {
                    const { data } = await oapiClient.GET(
                        "/api/admin/projects/{projectID}/actions/{actionID}",
                        {
                            params: {
                                path: { projectID: projectId, actionID: value.action_id },
                            },
                        },
                    )
                    return data ?? null
                }
                return null
            }, [projectId, value.action_id]),
        )

        const handleSearch = useCallback(
            async (query: string): Promise<(Action & { path: string })[]> => {
                const { data } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/actions",
                    {
                        params: {
                            path: { projectID: projectId },
                            query: {
                                limit: 50,
                                search: query || undefined,
                            },
                        },
                    },
                )
                return (data?.results ?? []).map((a) => ({ ...a, path: a.id }))
            },
            [projectId],
        )

        return (
            <div className="space-y-3">
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">
                        {t("action.singular", "Action")}
                        <span className="text-destructive"> *</span>
                    </Label>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "action_step_select_desc",
                            "Select the action to execute at this step.",
                        )}
                    </p>
                    <Combobox<Action & { path: string }>
                        onSearch={handleSearch}
                        value={value.action_id === NIL ? "" : value.action_id}
                        displayValue={currentAction?.name}
                        onValueChange={(id) =>
                            onChange({ ...value, action_id: (id || NIL) as UUID })
                        }
                        placeholder={t("action.singular", "Action")}
                        renderOption={(option) => (
                            <div className="flex items-center gap-2">
                                <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded bg-muted [&>svg]:h-3 [&>svg]:w-3">
                                    {actionTypeIcons[option.type] ?? (
                                        <Zap className="h-3 w-3" />
                                    )}
                                </span>
                                <span>{option.name}</span>
                                <Badge variant="secondary" className="ml-auto text-[10px]">
                                    {snakeToTitle(option.type)}
                                </Badge>
                            </div>
                        )}
                    />
                </div>
            </div>
        )
    },

    validate: ({ action_id }) => {
        return !!action_id && action_id !== NIL
    },

    hasDataKey: true,
}
