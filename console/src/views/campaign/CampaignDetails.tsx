import { useCallback, useContext, useMemo, useState, useEffect } from "react"
import { CampaignContext, ProjectContext, TemplateContext } from "@/contexts"
import type { Campaign, Template, Subscription } from "@/types"
import { useTranslation } from "react-i18next"
import { Controller, useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import oapiClient from "@/oapi/client"
import { useResolver } from "@/hooks"

import { channels } from "./template/channels"
import { CampaignVariables } from "./CampaignVariables"

import { CampaignVariableProvider } from "./CampaignVariableContext"
import { Tabs, TabsContent } from "@/components/ui/tabs"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { ProviderSelect } from "@/components/provider/select"
import { Switch } from "@/components/ui/switch"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"

const campaignVariableSchema = z.object({
    name: z.string(),
    default: z.string().optional(),
})

const campaignSchema = z.object({
    name: z.string().min(1, "Name is required"),
    provider_id: z.string().optional(),
    variables: z.array(campaignVariableSchema),
})

type CampaignReviewFormData = z.infer<typeof campaignSchema>

function CampaignReview({ campaign, template }: { campaign: Campaign; template: Template }) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [, setCampaign] = useContext(CampaignContext)
    const templateState = useState<Template>(template)
    const [isSubmitting, setIsSubmitting] = useState(false)
    const [isTransactional, setTransactional] = useState(campaign.transactional ?? false)
    const [subscriptionId, setSubscriptionId] = useState<string>(campaign.subscription_id ?? "")

    const [subscriptions] = useResolver(
        useCallback(async (): Promise<Subscription[]> => {
            if (!project?.id) return []
            const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/subscriptions", {
                params: {
                    path: { projectID: project.id },
                    query: { limit: 100 },
                },
            })
            return data?.results ?? []
        }, [project?.id]),
    )

    const filteredSubscriptions = useMemo(() => {
        if (!subscriptions) return []
        return subscriptions.filter((s) => {
            return s.channel === campaign.channel
        })
    }, [subscriptions, campaign.channel])

    const form = useForm<CampaignReviewFormData>({
        resolver: zodResolver(campaignSchema),
        defaultValues: {
            name: campaign.name || "",
            provider_id: campaign.provider?.id,
            variables: campaign.variables ?? [],
        },
    })

    const config = channels[campaign.channel]
    if (!config) {
        return null
    }

    const channelForm = config.form(campaign, template)
    const { ContentPreview } = config

    const onSubmit = async (data: CampaignReviewFormData) => {
        if (!project) return

        setIsSubmitting(true)
        try {
            const { data: updatedCampaign } = await oapiClient.PATCH(
                "/api/admin/projects/{projectID}/campaigns/{campaignID}",
                {
                    params: {
                        path: { projectID: project.id, campaignID: campaign.id },
                    },
                    body: {
                        name: data.name,
                        ...(data.provider_id ? { provider_id: data.provider_id } : {}),
                        transactional: isTransactional,
                        subscription_id: isTransactional ? undefined : subscriptionId || undefined,
                        variables: data.variables.filter((v) => v.name),
                    },
                },
            )

            if (updatedCampaign) {
                setCampaign((prevCampaign) => ({
                    ...prevCampaign,
                    name: updatedCampaign.name,
                    updated_at: updatedCampaign.updated_at,
                    delivery: updatedCampaign.delivery,
                    subscription_id: updatedCampaign.subscription_id,
                    transactional: updatedCampaign.transactional,
                    provider: updatedCampaign.provider
                        ? {
                              ...prevCampaign.provider,
                              ...updatedCampaign.provider,
                              data:
                                  updatedCampaign.provider.data ??
                                  prevCampaign.provider?.data ??
                                  {},
                              setup: prevCampaign.provider?.setup ?? [],
                          }
                        : undefined,
                    variables: updatedCampaign.variables ?? [],
                }))
            }
        } finally {
            setIsSubmitting(false)
        }
    }

    return (
        <CampaignVariableProvider>
            <TemplateContext.Provider value={templateState}>
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
                                    name="provider_id"
                                    control={form.control}
                                    render={({ field, fieldState }) => (
                                        <Field data-invalid={fieldState.invalid} className="gap-2">
                                            <FieldLabel htmlFor="provider">
                                                {t("campaign.setup.form.provider.label")}
                                            </FieldLabel>
                                            <ProviderSelect
                                                value={field.value}
                                                onChange={field.onChange}
                                                channel={campaign.channel}
                                            />
                                            <FieldDescription className="whitespace-pre-line">
                                                {t("campaign.setup.form.provider.description")}
                                            </FieldDescription>
                                            {fieldState.invalid && (
                                                <FieldError errors={[fieldState.error]} />
                                            )}
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
                                        <Select
                                            value={subscriptionId}
                                            onValueChange={setSubscriptionId}
                                        >
                                            <SelectTrigger id="subscription-select">
                                                <SelectValue
                                                    placeholder={t(
                                                        "campaign.subscription.placeholder",
                                                        "Select subscription",
                                                    )}
                                                />
                                            </SelectTrigger>
                                            <SelectContent className="z-[1100]">
                                                {filteredSubscriptions.length === 0 && (
                                                    <SelectItem value="__empty" disabled>
                                                        {t(
                                                            "campaign.subscription.empty",
                                                            "No subscriptions for this channel",
                                                        )}
                                                    </SelectItem>
                                                )}
                                                {filteredSubscriptions.map((subscription) => (
                                                    <SelectItem
                                                        key={subscription.id}
                                                        value={subscription.id}
                                                    >
                                                        {subscription.name}
                                                    </SelectItem>
                                                ))}
                                            </SelectContent>
                                        </Select>
                                    </Field>
                                </FieldGroup>
                            )}

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
                                            />
                                        </Field>
                                    )}
                                />
                            </FieldGroup>

                            <div>
                                <Button
                                    type="submit"
                                    disabled={isSubmitting}
                                    isLoading={isSubmitting}
                                >
                                    {t("actions.save")}
                                </Button>
                            </div>
                        </form>
                    </div>

                    <div className="w-3/5 bg-background p-8 pb-0 border-l">
                        <Tabs defaultValue="preview" className="h-full flex flex-col">
                            {/* <div>
                            <TabsList className="mb-2">
                                <TabsTrigger value="preview">Preview</TabsTrigger>
                            </TabsList>
                        </div> */}
                            <TabsContent value="preview" className="flex-1">
                                <ContentPreview campaign={campaign} form={channelForm} edit />
                            </TabsContent>
                        </Tabs>
                    </div>
                </div>
            </TemplateContext.Provider>
        </CampaignVariableProvider>
    )
}

export default function CampaignDetails() {
    const [campaign] = useContext(CampaignContext)
    const [project] = useContext(ProjectContext)
    const [template, setTemplate] = useState<Template | null>(null)

    useEffect(() => {
        if (!campaign || campaign.templates.length === 0) {
            return
        }

        const template =
            campaign.templates.find((template) => template.locale === project.locale) ??
            campaign.templates[0]
        setTemplate(template)
    }, [campaign, project.locale])

    if (!campaign || !project || !template) {
        return null
    }

    return <CampaignReview campaign={campaign} template={template} />
}
