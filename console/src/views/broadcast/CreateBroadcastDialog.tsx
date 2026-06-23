import { useCallback, useContext, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Radio, Mail, Smartphone, MessageSquareDot, Info, Calendar } from "lucide-react"

import { toast } from "sonner"

import api from "../../api"
import oapiClient from "../../oapi/client"
import { useResolver } from "../../hooks"
import { ProjectContext } from "../../contexts"

import type { Broadcast, Campaign, List, ChannelType } from "@/types"
import type { UUID } from "@/types/common"

import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Switch } from "@/components/ui/switch"
import { Input } from "@/components/ui/input"
import {
    DEFAULT_TIME_INPUT_VALUE,
    dateInputValueFromDate,
    parseDateAndTime,
    toIsoFromDateAndTime,
} from "@/lib/date-time"

const channelIcons: Record<ChannelType, typeof Mail> = {
    email: Mail,
    sms: Smartphone,
    push: MessageSquareDot,
}

interface CreateBroadcastDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    campaignId?: UUID
    listId?: UUID
    /** Pass the full list object to ensure display even if the search API returns stale data */
    list?: List
    onCreated?: (broadcast: Broadcast) => void
}

export function CreateBroadcastDialog({
    open,
    onOpenChange,
    campaignId: preselectedCampaignId,
    listId: preselectedListId,
    list: preselectedList,
    onCreated,
}: CreateBroadcastDialogProps) {
    const [project] = useContext(ProjectContext)
    const { t } = useTranslation()

    const [selectedCampaignId, setSelectedCampaignId] = useState<UUID | undefined>(
        preselectedCampaignId,
    )
    const [selectedListId, setSelectedListId] = useState<UUID | undefined>(preselectedListId)
    const [isSubmitting, setIsSubmitting] = useState(false)
    const [isScheduled, setIsScheduled] = useState(false)
    const [scheduledDate, setScheduledDate] = useState("")
    const [scheduledTime, setScheduledTime] = useState(DEFAULT_TIME_INPUT_VALUE)

    // Sync pre-selected props when they change (useState only uses initial value)
    useEffect(() => {
        if (preselectedCampaignId) setSelectedCampaignId(preselectedCampaignId)
    }, [preselectedCampaignId])

    useEffect(() => {
        if (preselectedListId) setSelectedListId(preselectedListId)
    }, [preselectedListId])

    // Load campaigns
    const [campaignsResult] = useResolver(
        useCallback(async () => {
            return await api.campaigns.search(project.id, { limit: 100 })
        }, [project.id]),
    )

    // Load lists (only ready/published)
    const [listsResult] = useResolver(
        useCallback(async () => {
            return await api.lists.search(project.id, { limit: 100 })
        }, [project.id]),
    )

    const campaigns = campaignsResult?.results ?? []
    const fetchedLists = (listsResult?.results ?? []).filter((l: List) => l.state === "ready")

    // If a pre-selected list was passed but isn't in the fetched results (e.g. just published),
    // merge it in so the Select component can render its name.
    const lists =
        preselectedList &&
        preselectedList.state === "ready" &&
        !fetchedLists.some((l: List) => l.id === preselectedList.id)
            ? [preselectedList, ...fetchedLists]
            : fetchedLists

    const selectedCampaign = campaigns.find((c: Campaign) => c.id === selectedCampaignId)
    const selectedList = lists.find((l: List) => l.id === selectedListId)
    const selectedListCount = selectedList?.users_count ?? 0
    const selectedListCountLabel = selectedListCount.toLocaleString()

    const canSubmit =
        !!selectedCampaignId &&
        !!selectedListId &&
        !isSubmitting &&
        (!isScheduled || !!scheduledDate)

    const handleScheduledDateChange = (nextDate: string) => {
        setScheduledDate(nextDate)
        if (!nextDate) {
            setScheduledTime(DEFAULT_TIME_INPUT_VALUE)
        }
    }

    const handleSubmit = async () => {
        if (!selectedCampaignId || !selectedListId) return

        const scheduledAtIso = toIsoFromDateAndTime(scheduledDate, scheduledTime)

        // Validate scheduled time is in the future
        if (isScheduled && scheduledDate) {
            const scheduled = parseDateAndTime(scheduledDate, scheduledTime)
            if (!scheduled || scheduled <= new Date()) {
                toast.error(
                    t("scheduled_at_must_be_future", "Scheduled time must be in the future"),
                )
                return
            }
        }

        setIsSubmitting(true)
        try {
            const { data: broadcast, error } = await oapiClient.POST(
                "/api/admin/projects/{projectID}/broadcasts",
                {
                    params: {
                        path: { projectID: project.id },
                    },
                    body: {
                        campaign_id: selectedCampaignId,
                        list_id: selectedListId,
                        ...(isScheduled && scheduledAtIso
                            ? {
                                  scheduled_at: scheduledAtIso,
                              }
                            : {}),
                    },
                },
            )
            if (error) throw error
            onCreated?.(broadcast as Broadcast)
        } catch (err) {
            const detail =
                err instanceof Object && "detail" in err && typeof err.detail === "string"
                    ? err.detail
                    : null
            toast.error(detail || t("broadcast_create_error", "Failed to create broadcast"))
        } finally {
            setIsSubmitting(false)
        }
    }

    const handleOpenChange = (value: boolean) => {
        if (!value) {
            // Reset state on close (only if not pre-selected)
            if (!preselectedCampaignId) setSelectedCampaignId(undefined)
            if (!preselectedListId) setSelectedListId(undefined)
            setIsScheduled(false)
            setScheduledDate("")
            setScheduledTime(DEFAULT_TIME_INPUT_VALUE)
        }
        onOpenChange(value)
    }

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t("send_broadcast", "Send Broadcast")}</DialogTitle>
                    <DialogDescription>
                        {t("send_broadcast_description", "Send a campaign to all users in a list.")}
                    </DialogDescription>
                </DialogHeader>

                <div className="grid gap-4 py-2">
                    {/* Campaign Selector */}
                    <div className="grid gap-2">
                        <Label>{t("campaign.singular", "Campaign")}</Label>
                        <Select
                            value={selectedCampaignId}
                            onValueChange={(v) => setSelectedCampaignId(v as UUID)}
                            disabled={!!preselectedCampaignId}
                        >
                            <SelectTrigger>
                                <SelectValue
                                    placeholder={t("select_campaign", "Select a campaign...")}
                                />
                            </SelectTrigger>
                            <SelectContent>
                                {campaigns.length === 0 ? (
                                    <div className="px-2 py-4 text-center text-sm text-muted-foreground">
                                        {t(
                                            "no_campaigns_available",
                                            "No campaigns available. Create a campaign first.",
                                        )}
                                    </div>
                                ) : (
                                    campaigns.map((campaign: Campaign) => {
                                        const ChannelIcon = channelIcons[campaign.channel] ?? Mail
                                        return (
                                            <SelectItem key={campaign.id} value={campaign.id}>
                                                <div className="flex items-center gap-2">
                                                    <ChannelIcon className="h-3.5 w-3.5 text-muted-foreground" />
                                                    <span>{campaign.name}</span>
                                                </div>
                                            </SelectItem>
                                        )
                                    })
                                )}
                            </SelectContent>
                        </Select>
                    </div>

                    {/* List Selector */}
                    <div className="grid gap-2">
                        <Label>{t("list", "List")}</Label>
                        <Select
                            value={selectedListId}
                            onValueChange={(v) => setSelectedListId(v as UUID)}
                            disabled={!!preselectedListId}
                        >
                            <SelectTrigger>
                                <SelectValue placeholder={t("select_list", "Select a list...")} />
                            </SelectTrigger>
                            <SelectContent>
                                {lists.length === 0 ? (
                                    <div className="px-2 py-4 text-center text-sm text-muted-foreground">
                                        {t(
                                            "no_lists_available",
                                            "No published lists available. Publish a list first.",
                                        )}
                                    </div>
                                ) : (
                                    lists.map((list: List) => (
                                        <SelectItem key={list.id} value={list.id}>
                                            <div className="flex items-center gap-2">
                                                <span>{list.name}</span>
                                                <span className="text-muted-foreground text-xs">
                                                    ({list.users_count?.toLocaleString() ?? 0}{" "}
                                                    {t("users").toLowerCase()})
                                                </span>
                                            </div>
                                        </SelectItem>
                                    ))
                                )}
                            </SelectContent>
                        </Select>
                    </div>

                    {/* Schedule Toggle */}
                    <div className="grid gap-3">
                        <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                                <Calendar className="h-4 w-4 text-muted-foreground" />
                                <Label htmlFor="schedule-toggle">
                                    {t("schedule_broadcast", "Schedule for later")}
                                </Label>
                            </div>
                            <Switch
                                id="schedule-toggle"
                                checked={isScheduled}
                                onCheckedChange={setIsScheduled}
                            />
                        </div>
                        {isScheduled && (
                            <div className="flex gap-2">
                                <Input
                                    type="date"
                                    value={scheduledDate}
                                    onChange={(e) => handleScheduledDateChange(e.target.value)}
                                    min={dateInputValueFromDate(new Date(Date.now() + 60_000))}
                                />
                                <Input
                                    type="time"
                                    value={scheduledTime}
                                    onChange={(e) => setScheduledTime(e.target.value)}
                                    disabled={!scheduledDate}
                                />
                            </div>
                        )}
                    </div>

                    {/* Confirmation Info */}
                    {selectedCampaign && selectedList && (
                        <Alert className="border-blue-200 bg-blue-50/50 dark:border-blue-800 dark:bg-blue-950/20">
                            <Info className="h-4 w-4 !text-blue-600 dark:!text-blue-400" />
                            <AlertDescription className="text-blue-700 dark:text-blue-300">
                                {t(
                                    "broadcast_confirm_info",
                                    `This will send "${selectedCampaign.name}" to ${selectedListCountLabel} users in "${selectedList.name}".`,
                                    {
                                        campaign: selectedCampaign.name,
                                        count: selectedListCount,
                                        list: selectedList.name,
                                    },
                                )}
                            </AlertDescription>
                        </Alert>
                    )}
                </div>

                <DialogFooter>
                    <Button
                        variant="outline"
                        onClick={() => handleOpenChange(false)}
                        disabled={isSubmitting}
                    >
                        {t("cancel")}
                    </Button>
                    <Button onClick={handleSubmit} disabled={!canSubmit}>
                        {isSubmitting ? (
                            <Radio className="mr-2 h-4 w-4 animate-spin" />
                        ) : isScheduled ? (
                            <Calendar className="mr-2 h-4 w-4" />
                        ) : (
                            <Radio className="mr-2 h-4 w-4" />
                        )}
                        {isScheduled
                            ? t("schedule_broadcast_action", "Schedule Broadcast")
                            : t("send_broadcast", "Send Broadcast")}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
