import type { JourneyStepType } from "../../../types"
import { EventStepIcon } from "../../../components/icons"
import { CodeEditor } from "@/components/ui/code-editor"
import { JsonEditor } from "@/components/ui/json-editor"
import { Label } from "@/components/ui/label"
import { Input } from "@/components/ui/input"
import { useTranslation } from "react-i18next"

interface EventConfig {
    event_name: string
    template: string // handlebars template for json object
}

export const eventStep: JourneyStepType<EventConfig> = {
    name: "trigger_event",
    icon: <EventStepIcon />,
    category: "action",
    description: "trigger_event_desc",
    Describe({ value }) {
        const { t } = useTranslation()
        if (value?.template) {
            try {
                JSON.parse(value.template)
                return <JsonEditor value={value.template} onChange={() => {}} readOnly />
            } catch {
                return <>{t("trigger_event_empty")}</>
            }
        }
        return null
    },
    newData: async () => ({
        template: "{\n\n}\n",
        event_name: "Journey Triggered",
    }),
    Edit: ({ onChange, value }) => {
        const { t } = useTranslation()
        return (
            <>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("event_name")}</Label>
                    <Input
                        value={value.event_name}
                        onChange={(e) => onChange({ ...value, event_name: e.target.value })}
                    />
                </div>
                <p className="max-w-[400px] text-sm text-muted-foreground">
                    {t("trigger_event_desc1")}
                    {t("trigger_event_desc2")}
                    <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">{"user"}</code>
                    {t("trigger_event_desc3")}
                    <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
                        {"journey[data_key]"}
                    </code>
                    {"."}
                </p>
                <CodeEditor
                    onChange={(template) => onChange({ ...value, template })}
                    value={value.template ?? ""}
                    minHeight={500}
                    maxHeight={500}
                />
            </>
        )
    },
}
