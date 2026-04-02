import { useState } from "react"
import type { Meta, StoryObj } from "@storybook/react"
import { Switch } from "./switch"

const meta: Meta<typeof Switch> = {
    component: Switch,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Switch>

export const Default: Story = {
    render: () => {
        const [checked, setChecked] = useState(false)
        return (
            <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <Switch
                    id="airplane-mode"
                    checked={checked}
                    onCheckedChange={setChecked}
                />
                <label htmlFor="airplane-mode" style={{ cursor: "pointer" }}>
                    Airplane Mode
                </label>
            </div>
        )
    },
}

export const Enabled: Story = {
    render: () => {
        const [checked, setChecked] = useState(true)
        return (
            <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <Switch
                    id="notifications"
                    checked={checked}
                    onCheckedChange={setChecked}
                />
                <label htmlFor="notifications" style={{ cursor: "pointer" }}>
                    Enable Notifications
                </label>
            </div>
        )
    },
}

export const Disabled: Story = {
    args: {
        disabled: true,
        checked: false,
    },
}

export const DisabledChecked: Story = {
    args: {
        disabled: true,
        checked: true,
    },
}
