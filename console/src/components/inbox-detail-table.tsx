import { useCallback, useContext, useRef, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import { ChevronLeft, ChevronRight, Inbox, Plus, Search } from "lucide-react"
import { toast } from "sonner"
import { ProjectContext } from "../contexts"
import { getPageNumbers } from "../utils"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table"
import { CreateInboxMessageDialog } from "./inbox/create-dialog"
import { InboxMessageRow } from "./inbox/message-row"
import {
    INBOX_PAGE_SIZE,
    inboxListQuery,
    useUpdateInboxMessage,
    type InboxEvent,
    type InboxMessage,
    type InboxStatus,
} from "./inbox/queries"

interface InboxDetailTableProps {
    subjectId: string
    subjectType: "users" | "organizations"
}

const limit = INBOX_PAGE_SIZE

export default function InboxDetailTable({ subjectId, subjectType }: InboxDetailTableProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [page, setPage] = useState(1)
    const [status, setStatus] = useState<InboxStatus>("all")
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [expandedMessageId, setExpandedMessageId] = useState<string | null>(null)
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

    const toggleExpanded = useCallback((messageId: string) => {
        setExpandedMessageId((prev) => (prev === messageId ? null : messageId))
    }, [])

    const handleSearch = (value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }

    // Keyed by page, search and status, so a slow response for an old search
    // or filter can't replace the current one.
    const list = useQuery(
        inboxListQuery(project.id, subjectType, subjectId, {
            page,
            search: debouncedQuery,
            status,
        }),
    )
    const result = list.data

    const update = useUpdateInboxMessage(subjectType, subjectId)

    const successMessage = (event: InboxEvent) =>
        event === "scheduled"
            ? t("inbox_message_scheduled", "Message schedule updated")
            : event === "archived"
              ? t("inbox_message_archived", "Message archived")
              : event === "unarchived"
                ? t("inbox_message_unarchived", "Message unarchived")
                : event === "unread"
                  ? t("inbox_message_unread", "Message marked unread")
                  : t("inbox_message_opened", "Message marked opened")

    const updateMessage = (message: InboxMessage, event: Exclude<InboxEvent, "scheduled">) =>
        update.mutate(
            { messageId: message.id, event },
            { onSuccess: () => toast.success(successMessage(event)) },
        )

    const rescheduleMessage = async (message: InboxMessage, scheduledAt: string) => {
        await update.mutateAsync({ messageId: message.id, event: "scheduled", scheduledAt })
        toast.success(successMessage("scheduled"))
    }

    const total = result?.total ?? 0
    const totalPages = result ? Math.ceil(total / limit) : 0
    const hasPrevPage = page > 1
    const hasNextPage = page < totalPages

    return (
        <div className="space-y-4">
            {/* Search + Status + Create */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 sm:gap-4">
                <div className="relative sm:max-w-sm flex-1">
                    <Label htmlFor="inbox-search" className="sr-only">
                        {t("search_inbox", "Search inbox")}
                    </Label>
                    <Search
                        aria-hidden="true"
                        className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                    />
                    <Input
                        id="inbox-search"
                        value={searchQuery}
                        onChange={(event) => handleSearch(event.target.value)}
                        placeholder={t("search_inbox", "Search inbox")}
                        className="pl-9"
                    />
                </div>
                <div className="flex items-center gap-2 sm:gap-3">
                    <Label htmlFor="inbox-status" className="sr-only">
                        {t("status", "Status")}
                    </Label>
                    <Select
                        value={status}
                        onValueChange={(value) => {
                            setStatus(value as InboxStatus)
                            setPage(1)
                        }}
                    >
                        <SelectTrigger id="inbox-status" className="w-[140px]">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">{t("all", "All")}</SelectItem>
                            <SelectItem value="unread">{t("unread", "Unread")}</SelectItem>
                            <SelectItem value="read">{t("read", "Read")}</SelectItem>
                            <SelectItem value="archived">{t("archived", "Archived")}</SelectItem>
                            <SelectItem value="failed">
                                {t("inbox_status_not_delivered", "Not delivered")}
                            </SelectItem>
                        </SelectContent>
                    </Select>
                    <Button
                        onClick={() => setIsCreateOpen(true)}
                        className="flex-1 sm:flex-initial"
                    >
                        <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
                        {t("new_message", "New message")}
                    </Button>
                </div>
            </div>

            {/* Inbox Table */}
            <div className="border rounded-lg">
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead className="w-8 p-0" />
                            <TableHead>{t("message", "Message")}</TableHead>
                            <TableHead>{t("channel", "Channel")}</TableHead>
                            <TableHead>{t("status", "Status")}</TableHead>
                            <TableHead>{t("tags", "Tags")}</TableHead>
                            <TableHead>{t("scheduled", "Scheduled")}</TableHead>
                            <TableHead className="w-10" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {list.isPending ? (
                            Array.from({ length: 5 }).map((_, index) => (
                                <TableRow key={index}>
                                    <TableCell className="p-0 pl-2">
                                        <Skeleton className="h-4 w-4" />
                                    </TableCell>
                                    <TableCell>
                                        <div className="space-y-2">
                                            <Skeleton className="h-4 w-48" />
                                            <Skeleton className="h-3 w-64" />
                                        </div>
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-5 w-16" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-5 w-20" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-5 w-24" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-28" />
                                    </TableCell>
                                    <TableCell>
                                        <Skeleton className="h-4 w-4" />
                                    </TableCell>
                                </TableRow>
                            ))
                        ) : list.isError || !result ? (
                            <TableRow>
                                <TableCell colSpan={7} className="h-32 text-center">
                                    <div className="flex flex-col items-center gap-3 text-muted-foreground">
                                        <p>
                                            {t(
                                                "inbox_load_failed",
                                                "Couldn't load inbox messages.",
                                            )}
                                        </p>
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            onClick={() => list.refetch()}
                                        >
                                            {t("retry", "Retry")}
                                        </Button>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : result.results.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={7} className="h-48">
                                    <div className="flex flex-col items-center justify-center">
                                        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-4">
                                            <Inbox
                                                aria-hidden="true"
                                                className="h-6 w-6 text-muted-foreground"
                                            />
                                        </div>
                                        <p className="font-medium mb-1">
                                            {t("no_inbox_messages", "No inbox messages yet")}
                                        </p>
                                        <p className="text-sm text-muted-foreground max-w-xs text-center">
                                            {t(
                                                "no_inbox_messages_description",
                                                "Inbox messages will appear here when they are created",
                                            )}
                                        </p>
                                    </div>
                                </TableCell>
                            </TableRow>
                        ) : (
                            result.results.map((message) => (
                                <InboxMessageRow
                                    key={message.id}
                                    message={message}
                                    isExpanded={expandedMessageId === message.id}
                                    onToggle={() => toggleExpanded(message.id)}
                                    onUpdate={(event) => updateMessage(message, event)}
                                    onReschedule={(newIso) => rescheduleMessage(message, newIso)}
                                />
                            ))
                        )}
                    </TableBody>
                </Table>

                {/* Pagination */}
                <div className="flex items-center justify-between border-t px-4 py-3">
                    <p className="text-sm text-muted-foreground">
                        {total} {t("messages", "messages")}
                    </p>
                    {totalPages > 1 && (
                        <div className="flex items-center gap-1">
                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => setPage((p) => p - 1)}
                                disabled={!hasPrevPage}
                                className="h-8 w-8 p-0"
                                aria-label={t("previous_page", "Previous page")}
                            >
                                <ChevronLeft aria-hidden="true" className="h-4 w-4" />
                            </Button>

                            {getPageNumbers(page, totalPages).map((pageNum, idx) =>
                                pageNum === "..." ? (
                                    <span
                                        key={`ellipsis-${idx}`}
                                        className="px-1 text-muted-foreground"
                                    >
                                        ...
                                    </span>
                                ) : (
                                    <Button
                                        key={pageNum}
                                        variant={page === pageNum ? "default" : "ghost"}
                                        size="sm"
                                        onClick={() => setPage(pageNum as number)}
                                        className="h-8 w-8 p-0"
                                    >
                                        {pageNum}
                                    </Button>
                                ),
                            )}

                            <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => setPage((p) => p + 1)}
                                disabled={!hasNextPage}
                                className="h-8 w-8 p-0"
                                aria-label={t("next_page", "Next page")}
                            >
                                <ChevronRight aria-hidden="true" className="h-4 w-4" />
                            </Button>
                        </div>
                    )}
                </div>
            </div>

            <CreateInboxMessageDialog
                open={isCreateOpen}
                onOpenChange={setIsCreateOpen}
                subjectType={subjectType}
                subjectId={subjectId}
            />
        </div>
    )
}
