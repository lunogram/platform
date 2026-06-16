import { useCallback, useContext, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"
import {
    Archive,
    ArchiveRestore,
    Bell,
    CalendarClock,
    Check,
    ChevronLeft,
    ChevronRight,
    EyeOff,
    Inbox,
    Info,
    Mail,
    MessageSquare,
    MoreHorizontal,
    Plus,
    Search,
} from "lucide-react"
import { toast } from "sonner"
import { ProjectContext } from "../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { useResolver } from "../hooks"
import { formatDate, getPageNumbers } from "../utils"
import oapiClient from "../oapi/client"
import type { components } from "../oapi/management.generated"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { SenderIdentityCombobox } from "@/components/sender-identity-combobox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { DateTimeEdit } from "@/components/ui/datetime-edit"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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

type InboxMessage = components["schemas"]["InboxMessage"]
type InboxStatus = "all" | "unread" | "read" | "archived"
type InboxChannel = components["schemas"]["Channel"]

interface InboxDetailTableProps {
    subjectId: string
    subjectType: "users" | "organizations"
}

const limit = 15

export default function InboxDetailTable({ subjectId, subjectType }: InboxDetailTableProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [preferences] = useContext(PreferencesContext)
    const [page, setPage] = useState(1)
    const [status, setStatus] = useState<InboxStatus>("all")
    const [searchQuery, setSearchQuery] = useState("")
    const [debouncedQuery, setDebouncedQuery] = useState("")
    const [isCreateOpen, setIsCreateOpen] = useState(false)
    const [isCreating, setIsCreating] = useState(false)
    const [channel, setChannel] = useState<InboxChannel>("inbox")
    const [senderIdentityId, setSenderIdentityId] = useState("")
    const [title, setTitle] = useState("")
    const [body, setBody] = useState("")
    const [tags, setTags] = useState("")
    const [scheduledAt, setScheduledAt] = useState("")
    const searchTimeoutRef = useRef<ReturnType<typeof setTimeout>>()

    const handleSearch = (value: string) => {
        setSearchQuery(value)
        setPage(1)
        clearTimeout(searchTimeoutRef.current)
        searchTimeoutRef.current = setTimeout(() => {
            setDebouncedQuery(value)
        }, 300)
    }

    const resetForm = () => {
        setChannel("inbox")
        setSenderIdentityId("")
        setTitle("")
        setBody("")
        setTags("")
        setScheduledAt("")
    }

    const [result, , reload] = useResolver(
        useCallback(async () => {
            const params = {
                limit,
                offset: (page - 1) * limit,
                search: debouncedQuery || undefined,
                status: status === "all" ? undefined : status,
                include_archived: status === "all" ? true : undefined,
                include_scheduled: true,
            }

            if (subjectType === "users") {
                const { data, error } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox",
                    {
                        params: {
                            path: { projectID: project.id, userID: subjectId },
                            query: params,
                        },
                    },
                )
                if (error) throw error
                return data ?? { results: [], total: 0 }
            }

            const { data, error } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox",
                {
                    params: {
                        path: { projectID: project.id, organizationID: subjectId },
                        query: params,
                    },
                },
            )
            if (error) throw error
            return data ?? { results: [], total: 0 }
        }, [page, project.id, debouncedQuery, status, subjectId, subjectType]),
    )

    const createMessage = async () => {
        if (!title.trim()) return
        if ((channel === "email" || channel === "sms") && !senderIdentityId) return

        setIsCreating(true)
        try {
            const dedupedTags = Array.from(
                new Set(
                    tags
                        .split(",")
                        .map((tag) => tag.trim())
                        .filter(Boolean),
                ),
            )

            const payload = {
                channel,
                sender_identity_id:
                    channel === "push" || channel === "inbox" ? undefined : senderIdentityId,
                content: {
                    title: title.trim(),
                    body: body.trim() || undefined,
                },
                tags: dedupedTags,
                scheduled_at: scheduledAt || undefined,
            }

            if (subjectType === "users") {
                const { error } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox",
                    {
                        params: { path: { projectID: project.id, userID: subjectId } },
                        body: payload,
                    },
                )
                if (error) throw error
            } else {
                const { error } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox",
                    {
                        params: { path: { projectID: project.id, organizationID: subjectId } },
                        body: payload,
                    },
                )
                if (error) throw error
            }

            toast.success(t("inbox_message_created", "Inbox message created"))
            resetForm()
            setIsCreateOpen(false)
            await reload()
        } catch {
            toast.error(t("inbox_message_create_failed", "Failed to create inbox message"))
        } finally {
            setIsCreating(false)
        }
    }

    const updateMessage = async (
        message: InboxMessage,
        event: "read" | "archived" | "scheduled" | "unarchived" | "unread",
        newScheduledAt?: string,
    ) => {
        try {
            if (subjectType === "users") {
                if (event === "scheduled") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/schedule",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    userID: subjectId,
                                    messageID: message.id,
                                },
                            },
                            body: { scheduled_at: newScheduledAt ?? "" },
                        },
                    )
                    if (error) throw error
                } else if (event === "read") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/read",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    userID: subjectId,
                                    messageID: message.id,
                                },
                            },
                        },
                    )
                    if (error) throw error
                } else if (event === "archived") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/archive",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    userID: subjectId,
                                    messageID: message.id,
                                },
                            },
                        },
                    )
                    if (error) throw error
                } else if (event === "unarchived") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/unarchive",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    userID: subjectId,
                                    messageID: message.id,
                                },
                            },
                        },
                    )
                    if (error) throw error
                } else if (event === "unread") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/users/{userID}/inbox/{messageID}/unread",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    userID: subjectId,
                                    messageID: message.id,
                                },
                            },
                        },
                    )
                    if (error) throw error
                }
            } else {
                if (event === "scheduled") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/schedule",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    organizationID: subjectId,
                                    messageID: message.id,
                                },
                            },
                            body: { scheduled_at: newScheduledAt ?? "" },
                        },
                    )
                    if (error) throw error
                } else if (event === "read") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/read",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    organizationID: subjectId,
                                    messageID: message.id,
                                },
                            },
                        },
                    )
                    if (error) throw error
                } else if (event === "archived") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/archive",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    organizationID: subjectId,
                                    messageID: message.id,
                                },
                            },
                        },
                    )
                    if (error) throw error
                } else if (event === "unarchived") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/unarchive",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    organizationID: subjectId,
                                    messageID: message.id,
                                },
                            },
                        },
                    )
                    if (error) throw error
                } else if (event === "unread") {
                    const { error } = await oapiClient.POST(
                        "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}/inbox/{messageID}/unread",
                        {
                            params: {
                                path: {
                                    projectID: project.id,
                                    organizationID: subjectId,
                                    messageID: message.id,
                                },
                            },
                        },
                    )
                    if (error) throw error
                }
            }

            toast.success(
                event === "scheduled"
                    ? t("inbox_message_scheduled", "Message schedule updated")
                    : event === "archived"
                      ? t("inbox_message_archived", "Message archived")
                      : event === "unarchived"
                        ? t("inbox_message_unarchived", "Message unarchived")
                        : event === "unread"
                          ? t("inbox_message_unread", "Message marked unread")
                          : t("inbox_message_read", "Message marked read"),
            )
            await reload()
        } catch {
            toast.error(t("inbox_message_update_failed", "Failed to update inbox message"))
        }
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
                            <TableHead>{t("message", "Message")}</TableHead>
                            <TableHead>{t("channel", "Channel")}</TableHead>
                            <TableHead>{t("status", "Status")}</TableHead>
                            <TableHead>{t("tags", "Tags")}</TableHead>
                            <TableHead>{t("scheduled", "Scheduled")}</TableHead>
                            <TableHead className="w-10" />
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {result === null ? (
                            Array.from({ length: 5 }).map((_, index) => (
                                <TableRow key={index}>
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
                        ) : result.results.length === 0 ? (
                            <TableRow>
                                <TableCell colSpan={6} className="h-48">
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
                            result.results.map((message) => {
                                const visible = isMessageVisible(message)
                                const channelMeta = getChannelMeta(message.channel, t)
                                const ChannelIcon = channelMeta.icon
                                const messageTitle =
                                    typeof message.content?.title === "string"
                                        ? message.content.title
                                        : ""
                                const messageBody =
                                    typeof message.content?.body === "string"
                                        ? message.content.body
                                        : ""

                                return (
                                    <TableRow key={message.id}>
                                        <TableCell>
                                            <div className="space-y-1">
                                                <div className="font-medium">{messageTitle}</div>
                                                {messageBody && (
                                                    <div className="line-clamp-2 max-w-xl text-sm text-muted-foreground">
                                                        {messageBody}
                                                    </div>
                                                )}
                                            </div>
                                        </TableCell>
                                        <TableCell>
                                            <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
                                                <ChannelIcon
                                                    aria-hidden="true"
                                                    className="h-3.5 w-3.5"
                                                />
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
                                            {new Date(message.scheduled_at) > new Date() ? (
                                                <DateTimeEdit
                                                    value={message.scheduled_at}
                                                    onSave={(newIso) =>
                                                        updateMessage(message, "scheduled", newIso)
                                                    }
                                                >
                                                    <span className="inline-flex items-center gap-1.5 text-sm">
                                                        <CalendarClock
                                                            aria-hidden="true"
                                                            className="h-3.5 w-3.5"
                                                        />
                                                        {formatDate(
                                                            preferences,
                                                            message.scheduled_at,
                                                            "Pp",
                                                        )}
                                                    </span>
                                                </DateTimeEdit>
                                            ) : (
                                                <span className="inline-flex items-center gap-1.5 text-sm">
                                                    <CalendarClock
                                                        aria-hidden="true"
                                                        className="h-3.5 w-3.5"
                                                    />
                                                    {formatDate(
                                                        preferences,
                                                        message.scheduled_at,
                                                        "Pp",
                                                    )}
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
                                                    >
                                                        <MoreHorizontal
                                                            aria-hidden="true"
                                                            className="h-4 w-4"
                                                        />
                                                    </Button>
                                                </DropdownMenuTrigger>
                                                <DropdownMenuContent align="end">
                                                    {!message.read_at &&
                                                        !message.archived_at &&
                                                        visible && (
                                                            <DropdownMenuItem
                                                                onClick={() =>
                                                                    updateMessage(
                                                                        message,
                                                                        "read",
                                                                    ).catch(console.error)
                                                                }
                                                            >
                                                                <Check
                                                                    aria-hidden="true"
                                                                    className="mr-2 h-4 w-4"
                                                                />
                                                                {t("mark_read", "Mark read")}
                                                            </DropdownMenuItem>
                                                        )}
                                                    {!message.archived_at && visible && (
                                                        <DropdownMenuItem
                                                            onClick={() =>
                                                                updateMessage(
                                                                    message,
                                                                    "archived",
                                                                ).catch(console.error)
                                                            }
                                                        >
                                                            <Archive
                                                                aria-hidden="true"
                                                                className="mr-2 h-4 w-4"
                                                            />
                                                            {t("archive", "Archive")}
                                                        </DropdownMenuItem>
                                                    )}
                                                    {message.archived_at && visible && (
                                                        <DropdownMenuItem
                                                            onClick={() =>
                                                                updateMessage(
                                                                    message,
                                                                    "unarchived",
                                                                ).catch(console.error)
                                                            }
                                                        >
                                                            <ArchiveRestore
                                                                aria-hidden="true"
                                                                className="mr-2 h-4 w-4"
                                                            />
                                                            {t("unarchive", "Unarchive")}
                                                        </DropdownMenuItem>
                                                    )}
                                                    {message.read_at &&
                                                        !message.archived_at &&
                                                        visible && (
                                                            <DropdownMenuItem
                                                                onClick={() =>
                                                                    updateMessage(
                                                                        message,
                                                                        "unread",
                                                                    ).catch(console.error)
                                                                }
                                                            >
                                                                <EyeOff
                                                                    aria-hidden="true"
                                                                    className="mr-2 h-4 w-4"
                                                                />
                                                                {t("mark_unread", "Mark unread")}
                                                            </DropdownMenuItem>
                                                        )}
                                                </DropdownMenuContent>
                                            </DropdownMenu>
                                        </TableCell>
                                    </TableRow>
                                )
                            })
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

            {/* Create Inbox Message Dialog */}
            <Dialog
                open={isCreateOpen}
                onOpenChange={(open) => {
                    setIsCreateOpen(open)
                    if (!open) resetForm()
                }}
            >
                <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle>{t("new_inbox_message", "New inbox message")}</DialogTitle>
                        <DialogDescription>
                            {t(
                                "new_inbox_message_description",
                                "Create a message that appears in this subject inbox.",
                            )}
                        </DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid sm:grid-cols-2 gap-4">
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="inbox-channel">{t("channel", "Channel")} *</Label>
                                <Select
                                    value={channel}
                                    onValueChange={(value) => {
                                        setChannel(value as InboxChannel)
                                        setSenderIdentityId("")
                                    }}
                                >
                                    <SelectTrigger id="inbox-channel">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="inbox">{t("inbox", "Inbox")}</SelectItem>
                                        <SelectItem value="email">{t("email", "Email")}</SelectItem>
                                        <SelectItem value="sms">{t("sms", "SMS")}</SelectItem>
                                        <SelectItem value="push">{t("push", "Push")}</SelectItem>
                                    </SelectContent>
                                </Select>
                                <p className="text-xs text-muted-foreground">
                                    {t(
                                        "inbox_channel_help",
                                        "Delivery channel the message represents.",
                                    )}
                                </p>
                            </div>
                            <div className="grid gap-2 content-start">
                                {channel === "push" || channel === "inbox" ? (
                                    <>
                                        <Label>{t("sender", "Sender")}</Label>
                                        <div className="flex items-start gap-2 rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">
                                            <Info
                                                aria-hidden="true"
                                                className="mt-0.5 h-4 w-4 shrink-0"
                                            />
                                            <span>
                                                {channel === "inbox"
                                                    ? t(
                                                          "inbox_no_sender_needed",
                                                          "Inbox messages do not require a sender identity.",
                                                      )
                                                    : t(
                                                          "push_uses_project_settings",
                                                          "Push messages use the project's push provider settings.",
                                                      )}
                                            </span>
                                        </div>
                                    </>
                                ) : (
                                    <>
                                        <Label htmlFor="inbox-sender-identity">
                                            {channel === "email"
                                                ? t("from_address", "From address")
                                                : t("from_number", "From number")}{" "}
                                            *
                                        </Label>
                                        <SenderIdentityCombobox
                                            projectId={project.id}
                                            channel={channel === "email" ? "email" : "sms"}
                                            value={senderIdentityId}
                                            onChange={setSenderIdentityId}
                                            placeholder={
                                                channel === "email"
                                                    ? t(
                                                          "select_from_address",
                                                          "Select from address...",
                                                      )
                                                    : t(
                                                          "select_from_number",
                                                          "Select from number...",
                                                      )
                                            }
                                        />
                                        <p className="text-xs text-muted-foreground">
                                            {channel === "email"
                                                ? t(
                                                      "inbox_from_address_help",
                                                      "Verified sender to use as the from address.",
                                                  )
                                                : t(
                                                      "inbox_from_number_help",
                                                      "Verified sender number to use.",
                                                  )}
                                        </p>
                                    </>
                                )}
                            </div>
                        </div>

                        <div className="grid gap-2">
                            <Label htmlFor="inbox-title">
                                {channel === "email"
                                    ? t("subject", "Subject")
                                    : t("title", "Title")}{" "}
                                *
                            </Label>
                            <Input
                                id="inbox-title"
                                value={title}
                                onChange={(event) => setTitle(event.target.value)}
                                placeholder={
                                    channel === "email"
                                        ? t("inbox_subject_placeholder", "Message subject")
                                        : t("inbox_title_placeholder", "Message title")
                                }
                            />
                        </div>

                        <div className="grid gap-2">
                            <Label htmlFor="inbox-body">{t("body", "Body")}</Label>
                            <Textarea
                                id="inbox-body"
                                value={body}
                                onChange={(event) => setBody(event.target.value)}
                                rows={4}
                                placeholder={t(
                                    "inbox_body_placeholder",
                                    "Message body shown to the recipient",
                                )}
                            />
                        </div>

                        <div className="grid sm:grid-cols-2 gap-4">
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="inbox-tags">{t("tags", "Tags")}</Label>
                                <Input
                                    id="inbox-tags"
                                    value={tags}
                                    onChange={(event) => setTags(event.target.value)}
                                    placeholder={t("inbox_tags_placeholder", "billing, onboarding")}
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t(
                                        "inbox_tags_help",
                                        "Comma separated. Duplicates are removed.",
                                    )}
                                </p>
                            </div>
                            <div className="grid gap-2 content-start">
                                <Label htmlFor="inbox-scheduled-at">
                                    {t("scheduled_at", "Scheduled at")}
                                </Label>
                                <Input
                                    id="inbox-scheduled-at"
                                    type="datetime-local"
                                    value={scheduledAt}
                                    onChange={(event) =>
                                        setScheduledAt(
                                            event.target.value
                                                ? new Date(event.target.value).toISOString()
                                                : "",
                                        )
                                    }
                                />
                                <p className="text-xs text-muted-foreground">
                                    {t(
                                        "inbox_scheduled_at_hint",
                                        "Leave empty to publish the inbox message immediately.",
                                    )}
                                </p>
                            </div>
                        </div>
                    </div>
                    <DialogFooter>
                        <Button
                            variant="ghost"
                            onClick={() => {
                                setIsCreateOpen(false)
                                resetForm()
                            }}
                            disabled={isCreating}
                        >
                            {t("cancel", "Cancel")}
                        </Button>
                        <Button
                            onClick={createMessage}
                            disabled={
                                isCreating ||
                                !title.trim() ||
                                ((channel === "email" || channel === "sms") && !senderIdentityId)
                            }
                        >
                            {isCreating ? t("creating", "Creating...") : t("create", "Create")}
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    )
}

function statusBadge(message: InboxMessage, t: TFunction) {
    if (!isMessageVisible(message)) {
        return (
            <Badge variant="outline" className="gap-1">
                <CalendarClock aria-hidden="true" className="h-3 w-3" />
                {t("scheduled", "Scheduled")}
            </Badge>
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
        return <Badge variant="secondary">{t("read", "Read")}</Badge>
    }
    return <Badge>{t("unread", "Unread")}</Badge>
}

function getChannelMeta(channel: InboxChannel, t: TFunction) {
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

function isMessageVisible(message: InboxMessage) {
    return new Date(message.scheduled_at) <= new Date()
}
