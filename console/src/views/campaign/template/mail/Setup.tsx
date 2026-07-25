import { useContext, useState, useEffect, useRef } from "react"
import { Controller, useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { Campaign, Template, User, Locale, EmailTemplateData } from "@/types"
import { useTranslation } from "react-i18next"
import { ProjectContext, TemplateContext } from "@/contexts"
import { useNavigate } from "react-router"
import oapiClient from "@/oapi/client"
import * as z from "zod"
import { Render } from "@/renderTemplates"
import { compileEmail } from "./editor/codeEditor/compileEmail"
import { templaticalPreviewHtml } from "@/lib/templatical-preview"
import { getSystemPreviewProps } from "./editor/variableScope"

import { Input } from "@/components/ui/input"
import { TemplateInput } from "@/components/ui/template-input"
import { Button } from "@/components/ui/button"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { EmailFrame } from "@/components/preview/EmailFrame"

import { UserSelection } from "../UserSelection"
import { SenderIdentityCombobox } from "@/components/sender-identity-combobox"
import type { SenderIdentity } from "@/oapi/client"
import { useCampaignVariableContext } from "../../CampaignVariableContext"

import { emailSetupFormSchema } from "@/validation/campaign/template/mail/setup"

const randomSubjects = [
    "You won't believe what we have for you...",
    "Your exclusive invitation inside 🎉",
    "Last chance: Don't miss out!",
    "We thought you'd love this",
    "Something special just for you",
    "Your weekly update is here",
    "Quick question for you...",
    "This might interest you",
    "A gift from us to you 🎁",
    "Breaking news you need to see",
]

function randomSubject() {
    const index = Math.floor(Math.random() * randomSubjects.length)
    return randomSubjects[index]
}

export function EmailForm(_campaign: Campaign, template?: Template<EmailTemplateData>) {
    const formSchema = emailSetupFormSchema.extend({
        sender_identity_id: z.string("From address is required").min(1),
    })

    const form = useForm({
        resolver: zodResolver(formSchema),
        defaultValues: {
            sender_identity_id: template?.sender_identity_id ?? "",
            from: {
                name: template?.data.from?.name ?? "",
            },
            subject: template?.data.subject ?? randomSubject(),
            replyTo: template?.data.reply_to ?? "",
        },
    })

    return form
}

interface EmailFormControlProps {
    campaign: Campaign
    form: UseFormReturn<z.infer<typeof emailSetupFormSchema>>
    disabled?: boolean
}

export function EmailFormControl({
    campaign: _campaign,
    form,
    disabled = false,
}: EmailFormControlProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const { variableGroups } = useCampaignVariableContext()

    return (
        <FieldGroup className="mt-7">
            <Controller
                name="subject"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-subject">
                            {t("campaign.setup.channels.email.subject.label")}
                        </FieldLabel>
                        <TemplateInput
                            value={field.value}
                            onChange={field.onChange}
                            id="form-rhf-demo-subject"
                            placeholder=""
                            disabled={disabled}
                            variables={variableGroups}
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
            <Controller
                name="sender_identity_id"
                control={form.control}
                render={({ field, fieldState }) => {
                    const handleIdentitySelect = (identity: SenderIdentity) => {
                        const currentName = form.getValues("from.name")
                        if (!currentName && typeof identity.traits?.name === "string") {
                            form.setValue("from.name", identity.traits.name)
                        }
                    }
                    return (
                        <Field data-invalid={fieldState.invalid}>
                            <FieldLabel htmlFor="form-rhf-demo-fromEmail">
                                {t("campaign.setup.channels.email.from.email.label")}
                            </FieldLabel>
                            <SenderIdentityCombobox
                                projectId={project.id}
                                channel="email"
                                value={field.value ?? ""}
                                onChange={field.onChange}
                                onIdentitySelect={handleIdentitySelect}
                                placeholder={t("select_from_address", "Select from address...")}
                                disabled={disabled}
                            />
                            {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                        </Field>
                    )
                }}
            />
            <Controller
                name="from.name"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor="form-rhf-demo-fromName">
                            {t("campaign.setup.channels.email.from.name.label")}
                        </FieldLabel>
                        <TemplateInput
                            value={field.value ?? ""}
                            onChange={field.onChange}
                            id="form-rhf-demo-fromName"
                            placeholder=""
                            disabled={disabled}
                            variables={variableGroups}
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
            <Controller
                name="replyTo"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor="form-rhf-demo-replyTo">
                            {t("campaign.setup.channels.email.reply_to.label")}
                        </FieldLabel>
                        <Input
                            {...field}
                            id="form-rhf-demo-replyTo"
                            aria-invalid={fieldState.invalid}
                            placeholder=""
                            autoComplete="off"
                            disabled={disabled}
                            readOnly={disabled}
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
        </FieldGroup>
    )
}

export interface EmailSetupProps {
    campaign: Campaign
    form: UseFormReturn<z.infer<typeof emailSetupFormSchema>>
    edit?: boolean
}

export function EmailPreview({ campaign: _campaign, form }: EmailSetupProps) {
    const [project] = useContext(ProjectContext)
    const [template] = useContext(TemplateContext)
    const { t } = useTranslation()
    const [selectedUser, setSelectedUser] = useState<User | null>(null)

    if (template.type != "email") {
        return (
            <div className="text-center py-12 text-gray-400 italic">
                {t("campaign.setup.channels.email.no_template_selected")}
            </div>
        )
    }

    const loadingSubjects = 3
    const { subject, from } = form.watch()
    let previewSubject = subject

    let displayFromName = from?.name || template?.data?.from?.name || ""

    if (selectedUser) {
        previewSubject = Render(subject, {
            user: selectedUser,
        })

        displayFromName = Render(displayFromName, {
            user: selectedUser,
        })
    }

    return (
        <>
            <div className="mb-4">
                <UserSelection
                    projectId={project?.id}
                    value={selectedUser}
                    onChange={setSelectedUser}
                />
            </div>

            <div className="bg-white border rounded-md shadow-sm w-full overflow-hidden">
                <div className="flex items-center gap-3 px-4 py-2 border-b">
                    <div className="flex items-center gap-3">
                        <input type="checkbox" className="h-4 w-4 rounded border-gray-300" />
                        <div className="flex items-center gap-2 text-gray-400">
                            <StarIcon />
                            <ChevronIcon />
                        </div>
                    </div>
                    <div className="w-2/5 flex items-center gap-1 truncate min-w-0">
                        {displayFromName && (
                            <span className="font-semibold text-gray-900 whitespace-nowrap">
                                {displayFromName}
                            </span>
                        )}
                    </div>
                    <div className="w-3/5 font-semibold text-sm truncate">{previewSubject}</div>
                </div>

                <div>
                    {Array.from({ length: loadingSubjects }).map((_, index) => (
                        <div key={index} className="flex items-center gap-3 px-4 py-2">
                            <div className="flex items-center gap-3">
                                <input
                                    type="checkbox"
                                    className="h-4 w-4 rounded border-gray-300"
                                    disabled
                                />
                                <div className="flex items-center gap-2 text-gray-400">
                                    <StarIcon />
                                    <ChevronIcon />
                                </div>
                            </div>
                            <div className="w-2/5">
                                <div className="h-3 w-1/2 bg-gray-200 rounded"></div>
                            </div>
                            <div className="w-3/5 flex gap-3">
                                <div className="h-3 w-1/5 bg-gray-200 rounded"></div>
                                <div className="h-3 w-4/5 bg-gray-200 rounded"></div>
                            </div>
                        </div>
                    ))}
                </div>
            </div>
            <p className="text-center leading-7 mt-4 text-muted-foreground">
                {t("campaign.setup.channels.email.preview_disclaimer")}
            </p>
        </>
    )
}

function StarIcon() {
    return (
        <svg
            className="h-4 w-4 opacity-40"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M12 17.75L18.25 21l-1.5-7 5.25-4.75-7-.75L12 2 9 8.5l-7 .75L7.25 14l-1.5 7L12 17.75z" />
        </svg>
    )
}

function ChevronIcon() {
    return (
        <svg
            className="h-4 w-4 opacity-40"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M9 5l7 7-7 7" />
        </svg>
    )
}

export function EmailContentPreview({ campaign, form, edit = false }: EmailSetupProps) {
    const [project] = useContext(ProjectContext)
    const [template, setTemplate] = useContext(TemplateContext)
    const { t } = useTranslation()
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [selectedLocale, setSelectedLocale] = useState(template.locale)
    const [locales, setLocales] = useState<Locale[]>([])
    const [compiledHtml, setCompiledHtml] = useState<string>("")
    const navigate = useNavigate()
    const abortRef = useRef<AbortController | null>(null)

    useEffect(() => {
        const fetchLocales = async () => {
            if (project?.id) {
                const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/locales", {
                    params: {
                        path: { projectID: project.id },
                        query: { limit: 100 },
                    },
                })
                setLocales(data?.results ?? [])
            }
        }
        fetchLocales()
    }, [project?.id])

    // Compile code.source to HTML whenever the template or selected user changes.
    // The user is passed as a prop so JSX expressions like `props.user.data.first_name`
    // resolve with real values at React render time.
    useEffect(() => {
        if (template.type != "email") {
            setCompiledHtml("")
            return
        }

        // Visually authored templates are rendered by the backend on save;
        // compiling code.source would show the JSX kept only for reversibility.
        if (template?.data?.type === "templatical") {
            setCompiledHtml(templaticalPreviewHtml(template?.data?.code?.bundle))
            return
        }

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
                }
            })

        return () => {
            abortController.abort()
        }
        // @ts-expect-error template.data.code can not be undefined here because this page doesn't load without the email being selected,
        // and the email template type requires code to be defined.
    }, [
        template?.data?.type,
        template?.data?.code?.bundle,
        template?.data?.code?.source,
        selectedUser,
        template?.type,
    ])

    const { subject, from, replyTo } = form.watch()

    // The useEffect above already clears compiledHtml when the template is not an
    // email, so we only need to bail out of rendering the email preview here.
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
                ) : null}
            </EmailFrame>

            <p className="text-center leading-7 mt-4 text-muted-foreground text-sm">
                {t("campaign.setup.channels.email.preview_disclaimer")}
            </p>
        </>
    )
}
