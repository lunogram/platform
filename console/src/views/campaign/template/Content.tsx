import { useContext, useEffect } from "react"
import { CampaignContext, TemplateContext as CurrentTemplateContext } from "@/contexts"
import { useTranslation } from "react-i18next"

import { useUpdateTemplate } from "../queries"
import { channels, type ChannelConfig } from "./channels"
import { TemplateWorkflowContext } from "./contexts"
import { UnsupportedChannel } from "./UnsupportedChannel"

export default function TemplateContent() {
    const [campaign] = useContext(CampaignContext)
    const config = channels[campaign.channel]

    if (!config) {
        return <UnsupportedChannel />
    }

    return <ChannelContent config={config} />
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function ChannelContent({ config }: { config: ChannelConfig<any> }) {
    const [campaign] = useContext(CampaignContext)
    const { onSubmit } = useContext(TemplateWorkflowContext)
    const [template] = useContext(CurrentTemplateContext)
    const { mutateAsync: updateTemplate } = useUpdateTemplate()
    const { t } = useTranslation()

    const { useForm: useChannelForm, FormControl, Preview } = config
    const form = useChannelForm(campaign, template)

    useEffect(() => {
        const unsubscribe = onSubmit(async () => {
            const isValid = await form.trigger()
            if (!isValid) {
                return false
            }

            const { sender_identity_id, ...data } = form.getValues()
            await updateTemplate({
                templateId: template.id,
                params: {
                    data: { ...template.data, ...data },
                    sender_identity_id: sender_identity_id || null,
                },
            })
            return true
        })
        return unsubscribe
    }, [onSubmit, form, template, updateTemplate])

    return (
        <div className="flex flex-1 bg-muted/20 overflow-hidden">
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
        </div>
    )
}
