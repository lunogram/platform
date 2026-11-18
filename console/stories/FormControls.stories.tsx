import type { Meta, StoryObj } from "@storybook/react-vite"
import { Copy, Search, Send } from "lucide-react"

import { Checkbox } from "@/components/ui/checkbox"
import {
    Field,
    FieldContent,
    FieldDescription,
    FieldError,
    FieldGroup,
    FieldLabel,
    FieldLegend,
    FieldSeparator,
    FieldSet,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
    InputGroup,
    InputGroupAddon,
    InputGroupButton,
    InputGroupInput,
    InputGroupText,
    InputGroupTextarea,
} from "@/components/ui/input-group"
import { Label } from "@/components/ui/label"
import {
    Select,
    SelectContent,
    SelectGroup,
    SelectItem,
    SelectLabel,
    SelectSeparator,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"

const meta: Meta = {
    title: "UI/Form Controls",
}

export default meta
type Story = StoryObj

export const Inputs: Story = {
    render: () => (
        <div className="flex max-w-sm flex-col gap-4">
            <Input placeholder="Default input" />
            <Input defaultValue="With value" />
            <Input disabled defaultValue="Disabled input" />
            <div className="grid gap-2">
                <Label htmlFor="labelled">With label</Label>
                <Input id="labelled" type="email" placeholder="jane@example.com" />
            </div>
            <Input type="file" />
        </div>
    ),
}

export const TextareaStory: Story = {
    name: "Textarea",
    render: () => (
        <div className="flex max-w-sm flex-col gap-4">
            <Textarea placeholder="Write something..." />
            <Textarea aria-invalid defaultValue="Invalid content" />
            <Textarea disabled defaultValue="Disabled textarea" />
        </div>
    ),
}

function ChannelSelect({
    defaultValue,
    disabled,
    elevation,
}: {
    defaultValue?: string
    disabled?: boolean
    elevation?: "default" | "flat"
}) {
    return (
        <Select defaultValue={defaultValue} disabled={disabled}>
            <SelectTrigger elevation={elevation}>
                <SelectValue placeholder="Select a channel" />
            </SelectTrigger>
            <SelectContent>
                <SelectGroup>
                    <SelectLabel>Channels</SelectLabel>
                    <SelectItem value="email">Email</SelectItem>
                    <SelectItem value="sms">SMS</SelectItem>
                    <SelectItem value="push">Push</SelectItem>
                </SelectGroup>
                <SelectSeparator />
                <SelectItem value="inbox" disabled>
                    Inbox (no provider)
                </SelectItem>
            </SelectContent>
        </Select>
    )
}

export const SelectStory: Story = {
    name: "Select",
    render: () => (
        <div className="flex max-w-sm flex-col gap-4">
            <ChannelSelect />
            <ChannelSelect defaultValue="email" />
            <ChannelSelect defaultValue="sms" elevation="flat" />
            <ChannelSelect defaultValue="push" disabled />
        </div>
    ),
}

export const InputGroups: Story = {
    name: "Input group",
    render: () => (
        <div className="flex max-w-sm flex-col gap-4">
            <InputGroup>
                <InputGroupAddon>
                    <Search />
                </InputGroupAddon>
                <InputGroupInput placeholder="Search campaigns..." />
            </InputGroup>
            <InputGroup>
                <InputGroupInput readOnly defaultValue="pk_live_7d1f0c2b9a" />
                <InputGroupAddon align="inline-end">
                    <InputGroupButton size="icon-xs" aria-label="Copy">
                        <Copy />
                    </InputGroupButton>
                </InputGroupAddon>
            </InputGroup>
            <InputGroup>
                <InputGroupInput placeholder="example" />
                <InputGroupAddon align="inline-end">
                    <InputGroupText>.lunogram.app</InputGroupText>
                </InputGroupAddon>
            </InputGroup>
            <InputGroup>
                <InputGroupInput aria-invalid defaultValue="not-an-email" />
            </InputGroup>
            <InputGroup>
                <InputGroupTextarea placeholder="Describe the change..." />
                <InputGroupAddon align="block-end" className="justify-between">
                    <InputGroupText>0 / 280</InputGroupText>
                    <InputGroupButton variant="default" size="sm">
                        <Send /> Send
                    </InputGroupButton>
                </InputGroupAddon>
            </InputGroup>
        </div>
    ),
}

export const Fields: Story = {
    render: () => (
        <FieldSet className="max-w-md">
            <FieldLegend>Sender</FieldLegend>
            <FieldDescription>Shown to recipients as the message origin.</FieldDescription>
            <FieldGroup>
                <Field>
                    <FieldLabel htmlFor="from-name">From name</FieldLabel>
                    <Input id="from-name" defaultValue="Northwind" />
                    <FieldDescription>Use your brand or product name.</FieldDescription>
                </Field>
                <Field data-invalid>
                    <FieldLabel htmlFor="from-email">From email</FieldLabel>
                    <Input id="from-email" aria-invalid defaultValue="hello@" />
                    <FieldError>Enter a valid email address.</FieldError>
                </Field>
                <FieldSeparator>Options</FieldSeparator>
                <Field orientation="horizontal">
                    <FieldContent>
                        <FieldLabel htmlFor="transactional">Transactional</FieldLabel>
                        <FieldDescription>
                            Bypass subscription preferences for receipts and alerts.
                        </FieldDescription>
                    </FieldContent>
                    <Switch id="transactional" />
                </Field>
                <Field orientation="horizontal">
                    <Checkbox id="track" defaultChecked />
                    <FieldLabel htmlFor="track">Track opens and clicks</FieldLabel>
                </Field>
                <Field orientation="horizontal" data-disabled>
                    <Checkbox id="locked" disabled />
                    <FieldLabel htmlFor="locked">Locked by organization policy</FieldLabel>
                </Field>
            </FieldGroup>
        </FieldSet>
    ),
}
