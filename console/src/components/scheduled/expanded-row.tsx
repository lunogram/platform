import { useContext } from "react"
import { useTranslation } from "react-i18next"
import { Loader2 } from "lucide-react"
import { toast } from "sonner"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { formatDate } from "@/utils"
import { JsonView } from "@/components/ui/json-view"
import { DateTimeEdit } from "@/components/ui/datetime-edit"
import { TableCell, TableRow } from "@/components/ui/table"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { useUpdateScheduled, type ScheduledItem, type ScheduledSubjectType } from "./queries"

interface ScheduledExpandedRowProps {
    item: ScheduledItem
    subjectType: ScheduledSubjectType
    subjectId: string
}

export function ScheduledExpandedRow({ item, subjectType, subjectId }: ScheduledExpandedRowProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const update = useUpdateScheduled(
        subjectType,
        subjectId,
        t("failed_to_update_scheduled", "Failed to update scheduled time"),
    )

    // Rejects on failure so the editor stays open with the chosen time.
    const handleScheduledAtSave = async (newIso: string) => {
        await update.mutateAsync({ itemId: item.id, patch: { scheduled_at: newIso } })
        toast.success(t("scheduled_updated", "Scheduled time updated"))
    }

    return (
        <TableRow className="bg-muted/30 hover:bg-muted/30">
            <TableCell colSpan={5} className="p-0">
                <div className="px-6 py-4 space-y-4">
                    {/* Info */}
                    <div className="flex flex-col sm:flex-row gap-4 sm:gap-8">
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("id", "ID")}
                            </p>
                            <code className="text-sm">{item.id}</code>
                        </div>
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("scheduled_at", "Scheduled At")}
                            </p>
                            <div className="flex items-center gap-2">
                                <DateTimeEdit
                                    value={item.scheduled_at}
                                    onSave={handleScheduledAtSave}
                                >
                                    <span className="text-sm">
                                        {formatDate(preferences, item.scheduled_at, "PPpp")}
                                    </span>
                                </DateTimeEdit>
                                {!item.paused_at && item.has_pending_events && (
                                    <Tooltip>
                                        <TooltipTrigger asChild>
                                            <span className="inline-flex items-center gap-1 text-xs text-blue-600 dark:text-blue-400">
                                                <Loader2 className="h-3 w-3 animate-spin" />
                                            </span>
                                        </TooltipTrigger>
                                        <TooltipContent>
                                            {t(
                                                "scheduled_sending_tooltip",
                                                "This message is scheduled to be sent out. It could take up to 1 minute to process.",
                                            )}
                                        </TooltipContent>
                                    </Tooltip>
                                )}
                            </div>
                        </div>
                        <div className="space-y-1">
                            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                {t("created_at", "Created At")}
                            </p>
                            <p className="text-sm">
                                {formatDate(preferences, item.created_at, "PPpp")}
                            </p>
                        </div>
                    </div>

                    {/* Data */}
                    <JsonView
                        data={(item.data as Record<string, unknown>) ?? {}}
                        title={t("scheduled_data", "Scheduled data")}
                        defaultExpanded
                    />
                </div>
            </TableCell>
        </TableRow>
    )
}
