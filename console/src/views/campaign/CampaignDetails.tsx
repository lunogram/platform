import { useContext, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { CampaignContext, ProjectContext } from "@/contexts"
import type { Campaign, Template } from "@/types"
import type { UUID } from "@/types/common"
import { useTranslation } from "react-i18next"
import { Controller, useForm, type FieldValues } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useNavigate } from "react-router"
import { defaultTemplateData } from "@/validation/campaign/template/data"
import { Radio } from "lucide-react"
import { isEnterprise } from "@/config/enterprise"

import { subscriptionsQuery, useCreateTemplate, useUpdateCampaign } from "./queries"
import { SubscriptionSelect } from "./SubscriptionSelect"
import { withChannel, type ChannelConfig } from "./template/channels"
import { TemplateProvider } from "./template/TemplateProvider"
import { UnsupportedChannel } from "./template/UnsupportedChannel"
import { CampaignVariables } from "./CampaignVariables"

import { CampaignVariableProvider } from "./CampaignVariableContext"
import { Tabs, TabsContent } from "@/components/ui/tabs"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import { CreateBroadcastDialog } from "@/views/broadcast/CreateBroadcastDialog"

import { campaignSchema, type CampaignReviewFormData } from "@/validation/campaign/campaign-details"

// DetailsPreview owns the channel form the preview reads from. It is keyed by
// template, so switching locale seeds subject, sender and body from the same template.
function DetailsPreview<T extends FieldValues>({
    campaign,
    template,
    config,
}: {
    campaign: Campaign
    template: Template
    config: ChannelConfig<T>
}) {
    const { useForm: useChannelForm, ContentPreview } = config
    const channelForm = useChannelForm(campaign, template)
    return <ContentPreview campaign={campaign} form={channelForm} edit />
}

function CampaignReview({
    campaign,
    template,
    onSelectTemplate,
}: {
    campaign: Campaign
    template: Template
    onSelectTemplate: (templateId: UUID) => void
}) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const updateCampaign = useUpdateCampaign()
    const [isBroadcastOpen, setIsBroadcastOpen] = useState(false)
    const [isTransactional, setTransactional] = useState(campaign.transactional ?? false)
    const [subscriptionId, setSubscriptionId] = useState<string>(campaign.subscription_id ?? "")

    const { data: subscriptions } = useQuery(subscriptionsQuery(project.id))

    const filteredSubscriptions = useMemo(() => {
        if (!subscriptions) return []
        return subscriptions.filter((s) => s.channel === campaign.channel)
    }, [subscriptions, campaign.channel])

    const effectiveSubscriptionId = useMemo(() => {
        if (isTransactional || filteredSubscriptions.length === 0) {
            return ""
        }
        const hasValidSelection = filteredSubscriptions.some(
            (subscription) => subscription.id === subscriptionId,
        )
        return hasValidSelection ? subscriptionId : filteredSubscriptions[0].id
    }, [isTransactional, filteredSubscriptions, subscriptionId])

    const form = useForm<CampaignReviewFormData>({
        resolver: zodResolver(campaignSchema),
        defaultValues: {
            name: campaign.name || "",
            variables: campaign.variables ?? [],
        },
    })

    const preview = withChannel(campaign.channel, (config) => (
        <DetailsPreview key={template.id} campaign={campaign} template={template} config={config} />
    ))
    if (!preview) {
        return <UnsupportedChannel />
    }

    const onSubmit = (data: CampaignReviewFormData) => {
        updateCampaign.mutate({
            name: data.name,
            transactional: isTransactional,
            subscription_id: isTransactional ? undefined : effectiveSubscriptionId || undefined,
            variables: data.variables.filter((v) => v.name),
        })
    }

    return (
        <CampaignVariableProvider>
            <TemplateProvider template={template} onSelect={onSelectTemplate}>
                <div className="flex h-screen bg-muted/20 overflow-hidden">
                    <div className="h-full overflow-y-auto w-2/5 bg-background p-8">
                        <div className="mb-6">
                            <h1 className="text-2xl font-semibold">
                                {t("campaign.details.title", "Campaign Details")}
                            </h1>
                            <p className="text-muted-foreground">
                                {t(
                                    "campaign.details.description",
                                    "Configure your campaign settings and preview how it will appear to users.",
                                )}
                            </p>
                        </div>

                        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-6">
                            <FieldGroup>
                                <Controller
                                    name="name"
                                    control={form.control}
                                    render={({ field, fieldState }) => (
                                        <Field data-invalid={fieldState.invalid} className="gap-2">
                                            <FieldLabel htmlFor="name">
                                                {t("campaign.setup.form.name.label")}
                                            </FieldLabel>
                                            <Input
                                                {...field}
                                                id="name"
                                                aria-invalid={fieldState.invalid}
                                            />
                                        </Field>
                                    )}
                                />
                            </FieldGroup>

                            <FieldGroup>
                                <Controller
                                    name="variables"
                                    control={form.control}
                                    render={({ field }) => (
                                        <Field className="gap-2">
                                            <FieldLabel>
                                                {t("campaign.variables.label", "Variables")}
                                            </FieldLabel>
                                            <FieldDescription>
                                                {t(
                                                    "campaign.variables.description",
                                                    "Define template variables that can be populated from journeys or the API.",
                                                )}
                                            </FieldDescription>
                                            <CampaignVariables
                                                variables={field.value}
                                                onChange={field.onChange}
                                                channel={campaign.channel}
                                            />
                                        </Field>
                                    )}
                                />
                            </FieldGroup>

                            <div className="flex items-center justify-between rounded-md border p-3">
                                <div className="space-y-1">
                                    <Label htmlFor="transactional-toggle">
                                        {t("campaign.transactional", "Transactional")}
                                    </Label>
                                    <p className="text-sm text-muted-foreground">
                                        {t(
                                            "campaign.transactional.help",
                                            "When enabled, subscription preference is ignored.",
                                        )}
                                    </p>
                                </div>
                                <Switch
                                    id="transactional-toggle"
                                    checked={isTransactional}
                                    onCheckedChange={(checked) => {
                                        setTransactional(checked)
                                        if (checked) setSubscriptionId("")
                                    }}
                                />
                            </div>

                            {!isTransactional && (
                                <FieldGroup>
                                    <Field className="gap-2">
                                        <FieldLabel htmlFor="subscription-select">
                                            {t("campaign.subscription", "Subscription")}
                                        </FieldLabel>
                                        <SubscriptionSelect
                                            id="subscription-select"
                                            channel={campaign.channel}
                                            value={effectiveSubscriptionId}
                                            onChange={setSubscriptionId}
                                        />
                                    </Field>
                                </FieldGroup>
                            )}

                            <div className="flex items-center gap-2">
                                <Button
                                    type="submit"
                                    disabled={updateCampaign.isPending}
                                    isLoading={updateCampaign.isPending}
                                >
                                    {t("actions.save")}
                                </Button>
                                {isEnterprise && (
                                    <Button
                                        variant="outline"
                                        onClick={() => setIsBroadcastOpen(true)}
                                    >
                                        <Radio className="mr-2 h-3.5 w-3.5" />
                                        {t("send_broadcast", "Send Broadcast")}
                                    </Button>
                                )}
                            </div>
                            <CreateBroadcastDialog
                                open={isBroadcastOpen}
                                onOpenChange={setIsBroadcastOpen}
                                campaignId={campaign.id}
                            />
                        </form>
                    </div>

                    <div className="w-3/5 bg-background p-8 pb-0 border-l">
                        <Tabs defaultValue="preview" className="h-full flex flex-col">
                            <TabsContent value="preview" className="flex-1">
                                {preview}
                            </TabsContent>
                        </Tabs>
                    </div>
                </div>
            </TemplateProvider>
        </CampaignVariableProvider>
    )
}

// NoTemplates is shown for a campaign whose template was never created, for
// example when creating the campaign failed halfway through.
function NoTemplates({ campaign }: { campaign: Campaign }) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const createTemplate = useCreateTemplate()

    const create = () =>
        createTemplate.mutate(
            { locale: project.locale, data: defaultTemplateData(campaign.channel) },
            {
                onSuccess: (template) =>
                    navigate(
                        `/projects/${project.id}/campaigns/${campaign.id}/templates/${template.id}`,
                    ),
            },
        )

    return (
        <div className="flex h-screen items-center justify-center bg-muted/20">
            <div className="flex flex-col items-center gap-4 text-center">
                <p className="text-muted-foreground">
                    {t("campaign.details.no_templates", "This campaign has no template yet.")}
                </p>
                <Button
                    onClick={create}
                    isLoading={createTemplate.isPending}
                    disabled={createTemplate.isPending}
                >
                    {t("campaign.details.create_template", "Create template")}
                </Button>
            </div>
        </div>
    )
}

export default function CampaignDetails() {
    const [campaign] = useContext(CampaignContext)
    const [project] = useContext(ProjectContext)
    // Only the choice is kept here; the template itself is always read from the
    // cached campaign, so it reflects the latest save.
    const [selectedTemplateId, setSelectedTemplateId] = useState<UUID>()

    if (campaign.templates.length === 0) {
        return <NoTemplates campaign={campaign} />
    }

    const template =
        campaign.templates.find((t) => t.id === selectedTemplateId) ??
        campaign.templates.find((t) => t.locale === project.locale) ??
        campaign.templates[0]

    return (
        <CampaignReview
            campaign={campaign}
            template={template}
            onSelectTemplate={setSelectedTemplateId}
        />
    )
}
