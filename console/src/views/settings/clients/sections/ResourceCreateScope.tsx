import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Loader2, Plus, SlidersHorizontal, X } from "lucide-react"

import type { RestrictableResource } from "../model"
import { cn } from "@/utils"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import {
    Command,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"

// English defaults, kept here so the i18n key and its fallback live together. The
// keys (client_scope_<resource>_*) are registered in public/locales/en.json.
const COPY: Record<RestrictableResource, { title: string; help: string; search: string }> = {
    events: {
        title: "Allowed event names",
        help: "Only these event names may be emitted. Caps the blast radius of a leaked key forging events to trigger journeys.",
        search: "Search or add an event name…",
    },
    subscriptions: {
        title: "Allowed subscriptions",
        help: "Only these subscriptions may be created or changed by this client.",
        search: "Search or add a subscription…",
    },
    scheduled: {
        title: "Allowed scheduled events",
        help: "Only these scheduled event names may be created or changed by this client.",
        search: "Search or add a scheduled event…",
    },
}

// ResourceCreateScope is the inline scope pill shown on a restrictable resource's
// row once it has create access. It summarizes the current scope ("Any" or
// "Limited to N") and opens a popover to pick existing names or create new ones.
// An empty list means unrestricted — there is no separate toggle. The stored
// values are names (the unit the backend allow-list enforces on), not record IDs;
// existing names are offered as suggestions, but any new name can be added.
export function ResourceCreateScope({
    resource,
    names,
    onChange,
    loadSuggestions,
}: {
    resource: RestrictableResource
    names: string[] | undefined
    onChange: (names: string[] | undefined) => void
    loadSuggestions: () => Promise<string[]>
}) {
    const { t } = useTranslation()
    const [open, setOpen] = useState(false)
    const [query, setQuery] = useState("")
    const [suggestions, setSuggestions] = useState<string[] | null>(null)
    const loadedRef = useRef(false)

    const copy = COPY[resource]
    const list = names ?? []
    const restricted = list.length > 0

    // Lazily fetch existing names the first time the popover opens.
    useEffect(() => {
        if (!open || loadedRef.current) return
        loadedRef.current = true
        let active = true
        loadSuggestions()
            .then((s) => active && setSuggestions(s))
            .catch(() => active && setSuggestions([]))
        return () => {
            active = false
        }
    }, [open, loadSuggestions])

    // Empty list collapses back to "unrestricted" so we never persist an
    // allow-nothing grant (that's just not granting create).
    const set = (next: string[]) => onChange(next.length > 0 ? next : undefined)
    const add = (name: string) => {
        const n = name.trim()
        if (!n || list.includes(n)) return
        set([...list, n])
        setQuery("")
    }
    const remove = (name: string) => set(list.filter((x) => x !== name))

    const q = query.trim().toLowerCase()
    const loading = suggestions === null
    const available = (suggestions ?? []).filter(
        (s) => !list.includes(s) && (!q || s.toLowerCase().includes(q)),
    )
    const canCreate =
        q.length > 0 &&
        !list.some((x) => x.toLowerCase() === q) &&
        !(suggestions ?? []).some((s) => s.toLowerCase() === q)

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <button
                    type="button"
                    className={cn(
                        "inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs transition-colors",
                        restricted
                            ? "border-transparent bg-surface-muted font-medium text-foreground"
                            : "border-dashed text-ink-soft hover:border-solid hover:text-foreground",
                    )}
                >
                    <SlidersHorizontal className="h-3 w-3" strokeWidth={1.75} />
                    {restricted
                        ? t("client_scope_limited_to", {
                              count: list.length,
                              defaultValue: "Limited to {{count}}",
                          })
                        : t("client_scope_any", "Any")}
                </button>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-80 p-0">
                <div className="border-b px-3.5 py-3">
                    <p className="text-sm font-medium">
                        {t(`client_scope_${resource}_title`, copy.title)}
                    </p>
                    <p className="mt-0.5 text-xs text-ink-soft">
                        {t(`client_scope_${resource}_help`, copy.help)}
                    </p>
                </div>

                {restricted && (
                    <div className="flex flex-wrap gap-1.5 border-b px-3.5 py-2.5">
                        {list.map((name) => (
                            <span
                                key={name}
                                className="inline-flex items-center gap-1 rounded bg-surface-muted px-1.5 py-0.5 font-mono text-xs"
                            >
                                {name}
                                <button
                                    type="button"
                                    aria-label={`${t("remove", "Remove")} ${name}`}
                                    className="text-ink-soft hover:text-foreground"
                                    onClick={() => remove(name)}
                                >
                                    <X className="h-3 w-3" />
                                </button>
                            </span>
                        ))}
                    </div>
                )}

                <Command shouldFilter={false}>
                    <CommandInput
                        value={query}
                        onValueChange={setQuery}
                        placeholder={t(`client_scope_${resource}_search`, copy.search)}
                    />
                    <CommandList>
                        {loading ? (
                            <div className="flex items-center justify-center py-6">
                                <Loader2 className="h-4 w-4 animate-spin text-ink-soft" />
                            </div>
                        ) : (
                            <>
                                {available.length > 0 && (
                                    <CommandGroup>
                                        {available.map((name) => (
                                            <CommandItem
                                                key={name}
                                                value={name}
                                                onSelect={() => add(name)}
                                                className="cursor-pointer font-mono text-sm"
                                            >
                                                {name}
                                            </CommandItem>
                                        ))}
                                    </CommandGroup>
                                )}
                                {canCreate && (
                                    <CommandItem
                                        value={`__create__${query}`}
                                        onSelect={() => add(query)}
                                        className="cursor-pointer"
                                    >
                                        <Plus className="mr-2 h-4 w-4" />
                                        {t("client_scope_create_named", {
                                            name: query.trim(),
                                            defaultValue: 'Create "{{name}}"',
                                        })}
                                    </CommandItem>
                                )}
                                {available.length === 0 && !canCreate && (
                                    <div className="py-6 text-center text-sm text-ink-soft">
                                        {t("no_results", "No results")}
                                    </div>
                                )}
                            </>
                        )}
                    </CommandList>
                </Command>

                <div className="border-t px-3.5 py-2">
                    <p className="text-xs text-ink-soft">
                        {restricted
                            ? t(
                                  "client_scope_hint_limited",
                                  "Only these may be created. Clear the list to allow any.",
                              )
                            : t(
                                  "client_scope_hint_open",
                                  "Add names to limit what this client can create.",
                              )}
                    </p>
                </div>
            </PopoverContent>
        </Popover>
    )
}
