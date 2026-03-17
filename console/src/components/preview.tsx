import { format } from "date-fns"
import type { Template } from "../types"
import Iframe from "@/components/iframe"
import "./preview.css"
import type { ReactNode } from "react"
import { useContext, useEffect, useRef, useState } from "react"
import { ProjectContext } from "../contexts"
import clsx from "clsx"
import { compileEmail } from "@/views/campaign/template/mail/editor/codeEditor/compileEmail"
import { getSystemPreviewProps } from "@/views/campaign/template/mail/editor/variableScope"

interface PreviewProps {
    template: Pick<Template, "type" | "data">
    size?: "small" | "large"
}

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
        <div className="email-frame">
            {data.from?.address && (
                <div className="email-frame-header">
                    <span className="email-from">
                        {data.from?.name} &lt;{data.from?.address}&gt;
                    </span>
                    <span className="email-subject">{data.subject}</span>
                </div>
            )}
            <Iframe content={compiledHtml} allowScroll={size !== "small"} />
        </div>
    )
}

export default function Preview({ template, size = "large" }: PreviewProps) {
    const [project] = useContext(ProjectContext)
    const { data, type } = template

    let preview: ReactNode = null
    if (type === "email") {
        preview = <EmailPreviewContent data={data} size={size} />
    } else if (type === "text") {
        preview = (
            <div className="text-frame phone-frame">
                <div className="text-frame-header">
                    <div className="text-frame-profile-image">
                        <i className="bi bi-person-fill" />
                    </div>
                </div>
                <span className="text-frame-context">
                    Text Message
                    <br />
                    Today {format(new Date(), "p")}
                </span>
                <div className="text-bubble">
                    {data.text}
                    <br />
                    {project.text_opt_out_message}
                </div>
            </div>
        )
    } else if (type === "push") {
        preview = (
            <div className="push-frame phone-frame">
                <div className="push-notification">
                    <div className="notification-icon"></div>
                    <div className="notification-header">
                        <span className="notification-title">{data.title}</span>
                        <span className="notification-time">now</span>
                    </div>
                    <span className="notification-body">{data.body}</span>
                </div>
            </div>
        )
    }

    return <section className={clsx("preview", size)}>{preview}</section>
}
