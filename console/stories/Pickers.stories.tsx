import type { Meta, StoryObj } from "@storybook/react-vite"
import { CalendarClock, Hash } from "lucide-react"
import { useState } from "react"
import * as z from "zod"

import { ColorPicker } from "@/components/ui/color-picker"
import { Combobox } from "@/components/ui/combobox"
import { DateTimeEdit } from "@/components/ui/datetime-edit"
import { InlineEdit } from "@/components/ui/inline-edit"
import { Label } from "@/components/ui/label"
import type { MultiSelectOption } from "@/components/ui/multi-select"
import { MultiSelect } from "@/components/ui/multi-select"

const meta: Meta = {
    title: "UI/Pickers",
}

export default meta
type Story = StoryObj

const paths = [
    { path: "user.email" },
    { path: "user.first_name" },
    { path: "user.last_name" },
    { path: "user.phone" },
    { path: "event.order.total" },
    { path: "event.order.currency" },
]

// Resolves from fixtures; stands in for a path-suggestion endpoint.
async function searchPaths(query: string) {
    return paths.filter((option) => option.path.includes(query.toLowerCase()))
}

function ComboboxDemo() {
    const [value, setValue] = useState("user.email")
    const [asyncValue, setAsyncValue] = useState("")
    return (
        <div className="flex max-w-sm flex-col gap-4">
            <div className="grid gap-2">
                <Label>Static options</Label>
                <Combobox options={paths} value={value} onValueChange={setValue} />
            </div>
            <div className="grid gap-2">
                <Label>Async search</Label>
                <Combobox
                    onSearch={searchPaths}
                    value={asyncValue}
                    displayValue={asyncValue}
                    onValueChange={setAsyncValue}
                    placeholder="Search user paths..."
                />
            </div>
            <div className="grid gap-2">
                <Label>Disabled</Label>
                <Combobox options={paths} value="user.phone" onValueChange={() => {}} disabled />
            </div>
        </div>
    )
}

export const ComboboxStory: Story = {
    name: "Combobox",
    render: () => <ComboboxDemo />,
}

const lists: MultiSelectOption[] = [
    { value: "newsletter", label: "Newsletter" },
    { value: "beta", label: "Beta testers" },
    { value: "vip", label: "VIP customers" },
    { value: "churn-risk", label: "Churn risk" },
    { value: "trial", label: "Trial users" },
]

function MultiSelectDemo({ initial, disabled }: { initial: string[]; disabled?: boolean }) {
    const [value, setValue] = useState(initial)
    return (
        <MultiSelect
            options={lists}
            value={value}
            onChange={setValue}
            placeholder="Select lists..."
            disabled={disabled}
        />
    )
}

export const MultiSelectStory: Story = {
    name: "Multi select",
    render: () => (
        <div className="flex max-w-sm flex-col gap-4">
            <MultiSelectDemo initial={[]} />
            <MultiSelectDemo initial={["newsletter", "vip"]} />
            <MultiSelectDemo initial={lists.map((list) => list.value)} />
            <MultiSelectDemo initial={["beta"]} disabled />
        </div>
    ),
}

function ColorPickerDemo({ inline }: { inline?: boolean }) {
    const [color, setColor] = useState("#2970ff")
    return (
        <ColorPicker
            value={color}
            onChange={setColor}
            onClear={() => setColor("")}
            inline={inline}
        />
    )
}

export const ColorPickerStory: Story = {
    name: "Color picker",
    render: () => (
        <div className="flex items-start gap-8">
            <ColorPickerDemo />
            <div className="w-64">
                <ColorPickerDemo inline />
            </div>
        </div>
    ),
}

const resolveSave = () => Promise.resolve()

export const InlineEditing: Story = {
    name: "Inline edit",
    render: () => (
        <div className="flex flex-col items-start gap-4 text-sm">
            <InlineEdit value="Welcome series" onSave={resolveSave} aria-label="Edit name" />
            <InlineEdit
                value=""
                placeholder="Add an external ID"
                icon={<Hash className="h-3.5 w-3.5" />}
                onSave={resolveSave}
            />
            <InlineEdit
                value="support@northwind.example"
                type="email"
                validate={z.email("Enter a valid email address")}
                onSave={resolveSave}
            />
            <DateTimeEdit value="2026-11-20T09:00:00Z" onSave={resolveSave}>
                <CalendarClock className="h-3.5 w-3.5" />
                <span>Nov 20, 2026, 9:00 AM UTC</span>
            </DateTimeEdit>
        </div>
    ),
}
