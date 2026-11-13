import * as React from "react"
import { useForm } from "react-hook-form"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import { ChevronsUpDown, Loader2, Mail, Phone } from "lucide-react"
import { useTranslation } from "react-i18next"
import * as z from "zod"
import { cn } from "@/utils"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import oapiClient, { type SenderIdentity } from "@/oapi/client"
import { phoneSchema } from "@/validation/phone"
import { CreateView } from "./sender-identity/create-view"
import { ListView } from "./sender-identity/list-view"
import {
    channelProvidersQuery,
    invalidateSenderIdentities,
    senderIdentitiesQuery,
    senderIdentityKeys,
} from "./sender-identity/queries"

interface SenderIdentityComboboxProps {
    projectId: string
    channel: "email" | "sms"
    value?: string
    onChange: (value: string) => void
    onIdentitySelect?: (identity: SenderIdentity) => void
    onCreateIdentity?: () => void
    placeholder?: string
    disabled?: boolean
    required?: boolean
    className?: string
}

const UUID_REGEX = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

type PopoverView = "list" | "create"

export function SenderIdentityCombobox({
    projectId,
    channel,
    value,
    onChange,
    onIdentitySelect,
    onCreateIdentity,
    placeholder,
    disabled = false,
    className,
}: SenderIdentityComboboxProps) {
    const { t } = useTranslation()
    const [open, setOpen] = React.useState(false)
    const [search, setSearch] = React.useState("")
    const queryClient = useQueryClient()

    // Inline creation state
    const [view, setView] = React.useState<PopoverView>("list")
    const [creating, setCreating] = React.useState(false)
    const [createError, setCreateError] = React.useState<string | null>(null)

    const createForm = useForm<{ address: string; name: string; provider_id: string }>({
        defaultValues: { address: "", name: "", provider_id: "" },
    })

    // All identities for the project+channel (no provider filter), loaded once a
    // value needs resolving or the popover opens, and shared with other pickers.
    const identitiesQuery = useQuery({
        ...senderIdentitiesQuery(projectId, { channel }),
        enabled: open || !!value,
    })
    const identities = React.useMemo(() => identitiesQuery.data ?? [], [identitiesQuery.data])

    const providersQuery = useQuery({
        ...channelProvidersQuery(projectId, channel),
        enabled: open && view === "create",
    })
    const providers = providersQuery.data ?? []

    const isEmail = channel === "email"
    const Icon = isEmail ? Mail : Phone

    const defaultPlaceholder = isEmail
        ? t("select_from_address", "Select sender address...")
        : t("select_from_number", "Select phone number...")

    // Resolve UUID value to display address
    const resolvedIdentity = React.useMemo(() => {
        if (!value) return null
        if (UUID_REGEX.test(value)) {
            return identities.find((i) => i.id === value) ?? null
        }
        return identities.find((i) => (i.traits?.address as string) === value) ?? null
    }, [value, identities])

    const displayName = React.useMemo(() => {
        if (!resolvedIdentity) return null
        if (isEmail && resolvedIdentity.traits?.name) {
            return resolvedIdentity.traits.name as string
        }
        return null
    }, [resolvedIdentity, isEmail])

    const displayAddress = React.useMemo(() => {
        if (!resolvedIdentity) return value && !UUID_REGEX.test(value) ? value : ""
        return (resolvedIdentity.traits?.address as string) ?? ""
    }, [resolvedIdentity, value])

    // A value the identities have not resolved (yet) is still a selected
    // sender: say it is loading or failed instead of showing the placeholder.
    const unresolvedId = !!value && UUID_REGEX.test(value) && !resolvedIdentity

    // Auto-select the provider when only one can send on this channel
    const onlyProviderId = providersQuery.data?.length === 1 ? providersQuery.data[0].id : null
    React.useEffect(() => {
        if (view === "create" && onlyProviderId && !createForm.getValues("provider_id")) {
            createForm.setValue("provider_id", onlyProviderId)
        }
    }, [view, onlyProviderId, createForm])

    // Reset the view when the popover opens
    React.useEffect(() => {
        if (open) {
            setSearch("")
            setView("list")
            resetCreateForm()
        }
    }, [open])

    // Client-side filtering
    const filteredIdentities = React.useMemo(() => {
        if (!search) return identities
        const query = search.toLowerCase()
        return identities.filter((i) => {
            const addr = (i.traits?.address as string) ?? ""
            if (addr.toLowerCase().includes(query)) return true
            const name = i.traits?.name
            return typeof name === "string" && name.toLowerCase().includes(query)
        })
    }, [identities, search])

    const handleSelect = (identityId: string) => {
        onChange(identityId)
        const identity = identities.find((i) => i.id === identityId)
        if (identity) onIdentitySelect?.(identity)
        setOpen(false)
    }

    const resetCreateForm = () => {
        createForm.reset()
        setCreateError(null)
        setCreating(false)
    }

    const handleSwitchToCreate = () => {
        resetCreateForm()
        setView("create")
    }

    const handleCancelCreate = () => {
        setView("list")
        resetCreateForm()
    }

    const handleCreate = async (formData: {
        address: string
        name: string
        provider_id: string
    }) => {
        const address = formData.address.trim()
        if (!formData.provider_id || !address) return

        if (isEmail) {
            const result = z.email().safeParse(address)
            if (!result.success) {
                setCreateError(t("invalid_email", "Please enter a valid email address."))
                return
            }
        } else {
            const result = phoneSchema.safeParse(address)
            if (!result.success) {
                setCreateError(
                    t(
                        "invalid_phone_e164",
                        "Please enter a valid phone number in E.164 format (e.g., +1234567890).",
                    ),
                )
                return
            }
        }

        setCreating(true)
        setCreateError(null)

        try {
            const traits: Record<string, unknown> = { address }
            if (isEmail && formData.name.trim()) {
                traits.name = formData.name.trim()
            }
            const { data: newIdentity, error } = await oapiClient.POST(
                "/api/admin/projects/{projectID}/sender-identities",
                {
                    params: { path: { projectID: projectId } },
                    body: { provider_id: formData.provider_id, channel, traits },
                },
            )
            if (error || !newIdentity) {
                setCreateError(
                    t(
                        "sender_identity_create_error",
                        "Failed to add address. It may already exist.",
                    ),
                )
                return
            }
            // Show the new identity right away, then refresh every list of the
            // project (e.g. the integration's sender list) in the background.
            queryClient.setQueryData(
                senderIdentityKeys.list(projectId, { channel }),
                (prev: SenderIdentity[] | undefined) => [...(prev ?? []), newIdentity],
            )
            void invalidateSenderIdentities(queryClient, projectId)
            onChange(newIdentity.id)
            onIdentitySelect?.(newIdentity)
            onCreateIdentity?.()
            setOpen(false)
        } catch {
            setCreateError(
                t("sender_identity_create_error", "Failed to add address. It may already exist."),
            )
        } finally {
            setCreating(false)
        }
    }

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    type="button"
                    disabled={disabled}
                    className={cn(
                        "h-9 w-full justify-between shadow-sm font-normal",
                        !displayAddress && "text-muted-foreground",
                        className,
                    )}
                >
                    <span className="flex items-center gap-2 truncate">
                        <Icon className="h-4 w-4 shrink-0 text-muted-foreground" />
                        {displayAddress ? (
                            <span className="truncate text-sm">
                                {displayName && <span className="font-sans">{displayName} </span>}
                                <span className="font-mono text-muted-foreground">
                                    {displayAddress}
                                </span>
                            </span>
                        ) : unresolvedId && identitiesQuery.isError ? (
                            <span className="font-sans text-destructive">
                                {t(
                                    "sender_identity_selected_load_failed",
                                    "Couldn't load the selected sender",
                                )}
                            </span>
                        ) : unresolvedId && identitiesQuery.isPending ? (
                            <span className="font-sans">
                                <Loader2 className="inline h-3 w-3 animate-spin mr-1.5" />
                                {t("loading", "Loading...")}
                            </span>
                        ) : (
                            <span className="font-sans">{placeholder || defaultPlaceholder}</span>
                        )}
                    </span>
                    <ChevronsUpDown className="h-4 w-4 shrink-0 text-muted-foreground" />
                </Button>
            </PopoverTrigger>
            <PopoverContent
                className="p-0 w-[var(--radix-popover-trigger-width)]"
                align="start"
                onOpenAutoFocus={(e) => e.preventDefault()}
            >
                {view === "list" ? (
                    <ListView
                        channel={channel}
                        loading={identitiesQuery.isPending}
                        onRetry={
                            identitiesQuery.isError ? () => identitiesQuery.refetch() : undefined
                        }
                        identities={filteredIdentities}
                        allIdentities={identities}
                        value={value}
                        search={search}
                        onSearchChange={setSearch}
                        onSelect={handleSelect}
                        onAddNew={handleSwitchToCreate}
                        t={t}
                    />
                ) : (
                    <CreateView
                        channel={channel}
                        address={createForm.watch("address")}
                        onAddressChange={(v) => createForm.setValue("address", v)}
                        name={createForm.watch("name")}
                        onNameChange={(v) => createForm.setValue("name", v)}
                        providerId={createForm.watch("provider_id")}
                        onProviderChange={(v) => createForm.setValue("provider_id", v)}
                        providers={providers}
                        providersLoading={providersQuery.isPending}
                        onRetryProviders={
                            providersQuery.isError ? () => providersQuery.refetch() : undefined
                        }
                        onSave={createForm.handleSubmit(handleCreate)}
                        onCancel={handleCancelCreate}
                        creating={creating}
                        error={createError}
                        t={t}
                    />
                )}
            </PopoverContent>
        </Popover>
    )
}
