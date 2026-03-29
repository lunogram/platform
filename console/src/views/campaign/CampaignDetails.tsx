import { useContext, useState, useEffect } from "react"
import { CampaignContext, ProjectContext, TemplateContext } from "@/contexts"
import type { Campaign, Template, User } from "@/types"
import { useTranslation } from "react-i18next"
import { Controller, useForm } from "react-hook-form"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import api from "@/api"
import { Radio } from "lucide-react"
import { isEnterprise } from "@/config/enterprise"

import { channels } from "./template/channels"
import { CampaignVariables } from "./CampaignVariables"

import { CampaignVariableProvider } from "./CampaignVariableContext"
import { Tabs, TabsContent } from "@/components/ui/tabs"
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { ProviderSelect } from "@/components/provider/select"
import { CreateBroadcastDialog } from "@/views/broadcast/CreateBroadcastDialog"

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
    const [isBroadcastOpen, setIsBroadcastOpen] = useState(false)

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
            const updatedCampaign = await api.campaigns.update(project.id, campaign.id, {
                name: data.name,
                ...(data.provider_id ? { provider_id: data.provider_id } : {}),
                variables: data.variables.filter((v) => v.name),
            })

            setCampaign(updatedCampaign)
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

                            <div className="flex items-center gap-2">
                                <Button
                                    type="submit"
                                    disabled={isSubmitting}
                                    isLoading={isSubmitting}
                                >
                                    {t("actions.save")}
                                </Button>
                                {isEnterprise && (
                                    <Tooltip>
                                        <TooltipTrigger asChild>
                                            <div>
                                                <Button
                                                    variant="outline"
                                                    onClick={() => setIsBroadcastOpen(true)}
                                                    disabled={!form.watch("provider_id")}
                                                >
                                                    <Radio className="mr-2 h-3.5 w-3.5" />
                                                    {t("send_broadcast", "Send Broadcast")}
                                                </Button>
                                            </div>
                                        </TooltipTrigger>
                                        {!form.watch("provider_id") && (
                                            <TooltipContent>
                                                {t(
                                                    "broadcast_requires_provider",
                                                    "Select an integration before sending a broadcast",
                                                )}
                                            </TooltipContent>
                                        )}
                                    </Tooltip>
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
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [template, setTemplate] = useState<Template | null>(null)

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
