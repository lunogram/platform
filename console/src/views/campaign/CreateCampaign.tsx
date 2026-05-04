import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { ProjectContext } from "@/contexts"
import { useContext, useState } from "react"
import type { ReactNode } from "react"

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
import { Button } from "@/components/ui/button"

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
    const [selectingChannel, setSelectingChannel] = useState<ChannelType | null>(null)

    async function selectChannel(channel: ChannelType) {
        if (!project?.id || selectingChannel) {
            return
        }

        setSelectingChannel(channel)

        try {
            if (onBeforeCreate) {
                await onBeforeCreate()
            }

            setIsOpen(false)
            navigate(`/projects/${project.id}/campaigns/new/${channel}`)
        } finally {
            setSelectingChannel(null)
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
                                disabled={selectingChannel !== null}
                                onClick={() => void selectChannel(channel.key)}
                            >
                                <ItemMedia variant="icon" className={channel.color}>
                                    {channel.icon}
                                </ItemMedia>
                                <ItemContent>
                                    <ItemTitle>{channel.title}</ItemTitle>
                                    <ItemDescription>{channel.description}</ItemDescription>
                                </ItemContent>
                                <ItemActions>
                                    {selectingChannel === channel.key ? (
                                        <span className="text-sm text-muted-foreground">
                                            {t("loading", "Loading...")}
                                        </span>
                                    ) : (
                                        <ArrowRight strokeWidth={1} />
                                    )}
                                </ItemActions>
                            </button>
                        </Item>
                    ))}
                </ItemGroup>
            </DialogContent>
        </Dialog>
    )
}
