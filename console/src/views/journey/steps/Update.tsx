import type { JourneyStepType } from "../../../types"
import { UpdateStepIcon } from "../../../components/icons"
import { JsonEditor } from "@/components/ui/json-editor"
import { useTranslation } from "react-i18next"

interface UpdateConfig {
    template: string // handlebars template for json object
}

export const updateStep: JourneyStepType<UpdateConfig> = {
    name: "user_update",
    icon: <UpdateStepIcon />,
    category: "action",
    description: "user_update_desc",
    Describe({ value }) {
        const { t } = useTranslation()
        if (value?.template) {
            try {
                JSON.parse(value.template)
                return <JsonEditor value={value.template} onChange={() => {}} readOnly />
            } catch {
                return <>{t("user_update_empty")}</>
            }
        }
        return null
    },
    newData: async () => ({
        template: "{\n\n}\n",
    }),
    Edit: ({ onChange, value }) => {
        const { t } = useTranslation()
        return (
            <>
                <p className="text-sm text-muted-foreground">
                    {t("user_update_edit_desc1")}
                    {t("user_update_edit_desc2")}
                    <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">{"user"}</code>
                    {t("user_update_edit_desc3")}
                    <code className="rounded bg-muted px-1 py-0.5 font-mono text-xs">
                        {"journey[data_key]"}
                    </code>
                    {"."}
                </p>
                <JsonEditor
                    onChange={(template) => onChange({ ...value, template })}
                    value={value.template ?? ""}
                    maxHeight={500}
                    className="rounded-md border"
                />
            </>
        )
    },
}
