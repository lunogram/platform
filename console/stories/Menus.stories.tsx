import type { Meta, StoryObj } from "@storybook/react-vite"
import {
    Archive,
    Copy,
    Mail,
    Megaphone,
    MoreHorizontal,
    Route,
    Settings,
    Users,
} from "lucide-react"
import { useState } from "react"

import { Button } from "@/components/ui/button"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
    CommandSeparator,
    CommandShortcut,
} from "@/components/ui/command"
import {
    DropdownMenu,
    DropdownMenuCheckboxItem,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuRadioGroup,
    DropdownMenuRadioItem,
    DropdownMenuSeparator,
    DropdownMenuShortcut,
    DropdownMenuSub,
    DropdownMenuSubContent,
    DropdownMenuSubTrigger,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"

const meta: Meta = {
    title: "UI/Menus",
    parameters: { layout: "fullscreen" },
}

export default meta
type Story = StoryObj

export const RowActions: Story = {
    name: "Dropdown menu / row actions",
    render: () => (
        <div className="min-h-[260px] p-4">
            <DropdownMenu defaultOpen modal={false}>
                <DropdownMenuTrigger asChild>
                    <Button variant="ghost" size="sm" className="h-8 w-8 p-0" aria-label="Actions">
                        <MoreHorizontal className="h-4 w-4" />
                    </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                    <DropdownMenuItem>
                        <Mail className="mr-2 h-4 w-4" />
                        Edit
                    </DropdownMenuItem>
                    <DropdownMenuItem>
                        <Copy className="mr-2 h-4 w-4" />
                        Duplicate
                        <DropdownMenuShortcut>⌘D</DropdownMenuShortcut>
                    </DropdownMenuItem>
                    <DropdownMenuItem disabled>Export (disabled)</DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem className="text-destructive">
                        <Archive className="mr-2 h-4 w-4" />
                        Archive
                    </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>
        </div>
    ),
}

function OptionsMenu() {
    const [columns, setColumns] = useState({ delivery: true, updated: true, created: false })
    const [sort, setSort] = useState("updated")
    return (
        <DropdownMenu defaultOpen modal={false}>
            <DropdownMenuTrigger asChild>
                <Button variant="outline">View options</Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-56">
                <DropdownMenuLabel>Columns</DropdownMenuLabel>
                <DropdownMenuCheckboxItem
                    checked={columns.delivery}
                    onCheckedChange={(delivery) => setColumns({ ...columns, delivery })}
                >
                    Delivery
                </DropdownMenuCheckboxItem>
                <DropdownMenuCheckboxItem
                    checked={columns.updated}
                    onCheckedChange={(updated) => setColumns({ ...columns, updated })}
                >
                    Updated at
                </DropdownMenuCheckboxItem>
                <DropdownMenuCheckboxItem
                    checked={columns.created}
                    onCheckedChange={(created) => setColumns({ ...columns, created })}
                >
                    Created at
                </DropdownMenuCheckboxItem>
                <DropdownMenuSeparator />
                <DropdownMenuLabel>Sort by</DropdownMenuLabel>
                <DropdownMenuRadioGroup value={sort} onValueChange={setSort}>
                    <DropdownMenuRadioItem value="name">Name</DropdownMenuRadioItem>
                    <DropdownMenuRadioItem value="updated">Updated at</DropdownMenuRadioItem>
                </DropdownMenuRadioGroup>
                <DropdownMenuSeparator />
                <DropdownMenuSub>
                    <DropdownMenuSubTrigger>Move to project</DropdownMenuSubTrigger>
                    <DropdownMenuSubContent>
                        <DropdownMenuItem>Northwind Mobile</DropdownMenuItem>
                        <DropdownMenuItem>Northwind Web</DropdownMenuItem>
                    </DropdownMenuSubContent>
                </DropdownMenuSub>
            </DropdownMenuContent>
        </DropdownMenu>
    )
}

export const Options: Story = {
    name: "Dropdown menu / checkbox, radio, submenu",
    render: () => (
        <div className="min-h-[360px] p-4">
            <OptionsMenu />
        </div>
    ),
}

export const PopoverStory: Story = {
    name: "Popover",
    render: () => (
        <div className="min-h-[220px] p-4">
            <Popover defaultOpen>
                <PopoverTrigger asChild>
                    <Button variant="outline">Send test</Button>
                </PopoverTrigger>
                <PopoverContent align="start">
                    <div className="grid gap-3">
                        <div className="space-y-1">
                            <h4 className="font-medium leading-none">Send a test</h4>
                            <p className="text-sm text-muted-foreground">
                                Delivered with sample variables.
                            </p>
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="test-recipient">Recipient</Label>
                            <Input id="test-recipient" defaultValue="jane@example.com" />
                        </div>
                        <Button size="sm">Send</Button>
                    </div>
                </PopoverContent>
            </Popover>
        </div>
    ),
}

export const TooltipStory: Story = {
    name: "Tooltip",
    render: () => (
        <div className="p-12">
            <Tooltip defaultOpen>
                <TooltipTrigger asChild>
                    <Button variant="outline" size="icon" aria-label="Settings">
                        <Settings />
                    </Button>
                </TooltipTrigger>
                <TooltipContent>Project settings</TooltipContent>
            </Tooltip>
        </div>
    ),
}

export const CommandStory: Story = {
    name: "Command",
    render: () => (
        <div className="p-4">
            <Command className="max-w-sm rounded-lg border shadow-md">
                <CommandInput placeholder="Type a command or search..." />
                <CommandList>
                    <CommandEmpty>No results found.</CommandEmpty>
                    <CommandGroup heading="Navigate">
                        <CommandItem>
                            <Megaphone />
                            Campaigns
                            <CommandShortcut>G C</CommandShortcut>
                        </CommandItem>
                        <CommandItem>
                            <Route />
                            Journeys
                            <CommandShortcut>G J</CommandShortcut>
                        </CommandItem>
                        <CommandItem>
                            <Users />
                            Users
                            <CommandShortcut>G U</CommandShortcut>
                        </CommandItem>
                    </CommandGroup>
                    <CommandSeparator />
                    <CommandGroup heading="Settings">
                        <CommandItem>
                            <Settings />
                            Project settings
                        </CommandItem>
                    </CommandGroup>
                </CommandList>
            </Command>
        </div>
    ),
}
