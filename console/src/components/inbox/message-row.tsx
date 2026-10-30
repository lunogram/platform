import { useContext } from "react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"
import {
    Archive,
    ArchiveRestore,
    CalendarClock,
    Check,
    ChevronDown,
    ChevronRight,
    EyeOff,
    Loader2,
    MoreHorizontal,
} from "lucide-react"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { formatDate, cn } from "@/utils"
import { MessageExpandedRow } from "@/components/message-expanded-row"
import { getChannelMeta } from "@/components/inbox-channel-meta"
import { failureBadgeClassName, getMessageFailure } from "@/components/inbox-failure-meta"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { DateTimeEdit } from "@/components/ui/datetime-edit"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { TableCell, TableRow } from "@/components/ui/table"
import type { InboxEvent, InboxMessage } from "./queries"

interface InboxMessageRowProps {
    message: InboxMessage
    isExpanded: boolean
    onToggle: () => void
    onUpdate: (event: Exclude<InboxEvent, "scheduled">) => void
    /** Rejects when the new time was not saved, so the editor stays open. */
    onReschedule: (scheduledAt: string) => Promise<void>
}

export function InboxMessageRow({
    message,
    isExpanded,
    onToggle,
    onUpdate,
    onReschedule,
}: InboxMessageRowProps) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const visible = isMessageVisible(message)
    const settled = !!message.failed_at
    const channelMeta = getChannelMeta(message.channel, t)
    const ChannelIcon = channelMeta.icon
    const messageTitle = typeof message.content?.title === "string" ? message.content.title : ""
    const messageBody = typeof message.content?.body === "string" ? message.content.body : ""

    return (
        <>
            <TableRow
                className={cn("cursor-pointer", isExpanded && "bg-muted/50")}
                onClick={onToggle}
            >
                <TableCell className="p-0 pl-3">
                    <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 w-8 p-0"
                        aria-expanded={isExpanded}
                        aria-label={t("toggle_message_details", "Toggle message details")}
                        onClick={(e) => {
                            e.stopPropagation()
                            onToggle()
                        }}
                    >
                        {isExpanded ? (
                            <ChevronDown
                                className="h-4 w-4 text-muted-foreground"
                                aria-hidden="true"
                            />
                        ) : (
                            <ChevronRight
                                className="h-4 w-4 text-muted-foreground"
                                aria-hidden="true"
                            />
                        )}
                    </Button>
                </TableCell>
                <TableCell>
                    <div className="space-y-1">
                        {messageTitle && <div className="font-medium">{messageTitle}</div>}
                        {messageBody && (
                            <div
                                className={cn(
                                    "line-clamp-2 max-w-xl text-sm",
                                    messageTitle ? "text-muted-foreground" : "font-medium",
                                )}
                            >
                                {messageBody}
                            </div>
                        )}
                        {!messageTitle && !messageBody && (
                            <span className="text-muted-foreground">—</span>
                        )}
                    </div>
                </TableCell>
                <TableCell>
                    <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
                        <ChannelIcon aria-hidden="true" className="h-3.5 w-3.5" />
                        {channelMeta.label}
                    </span>
                </TableCell>
                <TableCell>{statusBadge(message, t)}</TableCell>
                <TableCell>
                    {message.tags.length > 0 ? (
                        <div className="flex flex-wrap gap-1">
                            {message.tags.map((tag) => (
                                <Badge key={tag} variant="outline">
                                    {tag}
                                </Badge>
                            ))}
                        </div>
                    ) : (
                        <span className="text-muted-foreground">—</span>
                    )}
                </TableCell>
                <TableCell className="text-muted-foreground whitespace-nowrap">
                    {!settled && new Date(message.scheduled_at) > new Date() ? (
                        <DateTimeEdit
                            value={message.scheduled_at}
                            onSave={(newIso) => onReschedule(newIso)}
                        >
                            <span className="inline-flex items-center gap-1.5 text-sm">
                                {formatDate(preferences, message.scheduled_at, "Pp")}
                            </span>
                        </DateTimeEdit>
                    ) : (
                        <span className="inline-flex items-center gap-1.5 text-sm">
                            {formatDate(preferences, message.scheduled_at, "Pp")}
                        </span>
                    )}
                </TableCell>
                <TableCell>
                    <DropdownMenu modal={false}>
                        <DropdownMenuTrigger asChild>
                            <Button
                                variant="ghost"
                                size="sm"
                                className="h-8 w-8 p-0"
                                aria-label={t("open_menu", "Open menu")}
                                onClick={(e) => e.stopPropagation()}
                            >
                                <MoreHorizontal aria-hidden="true" className="h-4 w-4" />
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                            {!message.read_at && !message.archived_at && visible && !settled && (
                                <DropdownMenuItem onClick={() => onUpdate("opened")}>
                                    <Check aria-hidden="true" className="mr-2 h-4 w-4" />
                                    {t("mark_read", "Mark as read")}
                                </DropdownMenuItem>
                            )}
                            {!message.archived_at && visible && (
                                <DropdownMenuItem onClick={() => onUpdate("archived")}>
                                    <Archive aria-hidden="true" className="mr-2 h-4 w-4" />
                                    {t("archive", "Archive")}
                                </DropdownMenuItem>
                            )}
                            {message.archived_at && visible && (
                                <DropdownMenuItem onClick={() => onUpdate("unarchived")}>
                                    <ArchiveRestore aria-hidden="true" className="mr-2 h-4 w-4" />
                                    {t("unarchive", "Unarchive")}
                                </DropdownMenuItem>
                            )}
                            {message.read_at && !message.archived_at && visible && !settled && (
                                <DropdownMenuItem onClick={() => onUpdate("unread")}>
                                    <EyeOff aria-hidden="true" className="mr-2 h-4 w-4" />
                                    {t("mark_unread", "Mark unread")}
                                </DropdownMenuItem>
                            )}
                        </DropdownMenuContent>
                    </DropdownMenu>
                </TableCell>
            </TableRow>

            {/* Expanded Row */}
            {isExpanded && <MessageExpandedRow key={`${message.id}-expanded`} message={message} />}
        </>
    )
}

function statusBadge(message: InboxMessage, t: TFunction) {
    const failure = getMessageFailure(message, t)
    if (failure) {
        const FailureIcon = failure.icon
        return (
            <Tooltip>
                <TooltipTrigger asChild>
                    <div className="flex flex-col items-start gap-1">
                        <Badge
                            variant="outline"
                            className={cn("gap-1", failureBadgeClassName(failure.kind))}
                        >
                            <FailureIcon aria-hidden="true" className="h-3 w-3" />
                            {failure.label}
                        </Badge>
                        <span className="text-xs text-muted-foreground">{failure.reason}</span>
                    </div>
                </TooltipTrigger>
                <TooltipContent className="max-w-xs">{failure.description}</TooltipContent>
            </Tooltip>
        )
    }
    if (!isMessageVisible(message)) {
        return (
            <Badge variant="outline" className="gap-1">
                <CalendarClock aria-hidden="true" className="h-3 w-3" />
                {t("scheduled", "Scheduled")}
            </Badge>
        )
    }
    if (!message.sent_at && new Date(message.scheduled_at) <= new Date()) {
        return (
            <Tooltip>
                <TooltipTrigger asChild>
                    <Badge
                        variant="outline"
                        className="border-0 bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400 gap-1"
                    >
                        <Loader2 className="h-3 w-3 animate-spin" />
                        {t("sending", "Sending")}
                    </Badge>
                </TooltipTrigger>
                <TooltipContent>
                    {t(
                        "scheduled_sending_tooltip",
                        "This message is scheduled to be sent out. It could take up to 1 minute to process.",
                    )}
                </TooltipContent>
            </Tooltip>
        )
    }
    if (message.archived_at) {
        return (
            <Badge variant="outline" className="border-0 bg-muted text-muted-foreground">
                {t("archived", "Archived")}
            </Badge>
        )
    }
    if (message.read_at) {
        return <Badge variant="secondary">{t("opened", "Opened")}</Badge>
    }
    return <Badge>{t("unread", "Unread")}</Badge>
}

function isMessageVisible(message: InboxMessage) {
    return new Date(message.scheduled_at) <= new Date()
}
