import { useContext } from "react"
import { useTranslation } from "react-i18next"
import {
    Archive,
    ArchiveRestore,
    Copy,
    Inbox,
    Mail,
    MessageSquareDot,
    MoreHorizontal,
    Smartphone,
} from "lucide-react"
import type { components } from "@/oapi/management.generated"
import type { CampaignDelivery, ChannelType } from "@/types"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { getRandomColor } from "@/lib/colors"
import { formatDate, snakeToTitle } from "@/utils"
import { Button } from "@/components/ui/button"
import { TableCell, TableRow } from "@/components/ui/table"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

type CampaignListItem = components["schemas"]["Campaign"]

const channelIcons: Record<ChannelType, typeof Mail> = {
    email: Mail,
    sms: Smartphone,
    push: MessageSquareDot,
    inbox: Inbox,
}

function formatDelivery(delivery: CampaignDelivery) {
    const sent = delivery?.sent ?? 0
    const total = delivery?.total ?? 0
    const ratio = total > 0 ? sent / total : 0
    return `${sent.toLocaleString()} (${ratio.toLocaleString(undefined, { style: "percent", minimumFractionDigits: 0 })})`
}

interface CampaignRowProps {
    campaign: CampaignListItem
    /** Whether the row is shown in the archived list, which offers Unarchive instead of Archive. */
    archived: boolean
    onOpen: () => void
    onDuplicate: (e: React.MouseEvent) => void
    onArchive: (e: React.MouseEvent) => void
    onUnarchive: () => void
}

/** One campaign in the campaigns list, with its actions menu. */
export function CampaignRow({
    campaign,
    archived,
    onOpen,
    onDuplicate,
    onArchive,
    onUnarchive,
}: CampaignRowProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const campaignColor = getRandomColor(campaign.name ?? campaign.id)
    const ChannelIcon = channelIcons[campaign.channel] ?? Mail

    return (
        <TableRow className="cursor-pointer" onClick={onOpen}>
            <TableCell>
                <div className="flex items-center gap-3">
                    <div
                        className="flex h-8 w-8 items-center justify-center rounded-md shrink-0"
                        style={{ backgroundColor: campaignColor }}
                    >
                        <ChannelIcon className="h-4 w-4 text-white" />
                    </div>
                    <div>
                        <div className="font-medium">{campaign.name}</div>
                        <div className="text-sm text-muted-foreground">
                            {snakeToTitle(campaign.channel)}
                        </div>
                    </div>
                </div>
            </TableCell>
            <TableCell className="hidden sm:table-cell text-muted-foreground">
                {campaign.delivery?.sent > 0 ? formatDelivery(campaign.delivery) : "—"}
            </TableCell>
            <TableCell className="hidden md:table-cell text-muted-foreground">
                {formatDate(preferences, campaign.updated_at, "PP")}
            </TableCell>
            <TableCell>
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <Button
                            variant="ghost"
                            size="sm"
                            className="h-8 w-8 p-0"
                            onClick={(e) => e.stopPropagation()}
                        >
                            <MoreHorizontal className="h-4 w-4" />
                        </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                        <DropdownMenuItem
                            onClick={(e) => {
                                e.stopPropagation()
                                onOpen()
                            }}
                        >
                            <Mail className="mr-2 h-4 w-4" />
                            {t("edit")}
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={onDuplicate}>
                            <Copy className="mr-2 h-4 w-4" />
                            {t("duplicate")}
                        </DropdownMenuItem>
                        {archived ? (
                            <DropdownMenuItem
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onUnarchive()
                                }}
                            >
                                <ArchiveRestore className="mr-2 h-4 w-4" />
                                {t("unarchive", "Unarchive")}
                            </DropdownMenuItem>
                        ) : (
                            <DropdownMenuItem onClick={onArchive} className="text-destructive">
                                <Archive className="mr-2 h-4 w-4" />
                                {t("archive")}
                            </DropdownMenuItem>
                        )}
                    </DropdownMenuContent>
                </DropdownMenu>
            </TableCell>
        </TableRow>
    )
}
