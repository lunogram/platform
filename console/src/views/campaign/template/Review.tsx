import { useCallback, useContext, useState } from "react"
import type { FieldValues } from "react-hook-form"
import {
    CampaignContext,
    ProjectContext,
    TemplateContext as CurrentTemplateContext,
} from "@/contexts"
import { useTranslation } from "react-i18next"
import api from "@/api"
import { Radio } from "lucide-react"
import { isEnterprise } from "@/config/enterprise"

import { withChannel, type ChannelConfig } from "./channels"
import { saveWith } from "./contexts"
import { StepLayout } from "./StepLayout"

import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { UnsupportedChannel } from "./UnsupportedChannel"
import { CreateBroadcastDialog } from "@/views/broadcast/CreateBroadcastDialog"

export default function TemplateReview() {
    const [campaign] = useContext(CampaignContext)

    return (
        withChannel(campaign.channel, (config) => <ChannelReview config={config} />) ?? (
            <StepLayout>
                <UnsupportedChannel />
            </StepLayout>
        )
    )
}

// The review step has no edits of its own, so it keeps StepLayout's default
// save; its only action is Publish.
function ChannelReview<T extends FieldValues>({ config }: { config: ChannelConfig<T> }) {
    const [campaign] = useContext(CampaignContext)
    const [project] = useContext(ProjectContext)
    const [template] = useContext(CurrentTemplateContext)
    const { t } = useTranslation()
    const [isBroadcastOpen, setIsBroadcastOpen] = useState(false)

    const { useForm: useChannelForm, FormControl, ContentPreview } = config
    const form = useChannelForm(campaign, template)

    const publish = useCallback(
        () =>
            saveWith(() =>
                api.campaigns.update(project.id, campaign.id, {
                    state: "running",
                }),
            ),
        [project.id, campaign.id],
    )

    return (
        <StepLayout onPublish={publish}>
            <div className="h-full overflow-y-auto w-2/5 bg-background p-8">
                <div className="mb-6">
                    <h1 className="text-2xl font-semibold">
                        {t("campaign.review.title", "Review")}
                    </h1>
                    <p className="text-muted-foreground">
                        {t("campaign.review.description", "Review your campaign before sending.")}
                    </p>
                </div>

                <div className="space-y-6">
                    <FieldGroup>
                        <Field className="gap-2">
                            <FieldLabel>{t("campaign.setup.form.name.label")}</FieldLabel>
                            <Input value={campaign.name} readOnly disabled />
                        </Field>
                    </FieldGroup>

                    <FormControl campaign={campaign} form={form} disabled />

                    {isEnterprise && (
                        <div className="pt-4">
                            <Button variant="outline" onClick={() => setIsBroadcastOpen(true)}>
                                <Radio className="mr-2 h-3.5 w-3.5" />
                                {t("send_broadcast", "Send Broadcast")}
                            </Button>
                        </div>
                    )}
                    <CreateBroadcastDialog
                        open={isBroadcastOpen}
                        onOpenChange={setIsBroadcastOpen}
                        campaignId={campaign.id}
                    />
                </div>
            </div>

            <div className="w-3/5 bg-background p-8 pb-0 border-l">
                <ContentPreview campaign={campaign} form={form} />
            </div>
        </StepLayout>
    )
}
