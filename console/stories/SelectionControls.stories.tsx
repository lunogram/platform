import type { Meta, StoryObj } from "@storybook/react-vite"
import { Inbox, Mail, MessageSquareDot, Smartphone } from "lucide-react"
import { useState } from "react"

import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { SelectableCard } from "@/components/ui/selectable-card"
import { Switch } from "@/components/ui/switch"

const meta: Meta = {
    title: "UI/Selection Controls",
}

export default meta
type Story = StoryObj

export const Checkboxes: Story = {
    render: () => (
        <div className="flex flex-col gap-3">
            <div className="flex items-center gap-2">
                <Checkbox id="unchecked" />
                <Label htmlFor="unchecked">Unchecked</Label>
            </div>
            <div className="flex items-center gap-2">
                <Checkbox id="checked" defaultChecked />
                <Label htmlFor="checked">Checked</Label>
            </div>
            <div className="flex items-center gap-2">
                <Checkbox id="indeterminate" checked="indeterminate" />
                <Label htmlFor="indeterminate">Indeterminate</Label>
            </div>
            <div className="flex items-center gap-2">
                <Checkbox id="disabled" disabled />
                <Label htmlFor="disabled">Disabled</Label>
            </div>
            <div className="flex items-center gap-2">
                <Checkbox id="disabled-checked" disabled defaultChecked />
                <Label htmlFor="disabled-checked">Disabled checked</Label>
            </div>
        </div>
    ),
}

export const Switches: Story = {
    render: () => (
        <div className="flex flex-col gap-3">
            <div className="flex items-center gap-2">
                <Switch id="off" />
                <Label htmlFor="off">Off</Label>
            </div>
            <div className="flex items-center gap-2">
                <Switch id="on" defaultChecked />
                <Label htmlFor="on">On</Label>
            </div>
            <div className="flex items-center gap-2">
                <Switch id="switch-disabled" disabled />
                <Label htmlFor="switch-disabled">Disabled</Label>
            </div>
            <div className="flex items-center gap-2">
                <Switch id="switch-disabled-on" disabled defaultChecked />
                <Label htmlFor="switch-disabled-on">Disabled on</Label>
            </div>
        </div>
    ),
}

const channels = [
    { key: "email", title: "Email", summary: "Newsletters and receipts", icon: Mail },
    { key: "sms", title: "SMS", summary: "Short transactional texts", icon: Smartphone },
    { key: "push", title: "Push", summary: "Mobile notifications", icon: MessageSquareDot },
    { key: "inbox", title: "Inbox", summary: "No provider configured", icon: Inbox },
]

function SelectableCardDemo() {
    const [active, setActive] = useState("email")
    return (
        <div className="grid max-w-xl grid-cols-2 gap-3">
            {channels.map(({ key, title, summary, icon: Icon }) => (
                <SelectableCard
                    key={key}
                    title={title}
                    summary={summary}
                    icon={<Icon className="h-4 w-4" />}
                    active={active === key}
                    disabled={key === "inbox"}
                    onClick={() => setActive(key)}
                />
            ))}
        </div>
    )
}

export const SelectableCards: Story = {
    name: "Selectable cards",
    render: () => <SelectableCardDemo />,
}
