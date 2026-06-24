import type { JourneyStepType } from "../../../types"
import { ScheduleStepIcon } from "../../../components/icons"
import { CodeEditor } from "@/components/ui/code-editor"
import { Label } from "@/components/ui/label"
import { TemplateInput } from "@/components/ui/template-input"
import { useTranslation } from "react-i18next"
import { useJourneyVariableContext } from "../JourneyVariableContext"
import { CalendarClock } from "lucide-react"

interface ScheduleConfig {
    schedule_name: string
    scheduled_at?: string
    interval?: string
    start_at?: string
    template?: string // handlebars template for json object
}

export const scheduleStep: JourneyStepType<ScheduleConfig> = {
    name: "assign_schedule",
    icon: <ScheduleStepIcon />,
    category: "action",
    description: "assign_schedule_desc",
    Describe({ value }) {
        const { t } = useTranslation()
        if (!value?.schedule_name && !value?.template) return null
        return (
            <div className="space-y-2">
                <div className="flex items-center gap-1.5 rounded-md bg-muted px-2.5 py-1.5 text-xs text-muted-foreground font-mono truncate">
                    <CalendarClock className="h-3.5 w-3.5 shrink-0" />
                    {value.schedule_name || (
                        <span className="text-muted-foreground">{t("assign_schedule_empty")}</span>
                    )}
                </div>
                {value?.template &&
                    (() => {
                        try {
                            JSON.parse(value.template)
                            return (
                                <CodeEditor
                                    value={value.template}
                                    onChange={() => {}}
                                    readOnly
                                    language="json"
                                />
                            )
                        } catch {
                            return null
                        }
                    })()}
            </div>
        )
    },
    newData: async () => ({
        schedule_name: "",
        template: "{\n\n}\n",
    }),
    validate: (data) => !!data.schedule_name,
    Edit: ({ onChange, value, nodeId }) => {
        const { t } = useTranslation()
        const { getVariablesForNode } = useJourneyVariableContext()
        const variables = nodeId ? getVariablesForNode(nodeId) : []
        return (
            <>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("schedule_name")}</Label>
                    <TemplateInput
                        value={value.schedule_name}
                        onChange={(schedule_name) => onChange({ ...value, schedule_name })}
                        variables={variables}
                    />
                </div>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">
                        {t("assign_schedule_scheduled_at")}
                    </Label>
                    <TemplateInput
                        value={value.scheduled_at ?? ""}
                        onChange={(scheduled_at) =>
                            onChange({ ...value, scheduled_at: scheduled_at || undefined })
                        }
                        variables={variables}
                    />
                    <p className="text-xs text-muted-foreground">
                        {t("assign_schedule_scheduled_at_hint")}
                    </p>
                </div>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("assign_schedule_interval")}</Label>
                    <TemplateInput
                        value={value.interval ?? ""}
                        onChange={(interval) =>
                            onChange({ ...value, interval: interval || undefined })
                        }
                        variables={variables}
                    />
                    <p className="text-xs text-muted-foreground">
                        {t("assign_schedule_interval_hint")}
                    </p>
                </div>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("assign_schedule_start_at")}</Label>
                    <TemplateInput
                        value={value.start_at ?? ""}
                        onChange={(start_at) =>
                            onChange({ ...value, start_at: start_at || undefined })
                        }
                        variables={variables}
                    />
                    <p className="text-xs text-muted-foreground">
                        {t("assign_schedule_start_at_hint")}
                    </p>
                </div>
                <p className="text-sm text-muted-foreground">
                    {t("assign_schedule_desc1")}
                    {t("assign_schedule_desc2")}
                    <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">{"user"}</code>
                    {t("assign_schedule_desc3")}
                    <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
                        {"journey[data_key]"}
                    </code>
                    {"."}
                </p>
                <CodeEditor
                    onChange={(template) => onChange({ ...value, template })}
                    value={value.template ?? ""}
                    maxHeight={500}
                    language="json"
                    className="rounded-md border"
                    variables={variables}
                />
            </>
        )
    },
}
