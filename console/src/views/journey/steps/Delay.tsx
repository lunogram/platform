import { useContext } from "react"
import type { JourneyStepType } from "../../../types"
import { DelayStepIcon } from "../../../components/icons"
import { formatDate, formatDuration, snakeToTitle } from "../../../utils"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { parse, parseISO } from "date-fns"
import { useTranslation } from "react-i18next"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { TemplateInput } from "@/components/ui/template-input"
import { Timer, Clock, CalendarDays } from "lucide-react"
import { cn } from "@/utils"
import { useJourneyVariableContext } from "../JourneyVariableContext"

interface DelayStepConfig {
    format: "duration" | "time" | "date"
    minutes: number | string
    hours: number | string
    days: number | string
    time?: string
    date?: string
    exclusion_days?: number[]
}

const formats = ["duration", "time", "date"] as const

const formatConfig = {
    duration: { icon: Timer, label: "Duration" },
    time: { icon: Clock, label: "Time" },
    date: { icon: CalendarDays, label: "Date" },
} as const

export const dayOptions = [
    { key: 0, label: "Sun" },
    { key: 1, label: "Mon" },
    { key: 2, label: "Tue" },
    { key: 3, label: "Wed" },
    { key: 4, label: "Thu" },
    { key: 5, label: "Fri" },
    { key: 6, label: "Sat" },
]

export const delayStep: JourneyStepType<DelayStepConfig> = {
    name: "delay",
    icon: <DelayStepIcon />,
    category: "delay",
    description: "delay_desc",
    Describe({ value }) {
        const { t } = useTranslation()
        const [preferences] = useContext(PreferencesContext)
        if (value.format === "duration") {
            const hasTemplate = [value.days, value.hours, value.minutes].some(
                (v) => typeof v === "string" && v.includes("{{"),
            )
            return (
                <>
                    {t("wait") + " "}
                    <strong>
                        {hasTemplate ? (
                            <>
                                {value.days ? `${value.days}d ` : ""}
                                {value.hours ? `${value.hours}h ` : ""}
                                {value.minutes ? `${value.minutes}m` : ""}
                            </>
                        ) : (
                            formatDuration(preferences, {
                                days: Number(value.days) || 0,
                                hours: Number(value.hours) || 0,
                                minutes: Number(value.minutes) || 0,
                            }) || <>&#8211;</>
                        )}
                    </strong>
                </>
            )
        }
        if (value.format === "time") {
            const timeVal = value.time ?? ""
            const hasTemplate = timeVal.includes("{{")
            const parsed = parse(timeVal, "HH:mm", new Date())
            return (
                <>
                    {t("wait_until") + " "}
                    <strong>
                        {hasTemplate
                            ? timeVal
                            : isNaN(parsed.getTime())
                              ? "--:--"
                              : formatDate(preferences, parsed, "p")}
                    </strong>
                </>
            )
        }
        if (value.format === "date") {
            const parsed = parseISO(value.date ?? "")
            return (
                <>
                    {t("wait_until") + " "}
                    <strong>
                        {isNaN(parsed.getTime()) ? (
                            value.date?.includes("{{") ? (
                                <>
                                    <br />
                                    {value.date}
                                </>
                            ) : (
                                <>&#8211;</>
                            )
                        ) : (
                            formatDate(preferences, parsed, "PP")
                        )}
                    </strong>
                </>
            )
        }
        return null
    },
    newData: async () => ({
        minutes: 0,
        hours: 0,
        days: 0,
        format: "duration",
    }),
    Edit({ onChange, value, nodeId }) {
        const { t } = useTranslation()
        const { getVariablesForNode } = useJourneyVariableContext()
        const variables = nodeId ? getVariablesForNode(nodeId) : []
        return (
            <>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("type")}</Label>
                    <Tabs
                        value={value.format}
                        onValueChange={(format) =>
                            onChange({
                                ...value,
                                format: format as DelayStepConfig["format"],
                            })
                        }
                    >
                        <TabsList className="w-full">
                            {formats.map((key) => {
                                const { icon: Icon, label } = formatConfig[key]
                                return (
                                    <TabsTrigger key={key} value={key} className="flex-1 gap-1.5">
                                        <Icon className="h-3.5 w-3.5" />
                                        {label}
                                    </TabsTrigger>
                                )
                            })}
                        </TabsList>
                    </Tabs>
                </div>
                {value.format === "duration" &&
                    (["days", "hours", "minutes"] as const).map((name) => (
                        <div key={name} className="space-y-1.5">
                            <Label className="text-sm font-medium">{snakeToTitle(name)}</Label>
                            <TemplateInput
                                value={String(value[name] ?? 0)}
                                onChange={(v) => {
                                    const num = Number(v)
                                    onChange({
                                        ...value,
                                        [name]:
                                            v.includes("{{") || v.includes("{%") || isNaN(num)
                                                ? v
                                                : num,
                                    })
                                }}
                                variables={variables}
                            />
                        </div>
                    ))}
                {value.format === "time" && (
                    <>
                        <div className="space-y-1.5">
                            <Label className="text-sm font-medium">{t("time")}</Label>
                            <p className="text-xs text-muted-foreground">{t("delay_time_desc")}</p>
                            <TemplateInput
                                placeholder="HH:mm or {{ variable }}"
                                value={value.time ?? ""}
                                onChange={(time) => onChange({ ...value, time })}
                                variables={variables}
                            />
                        </div>
                        <div className="space-y-1.5">
                            <Label className="text-sm font-medium">
                                {t("delay_exclusion_dates")}
                            </Label>
                            <p className="text-xs text-muted-foreground">
                                {t("delay_exclusion_dates_desc")}
                            </p>
                            <div className="flex flex-wrap gap-1.5">
                                {dayOptions.map(({ key, label }) => {
                                    const selected = value.exclusion_days?.includes(key)
                                    const atLimit =
                                        !selected && (value.exclusion_days?.length ?? 0) >= 6
                                    return (
                                        <button
                                            key={key}
                                            type="button"
                                            disabled={atLimit}
                                            onClick={() => {
                                                const days = value.exclusion_days ?? []
                                                onChange({
                                                    ...value,
                                                    exclusion_days: selected
                                                        ? days.filter((d) => d !== key)
                                                        : [...days, key],
                                                })
                                            }}
                                            className={cn(
                                                "rounded-md border px-2.5 py-1 text-xs font-medium transition-colors",
                                                selected
                                                    ? "border-primary bg-primary text-primary-foreground"
                                                    : "border-input bg-background hover:bg-accent hover:text-accent-foreground",
                                                atLimit && "cursor-not-allowed opacity-50",
                                            )}
                                        >
                                            {label}
                                        </button>
                                    )
                                })}
                            </div>
                        </div>
                    </>
                )}
                {value.format === "date" && (
                    <div className="space-y-1.5">
                        <Label className="text-sm font-medium">{t("date")}</Label>
                        <p className="text-xs text-muted-foreground">{t("delay_date_desc")}</p>
                        <TemplateInput
                            placeholder="2025-01-15T09:00:00Z, 2025-01-15T09:00:00, 2025-01-15 or {{ variable }}"
                            value={value.date ?? ""}
                            onChange={(date) => onChange({ ...value, date })}
                            variables={variables}
                        />
                    </div>
                )}
            </>
        )
    },
}
