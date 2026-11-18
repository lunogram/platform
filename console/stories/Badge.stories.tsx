import type { Meta, StoryObj } from "@storybook/react-vite"
import { Inbox, Mail, MessageSquareDot, Smartphone } from "lucide-react"

import { Badge } from "@/components/ui/badge"

const meta: Meta<typeof Badge> = {
    title: "UI/Badge",
    component: Badge,
}

export default meta
type Story = StoryObj<typeof Badge>

export const Variants: Story = {
    render: () => (
        <div className="flex flex-wrap items-center gap-3">
            <Badge>Default</Badge>
            <Badge variant="secondary">Secondary</Badge>
            <Badge variant="outline">Outline</Badge>
            <Badge variant="destructive">Destructive</Badge>
        </div>
    ),
}

export const WithIcons: Story = {
    name: "With icons",
    render: () => (
        <div className="flex flex-wrap items-center gap-3">
            <Badge variant="secondary" className="gap-1">
                <Mail className="h-3.5 w-3.5" /> Email
            </Badge>
            <Badge variant="secondary" className="gap-1">
                <Smartphone className="h-3.5 w-3.5" /> SMS
            </Badge>
            <Badge variant="secondary" className="gap-1">
                <MessageSquareDot className="h-3.5 w-3.5" /> Push
            </Badge>
            <Badge variant="secondary" className="gap-1">
                <Inbox className="h-3.5 w-3.5" /> Inbox
            </Badge>
        </div>
    ),
}

export const Counts: Story = {
    render: () => (
        <div className="flex flex-wrap items-center gap-3">
            <Badge variant="secondary" className="h-5 min-w-5 px-1 text-[10px]">
                3
            </Badge>
            <Badge variant="secondary" className="h-5 min-w-5 px-1 text-[10px]">
                128
            </Badge>
            <Badge variant="outline">+4 more</Badge>
        </div>
    ),
}
