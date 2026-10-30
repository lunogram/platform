import type * as React from "react"
import { ArrowLeft, Loader2 } from "lucide-react"
import type { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import type { components } from "@/oapi/management.generated"

type Provider = components["schemas"]["Provider"]

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
    /** Set when the providers failed to load; retries loading them. */
    onRetryProviders?: () => void
    onSave: () => void
    onCancel: () => void
    creating: boolean
    error: string | null
    t: ReturnType<typeof useTranslation>["t"]
}

export function CreateView({
    channel,
    address,
    onAddressChange,
    name,
    onNameChange,
    providerId,
    onProviderChange,
    providers,
    providersLoading,
    onRetryProviders,
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
                    {onRetryProviders ? (
                        <div className="flex items-center justify-between gap-2 h-8 text-sm">
                            <span className="text-destructive">
                                {t("integrations_load_failed", "Couldn't load integrations.")}
                            </span>
                            <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                className="h-7"
                                onClick={onRetryProviders}
                            >
                                {t("retry", "Retry")}
                            </Button>
                        </div>
                    ) : providersLoading ? (
                        <div className="flex items-center gap-2 h-8 px-3 text-sm text-muted-foreground">
                            <Loader2 className="h-3 w-3 animate-spin" />
                            {t("loading", "Loading...")}
                        </div>
                    ) : (
                        <Select value={providerId} onValueChange={onProviderChange}>
                            <SelectTrigger className="h-8 w-full">
                                <SelectValue
                                    placeholder={t("select_integration", "Select integration...")}
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
