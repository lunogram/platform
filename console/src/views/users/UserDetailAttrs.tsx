import { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Save, Smartphone, Monitor, Tablet, Trash2 } from "lucide-react"
import { toast } from "react-hot-toast/headless"
import { ProjectContext, UserContext } from "../../contexts"
import { useResolver } from "../../hooks"
import api from "../../api"
import oapiClient from "../../oapi/client"

import { Button } from "@/components/ui/button"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import type { User } from "../../types"

function getDeviceIcon(os: string) {
    const lower = os.toLowerCase()
    if (lower.includes("ios") || lower.includes("iphone") || lower.includes("android")) {
        return Smartphone
    }
    if (lower.includes("ipad") || lower.includes("tablet")) {
        return Tablet
    }
    return Monitor
}

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
            const updatedUser = await api.users.update(project.id, user.id, { data } as User)
            if (updatedUser) {
                setUser(updatedUser)
                setIsDirty(false)
                toast.success(t("save_success", "Attributes saved successfully"))
            }
        } catch {
            toast.error(t("save_error", "Failed to save attributes"))
        } finally {
            setIsSaving(false)
        }
    }

    const [devicesResult, , reloadDevices] = useResolver(
        useCallback(async () => {
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
        }, [project.id, user.id]),
    )

    const devices = devicesResult ?? []
    const [deletingDeviceId, setDeletingDeviceId] = useState<string | null>(null)

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
            {/* Devices Section */}
            {devices.length > 0 && (
                <div className="space-y-3">
                    <div>
                        <h2 className="text-base font-medium">{t("devices", "Devices")}</h2>
                        <p className="text-sm text-muted-foreground mt-0.5">
                            {t("devices_description", "Registered devices for push notifications")}
                        </p>
                    </div>
                    <div className="grid gap-2 sm:grid-cols-2">
                        {devices.map((device) => {
                            const Icon = getDeviceIcon(device.os ?? "")
                            const isDeleting = deletingDeviceId === device.id
                            return (
                                <div
                                    key={device.device_id}
                                    className="group flex items-start gap-3 rounded-lg border p-3 bg-card"
                                >
                                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted">
                                        <Icon className="h-4 w-4 text-muted-foreground" />
                                    </div>
                                    <div className="min-w-0 flex-1 space-y-0.5">
                                        <p className="text-sm font-medium leading-none">
                                            {device.model || device.os}
                                        </p>
                                        <p className="text-xs text-muted-foreground">
                                            {device.os}
                                            {device.app_version && ` · v${device.app_version}`}
                                            {device.app_build && ` (${device.app_build})`}
                                        </p>
                                        <p className="text-xs text-muted-foreground/60 font-mono truncate">
                                            {device.device_id}
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
                </div>
            )}

            {/* Custom Attributes Section */}
            <div className="space-y-6">
                {/* Section Header */}
                <div className="flex items-center justify-between">
                    <div>
                        <h2 className="text-base font-medium">
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
