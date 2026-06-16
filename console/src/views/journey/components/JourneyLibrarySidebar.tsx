import { Blocks, GripVertical, SquareFunction, Webhook, Zap } from "lucide-react"
import { useTranslation } from "react-i18next"

import { NavTabs } from "@/components/ui/nav-tabs"
import { ScrollArea } from "@/components/ui/scroll-area"
import type { Action } from "@/oapi/client"
import { cn, createComparator } from "@/utils"
import { DATA_FORMAT, stepCategoryColors } from "../hooks/JourneyEditor.constants"
import * as journeySteps from "../steps/index"

interface JourneyLibrarySidebarProps {
    actions: Action[] | null
    sidebarTab: "components" | "actions"
    onSidebarTabChange: (tab: "components" | "actions") => void
}

function setDragData(event: React.DragEvent<HTMLDivElement>, payload: Record<string, unknown>) {
    const rect = event.currentTarget.getBoundingClientRect()
    event.dataTransfer.setData(
        DATA_FORMAT,
        JSON.stringify({
            ...payload,
            x: event.clientX - rect.left,
            y: event.clientY - rect.top,
        }),
    )
}

export function JourneyLibrarySidebar({
    actions,
    sidebarTab,
    onSidebarTabChange,
}: JourneyLibrarySidebarProps) {
    const { t } = useTranslation()

    return (
        <>
            <NavTabs
                className="px-4 pt-3 border-b shrink-0"
                tabs={[
                    {
                        key: "components",
                        label: t("components"),
                        icon: Blocks,
                    },
                    {
                        key: "actions",
                        label: t("actions.plural", "Actions"),
                        icon: SquareFunction,
                        badge: actions?.length,
                    },
                ]}
                value={sidebarTab}
                onChange={(key) => onSidebarTabChange(key as "components" | "actions")}
            />
            <ScrollArea className="flex-1">
                {sidebarTab === "components" && (
                    <div className="p-4 space-y-1.5">
                        {Object.entries(journeySteps)
                            .filter(([key]) => key !== "action")
                            .sort(
                                createComparator((x) => {
                                    const order = {
                                        entrance: 0,
                                        flow: 1,
                                        delay: 2,
                                        action: 3,
                                        exit: 4,
                                        info: 5,
                                    }
                                    return order[x[1].category] ?? 99
                                }),
                            )
                            .map(([key, type]) => (
                                <div
                                    key={key}
                                    className="group flex items-start gap-3 rounded-lg border p-3 cursor-grab active:cursor-grabbing hover:bg-muted/50 transition-colors"
                                    draggable
                                    onDragStart={(event) => setDragData(event, { type: key })}
                                >
                                    <div
                                        className={cn(
                                            "flex h-8 w-8 shrink-0 items-center justify-center rounded-md [&_svg]:h-4 [&_svg]:w-4",
                                            stepCategoryColors[type.category] ??
                                                "bg-muted text-muted-foreground",
                                        )}
                                    >
                                        {type.icon}
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <p className="text-sm font-medium leading-none">
                                            {t(type.name)}
                                        </p>
                                        <p className="text-xs text-muted-foreground mt-1 leading-snug">
                                            {t(type.description)}
                                        </p>
                                    </div>
                                    <GripVertical className="h-4 w-4 text-muted-foreground/40 shrink-0 mt-0.5 opacity-0 group-hover:opacity-100 transition-opacity" />
                                </div>
                            ))}
                    </div>
                )}
                {sidebarTab === "actions" && (
                    <div className="p-4 space-y-2">
                        {!actions ? (
                            Array.from({ length: 3 }).map((_, i) => (
                                <div
                                    key={i}
                                    className="rounded-lg border border-dashed p-3 animate-pulse"
                                >
                                    <div className="flex items-center gap-3">
                                        <div className="h-9 w-9 rounded-md bg-muted" />
                                        <div className="flex-1 space-y-1.5">
                                            <div className="h-3.5 w-24 rounded bg-muted" />
                                            <div className="h-3 w-16 rounded bg-muted" />
                                        </div>
                                    </div>
                                </div>
                            ))
                        ) : actions.length === 0 ? (
                            <div className="rounded-lg border border-dashed p-6 text-center">
                                <Zap className="h-6 w-6 text-muted-foreground/50 mx-auto mb-2" />
                                <p className="text-sm font-medium text-muted-foreground">
                                    {t("no_actions_yet", "No actions yet")}
                                </p>
                                <p className="text-xs text-muted-foreground/70 mt-1">
                                    {t(
                                        "actions_drag_desc",
                                        "Integrations that support actions will appear here",
                                    )}
                                </p>
                            </div>
                        ) : (
                            actions.map((action) => {
                                const icon =
                                    action.type === "webhook" ? (
                                        <Webhook className="h-4 w-4" />
                                    ) : (
                                        <Zap className="h-4 w-4" />
                                    )
                                return (
                                    <div
                                        key={action.id}
                                        className="group flex items-center gap-3 rounded-lg border bg-card p-3 cursor-grab active:cursor-grabbing hover:bg-accent/50 hover:border-blue-300 dark:hover:border-blue-700 transition-colors"
                                        draggable
                                        onDragStart={(event) =>
                                            setDragData(event, {
                                                type: "action",
                                                name: action.name,
                                                data: { action_id: action.id },
                                            })
                                        }
                                    >
                                        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-blue-100 text-blue-600 dark:bg-blue-950 dark:text-blue-400 [&_svg]:h-4 [&_svg]:w-4">
                                            {icon}
                                        </div>
                                        <div className="flex-1 min-w-0">
                                            <p className="text-sm font-medium leading-none truncate">
                                                {action.name}
                                            </p>
                                            <p className="text-xs text-muted-foreground mt-1">
                                                {action.type}
                                            </p>
                                        </div>
                                        <GripVertical className="h-4 w-4 text-muted-foreground/40 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" />
                                    </div>
                                )
                            })
                        )}
                    </div>
                )}
            </ScrollArea>
        </>
    )
}
