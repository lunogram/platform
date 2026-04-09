import { useContext, useState, useEffect } from "react"
import {
    CampaignContext,
    ProjectContext,
    TemplateContext as CurrentTemplateContext,
} from "@/contexts"
import type { User } from "@/types"
import { useTranslation } from "react-i18next"
import api from "@/api"
import { Radio } from "lucide-react"
import { isEnterprise } from "@/config/enterprise"

import { channels } from "./channels"

import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { TemplateWorkflowContext } from "./contexts"
import { CreateBroadcastDialog } from "@/views/broadcast/CreateBroadcastDialog"

export default function TemplateReview() {
    const [campaign] = useContext(CampaignContext)
    const [project] = useContext(ProjectContext)
    const { onSubmit } = useContext(TemplateWorkflowContext)
    const [template] = useContext(CurrentTemplateContext)
    const { t } = useTranslation()
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [isBroadcastOpen, setIsBroadcastOpen] = useState(false)

    useEffect(() => {
        if (!selectedUser && project?.id) {
            api.users.search(project.id, { limit: 1 }).then((result) => {
                if (result.results && result.results.length > 0) {
                    setSelectedUser(result.results[0])
                }
            })
        }
    }, [project?.id, selectedUser])

    useEffect(() => {
        if (!campaign || !project || !template) {
            return
        }

        const unsubscribe = onSubmit(async () => {
            if (!template) {
                return false
            }

            await api.campaigns.update(project.id, campaign.id, {
                state: "running",
            })

            return true
        })
        return unsubscribe
    }, [onSubmit, template, project, campaign])

    if (!campaign || !project || !template) {
        return null
    }

    const config = channels[campaign.channel]

    if (!config) {
        return null
    }

    const form = config.form(campaign, template)
    const ChannelFormControl = config.FormControl
    const ChannelPreview = config.ContentPreview

    return (
        <div className="flex flex-1 bg-muted/20 overflow-hidden">
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

                    <ChannelFormControl campaign={campaign} form={form} disabled />

                    {isEnterprise && (
                        <div className="pt-4">
                            <Button
                                variant="outline"
                                onClick={() => setIsBroadcastOpen(true)}
                            >
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
                <ChannelPreview campaign={campaign} form={form} />
            </div>
        </div>
    )
}
