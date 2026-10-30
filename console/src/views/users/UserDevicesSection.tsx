import { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { Monitor, Trash2, Plus } from "lucide-react"
import { toast } from "sonner"
import { ProjectContext, UserContext } from "../../contexts"
import oapiClient from "../../oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { components } from "@/oapi/management.generated"
import { userKeys } from "./queries"

import { Button } from "@/components/ui/button"
import type { User } from "../../types"
import { RegisterDeviceDialog } from "./RegisterDeviceDialog"

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

/** The user's push devices, shown when the user has one or the project can send push. */
export function UserDevicesSection() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [user] = useContext(UserContext)
    const queryClient = useQueryClient()

    // Only decides whether to show the section; a failure counts as no push provider.
    const providers = useQuery({
        queryKey: ["projects", project.id, "providers"],
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/providers", {
                    params: { path: { projectID: project.id } },
                }),
            ).results,
    })

    const hasConfiguredPushProvider = (providers.data ?? []).some((provider: Provider) =>
        provider.channels?.includes("push"),
    )

    const hasPushDevice = Boolean((user as User & { has_push_device?: boolean }).has_push_device)
    const shouldShowDevicesSection = hasPushDevice || hasConfiguredPushProvider

    const devicesKey = [...userKeys.detail(project.id, user.id), "devices"]
    const devicesQuery = useQuery({
        queryKey: devicesKey,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/users/{userID}/devices",
                    { params: { path: { projectID: project.id, userID: user.id } } },
                ),
            ).results,
        enabled: shouldShowDevicesSection,
    })
    const devices = devicesQuery.data ?? []
    const invalidateDevices = () => queryClient.invalidateQueries({ queryKey: devicesKey })

    const [deletingDeviceId, setDeletingDeviceId] = useState<string | null>(null)
    const [isAddOpen, setIsAddOpen] = useState(false)

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
            await invalidateDevices()
        } catch {
            toast.error(t("device_delete_error", "Failed to delete device"))
        } finally {
            setDeletingDeviceId(null)
        }
    }

    if (!shouldShowDevicesSection) return null

    return (
        <>
            {/* Devices Section */}
            <div className="space-y-3">
                <div className="flex items-start justify-between gap-4">
                    <div>
                        <h2 className="text-base font-medium">{t("devices", "Devices")}</h2>
                        <p className="text-sm text-muted-foreground mt-0.5">
                            {t("devices_description", "Registered devices for push notifications")}
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

                {devicesQuery.isError ? (
                    <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed p-6 text-center">
                        <p className="text-sm text-muted-foreground">
                            {t("devices_load_failed", "Couldn't load devices.")}
                        </p>
                        <Button variant="outline" size="sm" onClick={() => devicesQuery.refetch()}>
                            {t("retry", "Retry")}
                        </Button>
                    </div>
                ) : devicesQuery.isPending ? null : devices.length === 0 ? (
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
                            const Icon = platform ? DEVICE_PLATFORM_STYLES[platform].icon : Monitor
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
                                            {device.model || device.os || t("unknown", "Unknown")}
                                        </p>
                                        <p className="text-xs text-muted-foreground">
                                            {device.os || t("unknown_os", "Unknown OS")}
                                            {device.os_version && ` ${device.os_version}`}
                                            {device.app_version && ` · v${device.app_version}`}
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

            <RegisterDeviceDialog
                open={isAddOpen}
                onOpenChange={setIsAddOpen}
                onRegistered={() => void invalidateDevices()}
            />
        </>
    )
}
