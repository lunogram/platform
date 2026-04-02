import type { Meta, StoryObj } from "@storybook/react"
import { Separator } from "./separator"

const meta: Meta<typeof Separator> = {
    component: Separator,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Separator>

export const Horizontal: Story = {
    render: () => (
        <div style={{ width: "300px" }}>
            <p style={{ marginBottom: "8px" }}>Above the separator</p>
            <Separator />
            <p style={{ marginTop: "8px" }}>Below the separator</p>
        </div>
    ),
}

export const Vertical: Story = {
    render: () => (
        <div style={{ display: "flex", height: "32px", alignItems: "center", gap: "12px" }}>
            <span>Home</span>
            <Separator orientation="vertical" />
            <span>About</span>
            <Separator orientation="vertical" />
            <span>Contact</span>
        </div>
    ),
}
