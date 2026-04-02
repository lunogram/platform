import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { Combobox } from "./combobox"

const meta: Meta<typeof Combobox> = {
    title: "UI/Combobox",
    component: Combobox,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof Combobox>

const options = [
    { path: "production" },
    { path: "staging" },
    { path: "development" },
    { path: "testing" },
    { path: "preview" },
]

export const Default: Story = {
    render: () => {
        const [value, setValue] = useState("")
        return (
            <div className="w-72">
                <Combobox
                    options={options}
                    value={value}
                    onValueChange={setValue}
                    placeholder="Select environment..."
                />
            </div>
        )
    },
}

export const WithSelectedValue: Story = {
    render: () => {
        const [value, setValue] = useState("production")
        return (
            <div className="w-72">
                <Combobox
                    options={options}
                    value={value}
                    onValueChange={setValue}
                    placeholder="Select environment..."
                />
            </div>
        )
    },
}

export const Disabled: Story = {
    render: () => (
        <div className="w-72">
            <Combobox
                options={options}
                value="production"
                onValueChange={() => {}}
                placeholder="Select environment..."
                disabled
            />
        </div>
    ),
}
