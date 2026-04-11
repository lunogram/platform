import {
    Mail,
    Smartphone,
    MessageSquareDot,
    Clock,
    CheckCircle2,
    XCircle,
    CalendarClock,
    Ban,
    RefreshCw,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"

import type { BroadcastState, BroadcastUser, ChannelType } from "@/types"

/** Subset of user fields used by the recipient list (from list preview). */
export interface ListUser {
    id: string
    full_name?: string
    identifier?: Array<{
        source: string
        external_id: string
        metadata?: Record<string, unknown> | null
    }>
    email?: string
    phone?: string
}

/** Union type for rows in the recipients table. */
export type RecipientRow = ListUser | BroadcastUser

/** States where the broadcast has been sent (or attempted). */
const SENT_STATES: BroadcastState[] = ["sending", "completed", "failed", "cancelled"]

export function isSentState(state: BroadcastState): boolean {
    return SENT_STATES.includes(state)
}

export const channelIcons: Record<ChannelType, typeof Mail> = {
    email: Mail,
    text: Smartphone,
    push: MessageSquareDot,
}

export const stateConfig: Record<
    BroadcastState,
    { icon: typeof Clock; label: string; className: string }
> = {
    scheduled: {
        icon: CalendarClock,
        label: "Scheduled",
        className: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
    },
    pending: {
        icon: Clock,
        label: "Pending",
        className: "bg-secondary text-secondary-foreground",
    },
    sending: {
        icon: RefreshCw,
        label: "Sending",
        className: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
    },
    completed: {
        icon: CheckCircle2,
        label: "Sent",
        className: "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400",
    },
    failed: {
        icon: XCircle,
        label: "Failed",
        className: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
    },
    cancelled: {
        icon: Ban,
        label: "Cancelled",
        className: "bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400",
    },
}

export function getStateBadge(
    state: BroadcastState,
    t: (key: string, fallback?: string) => string,
) {
    const { label, className } = stateConfig[state] ?? stateConfig.pending
    return (
        <Badge variant="outline" className={`border-0 ${className}`}>
            {t(label.toLowerCase(), label)}
        </Badge>
    )
}
