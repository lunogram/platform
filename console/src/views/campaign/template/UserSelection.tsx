import { useState, useCallback, useEffect } from "react"
import type { User } from "@/types"
import { cn } from "@/utils"
import { oapiClient } from "@/oapi/client"

import { Check, ChevronsUpDown } from "lucide-react"
import { Button } from "@/components/ui/button"

import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"

interface UserSelectionProps {
    projectId: string
    value?: User | null
    onChange?: (user: User) => void
    /** Render a compact trigger button (h-7, text-xs). Defaults to "default". */
    size?: "default" | "sm"
    /** Accessible label for the trigger button (combobox). */
    ariaLabel?: string
    /** Accessible label for the search input inside the popover. */
    searchInputAriaLabel?: string
}

export function UserSelection({
    projectId,
    value,
    onChange,
    size = "default",
    ariaLabel = "Select user",
    searchInputAriaLabel = "Search users",
}: UserSelectionProps) {
    const [open, setOpen] = useState(false)
    const [search, setSearch] = useState("")
    const [users, setUsers] = useState<User[]>([])

    const fetchUsers = useCallback(async () => {
        const { data } = await oapiClient.GET("/api/admin/projects/{projectID}/subjects/users", {
            params: {
                path: { projectID: projectId },
                query: { search: search || undefined, limit: 50 },
            },
        })

        setUsers((data?.results ?? []) as User[])
    }, [projectId, search])

    useEffect(() => {
        const handler = setTimeout(() => {
            fetchUsers()
        }, 200)

        return () => clearTimeout(handler)
    }, [search, fetchUsers])

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    aria-label={ariaLabel}
                    className={cn(
                        "max-w-sm justify-between min-w-0",
                        size === "sm" && "h-8 text-xs px-2.5 [&_svg]:size-3.5",
                    )}
                >
                    <span className="truncate">{value ? value.email : "Select user..."}</span>
                    <ChevronsUpDown className="opacity-50 shrink-0" />
                </Button>
            </PopoverTrigger>

            <PopoverContent className="w-(--radix-popover-trigger-width) min-w-[280px] p-0">
                <Command>
                    <CommandInput
                        placeholder="Search user..."
                        aria-label={searchInputAriaLabel}
                        className="h-9"
                        value={search}
                        onValueChange={setSearch}
                    />
                    <CommandList>
                        <CommandEmpty>No user found.</CommandEmpty>
                        <CommandGroup>
                            {users.map((user) => (
                                <CommandItem
                                    key={user.id}
                                    value={user.email}
                                    onSelect={() => {
                                        onChange?.(user)
                                        setOpen(false)
                                    }}
                                >
                                    {user.email}
                                    <Check
                                        className={cn(
                                            "ml-auto",
                                            value?.id === user.id ? "opacity-100" : "opacity-0",
                                        )}
                                    />
                                </CommandItem>
                            ))}
                        </CommandGroup>
                    </CommandList>
                </Command>
            </PopoverContent>
        </Popover>
    )
}
