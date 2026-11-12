import { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { ProjectContext, UserContext } from "../../contexts"
import oapiClient from "../../oapi/client"
import { deviceFormSchema, type DeviceFormValues } from "@/validation/users/device-form"

import { Button } from "@/components/ui/button"
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

interface RegisterDeviceDialogProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    onRegistered: () => void
}

/** Manually registers (or updates) a push device of the user in context. */
export function RegisterDeviceDialog({
    open,
    onOpenChange,
    onRegistered,
}: RegisterDeviceDialogProps) {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)
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
            onOpenChange(false)
            resetNewDeviceForm()
            onRegistered()
        } catch {
            toast.error(t("device_add_error", "Failed to register device"))
        } finally {
            setIsAddingDevice(false)
        }
    }

    return (
        <Dialog
            open={open}
            onOpenChange={(open) => {
                onOpenChange(open)
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
                            <Label htmlFor="new-device-id">{t("device_id", "Device ID")} *</Label>
                            <Input
                                id="new-device-id"
                                placeholder={t("device_id_placeholder", "e.g., ios-sim-001")}
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
                                            <SelectItem value="android">Android</SelectItem>
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
                                            {deviceForm.formState.errors.auth_key.message}
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
                                            {deviceForm.formState.errors.p256dh_key.message}
                                        </p>
                                    )}
                                </div>
                            </div>
                        </div>
                    ) : (
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="new-device-token">{t("token", "Token")} *</Label>
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
                            <Label htmlFor="new-device-model">{t("model", "Model")}</Label>
                            <Input
                                id="new-device-model"
                                placeholder={t("model_placeholder", "e.g., iPhone 15 Pro")}
                                {...deviceForm.register("model")}
                            />
                        </div>
                        <div className="grid gap-2 content-start">
                            <Label htmlFor="new-device-app-version">
                                {t("app_version", "App version")}
                            </Label>
                            <Input
                                id="new-device-app-version"
                                placeholder={t("app_version_placeholder", "e.g., 2.1.0")}
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
                            onOpenChange(false)
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
    )
}
