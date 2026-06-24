import type { TFunction } from "i18next"
import { Bell, Inbox, Mail, MessageSquare } from "lucide-react"

import type { components } from "../oapi/management.generated"

type InboxChannel = components["schemas"]["Channel"]

export function getChannelMeta(channel: InboxChannel, t: TFunction) {
    switch (channel) {
        case "inbox":
            return { label: t("inbox", "Inbox"), icon: Inbox }
        case "email":
            return { label: t("email", "Email"), icon: Mail }
        case "sms":
            return { label: t("sms", "SMS"), icon: MessageSquare }
        case "push":
            return { label: t("push", "Push"), icon: Bell }
        default:
            return { label: channel, icon: Inbox }
    }
}
