import { memo, useCallback, useContext, Fragment, createElement, useEffect, useRef } from "react"
import type { NodeProps } from "@xyflow/react"
import { Handle, Position, useReactFlow, NodeResizer, useStore } from "@xyflow/react"
import { useTranslation } from "react-i18next"
import { ChevronDown, FastForward, Play, User } from "lucide-react"
import { ProjectContext, JourneyContext } from "@/contexts"
import { cn } from "@/utils"
import { KeyIcon } from "@/components/icons"
import { Tooltip, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip"
import { getStepType } from "../editor/JourneyEditor.utils"
import { useJourneyHints } from "../editor/JourneyHints"
import { stepCategoryColors, stepCategoryBorderColors } from "../hooks/JourneyEditor.constants"
import type { JourneyNode } from "../editor/JourneyEditor.types"

import "./JourneyStepNode.css"

// Hint-pill palette per category, mirroring the icon container on the step
// card. Values are surfaced as CSS custom properties so the styling stays in
// `JourneyEditor.css` while colors stay in lockstep with the rest of the
// step's visual language.
const stepCategoryHintStyles: Record<string, React.CSSProperties> = {
    entrance: {
        ["--hint-bg" as string]: "var(--color-emerald-100)",
        ["--hint-bg-dark" as string]: "var(--color-emerald-950)",
        ["--hint-fg" as string]: "var(--color-emerald-700)",
        ["--hint-fg-dark" as string]: "var(--color-emerald-300)",
        ["--hint-border" as string]: "var(--color-emerald-400)",
        ["--hint-border-dark" as string]: "var(--color-emerald-700)",
        ["--hint-ring" as string]: "var(--color-emerald-500)",
    },
    action: {
        ["--hint-bg" as string]: "var(--color-blue-100)",
        ["--hint-bg-dark" as string]: "var(--color-blue-950)",
        ["--hint-fg" as string]: "var(--color-blue-700)",
        ["--hint-fg-dark" as string]: "var(--color-blue-300)",
        ["--hint-border" as string]: "var(--color-blue-400)",
        ["--hint-border-dark" as string]: "var(--color-blue-700)",
        ["--hint-ring" as string]: "var(--color-blue-500)",
    },
    flow: {
        ["--hint-bg" as string]: "var(--color-orange-100)",
        ["--hint-bg-dark" as string]: "var(--color-orange-950)",
        ["--hint-fg" as string]: "var(--color-orange-700)",
        ["--hint-fg-dark" as string]: "var(--color-orange-300)",
        ["--hint-border" as string]: "var(--color-orange-400)",
        ["--hint-border-dark" as string]: "var(--color-orange-700)",
        ["--hint-ring" as string]: "var(--color-orange-500)",
    },
    delay: {
        ["--hint-bg" as string]: "var(--color-amber-100)",
        ["--hint-bg-dark" as string]: "var(--color-amber-950)",
        ["--hint-fg" as string]: "var(--color-amber-700)",
        ["--hint-fg-dark" as string]: "var(--color-amber-300)",
        ["--hint-border" as string]: "var(--color-amber-400)",
        ["--hint-border-dark" as string]: "var(--color-amber-700)",
        ["--hint-ring" as string]: "var(--color-amber-500)",
    },
    exit: {
        ["--hint-bg" as string]: "var(--color-red-100)",
        ["--hint-bg-dark" as string]: "var(--color-red-950)",
        ["--hint-fg" as string]: "var(--color-red-700)",
        ["--hint-fg-dark" as string]: "var(--color-red-300)",
        ["--hint-border" as string]: "var(--color-red-400)",
        ["--hint-border-dark" as string]: "var(--color-red-700)",
        ["--hint-ring" as string]: "var(--color-red-500)",
    },
    info: {
        ["--hint-bg" as string]: "var(--color-purple-100)",
        ["--hint-bg-dark" as string]: "var(--color-purple-950)",
        ["--hint-fg" as string]: "var(--color-purple-700)",
        ["--hint-fg-dark" as string]: "var(--color-purple-300)",
        ["--hint-border" as string]: "var(--color-purple-400)",
        ["--hint-border-dark" as string]: "var(--color-purple-700)",
        ["--hint-ring" as string]: "var(--color-purple-500)",
    },
}

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
            connectedSourceHandles = [],
            visited = false,
            active = false,
        },
        selected,
    }: NodeProps<JourneyNode>) => {
        const { t } = useTranslation()
        const [project] = useContext(ProjectContext)
        const [journey] = useContext(JourneyContext)

        const { setNodes } = useReactFlow<JourneyNode>()

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

        const isInfoStep = type?.category === "info"

        const onResize = useCallback(
            (_: unknown, { width, height }: { width: number; height: number }) => {
                setNodes((nds) =>
                    nds.map((n) =>
                        n.id === id
                            ? {
                                  ...n,
                                  style: { ...n.style, width, height },
                                  data: { ...n.data, width, height },
                              }
                            : n,
                    ),
                )
            },
            [id, setNodes],
        )

        const contentRef = useRef<HTMLDivElement>(null)

        useEffect(() => {
            if (!isInfoStep || !contentRef.current) return

            const contentEl = contentRef.current

            const updateHeight = () => {
                const nodeEl = contentEl.closest(".react-flow__node") as HTMLElement | null
                const chromeHeight = nodeEl
                    ? Math.max(nodeEl.offsetHeight - contentEl.clientHeight, 0)
                    : 0
                const neededHeight = contentEl.scrollHeight + chromeHeight

                setNodes((nds) =>
                    nds.map((n) =>
                        n.id === id
                            ? (() => {
                                  const styleHeight =
                                      typeof n.style?.height === "number"
                                          ? n.style.height
                                          : typeof n.style?.height === "string"
                                            ? Number.parseFloat(n.style.height)
                                            : undefined
                                  const dataHeight =
                                      typeof n.data?.height === "number" ? n.data.height : undefined
                                  const currentHeight =
                                      dataHeight ?? styleHeight ?? nodeEl?.offsetHeight ?? 0

                                  if (neededHeight <= currentHeight) return n

                                  return {
                                      ...n,
                                      style: { ...n.style, height: neededHeight },
                                      data: { ...n.data, height: neededHeight },
                                  }
                              })()
                            : n,
                    ),
                )
            }

            const resizeObserver = new ResizeObserver(() => {
                updateHeight()
            })

            resizeObserver.observe(contentEl)
            updateHeight()

            return () => resizeObserver.disconnect()
        }, [id, isInfoStep, setNodes, data])

        const zoom = useStore((s) => s.transform[2])
        const handleSize = Math.min(24, Math.max(8, 10 / zoom))

        const connectedHandles = new Set(connectedSourceHandles)
        const { showConnectHint } = useJourneyHints()

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

        // Hint pill colors mirror the icon container on the step card so the
        // affordance reads as belonging to this step. Values are passed in as
        // CSS custom properties so we can keep all visual rules in CSS.
        const hintStyle = stepCategoryHintStyles[category]

        return (
            <>
                {isInfoStep && (
                    <NodeResizer
                        minWidth={200}
                        minHeight={100}
                        isVisible={selected}
                        handleStyle={{
                            opacity: 0,
                            width: handleSize,
                            height: handleSize,
                            borderRadius: 4,
                        }}
                        lineClassName="sticky-resize-line"
                        onResize={onResize}
                    />
                )}
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
                    <Handle
                        type="target"
                        position={Position.Top}
                        id={"t-" + id}
                        className="journey-handle journey-handle-target"
                    />
                )}

                <div
                    data-step-type={typeName}
                    className={cn(
                        "rounded-lg bg-background shadow-sm transition-all duration-300 min-w-[200px]",
                        isInfoStep
                            ? "bg-purple-50 dark:bg-purple-950/30 h-full w-full flex flex-col"
                            : "",

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
                        ref={contentRef}
                        className={cn(
                            "px-3 py-2.5 text-sm",
                            isInfoStep ? "pt-0 break-words hyphens-auto" : "",
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
                        const handleId = key + "-s-" + id
                        const isConnected = connectedHandles.has(handleId)
                        const showHint = showConnectHint && !isConnected
                        return (
                            <Fragment key={key}>
                                {key && (
                                    <span
                                        className="absolute text-xs text-muted-foreground font-medium"
                                        style={{
                                            left,
                                            bottom: showHint ? -36 : -20,
                                            transform: "translate(-50%, 0)",
                                        }}
                                    >
                                        {key}
                                    </span>
                                )}
                                <Handle
                                    type="source"
                                    position={Position.Bottom}
                                    id={handleId}
                                    className={cn(
                                        "journey-handle journey-handle-source",
                                        showHint && "journey-handle-suggest",
                                    )}
                                    aria-label={
                                        key
                                            ? t("connect_step_path", "Connect {{path}} path", {
                                                  path: key,
                                              })
                                            : t("connect_step", "Connect step")
                                    }
                                    style={{ left, ...(showHint ? hintStyle : null) }}
                                >
                                    {showHint && (
                                        <span className="journey-handle-suggest-inner">
                                            <ChevronDown
                                                size={14}
                                                strokeWidth={2.5}
                                                aria-hidden="true"
                                            />
                                        </span>
                                    )}
                                </Handle>
                            </Fragment>
                        )
                    })}
            </>
        )
    },
)
