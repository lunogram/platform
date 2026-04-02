import type { Meta, StoryObj } from "@storybook/react"
import { Label } from "./label"
import { Input } from "./input"

const meta: Meta<typeof Label> = {
    component: Label,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Label>

export const Default: Story = {
    args: {
        children: "Email address",
    },
}

export const WithInput: Story = {
    render: () => (
        <div style={{ display: "flex", flexDirection: "column", gap: "4px", width: "280px" }}>
            <Label htmlFor="email">Email address</Label>
            <Input id="email" type="email" placeholder="user@example.com" />
        </div>
    ),
}

export const Required: Story = {
    render: () => (
        <div style={{ display: "flex", flexDirection: "column", gap: "4px", width: "280px" }}>
            <Label htmlFor="username">
                Username <span style={{ color: "red" }}>*</span>
            </Label>
            <Input id="username" placeholder="johndoe" />
        </div>
    ),
}
