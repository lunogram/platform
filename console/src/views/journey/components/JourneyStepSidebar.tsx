import { createElement } from "react"
import { useTranslation } from "react-i18next"
import { MoreHorizontal, Trash2, CheckCircle2, Clock, Zap, ArrowRight } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/utils"
import { getStepType } from "../editor/JourneyEditor.utils"
import { stepCategoryColors } from "../hooks/JourneyEditor.constants"
import type { JourneyNode } from "../editor/JourneyEditor.types"
import type { Project, Journey } from "@/types"

interface JourneyStepSidebarProps {
    editNode: JourneyNode
    nodes: JourneyNode[]
    project: Project
    journey: Journey
    onUpdate: (partial: Partial<JourneyNode["data"]>) => void
    onDelete: (id: string) => void
    onViewUsers: (stepId: string, stepType: string, stepName: string) => void
    onSaveDraft: () => Promise<void>
}

export function JourneyStepSidebar({
    editNode,
    nodes,
    project,
    journey,
    onUpdate,
    onDelete,
    onViewUsers,
    onSaveDraft,
}: JourneyStepSidebarProps) {
    const { t } = useTranslation()
    const type = editNode.data.type ? getStepType(editNode.data.type) : null

    if (!type) return null

    const stats = editNode.data.stats ?? {}
    const categoryColor = stepCategoryColors[type.category as keyof typeof stepCategoryColors]

    const hasDelayStat = editNode.data.type === "delay" || !!stats.delay
    const hasCampaignStat = editNode.data.type === "campaign" || !!stats.campaign

    return (
        <div className="flex flex-col h-full">
            {/* Header */}
            <div className="flex items-center gap-3 px-4 py-3 border-b shrink-0">
                <span
                    className={cn(
                        "flex h-8 w-8 shrink-0 items-center justify-center rounded-md [&_svg]:h-4 [&_svg]:w-4",
                        categoryColor,
                    )}
                >
                    {type.icon}
                </span>
                <h4 className="flex-1 text-sm font-semibold truncate">{t(type.name)}</h4>

                {/* More menu */}
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="sm" className="h-8 w-8 p-0">
                            <MoreHorizontal className="h-4 w-4" />
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                        <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onClick={() => onDelete(editNode.id)}
                        >
                            <Trash2 className="h-4 w-4" />
                            {t("delete_step")}
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            </div>

            {/* User Stats */}
            {editNode.data.stepId && (
                <div className="px-4 pt-3">
                    <button
                        type="button"
                        className="w-full rounded-lg border p-3 text-left hover:bg-muted/50 transition-colors cursor-pointer"
                        onClick={() =>
                            onViewUsers(
                                editNode.data.stepId!,
                                editNode.data.type!,
                                editNode.data.name ?? t(type.name),
                            )
                        }
                    >
                        <div className="flex items-center gap-3">
                            <div className="flex items-center gap-1.5">
                                <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />
                                <span className="text-sm font-medium tabular-nums">
                                    {stats.completed ?? 0}
                                </span>
                                <span className="text-xs text-muted-foreground">
                                    {t("completed")}
                                </span>
                            </div>
                            {hasDelayStat && (
                                <div className="flex items-center gap-1.5">
                                    <Clock className="h-3.5 w-3.5 text-amber-500" />
                                    <span className="text-sm font-medium tabular-nums">
                                        {stats.delay ?? 0}
                                    </span>
                                    <span className="text-xs text-muted-foreground">
                                        {t("waiting")}
                                    </span>
                                </div>
                            )}
                            {hasCampaignStat && (
                                <div className="flex items-center gap-1.5">
                                    <Zap className="h-3.5 w-3.5 text-blue-500" />
                                    <span className="text-sm font-medium tabular-nums">
                                        {stats.campaign ?? 0}
                                    </span>
                                    <span className="text-xs text-muted-foreground">
                                        {t("sent")}
                                    </span>
                                </div>
                            )}
                            <div className="flex-1" />
                            <span className="flex items-center gap-1 text-xs text-muted-foreground">
                                {t("view")}
                                <ArrowRight className="h-3 w-3" />
                            </span>
                        </div>
                    </button>
                </div>
            )}

            {/* Edit form */}
            <ScrollArea className="flex-1">
                <div className="p-4 space-y-4">
                    <div className="space-y-1.5">
                        <Label htmlFor="stepName">{t("name")}</Label>
                        <Input
                            id="stepName"
                            value={editNode.data.name ?? ""}
                            onChange={(e) => onUpdate({ name: e.target.value })}
                        />
                    </div>

                    {type.hasDataKey && (
                        <div className="space-y-1.5">
                            <Label htmlFor="dataKey">{t("data_key")}</Label>
                            <Input
                                id="dataKey"
                                value={editNode.data.data_key ?? ""}
                                onChange={(e) => onUpdate({ data_key: e.target.value })}
                            />
                        </div>
                    )}

                    {/* Dynamic Type-Specific Editor */}
                    {type.Edit &&
                        createElement(type.Edit, {
                            value: editNode.data.data ?? {},
                            onChange: (data: Record<string, unknown>) => onUpdate({ data }),
                            project,
                            journey,
                            stepId: editNode.data.stepId,
                            nodes,
                            onSaveDraft,
                        })}
                </div>
            </ScrollArea>
        </div>
    )
}
