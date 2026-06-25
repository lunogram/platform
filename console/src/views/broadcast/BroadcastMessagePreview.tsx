import { useContext, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Bell, Ellipsis, Loader2, UserRound } from "lucide-react"

import api from "@/api"
import { ProjectContext } from "@/contexts"
import type { Campaign, Template, User } from "@/types"
import type { UUID } from "@/types/common"
import { Render } from "@/renderTemplates"
import { compileEmail } from "@/views/campaign/template/mail/editor/codeEditor/compileEmail"
import { getSystemPreviewProps } from "@/views/campaign/template/mail/editor/variableScope"
import { UserSelection } from "@/views/campaign/template/UserSelection"
import { InboxNotificationCenter } from "@/views/campaign/template/inbox/InboxNotificationCenter"

interface BroadcastMessagePreviewProps {
    campaignId: UUID
    /** When provided, auto-selects this user for the initial preview. */
    defaultUser?: User | null
}

export function BroadcastMessagePreview({ campaignId, defaultUser }: BroadcastMessagePreviewProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [campaign, setCampaign] = useState<Campaign | null>(null)
    const [template, setTemplate] = useState<Template | null>(null)
    const [selectedUser, setSelectedUser] = useState<User | null>(defaultUser ?? null)
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const defaultApplied = useRef(false)

    // Auto-select the default user once it becomes available
    useEffect(() => {
        if (defaultUser && !defaultApplied.current) {
            defaultApplied.current = true
            setSelectedUser(defaultUser)
        }
    }, [defaultUser])

    // Load full campaign (with templates) on mount
    useEffect(() => {
        let cancelled = false
        setLoading(true)
        setError(null)

        api.campaigns
            .get(project.id, campaignId)
            .then((data) => {
                if (cancelled) return
                setCampaign(data)

                // Pick template matching project locale, fallback to first
                const tpl =
                    data.templates.find((t) => t.locale === project.locale) ??
                    data.templates[0] ??
                    null
                setTemplate(tpl)
            })
            .catch(() => {
                if (!cancelled) setError(t("failed_to_load_campaign", "Failed to load campaign"))
            })
            .finally(() => {
                if (!cancelled) setLoading(false)
            })

        return () => {
            cancelled = true
        }
    }, [project.id, project.locale, campaignId, t])

    if (loading) {
        return (
            <div className="flex items-center justify-center py-16">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
        )
    }

    if (error || !campaign || !template) {
        return (
            <div className="flex items-center justify-center py-16 text-muted-foreground text-sm">
                {error || t("no_template_found", "No template found for this campaign")}
            </div>
        )
    }

    return (
        <div className="space-y-4">
            <UserSelection
                projectId={project.id}
                value={selectedUser}
                onChange={setSelectedUser}
                size="sm"
            />

            {campaign.channel === "email" && (
                <EmailBroadcastPreview
                    campaign={campaign}
                    template={template}
                    user={selectedUser}
                />
            )}
            {campaign.channel === "sms" && (
                <SmsBroadcastPreview
                    template={template}
                    user={selectedUser}
                    projectName={project.name}
                />
            )}
            {campaign.channel === "push" && (
                <PushBroadcastPreview template={template} user={selectedUser} />
            )}
            {campaign.channel === "inbox" && (
                <InboxBroadcastPreview
                    template={template}
                    user={selectedUser}
                    appName={project.name}
                />
            )}
        </div>
    )
}

function EmailBroadcastPreview({
    campaign: _campaign,
    template,
    user,
}: {
    campaign: Campaign
    template: Template
    user: User | null
}) {
    const { t } = useTranslation()
    const [compiledHtml, setCompiledHtml] = useState("")
    const abortRef = useRef<AbortController | null>(null)

    useEffect(() => {
        const source = template?.data?.code?.source
        if (!source) {
            setCompiledHtml("")
            return
        }

        if (abortRef.current) {
            abortRef.current.abort()
        }
        const abortController = new AbortController()
        abortRef.current = abortController

        const previewProps: Record<string, unknown> = {
            ...getSystemPreviewProps(),
            ...(user ? { user } : {}),
        }

        compileEmail(source, previewProps, abortController.signal)
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
    }, [template?.data?.code?.source, user])

    const rawSubject = template.data.subject ?? ""
    const rawFromName = template.data.from?.name ?? ""

    const displaySubject = user ? Render(rawSubject, { user }) : rawSubject
    const displayFromName = user ? Render(rawFromName, { user }) : rawFromName

    return (
        <div className="bg-white border rounded-lg w-full overflow-hidden">
            {/* Email header */}
            <div className="px-6 py-4">
                <div className="flex items-start justify-between mb-4">
                    <h1 className="text-[22px] font-normal text-gray-900 flex-1 pr-4">
                        {displaySubject || (
                            <span className="text-gray-400 italic">
                                {t("no_subject", "No subject")}
                            </span>
                        )}
                    </h1>
                </div>

                <div className="flex items-start gap-3">
                    <div className="w-10 h-10 rounded-full bg-gradient-to-br from-blue-400 to-blue-600 flex items-center justify-center text-white font-medium flex-shrink-0">
                        {displayFromName ? displayFromName.charAt(0).toUpperCase() : "?"}
                    </div>
                    <div className="flex-1 min-w-0">
                        <div className="flex items-baseline gap-2 mb-1">
                            <span className="font-medium text-gray-900 text-sm">
                                {displayFromName || (
                                    <span className="text-gray-400 italic">
                                        {t("unknown_sender", "Unknown sender")}
                                    </span>
                                )}
                            </span>
                            <span className="text-xs text-gray-500">
                                {new Date().toLocaleTimeString("en", {
                                    hour: "numeric",
                                    minute: "2-digit",
                                })}
                            </span>
                        </div>
                        <div className="text-xs text-gray-600">to me</div>
                    </div>
                </div>
            </div>

            {/* Email body */}
            <div className="border-t border-gray-100">
                {compiledHtml ? (
                    <iframe
                        srcDoc={compiledHtml}
                        className="w-full border-0 h-[400px]"
                        title="Email Preview"
                        sandbox="allow-same-origin"
                    />
                ) : (
                    <div className="text-center py-12 text-gray-400 italic text-sm">
                        {t("no_email_content", "No email content available")}
                    </div>
                )}
            </div>
        </div>
    )
}

function SmsBroadcastPreview({
    template,
    user,
    projectName,
}: {
    template: Template
    user: User | null
    projectName: string
}) {
    const { t } = useTranslation()

    const rawBody = template.data.body ?? ""
    const message = user ? Render(rawBody, { user }) : rawBody
    const phoneNumber = projectName.charAt(0).toUpperCase() + projectName.slice(1)

    return (
        <div className="flex justify-center">
            <div className="w-[390px] h-[533px] bg-zinc-900 rounded-t-[70px] p-3 pb-0 shadow-2xl">
                <div className="w-full h-full bg-white rounded-t-[58px] overflow-hidden flex flex-col">
                    <div className="h-12 bg-white flex items-start justify-center px-8 pt-3">
                        <div className="w-32 h-8 bg-zinc-900 rounded-full" />
                    </div>

                    <div className="bg-white border-b border-gray-200 px-4 py-3 flex items-center justify-center">
                        <div className="flex flex-col items-center">
                            <div className="w-12 h-12 bg-gray-300 rounded-full flex items-center justify-center mb-1">
                                <UserRound className="w-7 h-7 text-gray-500" strokeWidth={1.5} />
                            </div>
                            <span className="text-sm font-medium">{phoneNumber}</span>
                        </div>
                    </div>

                    <div className="flex-1 bg-white px-4 py-6 overflow-y-auto">
                        <div className="flex flex-col items-center mb-6">
                            <span className="text-gray-500 text-xs">
                                {t(
                                    "campaign.setup.channels.text.text_message_label",
                                    "Text Message",
                                )}
                            </span>
                            <span className="text-gray-400 text-xs">
                                {t("campaign.setup.channels.text.today", "Today")}
                            </span>
                        </div>

                        <div className="flex justify-start mb-6">
                            <div className="max-w-[75%]">
                                <div className="bg-gray-200 rounded-3xl rounded-bl-sm px-4 py-3">
                                    {message || <Ellipsis className="text-gray-500" />}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}

function PushBroadcastPreview({ template, user }: { template: Template; user: User | null }) {
    const rawTitle = template.data.title ?? ""
    const rawBody = template.data.body ?? ""

    const title = user ? Render(rawTitle, { user }) : rawTitle
    const body = user ? Render(rawBody, { user }) : rawBody
    const time = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })

    return (
        <div className="flex w-full items-end justify-end">
            <div className="w-full max-w-md">
                <div className="bg-white rounded-2xl shadow-xl overflow-hidden border border-gray-200">
                    <div className="px-4 py-3 flex items-start gap-3">
                        <div className="flex-shrink-0 w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                            <Bell className="w-5 h-5 text-white" />
                        </div>

                        <div className="flex-1 flex gap-1 flex-col">
                            <div className="flex items-center justify-between">
                                <span className="text-sm font-semibold text-gray-900">{title}</span>
                                <span className="text-xs text-gray-500">{time}</span>
                            </div>

                            <p className="text-sm text-gray-600 line-clamp-3">{body}</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}

function InboxBroadcastPreview({
    template,
    user,
    appName,
}: {
    template: Template
    user: User | null
    appName: string
}) {
    const rawTitle = template.data.title ?? ""
    const rawBody = template.data.body ?? ""

    const title = user ? Render(rawTitle, { user }) : rawTitle
    const body = user ? Render(rawBody, { user }) : rawBody

    return <InboxNotificationCenter title={title} body={body} appName={appName} />
}
