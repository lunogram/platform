import { useState } from "react"
import { useParams } from "react-router"
import { useTranslation } from "react-i18next"
import { Users, FileText } from "lucide-react"

import type { User } from "@/types"
import type { UUID } from "@/types/common"

import { useBroadcastDetail } from "./hooks/useBroadcastDetail"

import { Skeleton } from "@/components/ui/skeleton"
import { NavTabs } from "@/components/ui/nav-tabs"

import { BroadcastDetailHeader } from "./BroadcastDetailHeader"
import { BroadcastProgressBar } from "./BroadcastProgressBar"
import { BroadcastScheduleBanner } from "./BroadcastScheduleBanner"
import { RecipientsPanel } from "./RecipientsPanel"
import { BroadcastMessagePreview } from "./BroadcastMessagePreview"

interface BroadcastDetailProps {
    broadcastId: UUID
}

function BroadcastDetailSkeleton() {
    return (
        <div className="flex flex-col min-h-full">
            <div className="border-b bg-card/50">
                <div className="p-4 sm:p-6">
                    <Skeleton className="h-4 w-40 mb-4" />
                    <div className="flex items-start gap-4">
                        <Skeleton className="h-14 w-14 rounded-xl" />
                        <div className="space-y-2">
                            <Skeleton className="h-6 w-48" />
                            <Skeleton className="h-4 w-72" />
                        </div>
                    </div>
                </div>
            </div>
            <div className="flex-1 p-4 sm:p-6 space-y-6">
                <Skeleton className="h-32 rounded-lg" />
                <Skeleton className="h-64 rounded-lg" />
            </div>
        </div>
    )
}

export default function BroadcastDetail({ broadcastId }: BroadcastDetailProps) {
    const { t } = useTranslation()

    const {
        broadcast,
        users,
        usersTotal,
        displayTotal,
        isPreview,

        usersOffset,
        usersSearch,
        usersPageSize,
        setUsersOffset,
        handleUsersSearch,

        streamedSent,
        streamedTotal,

        isSending,
        isCancelling,
        isScheduling,
        scheduleValue,
        setScheduleValue,

        handleSend,
        handleReschedule,
        handleSetSchedule,
        handleRemoveSchedule,
        handleCancel,
    } = useBroadcastDetail(broadcastId)

    // Tab state for mobile/tablet (< lg) — desktop shows both panels side-by-side
    const [mobileTab, setMobileTab] = useState<string>("recipients")
    const mobileTabs = [
        { key: "recipients", label: t("recipients", "Recipients"), icon: Users },
        { key: "preview", label: t("message_preview", "Message Preview"), icon: FileText },
    ]

    if (!broadcast) {
        return <BroadcastDetailSkeleton />
    }

    const isEditable = broadcast.state === "pending" || broadcast.state === "scheduled"

    return (
        <div className="flex flex-col min-h-full">
            {/* Header Section — with ambient mosaic background */}
            <BroadcastDetailHeader
                broadcast={broadcast}
                users={users}
                usersTotal={usersTotal}
                streamedSent={streamedSent}
                streamedTotal={streamedTotal}
                isSending={isSending}
                isCancelling={isCancelling}
                onSend={handleSend}
                onCancel={handleCancel}
            />

            {/* Progress Bar for Sending State */}
            {broadcast.state === "sending" && (
                <BroadcastProgressBar streamedSent={streamedSent} streamedTotal={streamedTotal} />
            )}

            {/* Schedule Banner — colored bar below header for editable broadcasts */}
            {isEditable && (
                <BroadcastScheduleBanner
                    scheduledAt={broadcast.scheduled_at}
                    scheduleValue={scheduleValue}
                    isScheduling={isScheduling}
                    onSetScheduleValue={setScheduleValue}
                    onReschedule={handleReschedule}
                    onSetSchedule={handleSetSchedule}
                    onRemoveSchedule={handleRemoveSchedule}
                />
            )}

            {/* Content Area */}
            <div className="flex-1 flex flex-col lg:flex-row overflow-hidden">
                {/* Mobile/Tablet tabs — hidden on lg+ where both panels are visible */}
                <div className="border-b lg:hidden">
                    <div className="px-4 sm:px-6">
                        <NavTabs tabs={mobileTabs} value={mobileTab} onChange={setMobileTab} />
                    </div>
                </div>

                {/* Left panel — Recipients */}
                <div
                    className={`flex-1 lg:w-1/2 overflow-y-auto p-4 sm:p-6 ${mobileTab !== "recipients" ? "hidden lg:block" : ""}`}
                >
                    <RecipientsPanel
                        broadcast={broadcast}
                        users={users}
                        displayTotal={displayTotal}
                        usersTotal={usersTotal}
                        isPreview={isPreview}
                        usersOffset={usersOffset}
                        usersSearch={usersSearch}
                        usersPageSize={usersPageSize}
                        hasSearchQuery={usersSearch.length > 0}
                        onUsersSearch={handleUsersSearch}
                        onSetUsersOffset={setUsersOffset}
                    />
                </div>

                {/* Right panel — Message Preview */}
                <div
                    className={`lg:w-1/2 lg:border-l overflow-y-auto p-4 sm:p-6 ${mobileTab !== "preview" ? "hidden lg:block" : ""}`}
                >
                    <BroadcastMessagePreview
                        campaignId={broadcast.campaign_id}
                        defaultUser={users?.[0] as User | undefined}
                    />
                </div>
            </div>
        </div>
    )
}

export function BroadcastDetailRoute() {
    const { broadcastId = "" } = useParams<{ broadcastId: string }>()
    return <BroadcastDetail broadcastId={broadcastId as UUID} />
}
