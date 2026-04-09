import { Controller, useForm, type UseFormReturn } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { Campaign, Template, User, Device, Locale } from "@/types"
import { Bell, Rocket } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useContext, useState, useEffect } from "react"
import { ProjectContext, TemplateContext } from "@/contexts"
import { useNavigate } from "react-router"
import { Button } from "@/components/ui/button"
import { oapiClient } from "@/oapi/client"
import api from "@/api"
import * as z from "zod"

import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { TemplateInput } from "@/components/ui/template-input"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { UserSelection } from "../UserSelection"
import { useCampaignVariableContext } from "../../CampaignVariableContext"
import { useSendTestPush } from "./useSendTestPush"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

const pushSetupFormSchema = z.object({
    title: z.string("Title is required").min(1, "Title is required"),
    body: z.string("Body is required").min(1, "Body is required"),
    custom: z.record(z.string(), z.unknown()).optional(),
})

export function PushForm(_campaign: Campaign, template?: Template) {
    const formSchema = pushSetupFormSchema.extend({})

    return useForm({
        resolver: zodResolver(formSchema),
        defaultValues: {
            title: template?.data.title,
            body: template?.data.body,
            custom: template?.data.custom,
        },
    })
}

interface PushFormControlProps {
    campaign: Campaign
    form: UseFormReturn<z.infer<typeof pushSetupFormSchema>>
    disabled?: boolean
}

export function PushFormControl({ form, disabled = false }: PushFormControlProps) {
    const { t } = useTranslation()
    const { variableGroups } = useCampaignVariableContext()

    return (
        <FieldGroup className="mt-7">
            <Controller
                name="title"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-title">
                            {t("campaign.setup.channels.push.title.label")}
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
            <Controller
                name="body"
                control={form.control}
                render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid} className="gap-2">
                        <FieldLabel htmlFor="form-rhf-demo-message">
                            {t("campaign.setup.channels.push.body.label")}
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
            {/* TODO: expose custom field as JSON */}
        </FieldGroup>
    )
}

const randomNotifications = [
    [
        "Welcome to our App!",
        "Thank you for installing our app. We're excited to have you on board.",
    ],
    ["Don't miss out!", "Check out the latest features we've added to enhance your experience."],
    ["Special Offer!", "Get 20% off on your next purchase. Limited time offer!"],
    [
        "Update Available",
        "A new version of the app is available. Update now for the best experience.",
    ],
    ["Weekly Summary", "Here's what you've missed this week. Stay updated with the latest news."],
]

function randomNotification() {
    const index = Math.floor(Math.random() * randomNotifications.length)
    return randomNotifications[index]
}

export interface PushSetupProps {
    campaign: Campaign
    form: UseFormReturn<z.infer<typeof pushSetupFormSchema>>
    edit?: boolean
}

export function PushPreview({ campaign, form, edit = false }: PushSetupProps) {
    const [project] = useContext(ProjectContext)
    const [template, setTemplate] = useContext(TemplateContext)
    const { t } = useTranslation()
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [selectedLocale, setSelectedLocale] = useState(template.locale)
    const [locales, setLocales] = useState<Locale[]>([])
    const [devices, setDevices] = useState<Device[]>([])
    const [selectedDevice, setSelectedDevice] = useState<Device | null>(null)
    const navigate = useNavigate()

    const { sending, handleSendTest } = useSendTestPush({
        projectId: project.id,
        campaignId: campaign.id,
        templateId: template.id,
    })

    useEffect(() => {
        const fetchLocales = async () => {
            if (project?.id) {
                const result = await api.locales.search(project.id, { limit: 100 })
                setLocales(result.results)
            }
        }
        fetchLocales()
    }, [project?.id])

    useEffect(() => {
        const fetchDevices = async () => {
            if (!project?.id || !selectedUser?.id) {
                setDevices([])
                setSelectedDevice(null)
                return
            }
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/subjects/users/{userID}/devices",
                {
                    params: {
                        path: { projectID: project.id, userID: selectedUser.id },
                    },
                },
            )
            const results = data?.results ?? []
            setDevices(results as Device[])
            setSelectedDevice(results.length > 0 ? (results[0] as Device) : null)
        }
        fetchDevices()
    }, [project?.id, selectedUser?.id])

    const time = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })

    const [[placeholderTitle, placeholderBody]] = useState(() => randomNotification())
    const title = form.watch("title") ?? placeholderTitle
    const body = form.watch("body") ?? placeholderBody

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
                <div className="flex items-center gap-2">
                    <UserSelection
                        projectId={project?.id}
                        value={selectedUser}
                        onChange={setSelectedUser}
                    />
                    {devices.length > 1 && (
                        <Select
                            value={selectedDevice?.device_id ?? ""}
                            onValueChange={(deviceId) => {
                                const device = devices.find((d) => d.device_id === deviceId)
                                if (device) setSelectedDevice(device)
                            }}
                        >
                            <SelectTrigger className="w-[180px]">
                                <SelectValue placeholder={t("send_test_push.select_device", "Select device...")} />
                            </SelectTrigger>
                            <SelectContent>
                                {devices.map((d) => (
                                    <SelectItem key={d.device_id} value={d.device_id}>
                                        {d.model || d.device_id}{d.os ? ` (${d.os})` : ""}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    )}
                </div>
                {!edit && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <div>
                                <Button
                                    variant="default"
                                    size="sm"
                                    className="h-9 gap-1.5 shrink-0"
                                    onClick={() => {
                                        if (selectedDevice) {
                                            handleSendTest(selectedDevice, selectedUser)
                                        }
                                    }}
                                    disabled={sending || !selectedDevice?.token}
                                >
                                    <Rocket className="h-3.5 w-3.5" />
                                    {t("send_test_push.button", "Send test")}
                                </Button>
                            </div>
                        </TooltipTrigger>
                        {selectedUser && devices.length === 0 && (
                            <TooltipContent>
                                {t(
                                    "send_test_push.no_devices",
                                    "This user has no registered devices",
                                )}
                            </TooltipContent>
                        )}
                        {selectedUser && devices.length > 0 && !selectedDevice?.token && (
                            <TooltipContent>
                                {t(
                                    "send_test_push.no_token",
                                    "Selected device has no push token",
                                )}
                            </TooltipContent>
                        )}
                        {!selectedUser && (
                            <TooltipContent>
                                {t(
                                    "send_test_push.no_user",
                                    "Select a user to send a test push notification",
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
            <div className="flex w-full items-end justify-end">
                <div className="w-full max-w-md">
                    <div className="bg-white rounded-2xl shadow-xl overflow-hidden border border-gray-200">
                        <div className="px-4 py-3 flex items-start gap-3">
                            <div className="flex-shrink-0 w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                                <Bell className="w-5 h-5 text-white" />
                            </div>

                            <div className="flex-1 flex gap-1 flex-col">
                                <div className="flex items-center justify-between">
                                    <span className="text-sm font-semibold text-gray-900">
                                        {title}
                                    </span>
                                    <span className="text-xs text-gray-500">{time}</span>
                                </div>

                                <p className="text-sm text-gray-600 line-clamp-3">{body}</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </>
    )
}
