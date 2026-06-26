import { format } from "date-fns"
import type {
    Template,
    EmailTemplateData,
    TextTemplateData,
    PushTemplateData,
    InboxTemplateData,
} from "../types"
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
import { InboxNotificationCenter } from "@/views/campaign/template/inbox/InboxNotificationCenter"

type InboxMessage = components["schemas"]["InboxMessage"]

type EmailLabels = {
    noSubject: string
    unknownSender: string
    noContent: string
}

type PreviewProps = {
    size?: "small" | "large"
    variant?: "default" | "inbox-detail"
    senderName?: string
    className?: string
} & (
    | { template: Pick<Template, "type" | "data">; message?: never }
    | { message: InboxMessage; template?: never }
)

function EmailPreviewContent({
    data,
    size,
    labels,
}: {
    data: EmailTemplateData
    size: "small" | "large"
    labels: EmailLabels
}) {
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

    // In the small thumbnail (e.g. the Journey Send node) the Gmail-style
    // header chrome doesn't fit and just looks broken — show the rendered
    // email itself instead, clipped to the thumbnail height.
    if (size === "small") {
        return compiledHtml ? (
            <div className="h-full w-full overflow-hidden bg-white">
                <Iframe content={compiledHtml} allowScroll={false} width="100%" />
            </div>
        ) : (
            <div className="flex h-full w-full items-center justify-center bg-white text-sm italic text-gray-400">
                {labels.noContent}
            </div>
        )
    }

    return (
        <EmailFrame subject={data.subject} fromName={data.from?.name} labels={labels}>
            <Iframe content={compiledHtml} allowScroll />
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

    const emailLabels: EmailLabels = {
        noSubject: t("no_subject", "No subject"),
        unknownSender: t("unknown_sender", "Unknown sender"),
        noContent: t("no_content_available", "No content available"),
    }

    let preview: ReactNode = null

    if (template) {
        const { type } = template
        if (type === "email") {
            const data = template.data as EmailTemplateData
            preview = <EmailPreviewContent data={data} size={size} labels={emailLabels} />
        } else if (type === "sms") {
            const data = template.data as TextTemplateData
            preview = (
                <PhoneFrame
                    sender={project.name.charAt(0).toUpperCase() + project.name.slice(1)}
                    message={
                        <>
                            {data.body}
                            <br />
                            {project.text_opt_out_message}
                        </>
                    }
                    contextLabel={t("text_message", "Text Message")}
                    contextDate={`${t("today", "Today")} ${format(new Date(), "p")}`}
                />
            )
        } else if (type === "push") {
            const data = template.data as PushTemplateData
            preview = (
                <PushFrame
                    title={data.title ?? t("notification", "Notification")}
                    body={data.body ?? ""}
                    time={t("now", "now")}
                />
            )
        } else if (type === "inbox") {
            const data = template.data as InboxTemplateData
            preview = (
                <div className="p-4">
                    <InboxNotificationCenter
                        title={data.title}
                        body={data.body}
                        appName={project.name}
                    />
                </div>
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
            // inbox / fallback — notification-center card, matching the campaign preview
            preview = (
                <div className="w-full max-w-lg">
                    <InboxNotificationCenter
                        title={title}
                        body={body || (!title ? t("no_content", "No content") : "")}
                        appName={project.name}
                        time={sentTime ?? undefined}
                    />
                </div>
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
