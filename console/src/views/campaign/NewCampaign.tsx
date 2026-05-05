import { useCallback, useContext, useEffect, useMemo, useState } from "react"
import { useNavigate, useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { Mail, MessageSquareDot, Smartphone } from "lucide-react"
import { toast } from "sonner"

import api from "@/api"
import { ProjectContext } from "@/contexts"
import { useResolver } from "@/hooks"
import { oapiClient } from "@/oapi/client"
import type { CampaignVariable, ChannelType, Subscription } from "@/types"

import { Button } from "@/components/ui/button"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"

import { CampaignVariables } from "./CampaignVariables"

type ChannelConfig = {
    title: string
    description: string
    icon: JSX.Element
    colorClass: string
}

function isChannelType(value?: string): value is ChannelType {
    return value === "email" || value === "sms" || value === "push"
}

export default function NewCampaign() {
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()
    const { channel: channelParam } = useParams()

    const [name, setName] = useState("")
    const [variables, setVariables] = useState<CampaignVariable[]>([])
    const [isTransactional, setTransactional] = useState(false)
    const [subscriptionId, setSubscriptionId] = useState("")
    const [isCreating, setIsCreating] = useState(false)

    const channel = isChannelType(channelParam) ? channelParam : null

    const [subscriptions] = useResolver(
        useCallback(async (): Promise<Subscription[]> => {
            if (!project?.id) return []
            const response = await api.subscriptions.search(project.id, { limit: 100 })
            return response.results ?? []
        }, [project?.id]),
    )

    const filteredSubscriptions = useMemo(() => {
        if (!subscriptions || !channel) return []
        return subscriptions.filter((subscription) => subscription.channel === channel)
    }, [subscriptions, channel])

    const subscriptionsLoading = subscriptions === null

    useEffect(() => {
        if (!project?.id || channel) {
            return
        }

        navigate(`/projects/${project.id}/campaigns/new`, { replace: true })
    }, [project?.id, channel, navigate])

    useEffect(() => {
        if (isTransactional || subscriptionsLoading || filteredSubscriptions.length === 0) {
            return
        }

        const hasValidSelection = filteredSubscriptions.some(
            (subscription) => subscription.id === subscriptionId,
        )

        if (!hasValidSelection) {
            setSubscriptionId(filteredSubscriptions[0].id)
        }
    }, [isTransactional, subscriptionsLoading, filteredSubscriptions, subscriptionId])

    const channelConfig = useMemo<ChannelConfig | null>(() => {
        if (!channel) {
            return null
        }

        const config: Record<ChannelType, ChannelConfig> = {
            email: {
                title: t("channels.email.title"),
                description: t("channels.email.description"),
                icon: <Mail className="h-4 w-4" />,
                colorClass: "bg-green-50 text-green-600",
            },
            sms: {
                title: t("channels.sms.title"),
                description: t("channels.sms.description"),
                icon: <Smartphone className="h-4 w-4" />,
                colorClass: "bg-blue-50 text-blue-600",
            },
            push: {
                title: t("channels.push.title"),
                description: t("channels.push.description"),
                icon: <MessageSquareDot className="h-4 w-4" />,
                colorClass: "bg-purple-50 text-purple-600",
            },
        }

        return config[channel]
    }, [channel, t])

    const hasName = name.trim().length > 0
    const hasSubscription = isTransactional || subscriptionId.length > 0
    const canCreate = !!project?.id && !!channel && hasName && hasSubscription && !isCreating

    async function createCampaign() {
        if (!project?.id || !channel || !hasName || (!isTransactional && !subscriptionId)) {
            return
        }

        setIsCreating(true)

        try {
            const createResult = await oapiClient.POST(
                "/api/admin/projects/{projectID}/campaigns",
                {
                    params: {
                        path: {
                            projectID: project.id,
                        },
                    },
                    body: {
                        name: name.trim(),
                        channel,
                        transactional: isTransactional,
                        subscription_id: isTransactional ? undefined : subscriptionId,
                    },
                },
            )

            if (createResult.error || !createResult.data?.id) {
                toast.error(t("campaign.create.error", "Failed to create campaign"))
                return
            }

            const campaignId = createResult.data.id
            const cleanedVariables = variables
                .map((variable) => ({
                    name: variable.name.trim(),
                    default: variable.default?.trim() || undefined,
                }))
                .filter((variable) => variable.name)

            if (cleanedVariables.length > 0) {
                await api.campaigns.update(project.id, campaignId, {
                    variables: cleanedVariables,
                })
            }

            const template = await api.campaigns.templates.create(project.id, campaignId, {
                locale: project.locale,
                data: {},
            })

            navigate(`/projects/${project.id}/campaigns/${campaignId}/templates/${template.id}`)
        } catch {
            toast.error(t("campaign.create.error", "Failed to create campaign"))
        } finally {
            setIsCreating(false)
        }
    }

    if (!project || !channel || !channelConfig) {
        return null
    }

    return (
        <div className="flex h-screen bg-muted/20 overflow-hidden">
            <div className="h-full overflow-y-auto w-full lg:w-2/5 bg-background p-8">
                <div className="mb-6">
                    <h1 className="text-2xl font-semibold">
                        {t("campaign.create.title", "New Campaign")}
                    </h1>
                    <p className="text-muted-foreground">
                        {t(
                            "campaign.create.setup.description",
                            "Configure campaign settings before creating your first template.",
                        )}
                    </p>
                </div>

                <form
                    className="space-y-6"
                    onSubmit={(event) => {
                        event.preventDefault()
                        void createCampaign()
                    }}
                >
                    <FieldGroup>
                        <Field className="gap-2">
                            <FieldLabel>{t("channel", "Channel")}</FieldLabel>
                            <div className="flex items-start gap-3 rounded-md border p-3">
                                <div
                                    className={`mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md ${channelConfig.colorClass}`}
                                >
                                    {channelConfig.icon}
                                </div>
                                <div>
                                    <p className="font-medium">{channelConfig.title}</p>
                                    <p className="text-sm text-muted-foreground">
                                        {channelConfig.description}
                                    </p>
                                </div>
                            </div>
                        </Field>
                    </FieldGroup>

                    <FieldGroup>
                        <Field className="gap-2">
                            <FieldLabel htmlFor="campaign-name">
                                {t("campaign.setup.form.name.label")}
                            </FieldLabel>
                            <Input
                                id="campaign-name"
                                value={name}
                                onChange={(event) => setName(event.target.value)}
                                placeholder={t(
                                    "campaign.setup.form.name.placeholder",
                                    "Campaign name",
                                )}
                            />
                        </Field>
                    </FieldGroup>

                    <FieldGroup>
                        <Field className="gap-2">
                            <FieldLabel>{t("campaign.variables.label", "Variables")}</FieldLabel>
                            <FieldDescription>
                                {t(
                                    "campaign.variables.description",
                                    "Define template variables that can be populated from journeys or the API.",
                                )}
                            </FieldDescription>
                            <CampaignVariables
                                variables={variables}
                                onChange={setVariables}
                                channel={channel}
                            />
                        </Field>
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
                                if (checked) {
                                    setSubscriptionId("")
                                }
                            }}
                        />
                    </div>

                    {!isTransactional && (
                        <FieldGroup>
                            <Field className="gap-2">
                                <FieldLabel htmlFor="subscription-select">
                                    {t("campaign.subscription", "Subscription")}
                                </FieldLabel>
                                <Select value={subscriptionId} onValueChange={setSubscriptionId}>
                                    <SelectTrigger id="subscription-select">
                                        <SelectValue
                                            placeholder={t(
                                                "campaign.subscription.placeholder",
                                                "Select subscription",
                                            )}
                                        />
                                    </SelectTrigger>
                                    <SelectContent className="z-[1100]">
                                        {subscriptionsLoading && (
                                            <SelectItem value="__loading" disabled>
                                                {t("loading", "Loading...")}
                                            </SelectItem>
                                        )}
                                        {!subscriptionsLoading &&
                                            filteredSubscriptions.length === 0 && (
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

                    <div className="flex items-center gap-2">
                        <Button
                            variant="outline"
                            type="button"
                            onClick={() => navigate(`/projects/${project.id}/campaigns`)}
                        >
                            {t("cancel", "Cancel")}
                        </Button>
                        <Button type="submit" disabled={!canCreate}>
                            {t("create", "Create")}
                        </Button>
                    </div>
                </form>
            </div>

            <div className="relative hidden lg:block w-3/5 border-l bg-background overflow-hidden">
                <div
                    className="pointer-events-none absolute"
                    aria-hidden="true"
                    style={{
                        top: "-150%",
                        left: "-50%",
                        right: "-50%",
                        bottom: "-150%",
                        transform: "rotate(12deg)",
                        columns: "280px",
                        columnGap: "24px",
                    }}
                >
                    {Array.from({ length: 80 }).map((_, i) => {
                        const heights = [
                            220, 340, 280, 400, 260, 360, 300, 240, 380, 320, 200, 440, 270, 350,
                            310, 230, 390, 250, 370, 290,
                        ]
                        return (
                            <div
                                key={i}
                                className="rounded-xl mb-6"
                                style={{
                                    height: heights[i % heights.length],
                                    breakInside: "avoid",
                                    backgroundColor: "oklch(0.93 0.005 260)",
                                }}
                            />
                        )
                    })}
                </div>
            </div>
        </div>
    )
}
