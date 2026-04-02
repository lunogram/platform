import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { TemplateInput } from "./template-input"

const meta: Meta<typeof TemplateInput> = {
    title: "UI/TemplateInput",
    component: TemplateInput,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof TemplateInput>

export const Default: Story = {
    render: () => {
        const [value, setValue] = useState("Hello, world!")
        return (
            <div className="w-[400px] p-4">
                <TemplateInput
                    value={value}
                    onChange={setValue}
                    placeholder="Enter a message..."
                />
                <p className="text-xs text-muted-foreground mt-2 font-mono">{value}</p>
            </div>
        )
    },
}

export const WithInitialTemplate: Story = {
    render: () => {
        const [value, setValue] = useState("Welcome back, {{ user.name }}!")
        return (
            <div className="w-[400px] p-4">
                <TemplateInput value={value} onChange={setValue} placeholder="Enter a message..." />
                <p className="text-xs text-muted-foreground mt-2 font-mono">{value}</p>
            </div>
        )
    },
}

export const Compact: Story = {
    render: () => {
        const [value, setValue] = useState("{{ event.type }}")
        return (
            <div className="w-[300px] p-4">
                <TemplateInput
                    value={value}
                    onChange={setValue}
                    placeholder="Value..."
                    variant="compact"
                />
            </div>
        )
    },
}

export const Disabled: Story = {
    render: () => (
        <div className="w-[400px] p-4">
            <TemplateInput
                value="Hello, {{ user.name }}!"
                onChange={() => {}}
                placeholder="Enter a message..."
                disabled
            />
        </div>
    ),
}
