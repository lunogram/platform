import { useState, useCallback, useEffect } from "react"
import type { User } from "@/types"
import { cn } from "@/utils"
import api from "@/api"

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
}

export function UserSelection({ projectId, value, onChange }: UserSelectionProps) {
    const [open, setOpen] = useState(false)
    const [search, setSearch] = useState("")
    const [users, setUsers] = useState<User[]>([])

    const fetchUsers = useCallback(async () => {
        const users = await api.users.search(projectId, {
            search: search,
            limit: 50,
        })

        setUsers(users.results)
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
                    className="w-[200px] justify-between"
                >
                    {value ? value.email : "Select user..."}
                    <ChevronsUpDown className="opacity-50" />
                </Button>
            </PopoverTrigger>

            <PopoverContent className="w-[200px] p-0">
                <Command>
                    <CommandInput
                        placeholder="Search user..."
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
