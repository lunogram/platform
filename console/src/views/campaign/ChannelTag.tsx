import type { ChannelType } from "../../types"
import { EmailIcon, PushIcon, TextIcon } from "../../components/icons"
import { Badge, type BadgeProps } from "@/components/ui/badge"
import { useTranslation } from "react-i18next"

interface ChannelTagParams {
    channel: ChannelType
    showIcon?: boolean
}

export function ChannelIcon({ channel }: Pick<ChannelTagParams, "channel">) {
    const icons = {
        email: EmailIcon,
        text: TextIcon,
        push: PushIcon,
    }
    const Icon = icons[channel]
    return <Icon />
}

export default function ChannelTag({
    channel,
    showIcon = true,
    ...params
}: ChannelTagParams & BadgeProps) {
    const { t } = useTranslation()

    const title: Record<ChannelType, string> = {
        email: t("email"),
        text: t("text"),
        push: t("push"),
    }

    return (
        <Badge variant="secondary" {...params}>
            {showIcon && <ChannelIcon channel={channel} />}
            {title[channel]}
        </Badge>
    )
}
