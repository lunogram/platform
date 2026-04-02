import type { Meta, StoryObj } from "@storybook/react"
import { Badge } from "./badge"

const meta: Meta<typeof Badge> = {
    component: Badge,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Badge>

export const Default: Story = {
    args: {
        children: "Badge",
    },
}

export const Secondary: Story = {
    args: {
        variant: "secondary",
        children: "Secondary",
    },
}

export const Destructive: Story = {
    args: {
        variant: "destructive",
        children: "Destructive",
    },
}

export const Outline: Story = {
    args: {
        variant: "outline",
        children: "Outline",
    },
}

export const AllVariants: Story = {
    render: () => (
        <div style={{ display: "flex", gap: "8px", flexWrap: "wrap" }}>
            <Badge>Default</Badge>
            <Badge variant="secondary">Secondary</Badge>
            <Badge variant="destructive">Destructive</Badge>
            <Badge variant="outline">Outline</Badge>
        </div>
    ),
}
