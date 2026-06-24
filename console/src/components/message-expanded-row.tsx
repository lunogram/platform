import React, { useContext, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { ProjectContext } from "../contexts"
import { PreferencesContext } from "@/contexts/PreferencesContext"
import { formatDate } from "../utils"
import oapiClient from "../oapi/client"
import type { components } from "../oapi/management.generated"
import Preview from "@/components/preview"
import { JsonView } from "@/components/ui/json-view"
import { Badge } from "@/components/ui/badge"
import { TableRow, TableCell } from "@/components/ui/table"
import { getChannelMeta } from "./inbox-channel-meta"

type InboxMessage = components["schemas"]["InboxMessage"]

export function MessageExpandedRow({ message }: { message: InboxMessage }) {
    const { t } = useTranslation()
    const [preferences] = useContext(PreferencesContext)
    const [project] = useContext(ProjectContext)
    const [senderName, setSenderName] = useState<string | undefined>(undefined)

    useEffect(() => {
        if (!message.sender_identity_id) return
        let cancelled = false
        oapiClient
            .GET("/api/admin/projects/{projectID}/sender-identities/{senderIdentityID}", {
                params: {
                    path: {
                        projectID: project.id,
                        senderIdentityID: message.sender_identity_id,
                    },
                },
            })
            .then(({ data }) => {
                if (cancelled) return
                const name =
                    typeof data?.traits?.name === "string"
                        ? data.traits.name
                        : typeof data?.traits?.address === "string"
                          ? data.traits.address
                          : undefined
                setSenderName(name)
            })
            .catch(() => {
                if (!cancelled) {
                    setSenderName(undefined)
                }
            })
        return () => {
            cancelled = true
        }
    }, [message.sender_identity_id, project.id])

    const channelMeta = getChannelMeta(message.channel, t)
    const hasData = message.data && Object.keys(message.data).length > 0

    const ChannelMetaIcon = channelMeta.icon

    const metaRows: { label: string; value: React.ReactNode; show: boolean }[] = [
        {
            label: t("message_id", "Message ID"),
            value: <code className="text-xs break-all">{message.id}</code>,
            show: true,
        },
        {
            label: t("channel", "Channel"),
            value: (
                <span className="inline-flex items-center gap-1.5 text-sm">
                    <ChannelMetaIcon className="h-3.5 w-3.5" aria-hidden="true" />
                    {channelMeta.label}
                </span>
            ),
            show: true,
        },
        {
            label: t("priority", "Priority"),
            value: <span className="text-sm">{message.priority}</span>,
            show: true,
        },
        {
            label: t("created_at", "Created"),
            value: (
                <span className="text-sm">
                    {formatDate(preferences, message.created_at, "PPpp")}
                </span>
            ),
            show: true,
        },
        {
            label: t("scheduled_at", "Scheduled"),
            value: (
                <span className="text-sm">
                    {message.scheduled_at
                        ? formatDate(preferences, message.scheduled_at, "PPpp")
                        : "—"}
                </span>
            ),
            show: !!message.scheduled_at,
        },
        {
            label: t("sent_at", "Sent"),
            value: (
                <span className="text-sm">
                    {message.sent_at ? formatDate(preferences, message.sent_at, "PPpp") : "—"}
                </span>
            ),
            show: !!message.sent_at,
        },
        {
            label: t("read_at", "Read"),
            value: (
                <span className="text-sm">
                    {message.read_at ? formatDate(preferences, message.read_at, "PPpp") : "—"}
                </span>
            ),
            show: !!message.read_at,
        },
        {
            label: t("archived_at", "Archived"),
            value: (
                <span className="text-sm">
                    {message.archived_at
                        ? formatDate(preferences, message.archived_at, "PPpp")
                        : "—"}
                </span>
            ),
            show: !!message.archived_at,
        },
        {
            label: t("expires_at", "Expires"),
            value: (
                <span className="text-sm">
                    {message.expires_at ? formatDate(preferences, message.expires_at, "PPpp") : "—"}
                </span>
            ),
            show: !!message.expires_at,
        },
        {
            label: t("sender_identity", "Sender identity"),
            value: <code className="text-xs break-all">{message.sender_identity_id}</code>,
            show: !!message.sender_identity_id,
        },
        {
            label: t("campaign", "Campaign"),
            value: <code className="text-xs break-all">{message.campaign_id}</code>,
            show: !!message.campaign_id,
        },
        {
            label: t("broadcast", "Broadcast"),
            value: <code className="text-xs break-all">{message.broadcast_id}</code>,
            show: !!message.broadcast_id,
        },
        {
            label: t("external_id", "External ID"),
            value: <code className="text-xs break-all">{message.external_id}</code>,
            show: !!message.external_id,
        },
        {
            label: t("source", "Source"),
            value: <span className="text-sm">{message.source}</span>,
            show: !!message.source,
        },
    ]

    const visibleMetaRows = metaRows.filter((row) => row.show)

    return (
        <TableRow className="bg-muted/30 hover:bg-muted/30">
            <TableCell colSpan={7} className="p-0 first:pl-0 last:pr-0">
                <div className="grid grid-cols-1 md:grid-cols-5">
                    {/* Left – metadata */}
                    <div className="md:col-span-2 space-y-4 px-6 py-5">
                        <div className="space-y-3">
                            {visibleMetaRows.map((row) => (
                                <div key={row.label} className="space-y-0.5">
                                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                        {row.label}
                                    </p>
                                    {row.value}
                                </div>
                            ))}
                        </div>

                        {message.tags.length > 0 && (
                            <div className="space-y-1">
                                <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                                    {t("tags", "Tags")}
                                </p>
                                <div className="flex flex-wrap gap-1">
                                    {message.tags.map((tag) => (
                                        <Badge key={tag} variant="outline">
                                            {tag}
                                        </Badge>
                                    ))}
                                </div>
                            </div>
                        )}

                        {hasData && (
                            <JsonView
                                data={message.data}
                                title={t("data", "Data")}
                                defaultExpanded
                            />
                        )}
                    </div>

                    {/* Right – message preview */}
                    <div className="md:col-span-3 min-h-[360px] h-full border-l">
                        <Preview
                            message={message}
                            senderName={senderName}
                            variant="inbox-detail"
                            className="flex w-full h-full overflow-auto"
                        />
                    </div>
                </div>
            </TableCell>
        </TableRow>
    )
}
