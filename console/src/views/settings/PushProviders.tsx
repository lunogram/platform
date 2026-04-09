import { useCallback, useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Trash2, Loader2, Smartphone } from "lucide-react"
import { toast } from "sonner"
import { ProjectContext } from "../../contexts"
import { useResolver } from "../../hooks"
import oapiClient from "@/oapi/client"
import type { components } from "@/oapi/management.generated"

import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import { Badge } from "@/components/ui/badge"

type ProjectPushProvider = components["schemas"]["ProjectPushProvider"]
type Platform = components["schemas"]["ProjectPushProviderPlatform"]
type Provider = components["schemas"]["Provider"]

// Platform-specific SVG icons
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

const PLATFORMS: {
    key: Platform
    label: string
    description: string
    icon: typeof AppleIcon
    iconBg: string
    iconColor: string
}[] = [
    {
        key: "ios",
        label: "iOS",
        description: "Apple Push Notification service",
        icon: AppleIcon,
        iconBg: "bg-zinc-500/10 dark:bg-zinc-400/10",
        iconColor: "text-zinc-600 dark:text-zinc-400",
    },
    {
        key: "android",
        label: "Android",
        description: "Firebase Cloud Messaging",
        icon: AndroidIcon,
        iconBg: "bg-emerald-500/10 dark:bg-emerald-400/10",
        iconColor: "text-emerald-600 dark:text-emerald-400",
    },
    {
        key: "web",
        label: "Web",
        description: "Web Push API",
        icon: WebIcon,
        iconBg: "bg-blue-500/10 dark:bg-blue-400/10",
        iconColor: "text-blue-600 dark:text-blue-400",
    },
]

export default function PushProviders() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const [pushProviders, , reloadPushProviders] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/push-providers",
                { params: { path: { projectID: project.id } } },
            )
            return data?.results ?? []
        }, [project.id]),
    )

    const [providers] = useResolver(
        useCallback(async () => {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/providers",
                { params: { path: { projectID: project.id } } },
            )
            return (data?.results ?? []).filter((p: Provider) =>
                p.channels?.includes("push"),
            )
        }, [project.id]),
    )

    const loading = !pushProviders || !providers
    const configuredCount =
        pushProviders?.filter(
            (pp: ProjectPushProvider) => pp.provider_id,
        ).length ?? 0

    return (
        <div className="space-y-6">
            {/* Section header */}
            <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10">
                    <Smartphone className="h-4 w-4 text-primary" />
                </div>
                <div className="flex-1">
                    <h3 className="font-semibold leading-none tracking-tight">
                        {t("push_providers", "Push Providers")}
                    </h3>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "push_providers_description",
                            "Assign a default integration for each platform. Device tokens are routed by platform.",
                        )}
                    </p>
                </div>
                {!loading && (
                    <Badge variant="secondary" className="tabular-nums">
                        {configuredCount}/{PLATFORMS.length} configured
                    </Badge>
                )}
            </div>

            {/* Platform cards */}
            <div className="grid gap-4 sm:grid-cols-3">
                {loading
                    ? Array.from({ length: 3 }).map((_, i) => (
                          <div
                              key={i}
                              className="rounded-lg border bg-card p-4 space-y-3"
                          >
                              <div className="flex items-center gap-3">
                                  <Skeleton className="h-9 w-9 rounded-lg" />
                                  <div className="space-y-1.5">
                                      <Skeleton className="h-4 w-16" />
                                      <Skeleton className="h-3 w-28" />
                                  </div>
                              </div>
                              <Skeleton className="h-9 w-full rounded-md" />
                          </div>
                      ))
                    : PLATFORMS.map((platform) => (
                          <PlatformCard
                              key={platform.key}
                              platform={platform}
                              projectId={project.id}
                              providers={providers ?? []}
                              pushProvider={pushProviders?.find(
                                  (pp: ProjectPushProvider) =>
                                      pp.platform === platform.key,
                              )}
                              onChanged={reloadPushProviders}
                          />
                      ))}
            </div>
        </div>
    )
}

interface PlatformCardProps {
    platform: (typeof PLATFORMS)[number]
    projectId: string
    providers: Provider[]
    pushProvider?: ProjectPushProvider
    onChanged: () => void
}

function PlatformCard({
    platform,
    projectId,
    providers,
    pushProvider,
    onChanged,
}: PlatformCardProps) {
    const { t } = useTranslation()
    const [saving, setSaving] = useState(false)
    const Icon = platform.icon

    const isConfigured = !!pushProvider

    const handleSelect = async (providerId: string) => {
        setSaving(true)
        try {
            await oapiClient.PUT(
                "/api/admin/projects/{projectID}/push-providers/{platform}",
                {
                    params: {
                        path: { projectID: projectId, platform: platform.key },
                    },
                    body: { provider_id: providerId },
                },
            )
            onChanged()
        } catch {
            toast.error(
                t("push_provider_save_failed", "Failed to save push provider"),
            )
        } finally {
            setSaving(false)
        }
    }

    const handleRemove = async () => {
        setSaving(true)
        try {
            await oapiClient.DELETE(
                "/api/admin/projects/{projectID}/push-providers/{platform}",
                {
                    params: {
                        path: { projectID: projectId, platform: platform.key },
                    },
                },
            )
            onChanged()
        } catch {
            toast.error(
                t(
                    "push_provider_remove_failed",
                    "Failed to remove push provider",
                ),
            )
        } finally {
            setSaving(false)
        }
    }

    return (
        <div className="rounded-lg border bg-card">
            {/* Content */}
            <div className="p-4 space-y-3">
                {/* Platform identity */}
                <div className="flex items-center gap-3">
                    <div
                        className={`flex h-9 w-9 items-center justify-center rounded-lg ${platform.iconBg}`}
                    >
                        <Icon
                            className={`h-4 w-4 ${platform.iconColor}`}
                        />
                    </div>
                    <div>
                        <h4 className="text-sm font-semibold tracking-tight">
                            {platform.label}
                        </h4>
                        <p className="text-xs text-muted-foreground">
                            {platform.description}
                        </p>
                    </div>
                </div>

                {/* Provider assignment — always a select */}
                {providers.length === 0 ? (
                    <p className="text-xs text-muted-foreground">
                        {t(
                            "no_push_integrations",
                            "No push integrations available. Add one in Integrations first.",
                        )}
                    </p>
                ) : (
                    <div className="flex">
                        <Select
                            value={pushProvider?.provider_id ?? ""}
                            onValueChange={handleSelect}
                            disabled={saving}
                        >
                            <SelectTrigger
                                elevation="flat"
                                className={`w-full ${
                                    isConfigured
                                        ? "rounded-r-none border-r-0"
                                        : ""
                                }`}
                            >
                                {saving ? (
                                    <Loader2 className="h-4 w-4 animate-spin" />
                                ) : (
                                    <SelectValue
                                        placeholder={t(
                                            "select_integration",
                                            "Select integration...",
                                        )}
                                    />
                                )}
                            </SelectTrigger>
                            <SelectContent>
                                {providers.map((p) => (
                                    <SelectItem key={p.id} value={p.id}>
                                        <span>{p.name}</span>
                                        <span className="ml-2 text-muted-foreground">
                                            {p.type}
                                        </span>
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                        {isConfigured && (
                            <button
                                type="button"
                                className="cursor-pointer flex h-9 items-center justify-center rounded-r-md border border-input border-l-0 px-2.5 text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive disabled:pointer-events-none disabled:opacity-50"
                                onClick={handleRemove}
                                disabled={saving}
                                aria-label={t("remove", "Remove")}
                            >
                                <Trash2 className="h-3.5 w-3.5" />
                            </button>
                        )}
                    </div>
                )}
            </div>
        </div>
    )
}
