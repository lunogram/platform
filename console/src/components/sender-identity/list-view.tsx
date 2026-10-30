import { Check, Loader2, Mail, Phone, Plus } from "lucide-react"
import type { useTranslation } from "react-i18next"
import { cn } from "@/utils"
import { Button } from "@/components/ui/button"
import {
    Command,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import type { SenderIdentity } from "@/oapi/client"

interface ListViewProps {
    channel: "email" | "sms"
    loading: boolean
    /** Set when the identities failed to load; retries loading them. */
    onRetry?: () => void
    identities: SenderIdentity[]
    allIdentities: SenderIdentity[]
    value?: string
    search: string
    onSearchChange: (value: string) => void
    onSelect: (id: string) => void
    onAddNew: () => void
    t: ReturnType<typeof useTranslation>["t"]
}

export function ListView({
    channel,
    loading,
    onRetry,
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
                    {onRetry ? (
                        <div className="flex flex-col items-center gap-2 py-6 text-center">
                            <p className="text-sm text-destructive">
                                {t(
                                    "sender_identities_load_failed",
                                    "Couldn't load sender addresses.",
                                )}
                            </p>
                            <Button type="button" variant="outline" size="sm" onClick={onRetry}>
                                {t("retry", "Retry")}
                            </Button>
                        </div>
                    ) : loading ? (
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
