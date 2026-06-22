import { Controller, useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { Campaign, Template, User, Locale } from "@/types"
import { Rocket } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useContext, useState, useEffect } from "react"
import { ProjectContext, TemplateContext } from "@/contexts"
import { useNavigate } from "react-router"
import { Button } from "@/components/ui/button"
import api from "@/api"
import type { z } from "zod"

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { TemplateInput } from "@/components/ui/template-input"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { Render } from "@/renderTemplates"

import { UserSelection } from "../UserSelection"
import { useCampaignVariableContext } from "../../CampaignVariableContext"
import { InboxNotificationCenter } from "./InboxNotificationCenter"
import { useSendTestInbox } from "./useSendTestInbox"

import { inboxSetupFormSchema } from "@/validation/campaign/template/inbox/setup"

export function InboxForm(_campaign: Campaign, template?: Template) {
    const formSchema = inboxSetupFormSchema.extend({})

    const defaultValues: z.infer<typeof inboxSetupFormSchema> = {
        title: "",
        body: "",
    }

    if (template && template.type == "inbox") {
        defaultValues.title = template.data.title
        defaultValues.body = template.data.body
    }

    return useForm({
        resolver: zodResolver(formSchema),
        defaultValues,
    })
}

interface InboxFormControlProps {
    campaign: Campaign
    form: UseFormReturn<z.infer<typeof inboxSetupFormSchema>>
    disabled?: boolean
}

export function InboxFormControl({ form, disabled = false }: InboxFormControlProps) {
    const { t } = useTranslation()
    const { variableGroups } = useCampaignVariableContext()

    return (
        <FieldGroup className="mt-7">
            <Controller
                name="title"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="inbox-title">
                            {t("campaign.setup.channels.inbox.title.label", "Title")}
                        </FieldLabel>
                        <TemplateInput
                            id="inbox-title"
                            value={field.value}
                            onChange={field.onChange}
                            placeholder={t(
                                "campaign.setup.channels.inbox.title.placeholder",
                                "Message title",
                            )}
                            disabled={disabled}
                            variables={variableGroups}
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
            <Controller
                name="body"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="inbox-body">
                            {t("campaign.setup.channels.inbox.body.label", "Body")}
                        </FieldLabel>
                        <TemplateInput
                            id="inbox-body"
                            value={field.value}
                            onChange={field.onChange}
                            placeholder={t(
                                "campaign.setup.channels.inbox.body.placeholder",
                                "Message body shown to the recipient",
                            )}
                            disabled={disabled}
                            variables={variableGroups}
                        />
                        {fieldState.invalid && <FieldError errors={[fieldState.error]} />}
                    </Field>
                )}
            />
        </FieldGroup>
    )
}

const sampleMessages = [
    ["Welcome to the team", "We're glad you're here. Take a look around to get started."],
    ["Your order has shipped", "Track your package and see the estimated delivery date."],
    ["New feature available", "We've added something we think you'll love. Check it out."],
    ["Action needed", "Please review your account details to keep everything up to date."],
]

function sampleMessage() {
    const index = Math.floor(Math.random() * sampleMessages.length)
    return sampleMessages[index]
}

export interface InboxSetupProps {
    campaign: Campaign
    form: UseFormReturn<z.infer<typeof inboxSetupFormSchema>>
    edit?: boolean
}

export function InboxPreview({ campaign, form, edit = false }: InboxSetupProps) {
    const [project] = useContext(ProjectContext)
    const [template, setTemplate] = useContext(TemplateContext)
    const { t } = useTranslation()
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [selectedLocale, setSelectedLocale] = useState(template.locale)
    const [locales, setLocales] = useState<Locale[]>([])
    const navigate = useNavigate()

    const { sending, handleSendTest } = useSendTestInbox({ projectId: project.id })

    useEffect(() => {
        const fetchLocales = async () => {
            if (project?.id) {
                const result = await api.locales.search(project.id, { limit: 100 })
                setLocales(result.results)
            }
        }
        fetchLocales()
    }, [project?.id])

    const [[placeholderTitle, placeholderBody]] = useState(() => sampleMessage())
    const rawTitle = form.watch("title") || placeholderTitle
    const rawBody = form.watch("body") || placeholderBody

    const title = selectedUser ? Render(rawTitle, { user: selectedUser }) : rawTitle
    const body = selectedUser ? Render(rawBody, { user: selectedUser }) : rawBody

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
                <UserSelection
                    projectId={project?.id}
                    value={selectedUser}
                    onChange={setSelectedUser}
                />
                {!edit && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <div>
                                <Button
                                    variant="default"
                                    size="sm"
                                    className="h-9 gap-1.5 shrink-0"
                                    onClick={() => {
                                        if (selectedUser) {
                                            handleSendTest(selectedUser, title, body)
                                        }
                                    }}
                                    disabled={sending || !selectedUser}
                                >
                                    <Rocket className="h-3.5 w-3.5" />
                                    {t("send_test_inbox.button", "Send test")}
                                </Button>
                            </div>
                        </TooltipTrigger>
                        {!selectedUser && (
                            <TooltipContent>
                                {t(
                                    "send_test_inbox.no_user",
                                    "Select a user to send a test inbox message",
                                )}
                            </TooltipContent>
                        )}
                    </Tooltip>
                )}
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
            <div className="flex w-full flex-1 items-start justify-center pt-4">
                <InboxNotificationCenter title={title} body={body} appName={project?.name} />
            </div>
        </>
    )
}
