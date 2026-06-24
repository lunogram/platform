import { useContext } from "react"
import { Link } from "react-router"
import { useTranslation } from "react-i18next"
import { Radio, ChevronRight, RefreshCw, Ban, Play } from "lucide-react"

import { formatDate, snakeToTitle } from "@/utils"
import { getRandomColor } from "@/lib/colors"
import { ProjectContext } from "@/contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"

import type { Broadcast } from "@/types"

import { channelIcons, getStateBadge } from "./broadcast-state"
import type { RecipientRow } from "./broadcast-state"
import { BroadcastMosaic } from "./BroadcastMosaic"

import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

interface BroadcastDetailHeaderProps {
    broadcast: Broadcast
    users: RecipientRow[] | null
    usersTotal: number | null
    streamedSent: number | null
    streamedTotal: number | null
    isSending: boolean
    isCancelling: boolean
    onSend: () => void
    onCancel: () => void
}

export function BroadcastDetailHeader({
    broadcast,
    users,
    usersTotal,
    streamedSent,
    streamedTotal,
    isSending,
    isCancelling,
    onSend,
    onCancel,
}: BroadcastDetailHeaderProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [preferences] = useContext(PreferencesContext)

    const campaignName = broadcast.campaign?.name ?? broadcast.campaign_id
    const campaignColor = getRandomColor(campaignName ?? broadcast.id)
    const ChannelIcon = broadcast.campaign?.channel
        ? (channelIcons[broadcast.campaign.channel] ?? Radio)
        : Radio
    const isEditable = broadcast.state === "pending" || broadcast.state === "scheduled"

    return (
        <div className="border-b bg-card/50 relative overflow-hidden">
            {/* Ambient mosaic — faded right-side background */}
            <div
                className="absolute top-1/2 -translate-y-1/2 left-[50%] xl:left-[30%] right-0 hidden lg:block pointer-events-none opacity-[0.8]"
                style={{
                    maskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                    WebkitMaskImage: "linear-gradient(to right, transparent 0%, black 40%)",
                }}
            >
                <BroadcastMosaic color={campaignColor} users={users} />
            </div>

            <div className="p-4 sm:p-6 relative z-20">
                {/* Breadcrumb */}
                <nav className="flex items-center gap-1.5 text-sm text-muted-foreground mb-4">
                    <Link
                        to={`/projects/${project.id}/broadcasts`}
                        className="hover:text-foreground transition-colors"
                    >
                        {t("broadcasts", "Broadcasts")}
                    </Link>
                    <ChevronRight className="h-3.5 w-3.5" />
                    <span className="text-foreground font-medium">
                        {broadcast.campaign?.name ?? broadcast.id}
                    </span>
                </nav>

                {/* Broadcast Identity */}
                <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-4 sm:gap-6">
                    <div className="flex items-start gap-4 min-w-0">
                        <div
                            className="flex h-14 w-14 items-center justify-center rounded-xl shrink-0"
                            style={{ backgroundColor: campaignColor }}
                        >
                            <ChannelIcon className="h-7 w-7 text-white" />
                        </div>
                        <div className="space-y-1 min-w-0">
                            <div className="flex items-center gap-3">
                                <h1 className="text-2xl font-semibold tracking-tight">
                                    <Link
                                        to={`/projects/${project.id}/campaigns/${broadcast.campaign_id}`}
                                        className="hover:underline"
                                    >
                                        {broadcast.campaign?.name ?? "—"}
                                    </Link>
                                </h1>
                                {getStateBadge(broadcast.state, t)}
                            </div>
                            <p className="text-sm text-muted-foreground flex items-center flex-wrap gap-x-2 gap-y-1">
                                {broadcast.campaign?.channel && (
                                    <>
                                        <span className="inline-flex items-center gap-1">
                                            <ChannelIcon className="h-3 w-3" />
                                            {snakeToTitle(broadcast.campaign.channel)}
                                        </span>
                                        <span>·</span>
                                    </>
                                )}
                                <Link
                                    to={`/projects/${project.id}/lists/${broadcast.list_id}`}
                                    className="hover:text-foreground transition-colors hover:underline"
                                >
                                    {broadcast.list_name || "—"}
                                </Link>
                                <span>·</span>
                                <span>
                                    {streamedSent != null
                                        ? `${streamedSent.toLocaleString()}${streamedTotal != null && streamedTotal > 0 ? ` / ${streamedTotal.toLocaleString()}` : ""} ${t("sent", "sent")}`
                                        : broadcast.sent > 0
                                          ? `${broadcast.sent.toLocaleString()} / ${broadcast.total.toLocaleString()} ${t("sent", "sent")}`
                                          : usersTotal != null
                                            ? `${usersTotal.toLocaleString()} ${t("recipients", "recipients")}`
                                            : t("no_recipients", "No recipients")}
                                </span>
                                <span>·</span>
                                <span>
                                    {broadcast.completed_at
                                        ? `${t("completed", "Completed")} ${formatDate(preferences, broadcast.completed_at, "PP")}`
                                        : broadcast.started_at
                                          ? `${t("started", "Started")} ${formatDate(preferences, broadcast.started_at, "PP")}`
                                          : `${t("created")} ${formatDate(preferences, broadcast.created_at, "PP")}`}
                                </span>
                            </p>
                        </div>
                    </div>

                    {/* Action buttons for editable broadcasts */}
                    {isEditable && (
                        <div className="flex items-center gap-2 shrink-0">
                            <Button
                                variant="destructive"
                                size="sm"
                                onClick={onCancel}
                                disabled={isCancelling}
                            >
                                {isCancelling ? (
                                    <RefreshCw className="mr-2 h-3.5 w-3.5 animate-spin" />
                                ) : (
                                    <Ban className="mr-2 h-3.5 w-3.5" />
                                )}
                                {t("cancel_broadcast", "Cancel")}
                            </Button>
                            <Tooltip>
                                <TooltipTrigger asChild>
                                    <div>
                                        <Button
                                            onClick={onSend}
                                            disabled={isSending || broadcast.state === "scheduled"}
                                            size="sm"
                                        >
                                            {isSending ? (
                                                <RefreshCw className="mr-2 h-3.5 w-3.5 animate-spin" />
                                            ) : (
                                                <Play className="mr-2 h-3.5 w-3.5" />
                                            )}
                                            {t("send_now", "Send Now")}
                                        </Button>
                                    </div>
                                </TooltipTrigger>
                                {broadcast.state === "scheduled" && (
                                    <TooltipContent>
                                        {t(
                                            "send_now_disabled_scheduled",
                                            "Remove the schedule to send manually",
                                        )}
                                    </TooltipContent>
                                )}
                            </Tooltip>
                        </div>
                    )}
                </div>
            </div>
        </div>
    )
}
