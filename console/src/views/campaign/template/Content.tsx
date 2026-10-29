import { useContext, useEffect } from "react"
import { CampaignContext, TemplateContext as CurrentTemplateContext } from "@/contexts"
import { useTranslation } from "react-i18next"

import { useUpdateTemplate } from "../queries"
import { channels } from "./channels"
import { TemplateWorkflowContext } from "./contexts"
import { UnsupportedChannel } from "./UnsupportedChannel"

export default function TemplateContent() {
    const [campaign] = useContext(CampaignContext)
    const { onSubmit } = useContext(TemplateWorkflowContext)
    const [template] = useContext(CurrentTemplateContext)
    const updateTemplate = useUpdateTemplate()
    const { t } = useTranslation()

    const config = channels[campaign.channel]

    const form = config?.form(campaign, template)

    useEffect(() => {
        if (!config || !form) {
            return
        }

        const unsubscribe = onSubmit(async () => {
            if (!template) {
                return false
            }

            const isValid = await form.trigger()
            if (!isValid) {
                return false
            }

            const { sender_identity_id, ...data } = form.getValues()
            await updateTemplate.mutateAsync({
                templateId: template.id,
                params: {
                    data: { ...template.data, ...data },
                    sender_identity_id: sender_identity_id || null,
                },
            })
            return true
        })
        return unsubscribe
    })

    if (!config || !form) {
        return <UnsupportedChannel />
    }

    const FormControlComponent = config.FormControl
    const PreviewComponent = config.Preview

    return (
        <div className="flex flex-1 bg-muted/20 overflow-hidden">
            <div className="h-full overflow-y-auto w-2/5 bg-background p-8">
                <div className="mb-6">
                    <h1 className="text-2xl font-semibold">{t("campaign.template.setup.title")}</h1>
                    <p className="text-muted-foreground">
                        {t("campaign.template.setup.description")}
                    </p>
                </div>

                <FormControlComponent campaign={campaign} form={form} />
            </div>

            <div className="w-3/5 bg-background p-8 pb-0 flex flex-col border-l">
                <PreviewComponent campaign={campaign} form={form} />
            </div>
        </div>
    )
}
