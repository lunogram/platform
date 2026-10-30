import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { useQuery } from "@tanstack/react-query"
import { ArrowLeft, Check, ChevronsUpDown, ListChecks, Loader2, Plus, Zap } from "lucide-react"
import type { RulePath } from "@/types"
import type { UUID } from "@/types/common"
import { cn } from "@/utils"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Command, CommandInput, CommandItem, CommandList } from "@/components/ui/command"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { listSearchQuery } from "../queries"

type EventOption = Pick<RulePath, "name" | "path">
type EventComboboxView = "list" | "create"

interface EventNameComboboxProps {
    value?: string
    onChange: (eventName: string) => void
    onSearch: (query: string) => Promise<EventOption[]>
}

export function EventNameCombobox({ value, onChange, onSearch }: EventNameComboboxProps) {
    const { t } = useTranslation()
    const [open, setOpen] = useState(false)
    const [view, setView] = useState<EventComboboxView>("list")
    const [search, setSearch] = useState("")
    const [results, setResults] = useState<EventOption[]>([])
    const [loading, setLoading] = useState(false)
    const [failed, setFailed] = useState(false)
    const [newEventName, setNewEventName] = useState("")
    const debounceRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

    const resetCreateForm = useCallback((seedName = "") => {
        setNewEventName(seedName)
    }, [])

    useEffect(() => {
        if (!open || view !== "list") return
        if (debounceRef.current) clearTimeout(debounceRef.current)

        // A newer search (or closing) cancels this one, so a slow response for
        // an earlier query can't replace the results of the current one.
        let current = true
        setLoading(true)
        debounceRef.current = setTimeout(async () => {
            try {
                const found = await onSearch(search)
                if (current) {
                    setResults(found)
                    setFailed(false)
                }
            } catch {
                if (current) setFailed(true)
            } finally {
                if (current) setLoading(false)
            }
        }, 250)

        return () => {
            current = false
            if (debounceRef.current) clearTimeout(debounceRef.current)
        }
    }, [open, view, search, onSearch])

    useEffect(() => {
        if (!open) return
        setView("list")
        setSearch("")
        resetCreateForm()
    }, [open, resetCreateForm])

    const handleSelect = (eventName: string) => {
        onChange(eventName)
        setOpen(false)
    }

    const handleSwitchToCreate = () => {
        resetCreateForm(search || value || "")
        setView("create")
    }

    const handleUseEvent = () => {
        const name = newEventName.trim()
        if (!name) return
        onChange(name)
        setOpen(false)
    }

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    type="button"
                    className={cn(
                        "h-9 w-full justify-between shadow-none font-normal",
                        !value && "text-muted-foreground",
                    )}
                >
                    <span className="flex items-center gap-2 truncate">
                        <Zap className="h-4 w-4 shrink-0 text-muted-foreground" />
                        <span className="truncate font-mono text-sm">
                            {value || t("select_or_create_event", "Select or create event...")}
                        </span>
                    </span>
                    <ChevronsUpDown className="h-4 w-4 shrink-0 text-muted-foreground" />
                </Button>
            </PopoverTrigger>
            <PopoverContent
                className="w-[var(--radix-popover-trigger-width)] p-0"
                align="start"
                onOpenAutoFocus={(e) => e.preventDefault()}
            >
                {view === "list" ? (
                    <div>
                        <Command shouldFilter={false}>
                            <CommandInput
                                placeholder={t(
                                    "search_or_type_event",
                                    "Search or type an event...",
                                )}
                                value={search}
                                onValueChange={setSearch}
                            />
                            <CommandList>
                                {loading ? (
                                    <div className="flex items-center justify-center py-6">
                                        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                                    </div>
                                ) : failed ? (
                                    <div className="py-6 text-center text-sm text-muted-foreground">
                                        {t("journey_events_load_failed", "Couldn't load events.")}
                                    </div>
                                ) : results.length === 0 ? (
                                    <div className="py-6 text-center text-sm text-muted-foreground">
                                        {t("no_events_found", "No events found.")}
                                    </div>
                                ) : (
                                    <div className="max-h-64 overflow-y-auto p-1">
                                        {results.map((event) => (
                                            <CommandItem
                                                key={event.path}
                                                value={event.path}
                                                onSelect={() => handleSelect(event.name)}
                                                className="cursor-pointer"
                                            >
                                                <Check
                                                    className={cn(
                                                        "mr-2 h-4 w-4 shrink-0",
                                                        value === event.name
                                                            ? "opacity-100"
                                                            : "opacity-0",
                                                    )}
                                                />
                                                <span className="truncate font-mono text-sm">
                                                    {event.name}
                                                </span>
                                            </CommandItem>
                                        ))}
                                    </div>
                                )}
                            </CommandList>
                        </Command>
                        <div className="border-t p-1">
                            <button
                                type="button"
                                onClick={handleSwitchToCreate}
                                className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground cursor-pointer"
                            >
                                <Plus className="h-4 w-4" />
                                {t("create_event_inline", "Create event...")}
                            </button>
                        </div>
                    </div>
                ) : (
                    <div className="p-3 space-y-3">
                        <div className="flex items-center gap-2">
                            <button
                                type="button"
                                onClick={() => setView("list")}
                                className="p-1 rounded-sm hover:bg-accent text-muted-foreground cursor-pointer"
                            >
                                <ArrowLeft className="h-4 w-4" />
                            </button>
                            <span className="text-sm font-medium">
                                {t("create_event", "Create Event")}
                            </span>
                        </div>

                        <div className="space-y-1">
                            <Label htmlFor="new-entrance-event" className="text-xs font-medium">
                                {t("event_name", "Event Name")}
                            </Label>
                            <Input
                                id="new-entrance-event"
                                value={newEventName}
                                onChange={(e) => setNewEventName(e.target.value)}
                                onKeyDown={(e) => {
                                    if (e.key === "Enter") {
                                        e.preventDefault()
                                        handleUseEvent()
                                    }
                                    if (e.key === "Escape") {
                                        e.preventDefault()
                                        setView("list")
                                    }
                                }}
                                placeholder="product.purchased"
                                autoFocus
                                className="h-8 font-mono"
                            />
                        </div>

                        <p className="text-xs text-muted-foreground">
                            {t(
                                "create_entrance_event_hint",
                                "This adds the event name to the entrance. Save the draft to persist it for this project.",
                            )}
                        </p>

                        <Button
                            type="button"
                            size="sm"
                            onClick={handleUseEvent}
                            disabled={!newEventName.trim()}
                            className="h-8"
                        >
                            {t("create", "Create")}
                        </Button>
                    </div>
                )}
            </PopoverContent>
        </Popover>
    )
}

interface ListSelectorProps {
    projectId: UUID
    value?: UUID
    displayName?: string
    onChange: (list: { id: UUID; name: string }) => void
}

export function ListSelector({ projectId, value, displayName, onChange }: ListSelectorProps) {
    const { t } = useTranslation()
    const [open, setOpen] = useState(false)
    const [search, setSearch] = useState("")
    const [debouncedSearch, setDebouncedSearch] = useState("")

    useEffect(() => {
        const timeout = setTimeout(() => setDebouncedSearch(search), 250)
        return () => clearTimeout(timeout)
    }, [search])

    // The search is in the query key, so only the current search's lists show.
    const lists = useQuery({ ...listSearchQuery(projectId, debouncedSearch), enabled: open })
    const results = lists.data ?? []
    const loading = lists.isPending || search !== debouncedSearch

    useEffect(() => {
        if (!open) setSearch("")
    }, [open])

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    type="button"
                    className={cn(
                        "h-9 w-full justify-between shadow-none font-normal",
                        !value && "text-muted-foreground",
                    )}
                >
                    <span className="flex items-center gap-2 truncate">
                        <ListChecks className="h-4 w-4 shrink-0 text-muted-foreground" />
                        <span className="truncate text-sm">
                            {displayName || t("select_list", "Select a list...")}
                        </span>
                    </span>
                    <ChevronsUpDown className="h-4 w-4 shrink-0 text-muted-foreground" />
                </Button>
            </PopoverTrigger>
            <PopoverContent
                className="w-[var(--radix-popover-trigger-width)] p-0"
                align="start"
                onOpenAutoFocus={(e) => e.preventDefault()}
            >
                <Command shouldFilter={false}>
                    <CommandInput
                        placeholder={t("search_lists", "Search lists...")}
                        value={search}
                        onValueChange={setSearch}
                    />
                    <CommandList>
                        {loading ? (
                            <div className="flex items-center justify-center py-6">
                                <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                            </div>
                        ) : lists.isError ? (
                            <div className="flex flex-col items-center gap-2 py-6 text-sm text-muted-foreground">
                                {t("journey_lists_load_failed", "Couldn't load lists.")}
                                <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    onClick={() => lists.refetch()}
                                >
                                    {t("retry", "Retry")}
                                </Button>
                            </div>
                        ) : results.length === 0 ? (
                            <div className="py-6 text-center text-sm text-muted-foreground">
                                {t("no_lists_found", "No lists found.")}
                            </div>
                        ) : (
                            <div className="max-h-64 overflow-y-auto p-1">
                                {results.map((list) => (
                                    <CommandItem
                                        key={list.id}
                                        value={list.id}
                                        onSelect={() => {
                                            onChange({ id: list.id, name: list.name })
                                            setOpen(false)
                                        }}
                                        className="cursor-pointer"
                                    >
                                        <Check
                                            className={cn(
                                                "mr-2 h-4 w-4 shrink-0",
                                                value === list.id ? "opacity-100" : "opacity-0",
                                            )}
                                        />
                                        <span className="truncate text-sm">{list.name}</span>
                                        <Badge variant="secondary" className="ml-auto shrink-0">
                                            {list.type}
                                        </Badge>
                                    </CommandItem>
                                ))}
                            </div>
                        )}
                    </CommandList>
                </Command>
            </PopoverContent>
        </Popover>
    )
}
