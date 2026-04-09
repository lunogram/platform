import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { ProjectContext } from "@/contexts"
import { useContext, useState } from "react"

import { Button } from "@/components/ui/button"
import { ArrowRight, Mail, MessageSquareDot, PlusIcon, Smartphone } from "lucide-react"
import type { ChannelType } from "@/types"

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
import api from "@/api"

interface Channel {
    key: ChannelType
    color: string
    icon: JSX.Element
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

    async function create(channel: ChannelType) {
        if (onBeforeCreate) {
            await onBeforeCreate()
        }
        const campaign = await api.campaigns.create(project.id, {
            name: generateProjectName(),
            channel: channel,
        })

        // Create a default template and navigate directly to the template editor
        const template = await api.campaigns.templates.create(project.id, campaign.id, {
            locale: project.locale,
            data: {},
        })

        await navigate(
            `/projects/${project.id}/campaigns/${campaign.id}/templates/${template.id}`,
        )
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
        <Dialog open={isOpen} onOpenChange={() => setIsOpen(!isOpen)}>
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
                            <a
                                className="no-underline cursor-pointer"
                                onClick={() => create(channel.key)}
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
                            </a>
                        </Item>
                    ))}
                </ItemGroup>
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
