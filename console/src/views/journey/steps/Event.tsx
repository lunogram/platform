import type { JourneyStepType } from "../../../types"
import { EventStepIcon } from "../../../components/icons"
import { CodeEditor } from "@/components/ui/code-editor"
import { Label } from "@/components/ui/label"
import { TemplateInput } from "@/components/ui/template-input"
import { useTranslation } from "react-i18next"
import { useJourneyVariableContext } from "../JourneyVariableContext"

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
                return (
                    <CodeEditor
                        value={value.template}
                        onChange={() => {}}
                        readOnly
                        language="json"
                    />
                )
            } catch {
                return <>{t("trigger_event_empty")}</>
            }
        }
        return null
    },
    newData: async () => ({
        template: "{\n\n}\n",
        event_name: "user.updated",
    }),
    Edit: ({ onChange, value, nodeId }) => {
        const { t } = useTranslation()
        const { getVariablesForNode } = useJourneyVariableContext()
        const variables = nodeId ? getVariablesForNode(nodeId) : []
        return (
            <>
                <div className="space-y-1.5">
                    <Label className="text-sm font-medium">{t("event_name")}</Label>
                    <TemplateInput
                        value={value.event_name}
                        onChange={(event_name) => onChange({ ...value, event_name })}
                        variables={variables}
                    />
                </div>
                <p className="text-sm text-muted-foreground">
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
                    maxHeight={500}
                    language="json"
                    className="rounded-md border"
                    variables={variables}
                />
            </>
        )
    },
}
