import type { Meta, StoryObj } from "@storybook/react"
import { Button } from "./button"

const meta: Meta<typeof Button> = {
    component: Button,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Button>

export const Default: Story = {
    args: {
        children: "Button",
    },
}

export const Destructive: Story = {
    args: {
        variant: "destructive",
        children: "Delete",
    },
}

export const Outline: Story = {
    args: {
        variant: "outline",
        children: "Outline",
    },
}

export const Secondary: Story = {
    args: {
        variant: "secondary",
        children: "Secondary",
    },
}

export const Ghost: Story = {
    args: {
        variant: "ghost",
        children: "Ghost",
    },
}

export const Link: Story = {
    args: {
        variant: "link",
        children: "Link",
    },
}

export const Loading: Story = {
    args: {
        isLoading: true,
        children: "Saving...",
    },
}

export const Disabled: Story = {
    args: {
        disabled: true,
        children: "Disabled",
    },
}

export const AllSizes: Story = {
    render: () => (
        <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
            <Button size="sm">Small</Button>
            <Button size="default">Default</Button>
            <Button size="lg">Large</Button>
        </div>
    ),
}
