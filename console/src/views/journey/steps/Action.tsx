/* eslint-disable react-refresh/only-export-components */
import React, { useCallback, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Zap, Webhook, Play, Loader2, CheckCircle2, XCircle } from "lucide-react"

import type { JourneyStepType } from "../../../types"
import type { UUID } from "@/types/common"
import type { TestActionFunctionResult } from "@/oapi/client"
import { useResolver } from "../../../hooks"
import { useJourneyVariableContext } from "../JourneyVariableContext"
import { Combobox } from "@/components/ui/combobox"
import { JsonView } from "@/components/ui/json-view"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { SchemaFields, type Schema } from "@/components/schema-fields"
import { cn, snakeToTitle } from "@/utils"
import oapiClient from "@/oapi/client"
import { NIL } from "uuid"
import { ActionPreviewIframe } from "@/components/action-preview-iframe"

interface ActionConfig {
    action_id: UUID
    function_id: string
    input?: Record<string, unknown>
}

const actionTypeIcons: Record<string, React.ReactNode> = {
    webhook: <Webhook className="h-4 w-4" />,
}

function TestResultView({ result }: { result: TestActionFunctionResult }) {
    // status_code === 0 is a client-side sentinel indicating the request
    // failed before reaching the action (e.g. network error).
    const isError = result.status_code >= 400 || result.status_code === 0

    return (
        <div className="space-y-2 pt-1.5">
            <div
                className={cn(
                    "flex items-center gap-2 rounded-md border px-3 py-2 text-sm",
                    isError
                        ? "border-destructive/30 bg-destructive/5 text-destructive"
                        : "border-emerald-500/30 bg-emerald-500/5 text-emerald-600 dark:text-emerald-400",
                )}
            >
                {isError ? (
                    <XCircle className="h-4 w-4 shrink-0" />
                ) : (
                    <CheckCircle2 className="h-4 w-4 shrink-0" />
                )}
                <span className="font-medium">{result.status_code}</span>
            </div>

            {result.metadata && Object.keys(result.metadata).length > 0 && (
                <JsonView
                    data={result.metadata as Record<string, unknown>}
                    defaultExpanded={false}
                />
            )}
        </div>
    )
}

export const actionStep: JourneyStepType<ActionConfig> = {
    name: "action",
    icon: <Zap className="h-4 w-4" />,
    category: "action",
    description: "action_step_desc",

    Describe({ project: { id: projectId }, value: { action_id, function_id, input } }) {
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

        const [actionMeta] = useResolver(
            useCallback(async () => {
                if (action) {
                    const { data } = await oapiClient.GET(
                        "/api/admin/projects/{projectID}/actions/meta",
                        {
                            params: {
                                path: { projectID: projectId },
                            },
                        },
                    )
                    return data ?? null
                }
                return null
            }, [projectId, action]),
        )

        const functionTitle = useMemo(() => {
            if (!action || !actionMeta || !function_id) return null
            const meta = actionMeta.find((m) => m.type === action.type)
            const fn = meta?.functions?.find((f) => f.id === function_id)
            return fn?.title ?? null
        }, [action, actionMeta, function_id])

        if (!action) {
            return (
                <span className="text-sm text-muted-foreground">
                    {t("action_step_empty", "No action selected")}
                </span>
            )
        }

        return (
            <div className="space-y-2.5 max-w-[300px]">
                <div className="flex items-center gap-2.5">
                    <div className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-md bg-muted">
                        {actionTypeIcons[action.type] ?? <Zap className="h-4 w-4" />}
                    </div>
                    <div className="min-w-0 flex-1">
                        <span className="truncate text-sm font-medium">
                            {action.name}
                            {functionTitle && <> &rsaquo; {functionTitle}</>}
                        </span>
                        <Badge variant="secondary" className="ml-2 text-[10px]">
                            {snakeToTitle(action.type)}
                        </Badge>
                    </div>
                </div>
                {function_id && (
                    <div className="w-[280px] overflow-hidden rounded-md">
                        <ActionPreviewIframe
                            actionType={action.type}
                            projectId={projectId}
                            mode="function-call"
                            data={{
                                functionId: function_id,
                                input: input ?? {},
                            }}
                        />
                    </div>
                )}
            </div>
        )
    },

    newData: async () => ({
        action_id: NIL as UUID,
        function_id: "",
    }),

    Edit({ project, onChange, value, nodeId }) {
        const { t } = useTranslation()
        const projectId = project.id
        const { getVariablesForNode } = useJourneyVariableContext()
        const variables = nodeId ? getVariablesForNode(nodeId) : []

        const [isTesting, setIsTesting] = useState(false)
        const [testResult, setTestResult] = useState<TestActionFunctionResult | null>(null)

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

        const [actionMetaList] = useResolver(
            useCallback(async () => {
                const { data } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/actions/meta",
                    {
                        params: {
                            path: { projectID: projectId },
                        },
                    },
                )
                return data ?? null
            }, [projectId]),
        )

        const availableFunctions = useMemo(() => {
            if (!currentAction || !actionMetaList) return []
            const meta = actionMetaList.find((m) => m.type === currentAction.type)
            return (meta?.functions ?? []).map((fn) => ({
                ...fn,
                path: fn.id,
            }))
        }, [currentAction, actionMetaList])

        const currentFunction = useMemo(() => {
            if (!value.function_id) return null
            return availableFunctions.find((fn) => fn.id === value.function_id) ?? null
        }, [availableFunctions, value.function_id])

        useEffect(() => {
            if (!value.function_id && availableFunctions.length > 0) {
                onChange({
                    ...value,
                    function_id: availableFunctions[0].id,
                })
            }
            // eslint-disable-next-line react-hooks/exhaustive-deps
        }, [availableFunctions])

        const handleFunctionSearch = useCallback(
            async (query: string) => {
                if (!query) return availableFunctions
                const lower = query.toLowerCase()
                return availableFunctions.filter((fn) => fn.title.toLowerCase().includes(lower))
            },
            [availableFunctions],
        )

        const handleTest = useCallback(async () => {
            setIsTesting(true)
            setTestResult(null)
            try {
                const { data, error } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/actions/{actionID}/functions/{functionID}/test",
                    {
                        params: {
                            path: {
                                projectID: projectId,
                                actionID: value.action_id,
                                functionID: value.function_id,
                            },
                        },
                        body: {
                            input: value.input as Record<string, unknown>,
                        },
                    },
                )
                if (data) {
                    setTestResult(data)
                } else {
                    // Use status_code 0 as sentinel for client-side failures
                    setTestResult({
                        status_code: 0,
                        metadata: {
                            error: (error as Record<string, unknown>)?.detail ?? "Request failed",
                        },
                    })
                }
            } catch {
                // Use status_code 0 as sentinel for client-side failures
                setTestResult({
                    status_code: 0,
                    metadata: { error: "Request failed" },
                })
            } finally {
                setIsTesting(false)
            }
        }, [projectId, value.action_id, value.function_id, value.input])

        return (
            <div className="min-w-0 space-y-3">
                {currentAction && (
                    <div className="space-y-1.5">
                        <Label className="text-sm font-medium">{t("action")}</Label>
                        <div className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
                            <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded bg-muted [&>svg]:h-3 [&>svg]:w-3">
                                {actionTypeIcons[currentAction.type] ?? <Zap className="h-3 w-3" />}
                            </span>
                            <span>{currentAction.name}</span>
                            <Badge variant="secondary" className="ml-auto text-[10px]">
                                {snakeToTitle(currentAction.type)}
                            </Badge>
                        </div>
                    </div>
                )}

                {currentAction && availableFunctions.length > 0 && (
                    <div className="space-y-1.5">
                        <Label className="text-sm font-medium">
                            {t("action_function", "Function")}
                            <span className="text-destructive"> *</span>
                        </Label>
                        <p className="text-sm text-muted-foreground">
                            {t("action_step_function_desc", "Select the function to execute.")}
                        </p>
                        <Combobox<(typeof availableFunctions)[number]>
                            onSearch={handleFunctionSearch}
                            value={value.function_id}
                            displayValue={currentFunction?.title}
                            onValueChange={(id) =>
                                onChange({
                                    ...value,
                                    function_id: id ?? "",
                                    input: undefined,
                                })
                            }
                            placeholder={t("action_function", "Function")}
                            renderOption={(option) => (
                                <div className="flex flex-col gap-0.5">
                                    <span className="text-sm">{option.title}</span>
                                    {option.description && (
                                        <span className="text-xs text-muted-foreground">
                                            {option.description}
                                        </span>
                                    )}
                                </div>
                            )}
                        />
                    </div>
                )}

                {currentFunction?.input_schema && (
                    <div className="space-y-1.5 border-t pt-3">
                        <div className="flex items-center justify-between gap-2">
                            <div className="space-y-1">
                                <Label className="text-sm font-medium">
                                    {t("action_function_input", "Input")}
                                </Label>
                                <p className="text-sm text-muted-foreground">
                                    {t(
                                        "action_step_input_desc",
                                        "Configure the input parameters for this function.",
                                    )}
                                </p>
                            </div>
                            {currentAction && value.function_id && (
                                <Button
                                    variant="outline"
                                    size="sm"
                                    className="shrink-0"
                                    disabled={isTesting}
                                    onClick={handleTest}
                                >
                                    {isTesting ? (
                                        <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                                    ) : (
                                        <Play className="mr-1.5 h-3.5 w-3.5" />
                                    )}
                                    {t("test", "Test")}
                                </Button>
                            )}
                        </div>
                        <SchemaFields
                            schema={currentFunction.input_schema as unknown as Schema}
                            value={(value.input as Record<string, unknown>) ?? {}}
                            onChange={(input) => onChange({ ...value, input })}
                            variables={variables}
                        />
                        {testResult && <TestResultView result={testResult} />}
                    </div>
                )}
            </div>
        )
    },

    validate: ({ action_id, function_id }) => {
        return !!action_id && action_id !== NIL && !!function_id
    },

    hasDataKey: true,
}
