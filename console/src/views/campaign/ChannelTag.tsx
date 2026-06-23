import type { ChannelType } from "../../types"
import { Mail, Smartphone, MessageSquareDot, Inbox } from "lucide-react"
import { Badge, type BadgeProps } from "@/components/ui/badge"
import { useTranslation } from "react-i18next"

interface ChannelTagParams {
    channel: ChannelType
    showIcon?: boolean
}

const channelIcons: Record<ChannelType, typeof Mail> = {
    email: Mail,
    sms: Smartphone,
    push: MessageSquareDot,
    inbox: Inbox,
}

export function ChannelIcon({
    channel,
    className = "h-4 w-4",
}: Pick<ChannelTagParams, "channel"> & { className?: string }) {
    const Icon = channelIcons[channel]
    return <Icon className={className} />
}

export default function ChannelTag({
    channel,
    showIcon = true,
    ...params
}: ChannelTagParams & BadgeProps) {
    const { t } = useTranslation()

    const title: Record<ChannelType, string> = {
        email: t("email"),
        sms: t("sms"),
        push: t("push"),
        inbox: t("inbox"),
    }

    return (
        <Badge variant="secondary" className="gap-1" {...params}>
            {showIcon && <ChannelIcon channel={channel} className="h-3.5 w-3.5" />}
            {title[channel]}
        </Badge>
    )
}
