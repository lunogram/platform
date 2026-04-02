import { useState } from "react"
import type { Meta, StoryObj } from "@storybook/react"
import { Checkbox } from "./checkbox"

const meta: Meta<typeof Checkbox> = {
    component: Checkbox,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Checkbox>

export const Default: Story = {
    render: () => {
        const [checked, setChecked] = useState(false)
        return (
            <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <Checkbox
                    id="terms"
                    checked={checked}
                    onCheckedChange={(val) => setChecked(!!val)}
                />
                <label htmlFor="terms" style={{ cursor: "pointer" }}>
                    Accept terms and conditions
                </label>
            </div>
        )
    },
}

export const Checked: Story = {
    render: () => {
        const [checked, setChecked] = useState(true)
        return (
            <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <Checkbox
                    id="checked-example"
                    checked={checked}
                    onCheckedChange={(val) => setChecked(!!val)}
                />
                <label htmlFor="checked-example" style={{ cursor: "pointer" }}>
                    Already checked
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
