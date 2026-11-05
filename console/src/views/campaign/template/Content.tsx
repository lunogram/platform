import { useCallback, useContext } from "react"
import type { FieldValues } from "react-hook-form"
import { CampaignContext, TemplateContext as CurrentTemplateContext } from "@/contexts"
import { useTranslation } from "react-i18next"
import { knownTemplateData } from "@/validation/campaign/template/data"

import { useUpdateTemplate } from "../queries"
import { withChannel, type ChannelConfig } from "./channels"
import { saveWith, type SaveResult } from "./contexts"
import { StepLayout } from "./StepLayout"
import { UnsupportedChannel } from "./UnsupportedChannel"

export default function TemplateContent() {
    const [campaign] = useContext(CampaignContext)

    return (
        withChannel(campaign.channel, (config) => <ChannelContent config={config} />) ?? (
            <StepLayout>
                <UnsupportedChannel />
            </StepLayout>
        )
    )
}

function ChannelContent<T extends FieldValues>({ config }: { config: ChannelConfig<T> }) {
    const [campaign] = useContext(CampaignContext)
    const [template] = useContext(CurrentTemplateContext)
    const { mutateAsync: updateTemplate } = useUpdateTemplate()
    const { t } = useTranslation()

    const { useForm: useChannelForm, toTemplate, FormControl, Preview } = config
    const form = useChannelForm(campaign, template)

    const save = useCallback(async (): Promise<SaveResult> => {
        if (!(await form.trigger())) {
            return { ok: false, reason: "invalid" }
        }

        const changes = toTemplate(form.getValues())
        return saveWith(() =>
            updateTemplate({
                templateId: template.id,
                params: {
                    data: knownTemplateData(template.type, { ...template.data, ...changes.data }),
                    sender_identity_id: changes.sender_identity_id,
                },
            }),
        )
    }, [form, toTemplate, template, updateTemplate])

    return (
        <StepLayout save={save}>
            <div className="h-full overflow-y-auto w-2/5 bg-background p-8">
                <div className="mb-6">
                    <h1 className="text-2xl font-semibold">{t("campaign.template.setup.title")}</h1>
                    <p className="text-muted-foreground">
                        {t("campaign.template.setup.description")}
                    </p>
                </div>

                <FormControl campaign={campaign} form={form} />
            </div>

            <div className="w-3/5 bg-background p-8 pb-0 flex flex-col border-l">
                <Preview campaign={campaign} form={form} />
            </div>
        </StepLayout>
    )
}
