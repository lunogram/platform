import { format } from "date-fns"
import type { Template } from "../types"
import type { components } from "../oapi/management.generated"
import Iframe from "@/components/iframe"
import "./preview.css"
import type { ReactNode } from "react"
import { useContext, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { ProjectContext } from "../contexts"
import clsx from "clsx"
import { compileEmail } from "@/views/campaign/template/mail/editor/codeEditor/compileEmail"
import { getSystemPreviewProps } from "@/views/campaign/template/mail/editor/variableScope"
import { EmailFrame } from "@/components/preview/EmailFrame"
import { PhoneFrame } from "@/components/preview/PhoneFrame"
import { PushFrame } from "@/components/preview/PushFrame"
import { InboxFrame } from "@/components/preview/InboxFrame"

type InboxMessage = components["schemas"]["InboxMessage"]

type PreviewProps = {
    size?: "small" | "large"
    variant?: "default" | "inbox-detail"
    senderName?: string
    className?: string
} & (
    | { template: Pick<Template, "type" | "data">; message?: never }
    | { message: InboxMessage; template?: never }
)

function EmailPreviewContent({ data, size }: { data: Template["data"]; size: "small" | "large" }) {
    const [compiledHtml, setCompiledHtml] = useState<string>("")
    const abortRef = useRef<AbortController | null>(null)

    useEffect(() => {
        const source = data?.code?.source
        if (!source) {
            setCompiledHtml("")
            return
        }

        if (abortRef.current) {
            abortRef.current.abort()
        }
        const abortController = new AbortController()
        abortRef.current = abortController

        compileEmail(source, getSystemPreviewProps(), abortController.signal)
            .then((result) => {
                if (!abortController.signal.aborted) {
                    setCompiledHtml(result.html)
                }
            })
            .catch((err) => {
                if (err instanceof DOMException && err.name === "AbortError") return
                if (!abortController.signal.aborted) {
                    setCompiledHtml("")
                }
            })

        return () => {
            abortController.abort()
        }
    }, [data?.code?.source])

    return (
        <EmailFrame subject={data.subject} fromName={data.from?.name} labels={emailLabels}>
            <Iframe content={compiledHtml} allowScroll={size !== "small"} />
        </EmailFrame>
    )
}

export default function Preview({
    template,
    message,
    size = "large",
    variant = "default",
    senderName,
    className,
}: PreviewProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const isInboxDetailPreview = variant === "inbox-detail"
    const isSmsInboxDetailPreview = isInboxDetailPreview && message?.channel === "sms"
    const emailFrameClassName = isInboxDetailPreview
        ? "bg-white border rounded-xl w-full max-w-3xl overflow-hidden flex flex-col shadow-sm"
        : undefined

    const emailLabels = {
        noSubject: t("no_subject", "No subject"),
        unknownSender: t("unknown_sender", "Unknown sender"),
        noContent: t("no_content_available", "No content available"),
    }

    const inboxLabels = {
        noTitle: t("no_title", "No title"),
        priorityUrgent: t("priority_urgent", "Urgent"),
        priorityHigh: t("priority_high", "High"),
        priorityMedium: t("priority_medium", "Medium"),
        priorityLow: t("priority_low", "Low"),
    }

    let preview: ReactNode = null

    if (template) {
        const { data, type } = template
        if (type === "email") {
            preview = <EmailPreviewContent data={data} size={size} />
        } else if (type === "sms") {
            preview = (
                <PhoneFrame
                    sender={project.name.charAt(0).toUpperCase() + project.name.slice(1)}
                    message={
                        <>
                            {data.text}
                            <br />
                            {project.text_opt_out_message}
                        </>
                    }
                    contextLabel={t("text_message", "Text Message")}
                    contextDate={`${t("today", "Today")} ${format(new Date(), "p")}`}
                />
            )
        } else if (type === "push") {
            preview = (
                <PushFrame
                    title={data.title ?? t("notification", "Notification")}
                    body={data.body ?? ""}
                    time={t("now", "now")}
                />
            )
        }
    } else if (message) {
        const title = typeof message.content?.title === "string" ? message.content.title : ""
        const body = typeof message.content?.body === "string" ? message.content.body : ""
        const html = typeof message.content?.html === "string" ? message.content.html : ""
        const sentAt = message.sent_at ? new Date(message.sent_at) : null
        const sentTime = sentAt && !Number.isNaN(sentAt.getTime()) ? format(sentAt, "p") : null

        if (message.channel === "email" && html) {
            preview = (
                <EmailFrame
                    subject={title}
                    fromName={senderName}
                    time={sentTime ?? undefined}
                    className={emailFrameClassName}
                    labels={emailLabels}
                >
                    <Iframe content={html} allowScroll={size !== "small"} />
                </EmailFrame>
            )
        } else if (message.channel === "email") {
            // Email without HTML — show title/body in the frame
            preview = (
                <EmailFrame
                    subject={title}
                    fromName={senderName}
                    time={sentTime ?? undefined}
                    className={emailFrameClassName}
                    labels={emailLabels}
                >
                    {body ? (
                        <div className="px-6 py-4 text-sm text-foreground/80 whitespace-pre-wrap">
                            {body}
                        </div>
                    ) : null}
                </EmailFrame>
            )
        } else if (message.channel === "sms") {
            const smsContextDate = sentTime
                ? sentTime
                : `${t("today", "Today")} ${format(new Date(), "p")}`

            preview = (
                <PhoneFrame
                    message={body || t("no_content", "No content")}
                    contextLabel={t("text_message", "Text Message")}
                    contextDate={smsContextDate}
                />
            )
        } else if (message.channel === "push") {
            preview = (
                <PushFrame
                    title={title || t("notification", "Notification")}
                    body={body}
                    time={sentTime ?? t("now", "now")}
                />
            )
        } else {
            // inbox / fallback — dedicated inbox card
            preview = (
                <InboxFrame
                    title={title || undefined}
                    time={sentTime ?? undefined}
                    priority={message.priority}
                    tags={message.tags}
                    labels={inboxLabels}
                >
                    {body ? body : !title ? t("no_content", "No content") : null}
                </InboxFrame>
            )
        }
    }

    return (
        <section
            className={clsx(
                "preview",
                className,
                size,
                isInboxDetailPreview && "preview--inbox-detail",
                isSmsInboxDetailPreview && "preview--sms-bottom",
            )}
        >
            {preview}
        </section>
    )
}
