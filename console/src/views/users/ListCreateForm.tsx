import { useContext } from "react"
import api from "../../api"
import { ProjectContext } from "../../contexts"
import type { List, ListCreateParams } from "../../types"
import FormWrapper from "@/components/form/FormWrapper"
import RadioInput from "@/components/form/RadioInput"
import TextInput from "@/components/form/TextInput"
import { useTranslation } from "react-i18next"
import { createWrapperRule } from "./rules/RuleHelpers"

interface ListCreateFormProps {
    onCreated?: (list: List) => void
}

export function ListCreateForm({ onCreated }: ListCreateFormProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const defaults: Partial<ListCreateParams> = {
        type: "dynamic",
        rule: createWrapperRule(),
    }

    return (
        <FormWrapper<ListCreateParams>
            onSubmit={async (list) => {
                const rule = list.rule ?? createWrapperRule()
                const created = await api.lists.create(project.id, {
                    ...list,
                    rule: list.type === "dynamic" ? rule : undefined,
                    is_visible: true,
                })
                onCreated?.(created)
            }}
            defaultValues={defaults}
            submitLabel={t("save")}
        >
            {(form) => (
                <>
                    <TextInput.Field form={form} name="name" label={t("list_save")} required />
                    <RadioInput.Field
                        form={form}
                        name="type"
                        label={t("type")}
                        options={[
                            { key: "dynamic", label: t("dynamic") },
                            { key: "static", label: t("static") },
                        ]}
                    />
                </>
            )}
        </FormWrapper>
    )
}
