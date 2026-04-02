import type { Meta, StoryObj } from "@storybook/react"
import { Textarea } from "./textarea"

const meta: Meta<typeof Textarea> = {
    component: Textarea,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Textarea>

export const Default: Story = {
    args: {
        placeholder: "Type your message here...",
    },
}

export const WithValue: Story = {
    args: {
        defaultValue: "This is some pre-filled content.\nIt spans multiple lines.",
        rows: 4,
    },
}

export const Disabled: Story = {
    args: {
        disabled: true,
        placeholder: "This textarea is disabled",
        defaultValue: "Read-only content",
    },
}

export const Tall: Story = {
    args: {
        placeholder: "Write a detailed description...",
        rows: 8,
    },
}
