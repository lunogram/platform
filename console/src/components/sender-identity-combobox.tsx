import * as React from "react"
import { Check, ChevronsUpDown, Loader2, Mail, Phone, Plus, ArrowLeft } from "lucide-react"
import { useTranslation } from "react-i18next"
import { cn } from "@/utils"
import { Button } from "@/components/ui/button"
import {
    Command,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import oapiClient, { type SenderIdentity } from "@/oapi/client"
import type { components } from "@/oapi/management.generated"

type Provider = components["schemas"]["Provider"]

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
    const [identities, setIdentities] = React.useState<SenderIdentity[]>([])
    const [loading, setLoading] = React.useState(false)
    const fetchedRef = React.useRef(false)

    // Inline creation state
    const [view, setView] = React.useState<PopoverView>("list")
    const [newAddress, setNewAddress] = React.useState("")
    const [newName, setNewName] = React.useState("")
    const [newProviderId, setNewProviderId] = React.useState("")
    const [providers, setProviders] = React.useState<Provider[]>([])
    const [providersLoading, setProvidersLoading] = React.useState(false)
    const [creating, setCreating] = React.useState(false)
    const [createError, setCreateError] = React.useState<string | null>(null)

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

    // Fetch all identities for the project+channel (no provider filter)
    const fetchIdentities = React.useCallback(async () => {
        if (fetchedRef.current) return
        setLoading(true)
        try {
            const { data, error } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/sender-identities",
                {
                    params: {
                        path: { projectID: projectId },
                        query: { channel },
                    },
                },
            )
            if (error) throw error
            setIdentities(data?.results ?? [])
            fetchedRef.current = true
        } catch (err) {
            console.error("Failed to fetch sender identities:", err)
        } finally {
            setLoading(false)
        }
    }, [projectId, channel])

    // Fetch providers for the create view
    const fetchProviders = React.useCallback(async () => {
        setProvidersLoading(true)
        try {
            const { data } = await oapiClient.GET(
                "/api/admin/projects/{projectID}/providers",
                {
                    params: {
                        path: { projectID: projectId },
                    },
                },
            )
            const allProviders = data?.results ?? []
            const filtered = allProviders.filter((p) => p.channels?.includes(channel))
            setProviders(filtered)
            // Auto-select the first provider if only one exists
            if (filtered.length === 1) {
                setNewProviderId(filtered[0].id)
            }
        } finally {
            setProvidersLoading(false)
        }
    }, [projectId, channel])

    // Refetch when filters change
    React.useEffect(() => {
        fetchedRef.current = false
    }, [projectId, channel])

    // Fetch on mount when a value is already set so the display resolves
    React.useEffect(() => {
        if (value && !fetchedRef.current) {
            fetchIdentities()
        }
    }, [value, fetchIdentities])

    // Fetch when popover opens, reset view
    React.useEffect(() => {
        if (open) {
            fetchIdentities()
            setSearch("")
            setView("list")
            resetCreateForm()
        }
    }, [open, fetchIdentities])

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
        setNewAddress("")
        setNewName("")
        setNewProviderId("")
        setCreateError(null)
        setCreating(false)
    }

    const handleSwitchToCreate = () => {
        resetCreateForm()
        setView("create")
        fetchProviders()
    }

    const handleCancelCreate = () => {
        setView("list")
        resetCreateForm()
    }

    const handleCreate = async () => {
        const address = newAddress.trim()
        if (!newProviderId || !address) return

        setCreating(true)
        setCreateError(null)

        try {
            const traits: Record<string, unknown> = { address }
            if (isEmail && newName.trim()) {
                traits.name = newName.trim()
            }
            const { data: newIdentity, error } = await oapiClient.POST(
                "/api/admin/projects/{projectID}/sender-identities",
                {
                    params: { path: { projectID: projectId } },
                    body: { provider_id: newProviderId, channel, traits },
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
            setIdentities((prev) => [...prev, newIdentity])
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
                        loading={loading}
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
                        address={newAddress}
                        onAddressChange={setNewAddress}
                        name={newName}
                        onNameChange={setNewName}
                        providerId={newProviderId}
                        onProviderChange={setNewProviderId}
                        providers={providers}
                        providersLoading={providersLoading}
                        onSave={handleCreate}
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

interface ListViewProps {
    channel: "email" | "sms"
    loading: boolean
    identities: SenderIdentity[]
    allIdentities: SenderIdentity[]
    value?: string
    search: string
    onSearchChange: (value: string) => void
    onSelect: (id: string) => void
    onAddNew: () => void
    t: (key: string, fallback?: string) => string
}

function ListView({
    channel,
    loading,
    identities,
    allIdentities,
    value,
    search,
    onSearchChange,
    onSelect,
    onAddNew,
    t,
}: ListViewProps) {
    const isEmail = channel === "email"
    const Icon = isEmail ? Mail : Phone

    return (
        <div>
            <Command shouldFilter={false}>
                <CommandInput
                    placeholder={
                        isEmail
                            ? t("search_addresses", "Search addresses...")
                            : t("search_numbers", "Search numbers...")
                    }
                    value={search}
                    onValueChange={onSearchChange}
                />
                <CommandList>
                    {loading ? (
                        <div className="flex items-center justify-center py-6">
                            <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                        </div>
                    ) : (
                        <>
                            {identities.length === 0 && (
                                <div className="py-6 text-center">
                                    {allIdentities.length === 0 ? (
                                        <div className="flex flex-col items-center gap-1">
                                            <Icon className="h-5 w-5 text-muted-foreground/50" />
                                            <p className="text-sm text-muted-foreground">
                                                {t(
                                                    "no_sender_addresses_configured",
                                                    "No sender addresses configured yet.",
                                                )}
                                            </p>
                                            <p className="text-xs text-muted-foreground">
                                                {t("add_one_to_start", "Add one to start sending.")}
                                            </p>
                                        </div>
                                    ) : (
                                        <p className="text-sm text-muted-foreground">
                                            {t("no_matching_addresses", "No matching addresses.")}
                                        </p>
                                    )}
                                </div>
                            )}

                            {identities.length > 0 && (
                                <CommandGroup className="max-h-64 overflow-y-auto">
                                    {identities.map((identity) => (
                                        <CommandItem
                                            key={identity.id}
                                            value={identity.id}
                                            onSelect={() => onSelect(identity.id)}
                                            className="cursor-pointer"
                                        >
                                            <div className="flex items-center gap-2 w-full">
                                                <Check
                                                    className={cn(
                                                        "h-4 w-4 shrink-0",
                                                        value === identity.id
                                                            ? "opacity-100"
                                                            : "opacity-0",
                                                    )}
                                                />
                                                <span className="truncate flex-1 text-sm">
                                                    {isEmail && identity.traits?.name ? (
                                                        <span className="flex flex-col">
                                                            <span>
                                                                {identity.traits.name as string}
                                                            </span>
                                                            <span className="font-mono text-xs text-muted-foreground">
                                                                {identity.traits?.address as string}
                                                            </span>
                                                        </span>
                                                    ) : (
                                                        <span className="font-mono">
                                                            {identity.traits?.address as string}
                                                        </span>
                                                    )}
                                                </span>
                                            </div>
                                        </CommandItem>
                                    ))}
                                </CommandGroup>
                            )}
                        </>
                    )}
                </CommandList>
            </Command>
            <div className="border-t p-1">
                <button
                    type="button"
                    onClick={onAddNew}
                    className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground cursor-pointer"
                >
                    <Plus className="h-4 w-4" />
                    {t("add_new_address", "Add new address...")}
                </button>
            </div>
        </div>
    )
}

interface CreateViewProps {
    channel: "email" | "sms"
    address: string
    onAddressChange: (value: string) => void
    name: string
    onNameChange: (value: string) => void
    providerId: string
    onProviderChange: (value: string) => void
    providers: Provider[]
    providersLoading: boolean
    onSave: () => void
    onCancel: () => void
    creating: boolean
    error: string | null
    t: (key: string, fallback?: string) => string
}

function CreateView({
    channel,
    address,
    onAddressChange,
    name,
    onNameChange,
    providerId,
    onProviderChange,
    providers,
    providersLoading,
    onSave,
    onCancel,
    creating,
    error,
    t,
}: CreateViewProps) {
    const isEmail = channel === "email"

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (e.key === "Enter") {
            e.preventDefault()
            if (address.trim() && providerId) handleSave()
        }
        if (e.key === "Escape") {
            e.preventDefault()
            onCancel()
        }
    }

    const handleSave = () => {
        if (!address.trim() || !providerId || creating) return
        onSave()
    }

    return (
        <div className="p-3 space-y-3">
            <div className="flex items-center gap-2">
                <button
                    type="button"
                    onClick={onCancel}
                    className="p-1 rounded-sm hover:bg-accent text-muted-foreground cursor-pointer"
                >
                    <ArrowLeft className="h-4 w-4" />
                </button>
                <span className="text-sm font-medium">
                    {isEmail
                        ? t("add_email_address", "Add email address")
                        : t("add_phone_number", "Add phone number")}
                </span>
            </div>

            <div className="space-y-3">
                <div className="space-y-1">
                    <label
                        htmlFor="new-sender-provider"
                        className="text-xs font-medium text-foreground"
                    >
                        {t("integration", "Integration")}
                    </label>
                    {providersLoading ? (
                        <div className="flex items-center gap-2 h-8 px-3 text-sm text-muted-foreground">
                            <Loader2 className="h-3 w-3 animate-spin" />
                            {t("loading", "Loading...")}
                        </div>
                    ) : (
                        <Select value={providerId} onValueChange={onProviderChange}>
                            <SelectTrigger className="h-8 w-full">
                                <SelectValue
                                    placeholder={t(
                                        "select_integration",
                                        "Select integration...",
                                    )}
                                />
                            </SelectTrigger>
                            <SelectContent>
                                {providers.length === 0 ? (
                                    <div className="py-2 px-2 text-sm text-muted-foreground">
                                        {t(
                                            "no_integrations_found",
                                            "No integrations found for this channel.",
                                        )}
                                    </div>
                                ) : (
                                    providers.map((provider) => (
                                        <SelectItem key={provider.id} value={provider.id}>
                                            {provider.name}
                                        </SelectItem>
                                    ))
                                )}
                            </SelectContent>
                        </Select>
                    )}
                </div>
                {isEmail && (
                    <div className="space-y-1">
                        <label
                            htmlFor="new-sender-name"
                            className="text-xs font-medium text-foreground"
                        >
                            {t("from_name", "From name")}
                        </label>
                        <Input
                            id="new-sender-name"
                            type="text"
                            value={name}
                            onChange={(e) => onNameChange(e.target.value)}
                            onKeyDown={handleKeyDown}
                            placeholder={t("from_name_placeholder", "e.g. Acme Support")}
                            className="h-8"
                        />
                    </div>
                )}
                <div className="space-y-1">
                    <label
                        htmlFor="new-sender-address"
                        className="text-xs font-medium text-foreground"
                    >
                        {isEmail
                            ? t("email_address", "Email address")
                            : t("phone_number", "Phone number")}
                    </label>
                    <Input
                        id="new-sender-address"
                        type={isEmail ? "email" : "tel"}
                        value={address}
                        onChange={(e) => {
                            onAddressChange(e.target.value)
                        }}
                        onKeyDown={handleKeyDown}
                        placeholder={isEmail ? "hello@example.com" : "+1234567890"}
                        autoFocus
                        className="h-8"
                    />
                </div>
            </div>

            {error && <p className="text-xs text-destructive">{error}</p>}

            <Button
                type="button"
                size="sm"
                onClick={handleSave}
                disabled={!address.trim() || !providerId || creating}
                className="h-8"
            >
                {creating && <Loader2 className="h-3 w-3 animate-spin mr-1.5" />}
                {isEmail
                    ? t("add_email_address", "Add email address")
                    : t("add_phone_number", "Add phone number")}
            </Button>
        </div>
    )
}
