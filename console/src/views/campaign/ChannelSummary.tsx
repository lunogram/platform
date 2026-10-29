import type { JSX } from "react"
import { useTranslation } from "react-i18next"
import { Inbox, Mail, MessageSquareDot, Smartphone } from "lucide-react"
import type { ChannelType } from "@/types"

interface ChannelDisplay {
    title: string
    description: string
    icon: JSX.Element
    colorClass: string
}

/** The channel a new campaign is being created for, with its icon and description. */
export function ChannelSummary({ channel }: { channel: ChannelType }) {
    const { t } = useTranslation()

    const display: Record<ChannelType, ChannelDisplay> = {
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
        inbox: {
            title: t("channels.inbox.title"),
            description: t("channels.inbox.description"),
            icon: <Inbox className="h-4 w-4" />,
            colorClass: "bg-amber-50 text-amber-600",
        },
    }
    const { title, description, icon, colorClass } = display[channel]

    return (
        <div className="flex items-start gap-3 rounded-md border p-3">
            <div
                className={`mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md ${colorClass}`}
            >
                {icon}
            </div>
            <div>
                <p className="font-medium">{title}</p>
                <p className="text-sm text-muted-foreground">{description}</p>
            </div>
        </div>
    )
}
