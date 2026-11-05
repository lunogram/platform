import { useContext, useState, useEffect, useRef } from "react"
import type { User } from "@/types"
import { useTranslation } from "react-i18next"
import { ProjectContext, TemplateContext } from "@/contexts"
import { useNavigate } from "react-router"
import { useProjectLocales } from "@/hooks/use-project-locales"
import { Render } from "@/renderTemplates"
import { compileEmail } from "./editor/codeEditor/compileEmail"
import { getSystemPreviewProps } from "./editor/variableScope"

import { Button } from "@/components/ui/button"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { EmailFrame } from "@/components/preview/EmailFrame"

import { UserSelection } from "../UserSelection"
import type { EmailSetupProps } from "./Setup"

// EmailContentPreview renders the compiled email body, as on the campaign details page.
export function EmailContentPreview({ campaign, form, edit = false }: EmailSetupProps) {
    const [project] = useContext(ProjectContext)
    const [template, setTemplate] = useContext(TemplateContext)
    const { t } = useTranslation()
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [selectedLocale, setSelectedLocale] = useState(template.locale)
    const locales = useProjectLocales(project.id)
    const [compiledHtml, setCompiledHtml] = useState<string>("")
    const [compileError, setCompileError] = useState<string | null>(null)
    const navigate = useNavigate()
    const abortRef = useRef<AbortController | null>(null)

    // Only an email template has code to compile.
    const source = template.type === "email" ? template.data.code?.source : undefined

    // Compile code.source to HTML whenever the template or selected user changes.
    // The user is passed as a prop so JSX expressions like `props.user.data.first_name`
    // resolve with real values at React render time.
    useEffect(() => {
        setCompileError(null)
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
            ...(selectedUser ? { user: selectedUser } : {}),
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
                    setCompileError(err instanceof Error ? err.message : String(err))
                }
            })

        return () => {
            abortController.abort()
        }
    }, [source, selectedUser])

    const { subject, from, reply_to: replyTo } = form.watch()

    if (template.type != "email") {
        return null
    }

    const rawFromName = from.name || template.data.from?.name || ""
    const displayReplyTo = replyTo || template.data.reply_to || ""

    const displaySubject = selectedUser ? Render(subject, { user: selectedUser }) : subject
    const displayFromName = selectedUser ? Render(rawFromName, { user: selectedUser }) : rawFromName
    // compiledHtml already has user data baked in from compileEmail (JSX expressions
    // are evaluated at React render time), so no Handlebars post-processing needed.
    const htmlTemplate = compiledHtml

    const handleEditTemplate = () => {
        navigate(`/projects/${project?.id}/campaigns/${campaign.id}/templates/${template.id}`)
    }

    const handleLocaleChange = async (locale: string) => {
        setSelectedLocale(locale)
        const newTemplate = campaign.templates.find((t) => t.locale === locale)
        if (!newTemplate) {
            return
        }
        setTemplate(newTemplate)
    }

    return (
        <>
            <div className="mb-4 flex items-center justify-between gap-4">
                <div className="flex-1">
                    <UserSelection
                        projectId={project?.id}
                        value={selectedUser}
                        onChange={setSelectedUser}
                    />
                </div>
                {edit && (
                    <div className="flex items-center gap-2">
                        <Select value={selectedLocale} onValueChange={handleLocaleChange}>
                            <SelectTrigger className="w-[180px]">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {Array.from(
                                    new Map(campaign.templates.map((t) => [t.locale, t])).values(),
                                ).map((t) => {
                                    const locale = locales.find((l) => l.key === t.locale)
                                    return (
                                        <SelectItem key={t.id} value={t.locale}>
                                            {locale?.label || t.locale}
                                        </SelectItem>
                                    )
                                })}
                            </SelectContent>
                        </Select>
                        <Button onClick={handleEditTemplate}>{t("campaign.template.edit")}</Button>
                    </div>
                )}
            </div>

            <EmailFrame
                subject={displaySubject}
                fromName={displayFromName}
                replyTo={displayReplyTo}
                emptyLabel={t("campaign.setup.channels.email.no_content_available")}
            >
                {htmlTemplate ? (
                    <iframe
                        srcDoc={htmlTemplate}
                        className="w-full border-0 h-[400px]"
                        title="Email Preview"
                        sandbox="allow-same-origin"
                    />
                ) : compileError ? (
                    <div className="m-4 rounded-lg border border-destructive/30 bg-destructive/5 p-4">
                        <p className="text-sm font-medium text-destructive">
                            {t("campaign.template.email.editor.compileError", "Compilation Error")}
                        </p>
                        <pre className="mt-2 text-xs text-destructive/80 whitespace-pre-wrap break-words font-mono">
                            {compileError}
                        </pre>
                    </div>
                ) : null}
            </EmailFrame>

            <p className="text-center leading-7 mt-4 text-muted-foreground text-sm">
                {t("campaign.setup.channels.email.preview_disclaimer")}
            </p>
        </>
    )
}
