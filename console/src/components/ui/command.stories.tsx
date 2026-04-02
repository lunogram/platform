import type { Meta, StoryObj } from "@storybook/react"
import {
    Command,
    CommandInput,
    CommandList,
    CommandEmpty,
    CommandGroup,
    CommandItem,
    CommandShortcut,
    CommandSeparator,
} from "./command"

const meta: Meta<typeof Command> = {
    component: Command,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Command>

export const Default: Story = {
    render: () => (
        <Command style={{ border: "1px solid hsl(var(--border))", borderRadius: "8px", width: "400px" }}>
            <CommandInput placeholder="Type a command or search..." />
            <CommandList>
                <CommandEmpty>No results found.</CommandEmpty>
                <CommandGroup heading="Suggestions">
                    <CommandItem>
                        Calendar
                        <CommandShortcut>⌘C</CommandShortcut>
                    </CommandItem>
                    <CommandItem>
                        Search
                        <CommandShortcut>⌘S</CommandShortcut>
                    </CommandItem>
                    <CommandItem>Settings</CommandItem>
                </CommandGroup>
                <CommandSeparator />
                <CommandGroup heading="Actions">
                    <CommandItem>New File</CommandItem>
                    <CommandItem>New Folder</CommandItem>
                </CommandGroup>
            </CommandList>
        </Command>
    ),
}

export const EmptyState: Story = {
    render: () => (
        <Command style={{ border: "1px solid hsl(var(--border))", borderRadius: "8px", width: "400px" }}>
            <CommandInput placeholder="Search for something rare..." defaultValue="xyzzy" />
            <CommandList>
                <CommandEmpty>No results found.</CommandEmpty>
            </CommandList>
        </Command>
    ),
}
