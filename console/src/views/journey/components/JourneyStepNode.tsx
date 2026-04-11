import { memo, useCallback, useContext, Fragment, createElement, useEffect } from "react"
import type { Connection, NodeProps } from "reactflow"
import { Handle, Position, useReactFlow, getConnectedEdges } from "reactflow"
import { useTranslation } from "react-i18next"
import { FastForward, Play, User } from "lucide-react"
import { ProjectContext, JourneyContext } from "@/contexts"
import { cn } from "@/utils"
import { KeyIcon } from "@/components/icons"
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip"
import { getStepType } from "../editor/JourneyEditor.utils"
import { stepCategoryColors, stepCategoryBorderColors } from "../hooks/JourneyEditor.constants"

import "reactflow/dist/style.css"

export const JourneyStepNode = memo(
    ({
        id,
        data: {
            stepId,
            type: typeName,
            name,
            data,
            data_key,
            editing,
            skipDelay,
            openUserModal,
            hasUnsavedChanges = false,
            visited = false,
            active = false,
        } = {},
        selected,
    }: NodeProps & { active?: boolean }) => {
        const { t } = useTranslation()
        const [project] = useContext(ProjectContext)
        const [journey] = useContext(JourneyContext)

        const { getNode, getEdges, setNodes } = useReactFlow()

        const type = getStepType(typeName)
        const isExit = typeName === "exit" || name?.toLowerCase() === "exit"
        const isActiveVisual = active && !isExit
        const isExitCompletedVisual = isExit && active
        const isVisitedVisual = visited && !isExit

        useEffect(() => {
            if (active && isExit) {
                const timer = setTimeout(() => {
                    setNodes((nds) =>
                        nds.map((node) => {
                            return {
                                ...node,
                                data: { ...node.data, active: false, visited: false },
                            }
                        }),
                    )
                }, 2000)
                return () => clearTimeout(timer)
            }
        }, [active, isExit, setNodes])

        const validateConnection = useCallback(
            (conn: Connection) => {
                if (!type) return false
                if (type.multiChildSources) return true
                const sourceNode = conn.source && getNode(conn.source)
                if (!sourceNode) return true
                const existing = getConnectedEdges([sourceNode], getEdges())
                return existing.filter((e) => e.sourceHandle === conn.sourceHandle).length < 1
            },
            [type, getNode, getEdges],
        )

        if (!type)
            return (
                <div className="rounded-lg border border-red-300 bg-red-50 dark:bg-red-950/30 dark:border-red-800 px-3 py-2 text-sm text-red-600 dark:text-red-400">
                    Invalid Step Type
                </div>
            )

        const category = type.category as string
        const categoryColorClass = stepCategoryColors[category] ?? ""
        const categoryBorderClass = stepCategoryBorderColors[category] ?? ""
        const isValid = isExit ? true : type.validate ? type.validate(data) : true
        const isInfoStep = category === "info"

        return (
            <>
                {isActiveVisual && (
                    <div
                        className={cn(
                            "absolute -top-4 -right-4 z-50 flex h-8 w-8 items-center justify-center rounded-full text-white shadow-xl animate-in zoom-in-75 fade-in duration-200 border-2 border-white",
                            categoryColorClass,
                        )}
                        style={{ pointerEvents: "none" }}
                    >
                        <User size={16} fill="currentColor" />
                    </div>
                )}

                {!type.hideTopHandle && (
                    <Handle type="target" position={Position.Top} id={"t-" + id} />
                )}

                <div
                    data-step-type={typeName}
                    className={cn(
                        "rounded-lg bg-background shadow-sm transition-all duration-300 min-w-[200px]",
                        // Info steps get a distinct look
                        isInfoStep ? "bg-purple-50 dark:bg-purple-950/30 max-w-[275px]" : "",
                        // Border states
                        !isValid
                            ? "border-2 border-red-500 ring-2 ring-red-200 dark:ring-red-900"
                            : isActiveVisual
                              ? "border-2 border-orange-500 shadow-lg scale-105 journey-active-pulse"
                              : isExitCompletedVisual
                                ? "border-2 border-green-500 shadow-lg"
                                : isVisitedVisual
                                  ? "border-2 border-green-500"
                                  : selected
                                    ? cn("border-2", categoryBorderClass)
                                    : "border border-border",
                        // Editing state
                        editing && "journey-editing-active",
                    )}
                >
                    {/* Header */}
                    <div
                        className={cn(
                            "flex items-center gap-2.5 px-3 py-2.5",
                            !isInfoStep && "border-b border-border",
                        )}
                    >
                        <span
                            className={cn(
                                "flex h-8 w-8 shrink-0 items-center justify-center rounded-md [&_svg]:h-4 [&_svg]:w-4",
                                categoryColorClass,
                            )}
                        >
                            {type.icon}
                        </span>
                        <span className="flex-1 text-sm font-medium truncate">
                            {name || t(type.name)}
                        </span>
                        {typeName === "delay" && active && (
                            <button
                                onClick={(e) => {
                                    e.stopPropagation()
                                    const targetStepId = stepId ?? id
                                    if (targetStepId) skipDelay?.(targetStepId)
                                }}
                                className={cn(
                                    "inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-semibold uppercase tracking-wide transition-colors",
                                    "cursor-pointer bg-amber-100 text-amber-700 hover:bg-amber-200 dark:bg-amber-900/40 dark:text-amber-400 dark:hover:bg-amber-900/60",
                                )}
                            >
                                <FastForward size={12} className="fill-current" />
                                {t("Skip")}
                            </button>
                        )}
                        {typeName === "entrance" && (
                            <Tooltip>
                                <TooltipTrigger asChild>
                                    <span className="inline-flex pointer-events-auto">
                                        <button
                                            onClick={(e) => {
                                                e.stopPropagation()
                                                if (!hasUnsavedChanges) openUserModal?.(id)
                                            }}
                                            disabled={hasUnsavedChanges}
                                            className={cn(
                                                "inline-flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-semibold uppercase tracking-wide transition-colors",
                                                hasUnsavedChanges
                                                    ? "bg-muted text-muted-foreground cursor-not-allowed opacity-50"
                                                    : "cursor-pointer bg-emerald-100 text-emerald-700 hover:bg-emerald-200 dark:bg-emerald-900/40 dark:text-emerald-400 dark:hover:bg-emerald-900/60",
                                            )}
                                        >
                                            <Play size={12} className="fill-current" />
                                            {t("run", "Run")}
                                        </button>
                                    </span>
                                </TooltipTrigger>
                                {hasUnsavedChanges && (
                                    <TooltipContent>
                                        {t("save_before_running", "Save the draft before running.")}
                                    </TooltipContent>
                                )}
                            </Tooltip>
                        )}
                    </div>

                    {/* Body */}
                    <div
                        className={cn(
                            "px-3 py-2.5 text-sm",
                            isInfoStep && "pt-0 break-words hyphens-auto",
                        )}
                    >
                        {type.Describe &&
                            createElement(type.Describe, {
                                project,
                                journey,
                                value: data,
                                onChange: () => {},
                            })}
                        {!!data_key && (
                            <div
                                className={cn(
                                    "flex items-center gap-1.5 rounded-md bg-muted px-2.5 py-1.5 text-xs text-muted-foreground font-mono",
                                    type.Describe && "mt-2",
                                )}
                            >
                                <KeyIcon className="h-3.5 w-3.5 shrink-0" />
                                {data_key}
                            </div>
                        )}
                    </div>
                </div>

                {!type.hideBottomHandle &&
                    (Array.isArray(type.sources) ? type.sources : [""]).map((key, index, arr) => {
                        const left = ((index + 1) / (arr.length + 1)) * 100 + "%"
                        return (
                            <Fragment key={key}>
                                {key && (
                                    <span
                                        className="absolute text-xs text-muted-foreground font-medium"
                                        style={{
                                            left,
                                            bottom: -20,
                                            transform: "translate(-50%, 0)",
                                        }}
                                    >
                                        {key}
                                    </span>
                                )}
                                <Handle
                                    type="source"
                                    position={Position.Bottom}
                                    id={key + "-s-" + id}
                                    isValidConnection={validateConnection}
                                    style={{ left }}
                                />
                            </Fragment>
                        )
                    })}
            </>
        )
    },
)
