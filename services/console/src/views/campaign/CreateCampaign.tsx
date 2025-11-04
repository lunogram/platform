import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { ProjectContext } from "@/contexts"
import { useContext } from "react"

import { Button } from "@/components/ui/button"
import { ArrowRight, Mail, MessageSquareDot, PlusIcon, Smartphone, Webhook } from "lucide-react"

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

export function CreateCampaign() {
    const [project] = useContext(ProjectContext)
    const navigate = useNavigate()
    const { t } = useTranslation()

    async function create(channel: string) {
        console.log(channel)
        console.log(`/projects/${project.id}/campaigns/00000000-0000-0000-0000-000000000000`)
        await navigate(`/projects/${project.id}/campaigns/00000000-0000-0000-0000-000000000000`)
    }

    const channels = [
        {
            key: 'email',
            color: 'bg-green-50 text-green-600',
            icon: <Mail strokeWidth={2} />,
            title: t('channels.email.title'),
            description: t('channels.email.description'),
        },
        {
            key: 'sms',
            color: 'bg-blue-50 text-blue-600',
            icon: <Smartphone strokeWidth={2} />,
            title: t('channels.sms.title'),
            description: t('channels.sms.description'),
        },
        {
            key: 'push',
            color: 'bg-purple-50 text-purple-600',
            icon: <MessageSquareDot strokeWidth={2} />,
            title: t('channels.push.title'),
            description: t('channels.push.description'),
        },
        {
            key: 'webhook',
            color: 'bg-yellow-50 text-yellow-600',
            icon: <Webhook strokeWidth={2} />,
            title: t('channels.webhook.title'),
            description: t('channels.webhook.description'),
        },
    ]

    return (
        <Dialog>
            <DialogTrigger>
                <Button size="lg"><PlusIcon /> {t('campaign.create.action')}</Button>
            </DialogTrigger>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t('campaign.create.title')}</DialogTitle>
                    <DialogDescription>
                        {t('campaign.create.description')}
                    </DialogDescription>
                </DialogHeader>

                <ItemGroup className="gap-2">
                    {channels.map((channel) => (
                        <Item key={channel.key} variant="outline" className="items-center" asChild>
                            <a className="no-underline cursor-pointer" onClick={() => create(channel.key)}>
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
