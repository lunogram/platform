import type { Meta, StoryObj } from "@storybook/react"
import { Input } from "./input"

const meta: Meta<typeof Input> = {
    component: Input,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Input>

export const Default: Story = {
    args: {
        placeholder: "Enter text...",
    },
}

export const WithValue: Story = {
    args: {
        defaultValue: "Hello, world!",
    },
}

export const Password: Story = {
    args: {
        type: "password",
        placeholder: "Enter password...",
    },
}

export const Disabled: Story = {
    args: {
        disabled: true,
        placeholder: "Disabled input",
        defaultValue: "Cannot edit this",
    },
}

export const Email: Story = {
    args: {
        type: "email",
        placeholder: "user@example.com",
    },
}
