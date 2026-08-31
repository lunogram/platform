import { useCallback, useContext, useEffect, useState } from "react"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Radio, Mail, Smartphone, MessageSquareDot, Inbox, Info, Calendar } from "lucide-react"

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
    createBroadcastSchema,
    type CreateBroadcastFormValues,
} from "@/validation/broadcast/create-broadcast"
import { broadcastResponseSchema } from "@/validation/broadcast/broadcast-response"

const channelIcons: Record<ChannelType, typeof Mail> = {
    email: Mail,
    sms: Smartphone,
    push: MessageSquareDot,
    inbox: Inbox,
}

import { VariantSelect } from "@/views/campaign/template/VariantSelect"
import { isEnterprise } from "@/config/enterprise"

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

    const [isSubmitting, setIsSubmitting] = useState(false)

    const form = useForm<CreateBroadcastFormValues>({
        resolver: zodResolver(createBroadcastSchema),
        defaultValues: {
            campaign_id: preselectedCampaignId ?? "",
            list_id: preselectedListId ?? "",
            is_scheduled: false,
            scheduled_at: "",
            variant: "",
        },
    })

    const selectedCampaignId = form.watch("campaign_id") || undefined
    const selectedListId = form.watch("list_id") || undefined
    const isScheduled = form.watch("is_scheduled")

    // Sync pre-selected props when they change
    useEffect(() => {
        if (preselectedCampaignId) form.setValue("campaign_id", preselectedCampaignId)
    }, [preselectedCampaignId, form])

    useEffect(() => {
        if (preselectedListId) form.setValue("list_id", preselectedListId)
    }, [preselectedListId, form])

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

    const handleSubmit = async (values: CreateBroadcastFormValues) => {
        if (!values.campaign_id || !values.list_id) return

        // Validate scheduled time is in the future
        if (values.is_scheduled && values.scheduled_at) {
            const scheduledDate = new Date(values.scheduled_at)
            if (scheduledDate <= new Date()) {
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
                        campaign_id: values.campaign_id,
                        list_id: values.list_id,
                        ...(values.is_scheduled && values.scheduled_at
                            ? { scheduled_at: new Date(values.scheduled_at).toISOString() }
                            : {}),
                        ...(values.variant ? { variant: values.variant } : {}),
                    },
                },
            )
            if (error) throw error
            if (!broadcast) throw new Error("Unexpected empty response")
            onCreated?.(broadcastResponseSchema.parse(broadcast))
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
            form.reset({
                campaign_id: preselectedCampaignId ?? "",
                list_id: preselectedListId ?? "",
                is_scheduled: false,
                scheduled_at: "",
            })
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
                        <Controller
                            control={form.control}
                            name="campaign_id"
                            render={({ field }) => (
                                <Select
                                    value={field.value}
                                    onValueChange={field.onChange}
                                    disabled={!!preselectedCampaignId}
                                >
                                    <SelectTrigger>
                                        <SelectValue
                                            placeholder={t(
                                                "select_campaign",
                                                "Select a campaign...",
                                            )}
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
                                                const ChannelIcon =
                                                    channelIcons[campaign.channel] ?? Mail
                                                return (
                                                    <SelectItem
                                                        key={campaign.id}
                                                        value={campaign.id}
                                                    >
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
                            )}
                        />
                        {form.formState.errors.campaign_id && (
                            <p className="text-sm text-destructive">
                                {form.formState.errors.campaign_id.message}
                            </p>
                        )}
                    </div>

                    {/* Variant Selector - only for campaigns that declare variants */}
                    {isEnterprise && (selectedCampaign?.variants?.length ?? 0) > 0 && (
                        <div className="grid gap-2">
                            <Label>{t("campaign.variants.title", "Variant")}</Label>
                            <Controller
                                control={form.control}
                                name="variant"
                                render={({ field }) => (
                                    <VariantSelect
                                        variants={selectedCampaign?.variants ?? []}
                                        value={field.value ?? ""}
                                        onChange={field.onChange}
                                    />
                                )}
                            />
                            <p className="text-xs text-muted-foreground">
                                {t(
                                    "broadcast.variant_description",
                                    "Sends every message under one design. Pick Default to let the campaign choose per recipient.",
                                )}
                            </p>
                        </div>
                    )}

                    {/* List Selector */}
                    <div className="grid gap-2">
                        <Label>{t("list", "List")}</Label>
                        <Controller
                            control={form.control}
                            name="list_id"
                            render={({ field }) => (
                                <Select
                                    value={field.value}
                                    onValueChange={field.onChange}
                                    disabled={!!preselectedListId}
                                >
                                    <SelectTrigger>
                                        <SelectValue
                                            placeholder={t("select_list", "Select a list...")}
                                        />
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
                                                            (
                                                            {list.users_count?.toLocaleString() ??
                                                                0}{" "}
                                                            {t("users").toLowerCase()})
                                                        </span>
                                                    </div>
                                                </SelectItem>
                                            ))
                                        )}
                                    </SelectContent>
                                </Select>
                            )}
                        />
                        {form.formState.errors.list_id && (
                            <p className="text-sm text-destructive">
                                {form.formState.errors.list_id.message}
                            </p>
                        )}
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
                            <Controller
                                control={form.control}
                                name="is_scheduled"
                                render={({ field }) => (
                                    <Switch
                                        id="schedule-toggle"
                                        checked={field.value}
                                        onCheckedChange={field.onChange}
                                    />
                                )}
                            />
                        </div>
                        {isScheduled && (
                            <Input
                                type="datetime-local"
                                {...form.register("scheduled_at")}
                                min={new Date(Date.now() + 60_000).toISOString().slice(0, 16)}
                            />
                        )}
                        {form.formState.errors.scheduled_at && (
                            <p className="text-sm text-destructive">
                                {form.formState.errors.scheduled_at.message}
                            </p>
                        )}
                    </div>

                    {/* Confirmation Info */}
                    {selectedCampaign && selectedList && (
                        <Alert className="border-blue-200 bg-blue-50/50 dark:border-blue-800 dark:bg-blue-950/20">
                            <Info className="h-4 w-4 !text-blue-600 dark:!text-blue-400" />
                            <AlertDescription className="text-blue-700 dark:text-blue-300">
                                {t(
                                    "broadcast_confirm_info",
                                    `This will send "${selectedCampaign.name}" to ${selectedList.users_count?.toLocaleString() ?? 0} users in "${selectedList.name}".`,
                                    {
                                        campaign: selectedCampaign.name,
                                        count: selectedList.users_count ?? 0,
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
                    <Button onClick={form.handleSubmit(handleSubmit)} disabled={isSubmitting}>
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
