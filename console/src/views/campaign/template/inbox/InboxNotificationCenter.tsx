import { Inbox } from "lucide-react"
import { useTranslation } from "react-i18next"

interface InboxNotificationCenterProps {
    /** Rendered message title (variables already substituted). */
    title: string
    /** Rendered message body (variables already substituted). */
    body: string
    /** App / project name shown in the header. */
    appName?: string
}

// A faux in-app inbox / notification center. The campaign message renders as the
// top, unread item; the dimmed rows beneath it exist purely to convey that the
// message lands inside a list. Mirrors the message layout used on the user
// detail inbox tab (bold title, muted body) so previews stay consistent.
export function InboxNotificationCenter({ title, body, appName }: InboxNotificationCenterProps) {
    const { t } = useTranslation()
    const time = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })

    const placeholders = [
        {
            title: t("campaign.setup.channels.inbox.sample.title_1", "Welcome aboard"),
            body: t(
                "campaign.setup.channels.inbox.sample.body_1",
                "Thanks for joining. Here are a few tips to get you started.",
            ),
        },
        {
            title: t("campaign.setup.channels.inbox.sample.title_2", "Your weekly summary"),
            body: t(
                "campaign.setup.channels.inbox.sample.body_2",
                "A recap of everything that happened this week.",
            ),
        },
    ]

    return (
        <div className="w-full">
            <div className="overflow-hidden rounded-xl border bg-card shadow-sm">
                {/* Header */}
                <div className="flex items-center justify-between border-b px-4 py-3">
                    <div className="flex items-center gap-2">
                        <Inbox className="h-4 w-4 text-muted-foreground" />
                        <span className="text-sm font-semibold text-foreground">
                            {appName || t("inbox", "Inbox")}
                        </span>
                    </div>
                    <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary">
                        1 {t("campaign.setup.channels.inbox.new", "new")}
                    </span>
                </div>

                {/* Active (campaign) message — unread */}
                <div className="relative flex gap-3 bg-muted/40 px-4 py-3">
                    <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-primary" />
                    <div className="min-w-0 flex-1">
                        <div className="flex items-center justify-between gap-2">
                            <span className="truncate text-sm font-semibold text-foreground">
                                {title || (
                                    <span className="italic text-muted-foreground">
                                        {t("campaign.setup.channels.inbox.no_title", "No title")}
                                    </span>
                                )}
                            </span>
                            <span className="shrink-0 text-xs text-muted-foreground">{time}</span>
                        </div>
                        {body && (
                            <p className="mt-0.5 line-clamp-3 whitespace-pre-line text-sm text-muted-foreground">
                                {body}
                            </p>
                        )}
                    </div>
                </div>

                {/* Placeholder messages, blurred — they only convey that the campaign
                    message lands inside a list, not real content. */}
                {placeholders.map((message, index) => (
                    <div
                        key={index}
                        aria-hidden
                        className="flex select-none gap-3 border-t px-4 py-3 opacity-60 blur-[2px]"
                    >
                        <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-transparent" />
                        <div className="min-w-0 flex-1">
                            <div className="flex items-center justify-between gap-2">
                                <span className="truncate text-sm font-medium text-foreground">
                                    {message.title}
                                </span>
                                <span className="shrink-0 text-xs text-muted-foreground">
                                    {index === 0 ? "1h" : "1d"}
                                </span>
                            </div>
                            <p className="mt-0.5 line-clamp-1 text-sm text-muted-foreground">
                                {message.body}
                            </p>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    )
}
