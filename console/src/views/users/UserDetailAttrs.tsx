import { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Save, Monitor, Trash2, Braces, Plus } from "lucide-react"
import { toast } from "sonner"
import { ProjectContext, UserContext } from "../../contexts"
import { useResolver } from "../../hooks"
import oapiClient from "../../oapi/client"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import type { components } from "@/oapi/management.generated"

import { Button } from "@/components/ui/button"
import { deviceFormSchema, type DeviceFormValues } from "@/validation/users/device-form"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"
import UserDetailIdentifiers from "./UserDetailIdentifiers"
import type { User } from "../../types"

type Provider = components["schemas"]["Provider"]

function AppleIcon({ className }: { className?: string }) {
    return (
        <svg viewBox="0 0 24 24" fill="currentColor" className={className}>
            <path d="M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.8-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z" />
        </svg>
    )
}

function AndroidIcon({ className }: { className?: string }) {
    return (
        <svg viewBox="0 0 24 24" fill="currentColor" className={className}>
            <path d="M17.6 9.48l1.84-3.18c.16-.31.04-.69-.27-.86-.31-.16-.69-.04-.86.27l-1.87 3.23C14.89 8.35 13.18 8 11.5 8c-1.68 0-3.39.35-4.94.94L4.69 5.71c-.16-.31-.54-.43-.86-.27-.31.16-.43.55-.27.86l1.84 3.18C2.86 11.07 1.37 13.57 1 16.5h21c-.37-2.93-1.86-5.43-4.4-7.02zM7 13.5c-.55 0-1-.45-1-1s.45-1 1-1 1 .45 1 1-.45 1-1 1zm9 0c-.55 0-1-.45-1-1s.45-1 1-1 1 .45 1 1-.45 1-1 1z" />
        </svg>
    )
}

function WebIcon({ className }: { className?: string }) {
    return (
        <svg viewBox="0 0 24 24" fill="currentColor" className={className}>
            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z" />
        </svg>
    )
}

function getDevicePlatform(os?: string | null): "ios" | "android" | "web" | null {
    if (!os) return null
    const lower = os.toLowerCase()
    if (lower.includes("ios") || lower.includes("iphone") || lower.includes("ipad")) {
        return "ios"
    }
    if (lower.includes("android")) {
        return "android"
    }
    if (lower.includes("web") || lower.includes("browser")) {
        return "web"
    }
    return null
}

const DEVICE_PLATFORM_STYLES = {
    ios: {
        icon: AppleIcon,
        iconBg: "bg-zinc-500/10 dark:bg-zinc-400/10",
        iconColor: "text-zinc-600 dark:text-zinc-400",
    },
    android: {
        icon: AndroidIcon,
        iconBg: "bg-emerald-500/10 dark:bg-emerald-400/10",
        iconColor: "text-emerald-600 dark:text-emerald-400",
    },
    web: {
        icon: WebIcon,
        iconBg: "bg-blue-500/10 dark:bg-blue-400/10",
        iconColor: "text-blue-600 dark:text-blue-400",
    },
} as const

export default function UserDetailAttrs() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [user, setUser] = useContext(UserContext)

    const initialData = (user.data as Record<string, unknown>) ?? {}
    const [data, setData] = useState<Record<string, unknown>>(initialData)
    const [isDirty, setIsDirty] = useState(false)
    const [isSaving, setIsSaving] = useState(false)

    const handleChange = (newData: Record<string, unknown>) => {
        setData(newData)
        setIsDirty(JSON.stringify(newData) !== JSON.stringify(user.data ?? {}))
    }

    const handleSave = async () => {
        setIsSaving(true)
        try {
            const { data: updatedUser } = await oapiClient.PATCH(
                "/api/admin/projects/{projectID}/subjects/users/{userID}",
                {
                    params: { path: { projectID: project.id, userID: user.id } },
                    body: { data },
                },
            )
            if (updatedUser) {
                setUser(updatedUser as User)
                setIsDirty(false)
                toast.success(t("save_success", "Attributes saved successfully"))
            }
        } catch {
            toast.error(t("save_error", "Failed to save attributes"))
        } finally {
            setIsSaving(false)
        }
    }

    const [providersResult] = useResolver(
        useCallback(async () => {
            const response = await oapiClient.GET("/api/admin/projects/{projectID}/providers", {
                params: {
                    path: {
                        projectID: project.id,
                    },
                },
            })

            if (response.error || !response.data) {
                return []
            }

            return response.data.results
        }, [project.id]),
    )

    const hasConfiguredPushProvider = (providersResult ?? []).some((provider: Provider) =>
        provider.channels?.includes("push"),
    )

    const hasPushDevice = Boolean((user as User & { has_push_device?: boolean }).has_push_device)
    const shouldShowDevicesSection = hasPushDevice || hasConfiguredPushProvider

    const [devicesResult, , reloadDevices] = useResolver(
        useCallback(async () => {
            if (!shouldShowDevicesSection) {
                return []
            }

            const response = await oapiClient.GET(
                "/api/admin/projects/{projectID}/subjects/users/{userID}/devices",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            userID: user.id,
                        },
                    },
                },
            )
            if (response.error || !response.data) {
                return []
            }
            return response.data.results
        }, [project.id, shouldShowDevicesSection, user.id]),
    )

    const devices = devicesResult ?? []
    const [deletingDeviceId, setDeletingDeviceId] = useState<string | null>(null)
    const [isAddOpen, setIsAddOpen] = useState(false)
    const [isAddingDevice, setIsAddingDevice] = useState(false)
    const [newDeviceData, setNewDeviceData] = useState<Record<string, unknown>>({})

    const deviceForm = useForm<DeviceFormValues>({
        resolver: zodResolver(deviceFormSchema),
        defaultValues: {
            device_id: "",
            os: "ios",
            token: "",
            endpoint: "",
            auth_key: "",
            p256dh_key: "",
            os_version: "",
            model: "",
            app_build: "",
            app_version: "",
        },
    })

    const watchedOS = deviceForm.watch("os")

    const resetNewDeviceForm = () => {
        deviceForm.reset()
        setNewDeviceData({})
    }

    const handleAddDevice = async (formData: DeviceFormValues) => {
        const deviceId = formData.device_id.trim()
        if (!deviceId) return

        const isWeb = formData.os === "web"
        const token = formData.token?.trim() ?? ""
        const endpoint = formData.endpoint?.trim() ?? ""
        const auth = formData.auth_key?.trim() ?? ""
        const p256dh = formData.p256dh_key?.trim() ?? ""

        const parsedData = Object.keys(newDeviceData).length > 0 ? newDeviceData : undefined

        setIsAddingDevice(true)
        try {
            const response = await oapiClient.POST(
                "/api/admin/projects/{projectID}/subjects/users/{userID}/devices",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            userID: user.id,
                        },
                    },
                    body: {
                        device_id: deviceId,
                        config: {
                            ...(isWeb
                                ? {
                                      endpoint,
                                      keys: {
                                          auth,
                                          p256dh,
                                      },
                                  }
                                : {
                                      token,
                                  }),
                        },
                        data: parsedData,
                        os: formData.os,
                        os_version: formData.os_version?.trim() || undefined,
                        model: formData.model?.trim() || undefined,
                        app_build: formData.app_build?.trim() || undefined,
                        app_version: formData.app_version?.trim() || undefined,
                    },
                },
            )
            if (response.error) {
                toast.error(t("device_add_error", "Failed to register device"))
                return
            }

            toast.success(t("device_added", "Device registered"))
            setIsAddOpen(false)
            resetNewDeviceForm()
            await reloadDevices()
        } catch {
            toast.error(t("device_add_error", "Failed to register device"))
        } finally {
            setIsAddingDevice(false)
        }
    }

    const handleDeleteDevice = async (deviceId: string) => {
        setDeletingDeviceId(deviceId)
        try {
            const response = await oapiClient.DELETE(
                "/api/admin/projects/{projectID}/subjects/users/{userID}/devices/{deviceID}",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            userID: user.id,
                            deviceID: deviceId,
                        },
                    },
                },
            )
            if (response.error) {
                toast.error(t("device_delete_error", "Failed to delete device"))
                return
            }
            toast.success(t("device_deleted", "Device deleted"))
            await reloadDevices()
        } catch {
            toast.error(t("device_delete_error", "Failed to delete device"))
        } finally {
            setDeletingDeviceId(null)
        }
    }

    return (
        <div className="space-y-8">
            {/* Identifiers Section */}
            <UserDetailIdentifiers />

            {shouldShowDevicesSection && (
                <>
                    {/* Devices Section */}
                    <div className="space-y-3">
                        <div className="flex items-start justify-between gap-4">
                            <div>
                                <h2 className="text-base font-medium">{t("devices", "Devices")}</h2>
                                <p className="text-sm text-muted-foreground mt-0.5">
                                    {t(
                                        "devices_description",
                                        "Registered devices for push notifications",
                                    )}
                                </p>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={() => setIsAddOpen(true)}
                                className="shrink-0"
                            >
                                <Plus className="h-4 w-4 mr-2" />
                                {t("register_device", "Register device")}
                            </Button>
                        </div>

                        {devices.length === 0 ? (
                            <div className="rounded-lg border border-dashed p-6 text-center">
                                <p className="font-medium">{t("no_devices", "No devices")}</p>
                                <p className="text-sm text-muted-foreground mt-1">
                                    {t(
                                        "no_devices_description",
                                        "Manually register a device to test push notifications for this user.",
                                    )}
                                </p>
                            </div>
                        ) : (
                            <div className="grid gap-2 sm:grid-cols-2">
                                {devices.map((device) => {
                                    const platform = getDevicePlatform(device.os)
                                    const Icon = platform
                                        ? DEVICE_PLATFORM_STYLES[platform].icon
                                        : Monitor
                                    const iconBgClass = platform
                                        ? DEVICE_PLATFORM_STYLES[platform].iconBg
                                        : "bg-muted"
                                    const iconColorClass = platform
                                        ? DEVICE_PLATFORM_STYLES[platform].iconColor
                                        : "text-muted-foreground"
                                    const isDeleting = deletingDeviceId === device.id
                                    return (
                                        <div
                                            key={device.device_id}
                                            className="group flex items-start gap-3 rounded-lg border p-3 bg-card"
                                        >
                                            <div
                                                className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-md ${iconBgClass}`}
                                            >
                                                <Icon className={`h-4 w-4 ${iconColorClass}`} />
                                            </div>
                                            <div className="min-w-0 flex-1 space-y-0.5">
                                                <p className="text-sm font-medium leading-none">
                                                    {device.model ||
                                                        device.os ||
                                                        t("unknown", "Unknown")}
                                                </p>
                                                <p className="text-xs text-muted-foreground">
                                                    {device.os || t("unknown_os", "Unknown OS")}
                                                    {device.os_version && ` ${device.os_version}`}
                                                    {device.app_version &&
                                                        ` · v${device.app_version}`}
                                                    {device.app_build && ` (${device.app_build})`}
                                                </p>
                                            </div>
                                            <Button
                                                variant="ghost"
                                                size="icon"
                                                className="h-7 w-7 shrink-0 opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive transition-opacity"
                                                disabled={isDeleting}
                                                onClick={() => handleDeleteDevice(device.id)}
                                            >
                                                <Trash2 className="h-3.5 w-3.5" />
                                            </Button>
                                        </div>
                                    )
                                })}
                            </div>
                        )}
                    </div>

                    <Dialog
                        open={isAddOpen}
                        onOpenChange={(open) => {
                            setIsAddOpen(open)
                            if (!open && !isAddingDevice) {
                                resetNewDeviceForm()
                            }
                        }}
                    >
                        <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto">
                            <DialogHeader>
                                <DialogTitle>{t("register_device", "Register device")}</DialogTitle>
                                <DialogDescription>
                                    {t(
                                        "register_device_description",
                                        "Manually register or update a device for this user.",
                                    )}
                                </DialogDescription>
                            </DialogHeader>

                            <div className="grid gap-4 py-4">
                                <div className="grid sm:grid-cols-2 gap-4">
                                    <div className="grid gap-2 content-start">
                                        <Label htmlFor="new-device-id">
                                            {t("device_id", "Device ID")} *
                                        </Label>
                                        <Input
                                            id="new-device-id"
                                            placeholder={t(
                                                "device_id_placeholder",
                                                "e.g., ios-sim-001",
                                            )}
                                            {...deviceForm.register("device_id")}
                                        />
                                        {deviceForm.formState.errors.device_id && (
                                            <p className="text-sm text-destructive">
                                                {deviceForm.formState.errors.device_id.message}
                                            </p>
                                        )}
                                    </div>
                                    <div className="grid gap-2 content-start">
                                        <Label htmlFor="new-device-os">{t("os", "OS")}</Label>
                                        <Controller
                                            control={deviceForm.control}
                                            name="os"
                                            render={({ field }) => (
                                                <Select
                                                    value={field.value}
                                                    onValueChange={(value) => field.onChange(value)}
                                                >
                                                    <SelectTrigger id="new-device-os">
                                                        <SelectValue />
                                                    </SelectTrigger>
                                                    <SelectContent>
                                                        <SelectItem value="ios">iOS</SelectItem>
                                                        <SelectItem value="android">
                                                            Android
                                                        </SelectItem>
                                                        <SelectItem value="web">Web</SelectItem>
                                                    </SelectContent>
                                                </Select>
                                            )}
                                        />
                                    </div>
                                </div>

                                {watchedOS === "web" ? (
                                    <div className="grid gap-4">
                                        <div className="grid gap-2 content-start">
                                            <Label htmlFor="new-device-endpoint">
                                                {t("endpoint", "Endpoint")} *
                                            </Label>
                                            <Input
                                                id="new-device-endpoint"
                                                autoComplete="off"
                                                placeholder={t(
                                                    "device_endpoint_placeholder",
                                                    "https://push.service/...",
                                                )}
                                                {...deviceForm.register("endpoint")}
                                            />
                                            {deviceForm.formState.errors.endpoint && (
                                                <p className="text-sm text-destructive">
                                                    {deviceForm.formState.errors.endpoint.message}
                                                </p>
                                            )}
                                        </div>

                                        <div className="grid sm:grid-cols-2 gap-4">
                                            <div className="grid gap-2 content-start">
                                                <Label htmlFor="new-device-auth-key">
                                                    {t("auth_key", "Auth key")} *
                                                </Label>
                                                <Input
                                                    id="new-device-auth-key"
                                                    type="password"
                                                    autoComplete="off"
                                                    placeholder={t(
                                                        "device_auth_key_placeholder",
                                                        "Web Push auth key",
                                                    )}
                                                    {...deviceForm.register("auth_key")}
                                                />
                                                {deviceForm.formState.errors.auth_key && (
                                                    <p className="text-sm text-destructive">
                                                        {
                                                            deviceForm.formState.errors.auth_key
                                                                .message
                                                        }
                                                    </p>
                                                )}
                                            </div>
                                            <div className="grid gap-2 content-start">
                                                <Label htmlFor="new-device-p256dh-key">
                                                    {t("p256dh_key", "P256DH key")} *
                                                </Label>
                                                <Input
                                                    id="new-device-p256dh-key"
                                                    type="password"
                                                    autoComplete="off"
                                                    placeholder={t(
                                                        "device_p256dh_key_placeholder",
                                                        "Web Push p256dh key",
                                                    )}
                                                    {...deviceForm.register("p256dh_key")}
                                                />
                                                {deviceForm.formState.errors.p256dh_key && (
                                                    <p className="text-sm text-destructive">
                                                        {
                                                            deviceForm.formState.errors.p256dh_key
                                                                .message
                                                        }
                                                    </p>
                                                )}
                                            </div>
                                        </div>
                                    </div>
                                ) : (
                                    <div className="grid gap-2 content-start">
                                        <Label htmlFor="new-device-token">
                                            {t("token", "Token")} *
                                        </Label>
                                        <Input
                                            id="new-device-token"
                                            type="password"
                                            autoComplete="off"
                                            placeholder={t(
                                                "device_token_placeholder",
                                                "e.g., fcm_token_abc123",
                                            )}
                                            {...deviceForm.register("token")}
                                        />
                                        {deviceForm.formState.errors.token && (
                                            <p className="text-sm text-destructive">
                                                {deviceForm.formState.errors.token.message}
                                            </p>
                                        )}
                                    </div>
                                )}

                                <div className="grid sm:grid-cols-2 gap-4">
                                    <div className="grid gap-2 content-start">
                                        <Label htmlFor="new-device-os-version">
                                            {t("os_version", "OS version")}
                                        </Label>
                                        <Input
                                            id="new-device-os-version"
                                            placeholder={t("os_version_placeholder", "e.g., 17.2")}
                                            {...deviceForm.register("os_version")}
                                        />
                                    </div>
                                    <div className="grid gap-2 content-start">
                                        <Label htmlFor="new-device-model">
                                            {t("model", "Model")}
                                        </Label>
                                        <Input
                                            id="new-device-model"
                                            placeholder={t(
                                                "model_placeholder",
                                                "e.g., iPhone 15 Pro",
                                            )}
                                            {...deviceForm.register("model")}
                                        />
                                    </div>
                                    <div className="grid gap-2 content-start">
                                        <Label htmlFor="new-device-app-version">
                                            {t("app_version", "App version")}
                                        </Label>
                                        <Input
                                            id="new-device-app-version"
                                            placeholder={t(
                                                "app_version_placeholder",
                                                "e.g., 2.1.0",
                                            )}
                                            {...deviceForm.register("app_version")}
                                        />
                                    </div>
                                    <div className="grid gap-2 content-start">
                                        <Label htmlFor="new-device-app-build">
                                            {t("app_build", "App build")}
                                        </Label>
                                        <Input
                                            id="new-device-app-build"
                                            placeholder={t("app_build_placeholder", "e.g., 142")}
                                            {...deviceForm.register("app_build")}
                                        />
                                    </div>
                                </div>

                                <div className="grid gap-2">
                                    <Label>{t("data", "Data")}</Label>
                                    <AttributeEditor
                                        value={newDeviceData}
                                        onChange={setNewDeviceData}
                                        emptyTitle={t("no_data", "No data")}
                                        emptyDescription={t(
                                            "no_device_data_description",
                                            "Add custom attributes for this device.",
                                        )}
                                    />
                                </div>
                            </div>

                            <DialogFooter>
                                <Button
                                    variant="outline"
                                    onClick={() => {
                                        setIsAddOpen(false)
                                        resetNewDeviceForm()
                                    }}
                                    disabled={isAddingDevice}
                                >
                                    {t("cancel", "Cancel")}
                                </Button>
                                <Button
                                    onClick={deviceForm.handleSubmit(handleAddDevice)}
                                    disabled={isAddingDevice}
                                >
                                    {isAddingDevice
                                        ? t("registering", "Registering...")
                                        : t("register_device", "Register device")}
                                </Button>
                            </DialogFooter>
                        </DialogContent>
                    </Dialog>
                </>
            )}

            {/* Custom Attributes Section */}
            <div className="space-y-6">
                {/* Section Header */}
                <div className="flex items-center justify-between">
                    <div>
                        <h2 className="text-base font-medium">
                            <Braces className="inline h-4 w-4 mr-1.5 -mt-0.5" />
                            {t("custom_attributes", "Custom attributes")}
                        </h2>
                        <p className="text-sm text-muted-foreground mt-0.5">
                            {t(
                                "user_data_description",
                                "Store custom metadata and attributes for this user",
                            )}
                        </p>
                    </div>
                    {isDirty && (
                        <div className="flex items-center gap-3">
                            <span className="text-sm text-amber-600 dark:text-amber-500">
                                {t("unsaved_changes", "Unsaved changes")}
                            </span>
                            <Button onClick={handleSave} disabled={isSaving} size="sm">
                                <Save className="h-4 w-4 mr-2" />
                                {isSaving ? t("saving") : t("save")}
                            </Button>
                        </div>
                    )}
                </div>

                {/* Attribute Editor */}
                <AttributeEditor
                    value={initialData}
                    onChange={handleChange}
                    emptyTitle={t("no_attributes", "No attributes yet")}
                    emptyDescription={t(
                        "no_user_data_description",
                        "Add custom data to store additional information about this user.",
                    )}
                />
            </div>
        </div>
    )
}
