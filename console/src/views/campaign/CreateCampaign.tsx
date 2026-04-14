import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { ProjectContext } from "@/contexts"
import { useCallback, useContext, useMemo, useState } from "react"
import type { ReactNode } from "react"
import { useResolver } from "@/hooks"

import { Button } from "@/components/ui/button"
import { ArrowRight, Mail, MessageSquareDot, PlusIcon, Smartphone } from "lucide-react"
import type { ChannelType } from "@/types"
import type { components } from "@/oapi/management.generated"

import {
    Item,
    ItemActions,
    ItemContent,
    ItemDescription,
    ItemGroup,
    ItemMedia,
    ItemTitle,
} from "@/components/ui/item"

import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { oapiClient } from "@/oapi/client"

type Subscription = components["schemas"]["Subscription"]

interface Channel {
    key: ChannelType
    color: string
    icon: ReactNode
    title: string
    description: string
}

interface CreateCampaignProps {
    open?: boolean
    onBeforeCreate?: () => Promise<void>
    trigger?: React.ReactNode
}

export function CreateCampaign({ open = false, onBeforeCreate, trigger }: CreateCampaignProps) {
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()
    const [isOpen, setIsOpen] = useState(open)
    const [selectedChannel, setSelectedChannel] = useState<ChannelType | null>(null)
    const [transactional, setTransactional] = useState(false)
    const [subscriptionId, setSubscriptionId] = useState<string>("")

    const [subscriptions] = useResolver(
        useCallback(async (): Promise<Subscription[]> => {
            if (!project?.id) return []

            const response = await oapiClient.GET("/api/admin/projects/{projectID}/subscriptions", {
                params: {
                    path: {
                        projectID: project.id,
                    },
                    query: {
                        limit: 100,
                    },
                },
            })

            if (response.error || !response.data?.results) {
                return []
            }

            return response.data.results
        }, [project?.id]),
    )

    const filteredSubscriptions = useMemo(() => {
        if (!selectedChannel) return []
        return (subscriptions ?? []).filter(
            (subscription) => subscription.channel === selectedChannel,
        )
    }, [selectedChannel, subscriptions])

    const subscriptionsLoading = subscriptions === null
    async function create() {
        if (!project?.id || !selectedChannel) {
            return
        }

        if (onBeforeCreate) {
            await onBeforeCreate()
        }

        const body: {
            name: string
            channel: components["schemas"]["Channel"]
            transactional?: boolean
            subscription_id?: string
        } = {
            name: generateProjectName(),
            channel: selectedChannel,
            transactional,
        }

        if (transactional) {
            body.subscription_id = undefined
        } else if (subscriptionId) {
            body.subscription_id = subscriptionId
        }

        const campaign = await oapiClient.POST("/api/admin/projects/{projectID}/campaigns", {
            params: {
                path: {
                    projectID: project.id,
                },
            },
            body,
        })

        if (campaign.data?.id) {
            const template = await api.campaigns.templates.create(project.id, campaign.data.id, {
                locale: project.locale,
                data: {},
            })

            navigate(
                `/projects/${project.id}/campaigns/${campaign.data.id}/templates/${template.id}`,
            )
        }
    }

    const channels: Array<Channel> = [
        {
            key: "email",
            color: "bg-green-50 text-green-600",
            icon: <Mail strokeWidth={2} />,
            title: t("channels.email.title"),
            description: t("channels.email.description"),
        },
        {
            key: "sms",
            color: "bg-blue-50 text-blue-600",
            icon: <Smartphone strokeWidth={2} />,
            title: t("channels.sms.title"),
            description: t("channels.sms.description"),
        },
        {
            key: "push",
            color: "bg-purple-50 text-purple-600",
            icon: <MessageSquareDot strokeWidth={2} />,
            title: t("channels.push.title"),
            description: t("channels.push.description"),
        },
    ]

    return (
        <Dialog open={isOpen} onOpenChange={setIsOpen}>
            <DialogTrigger asChild>
                {trigger ?? (
                    <Button size="lg">
                        <PlusIcon /> {t("campaign.create.action")}
                    </Button>
                )}
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t("campaign.create.title")}</DialogTitle>
                    <DialogDescription>{t("campaign.create.description")}</DialogDescription>
                </DialogHeader>

                <ItemGroup className="gap-2">
                    {channels.map((channel) => (
                        <Item key={channel.key} variant="outline" className="items-center" asChild>
                            <button
                                type="button"
                                className="cursor-pointer text-left"
                                onClick={() => {
                                    setSelectedChannel(channel.key)
                                    setSubscriptionId("")
                                }}
                            >
                                <ItemMedia variant="icon" className={channel.color}>
                                    {channel.icon}
                                </ItemMedia>
                                <ItemContent>
                                    <ItemTitle>{channel.title}</ItemTitle>
                                    <ItemDescription>{channel.description}</ItemDescription>
                                </ItemContent>
                                <ItemActions>
                                    <ArrowRight strokeWidth={1} />
                                </ItemActions>
                            </button>
                        </Item>
                    ))}
                </ItemGroup>

                {selectedChannel && (
                    <div className="mt-4 space-y-4">
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
                                checked={transactional}
                                onCheckedChange={(checked) => {
                                    setTransactional(checked)
                                    if (checked) setSubscriptionId("")
                                }}
                            />
                        </div>

                        {!transactional && (
                            <div className="space-y-2">
                                <Label htmlFor="subscription-select">
                                    {t("campaign.subscription", "Subscription")}
                                </Label>
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
                            </div>
                        )}

                        <Button onClick={() => create()}>{t("campaign.create.action")}</Button>
                    </div>
                )}
            </DialogContent>
        </Dialog>
    )
}

const adjectives = [
    "adaptive",
    "bold",
    "bright",
    "calm",
    "clear",
    "confident",
    "connected",
    "consistent",
    "conversational",
    "curated",
    "direct",
    "dynamic",
    "effective",
    "elegant",
    "engaging",
    "focused",
    "friendly",
    "impactful",
    "informative",
    "insightful",
    "intentional",
    "modern",
    "personal",
    "polished",
    "proactive",
    "relevant",
    "responsive",
    "smart",
    "smooth",
    "strategic",
    "targeted",
    "timely",
    "trusted",
    "unified",
    "useful",
    "warm",
]

const names = [
    "announcement",
    "beacon",
    "broadcast",
    "bulletin",
    "campaign",
    "cascade",
    "connect",
    "conversation",
    "dispatch",
    "engagement",
    "experience",
    "feature",
    "followup",
    "highlight",
    "insight",
    "invitation",
    "journey",
    "launch",
    "message",
    "moment",
    "nudge",
    "outreach",
    "pulse",
    "release",
    "reminder",
    "signal",
    "spotlight",
    "story",
    "touchpoint",
    "update",
    "wave",
]

function generateProjectName() {
    const adjective = adjectives[Math.floor(Math.random() * adjectives.length)]
    const name = names[Math.floor(Math.random() * names.length)]
    return `${adjective} ${name}`
}
