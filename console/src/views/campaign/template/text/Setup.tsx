import { Controller, useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { Ellipsis, UserRound } from "lucide-react"
import type { Campaign, Template, User, Locale } from "@/types"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import api from "@/api"
import * as z from "zod"

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"

import { SenderIdentityCombobox } from "@/components/sender-identity-combobox"
import { TemplateInput } from "@/components/ui/template-input"
import { Button } from "@/components/ui/button"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { UserSelection } from "../UserSelection"
import { useContext, useState, useEffect } from "react"
import { ProjectContext, TemplateContext } from "@/contexts"
import { useCampaignVariableContext } from "../../CampaignVariableContext"

const textSetupFormSchema = z.object({
    sender_identity_id: z.string().optional(),
    message: z.string("Message is required").min(1, "Message is required"),
})

export function TextForm(campaign: Campaign, template?: Template) {
    const formSchema = textSetupFormSchema.extend({
        sender_identity_id: campaign?.provider?.data.default_from
            ? z.string().optional()
            : z.string("From number is required").min(1),
    })

    return useForm({
        resolver: zodResolver(formSchema),
        defaultValues: {
            sender_identity_id: template?.sender_identity_id ?? "",
            message: template?.data.message,
        },
    })
}

interface TextFormControlProps {
    campaign: Campaign
    form: UseFormReturn<z.infer<typeof textSetupFormSchema>>
    disabled?: boolean
}

export function TextFormControl({ campaign, form, disabled = false }: TextFormControlProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const { variableGroups } = useCampaignVariableContext()

    return (
        <FieldGroup className="mt-7">
            <Controller
                name="sender_identity_id"
                control={form.control}
                render={({ field, fieldState }) => {
                    const defaultFrom = campaign?.provider?.data.default_from
                    return (
                        <Field data-invalid={fieldState.invalid} className="gap-2">
                            <FieldLabel htmlFor="form-rhf-demo-from">
                                {t("campaign.setup.channels.text.from.label")}
                            </FieldLabel>
                            <SenderIdentityCombobox
                                projectId={project.id}
                                channel="sms"
                                providerId={campaign.provider?.id}
                                value={field.value ?? ""}
                                onChange={field.onChange}
                                placeholder={
                                    defaultFrom || t("select_from_number", "Select from number...")
                                }
                                disabled={disabled}
                            />
                            {!field.value && defaultFrom && (
                                <p className="text-xs text-muted-foreground">
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
                name="message"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-message">
                            {t("campaign.setup.channels.text.message.label")}
                        </FieldLabel>
                        <TemplateInput
                            value={field.value}
                            onChange={field.onChange}
                            placeholder=""
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

export interface TextSetupProps {
    campaign: Campaign
    form: UseFormReturn<z.infer<typeof textSetupFormSchema>>
    edit?: boolean
}

export function TextPreview({ campaign, form, edit = false }: TextSetupProps) {
    const [project] = useContext(ProjectContext)
    const [template, setTemplate] = useContext(TemplateContext)
    const { t } = useTranslation()
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [selectedLocale, setSelectedLocale] = useState(template.locale)
    const [locales, setLocales] = useState<Locale[]>([])
    const navigate = useNavigate()

    useEffect(() => {
        const fetchLocales = async () => {
            if (project?.id) {
                const result = await api.locales.search(project.id, { limit: 100 })
                setLocales(result.results)
            }
        }
        fetchLocales()
    }, [project?.id])

    const message = form.watch("message")
    const phoneNumber = project.name.charAt(0).toUpperCase() + project.name.slice(1)

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
        <div className="flex h-full items-center flex-col">
            <div className="mb-8 m-auto flex items-center gap-4 w-full max-w-md">
                <div className={edit ? "flex-1" : "flex-1 flex justify-center"}>
                    <UserSelection
                        projectId={project?.id}
                        value={selectedUser}
                        onChange={setSelectedUser}
                    />
                </div>
                {edit && (
                    <>
                        <div className="flex-1">
                            <Select value={selectedLocale} onValueChange={handleLocaleChange}>
                                <SelectTrigger className="w-full">
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
                        </div>
                        <div className="flex-1">
                            <Button onClick={handleEditTemplate} className="w-full">
                                {t("campaign.template.edit")}
                            </Button>
                        </div>
                    </>
                )}
            </div>
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
                                {t("campaign.setup.channels.text.text_message_label")}
                            </span>
                            <span className="text-gray-400 text-xs">
                                {t("campaign.setup.channels.text.today")}
                            </span>
                        </div>

                        <div className="flex justify-start mb-6">
                            <div className="max-w-[75%]">
                                <div className="bg-gray-200 rounded-3xl rounded-bl-sm px-4 py-3">
                                    {message || <Ellipsis className="text-gray-500" />}
                                </div>
                            </div>
                        </div>

                        <div className="text-center">
                            <p className="text-gray-400 text-sm">
                                {t("campaign.setup.channels.text.preview_disclaimer")}
                            </p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}
