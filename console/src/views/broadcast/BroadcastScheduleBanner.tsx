import { useContext } from "react"
import { useTranslation } from "react-i18next"
import { CalendarClock, Clock } from "lucide-react"

import { formatDate } from "@/utils"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { splitLocalDateTimeValue, toLocalDateTimeValue } from "@/lib/date-time"

import { Button } from "@/components/ui/button"
import { DateTimeEdit } from "@/components/ui/datetime-edit"
import { Input } from "@/components/ui/input"

interface BroadcastScheduleBannerProps {
    scheduledAt: string | null | undefined
    scheduleValue: string
    isScheduling: boolean
    onSetScheduleValue: (value: string) => void
    onReschedule: (newIso: string) => Promise<void>
    onSetSchedule: () => Promise<void>
    onRemoveSchedule: () => Promise<void>
}

export function BroadcastScheduleBanner({
    scheduledAt,
    scheduleValue,
    isScheduling,
    onSetScheduleValue,
    onReschedule,
    onSetSchedule,
    onRemoveSchedule,
}: BroadcastScheduleBannerProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const { dateValue: scheduleDate, timeValue: scheduleTime } =
        splitLocalDateTimeValue(scheduleValue)

    return (
        <div
            className={`border-b px-4 sm:px-6 py-3 ${
                scheduledAt ? "bg-amber-50/50 dark:bg-amber-950/20" : "bg-muted/30"
            }`}
        >
            {scheduledAt ? (
                <div className="flex items-center justify-between gap-4">
                    <div className="flex items-center gap-3">
                        <CalendarClock className="h-4 w-4 text-amber-600 dark:text-amber-400" />
                        <span className="text-sm text-amber-700 dark:text-amber-300">
                            {t("scheduled_for", "Scheduled for")}
                        </span>
                        <DateTimeEdit value={scheduledAt} onSave={onReschedule}>
                            <span className="text-sm font-medium text-amber-800 dark:text-amber-200">
                                {formatDate(preferences, scheduledAt, "PPpp")}
                            </span>
                        </DateTimeEdit>
                    </div>
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={onRemoveSchedule}
                        disabled={isScheduling}
                    >
                        {t("remove_schedule", "Remove schedule")}
                    </Button>
                </div>
            ) : scheduleValue !== "" ? (
                <form
                    className="flex items-center gap-3 justify-end"
                    onSubmit={(e) => {
                        e.preventDefault()
                        onSetSchedule()
                    }}
                >
                    <CalendarClock className="h-4 w-4 text-muted-foreground" />
                    <div className="flex items-center gap-2">
                        <Input
                            type="date"
                            aria-label={t("schedule_date", "Schedule date")}
                            className="h-8 px-2 text-sm"
                            value={scheduleDate}
                            onChange={(e) => {
                                const date = e.target.value
                                onSetScheduleValue(toLocalDateTimeValue(date, scheduleTime))
                            }}
                            autoFocus
                            required
                            disabled={isScheduling}
                        />
                        <Input
                            type="time"
                            aria-label={t("schedule_time", "Schedule time")}
                            className="h-8 px-2 text-sm"
                            value={scheduleTime}
                            onChange={(e) => {
                                const time = e.target.value
                                onSetScheduleValue(toLocalDateTimeValue(scheduleDate, time))
                            }}
                            disabled={isScheduling || !scheduleDate}
                        />
                    </div>
                    <Button type="submit" size="sm" className="h-7 text-xs" disabled={isScheduling}>
                        {isScheduling ? t("saving", "Saving...") : t("save")}
                    </Button>
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="h-7 text-xs"
                        onClick={() => onSetScheduleValue("")}
                        disabled={isScheduling}
                    >
                        {t("cancel")}
                    </Button>
                </form>
            ) : (
                <div className="flex items-center justify-between gap-4">
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <Clock className="h-4 w-4" />
                        <span>
                            {t(
                                "no_schedule_set_short",
                                "No schedule — send manually or set a time",
                            )}
                        </span>
                    </div>
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 text-xs"
                        onClick={() => {
                            // Pre-fill with tomorrow at 9am
                            const tomorrow = new Date()
                            tomorrow.setDate(tomorrow.getDate() + 1)
                            tomorrow.setHours(9, 0, 0, 0)
                            const y = tomorrow.getFullYear()
                            const m = String(tomorrow.getMonth() + 1).padStart(2, "0")
                            const d = String(tomorrow.getDate()).padStart(2, "0")
                            onSetScheduleValue(toLocalDateTimeValue(`${y}-${m}-${d}`, "09:00"))
                        }}
                    >
                        <CalendarClock className="mr-1.5 h-3 w-3" />
                        {t("schedule_send", "Schedule")}
                    </Button>
                </div>
            )}
        </div>
    )
}
