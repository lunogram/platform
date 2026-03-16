import { useContext } from "react"
import {
    CampaignContext,
    ProjectContext,
    TemplateContext as CurrentTemplateContext,
} from "@/contexts"
import { useTranslation } from "react-i18next"
import api from "@/api"

import { channels } from "./channels"
import { TemplateWorkflowContext } from "./contexts"

export default function TemplateContent() {
    const [campaign] = useContext(CampaignContext)
    const { onSubmit } = useContext(TemplateWorkflowContext)
    const [template, setTemplate] = useContext(CurrentTemplateContext)
    const [project] = useContext(ProjectContext)
    const { t } = useTranslation()

    const config = channels[campaign.channel]

    if (!config) {
        return (
            <div className="flex flex-1 items-center justify-center bg-muted/20">
                <div className="text-center">
                    <p className="text-muted-foreground">This channel type is not yet supported.</p>
                </div>
            </div>
        )
    }

    const form = config.form(campaign, template)
    const FormControlComponent = config.FormControl
    const PreviewComponent = config.Preview

    onSubmit(async () => {
        if (!template) {
            return false
        }

        const isValid = await form.trigger()
        if (!isValid) {
            return false
        }

        const { sender_identity_id, ...data } = form.getValues()
        const updated = await api.campaigns.templates.update(project.id, campaign.id, template.id, {
            data: { ...template.data, ...data },
            sender_identity_id: sender_identity_id || null,
        })

        setTemplate(updated)
        return true
    })

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
