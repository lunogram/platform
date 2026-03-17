import { useContext, useState, useEffect, useRef } from "react"
import { Controller, useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { Campaign, Template, User, Locale } from "@/types"
import { useTranslation } from "react-i18next"
import { ProjectContext, TemplateContext } from "@/contexts"
import { useNavigate } from "react-router"
import api from "@/api"
import * as z from "zod"
import { Render } from "@/renderTemplates"
import { compileEmail } from "./editor/codeEditor/compileEmail"
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

import { UserSelection } from "../UserSelection"
import { SenderIdentityCombobox } from "@/components/sender-identity-combobox"
import type { SenderIdentity } from "@/oapi/client"
import { useCampaignVariableContext } from "../../CampaignVariableContext"

const emailSetupFormSchema = z.object({
    subject: z.string("Subject is required").min(1, "Subject is required"),
    sender_identity_id: z.string().optional(),
    from: z.object({
        name: z.string().optional(),
    }),
    replyTo: z.email("Invalid reply-to email address").optional().or(z.literal("")),
})

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

export function EmailForm(campaign: Campaign, template?: Template) {
    const formSchema = emailSetupFormSchema.extend({
        sender_identity_id: campaign?.provider?.data.default_from
            ? z.string().optional()
            : z.string("From address is required").min(1),
    })

    const form = useForm({
        resolver: zodResolver(formSchema),
        defaultValues: {
            sender_identity_id: template?.sender_identity_id ?? "",
            from: {
                name: template?.data.from?.name ?? "",
            },
            subject: template?.data.subject ?? randomSubject(),
            replyTo: template?.data.replyTo ?? "",
        },
    })

    return form
}

interface EmailFormControlProps {
    campaign: Campaign
    form: UseFormReturn<z.infer<typeof emailSetupFormSchema>>
    disabled?: boolean
}

export function EmailFormControl({ campaign, form, disabled = false }: EmailFormControlProps) {
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
                    const defaultFrom = campaign?.provider?.data.default_from
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
                                providerId={campaign.provider?.id}
                                value={field.value ?? ""}
                                onChange={field.onChange}
                                onIdentitySelect={handleIdentitySelect}
                                placeholder={
                                    defaultFrom ||
                                    t("select_from_address", "Select from address...")
                                }
                                disabled={disabled}
                            />
                            {!field.value && defaultFrom && (
                                <p className="text-xs text-muted-foreground mt-1">
                                    {t(
                                        "sender_fallback_hint",
                                        "Falls back to integration default: {{address}}",
                                        { address: defaultFrom },
                                    )}
                                </p>
                            )}
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

export function EmailPreview({ campaign, form }: EmailSetupProps) {
    const [project] = useContext(ProjectContext)
    const [template] = useContext(TemplateContext)
    const { t } = useTranslation()
    const [selectedUser, setSelectedUser] = useState<User | null>(null)

    const loadingSubjects = 3
    const { subject, from } = form.watch()
    let previewSubject = subject

    let displayFromName = from?.name || template?.data?.from?.name || ""

    const displayFromEmail = campaign?.provider?.data?.default_from || ""

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
                        {displayFromEmail && (
                            <span className="text-gray-400 whitespace-nowrap truncate">
                                &lt;{displayFromEmail}&gt;
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
                const result = await api.locales.search(project.id, { limit: 100 })
                setLocales(result.results)
            }
        }
        fetchLocales()
    }, [project?.id])

    // Compile code.source to HTML whenever the template or selected user changes.
    // The user is passed as a prop so JSX expressions like `props.user.data.first_name`
    // resolve with real values at React render time.
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
    }, [template?.data?.code?.source, selectedUser])

    const { subject, from, replyTo } = form.watch()

    const rawFromName = from.name || template.data.from?.name || ""
    const displayFromEmail = campaign?.provider?.data.default_from || ""
    const displayReplyTo = replyTo || template.data.replyTo || ""

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
                                {campaign.templates.map((t) => {
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

            <div className="bg-white border rounded-lg w-full overflow-hidden">
                <div className="px-6 py-4">
                    <div className="flex items-start justify-between mb-4">
                        <h1 className="text-[22px] font-normal text-gray-900 flex-1 pr-4">
                            {displaySubject || (
                                <span className="text-gray-400 italic">No subject</span>
                            )}
                        </h1>
                        <div className="flex items-center gap-1 text-gray-600 flex-shrink-0">
                            <button className="p-2 hover:bg-gray-100 rounded-full" title="Archive">
                                <ArchiveIcon />
                            </button>
                            <button
                                className="p-2 hover:bg-gray-100 rounded-full"
                                title="Report spam"
                            >
                                <ReportIcon />
                            </button>
                            <button className="p-2 hover:bg-gray-100 rounded-full" title="Delete">
                                <TrashIcon />
                            </button>
                        </div>
                    </div>

                    <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-blue-400 to-blue-600 flex items-center justify-center text-white font-medium flex-shrink-0">
                            {displayFromName
                                ? displayFromName.charAt(0).toUpperCase()
                                : displayFromEmail
                                  ? displayFromEmail.charAt(0).toUpperCase()
                                  : "?"}
                        </div>

                        <div className="flex-1 min-w-0">
                            <div className="flex items-baseline gap-2 mb-1">
                                <span className="font-medium text-gray-900 text-sm">
                                    {displayFromName || displayFromEmail || (
                                        <span className="text-gray-400 italic">Unknown sender</span>
                                    )}
                                </span>
                                <span className="text-xs text-gray-500">
                                    {new Date().toLocaleTimeString("en", {
                                        hour: "numeric",
                                        minute: "2-digit",
                                    })}
                                </span>
                            </div>
                            <div className="flex items-center gap-1 text-xs text-gray-600">
                                <span>to me</span>
                                <button className="hover:bg-gray-100 px-1 rounded">
                                    <ChevronDownIcon />
                                </button>
                            </div>
                            {displayFromEmail && displayFromName && (
                                <div className="text-xs text-gray-500 mt-1">
                                    &lt;{displayFromEmail}&gt;
                                </div>
                            )}
                            {displayReplyTo && (
                                <div className="text-xs text-gray-500 mt-1">
                                    Reply-To: {displayReplyTo}
                                </div>
                            )}
                        </div>

                        <div className="flex items-center gap-1 flex-shrink-0">
                            <button className="p-2 hover:bg-gray-100 rounded-full" title="Reply">
                                <ReplyIcon />
                            </button>
                            <button className="p-2 hover:bg-gray-100 rounded-full" title="More">
                                <MoreVertIcon />
                            </button>
                        </div>
                    </div>
                </div>

                <div className="border-t border-gray-100">
                    {htmlTemplate ? (
                        <iframe
                            srcDoc={htmlTemplate}
                            className="w-full border-0 h-[400px]"
                            title="Email Preview"
                            sandbox="allow-same-origin"
                        />
                    ) : (
                        <div className="text-center py-12 text-gray-400 italic text-sm">
                            {t("campaign.setup.channels.email.no_content_available")}
                        </div>
                    )}
                </div>
            </div>

            <p className="text-center leading-7 mt-4 text-muted-foreground text-sm">
                {t("campaign.setup.channels.email.preview_disclaimer")}
            </p>
        </>
    )
}

function ArchiveIcon() {
    return (
        <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m6 4.125l2.25 2.25m0 0l2.25-2.25M12 13.875l-2.25-2.25M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
        </svg>
    )
}

function ReportIcon() {
    return (
        <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
        </svg>
    )
}

function TrashIcon() {
    return (
        <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
        </svg>
    )
}

function ChevronDownIcon() {
    return (
        <svg
            className="w-3 h-3"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            viewBox="0 0 24 24"
        >
            <path d="M19 9l-7 7-7-7" />
        </svg>
    )
}

function ReplyIcon() {
    return (
        <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M9 15L3 9m0 0l6-6M3 9h12a6 6 0 010 12h-3" />
        </svg>
    )
}

function MoreVertIcon() {
    return (
        <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z" />
        </svg>
    )
}
