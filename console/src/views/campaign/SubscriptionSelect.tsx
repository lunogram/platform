import { useContext } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { ProjectContext } from "@/contexts"
import type { ChannelType } from "@/types"
import { Button } from "@/components/ui/button"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { subscriptionsQuery } from "./queries"

interface SubscriptionSelectProps {
    id?: string
    channel: ChannelType
    value: string
    onChange: (subscriptionId: string) => void
}

/**
 * Picks one of the project's subscriptions for a channel. A failed load says so
 * and offers a retry, instead of looking like there are no subscriptions.
 */
export function SubscriptionSelect({ id, channel, value, onChange }: SubscriptionSelectProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const subscriptions = useQuery(subscriptionsQuery(project.id))

    if (subscriptions.isError) {
        return (
            <div className="flex items-center justify-between gap-3 rounded-md border px-3 py-2">
                <span className="text-sm text-destructive">
                    {t("campaign.subscriptions_load_failed", "Couldn't load subscriptions.")}
                </span>
                <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => subscriptions.refetch()}
                >
                    {t("retry", "Retry")}
                </Button>
            </div>
        )
    }

    const options = (subscriptions.data ?? []).filter((s) => s.channel === channel)

    return (
        <Select value={value} onValueChange={onChange}>
            <SelectTrigger id={id}>
                <SelectValue
                    placeholder={t("campaign.subscription.placeholder", "Select subscription")}
                />
            </SelectTrigger>
            <SelectContent className="z-[1100]">
                {subscriptions.isPending && (
                    <SelectItem value="__loading" disabled>
                        {t("loading", "Loading...")}
                    </SelectItem>
                )}
                {!subscriptions.isPending && options.length === 0 && (
                    <SelectItem value="__empty" disabled>
                        {t("campaign.subscription.empty", "No subscriptions for this channel")}
                    </SelectItem>
                )}
                {options.map((subscription) => (
                    <SelectItem key={subscription.id} value={subscription.id}>
                        {subscription.name}
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    )
}
