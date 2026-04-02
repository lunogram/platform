import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { AttributeEditor } from "./attribute-editor"

const meta: Meta<typeof AttributeEditor> = {
    title: "UI/AttributeEditor",
    component: AttributeEditor,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof AttributeEditor>

export const Default: Story = {
    render: () => {
        const [value, setValue] = useState<Record<string, unknown>>({
            name: "John",
            age: 30,
            active: true,
        })
        return (
            <div className="w-[600px] p-4">
                <AttributeEditor value={value} onChange={setValue} />
                <pre className="mt-4 p-3 rounded bg-muted text-xs font-mono overflow-auto">
                    {JSON.stringify(value, null, 2)}
                </pre>
            </div>
        )
    },
}

export const StartInCodeMode: Story = {
    render: () => {
        const [value, setValue] = useState<Record<string, unknown>>({
            plan: "pro",
            seats: 10,
            features: { analytics: true, exports: false },
        })
        return (
            <div className="w-[600px] p-4">
                <AttributeEditor value={value} onChange={setValue} defaultTab="code" />
            </div>
        )
    },
}

export const Empty: Story = {
    render: () => {
        const [value, setValue] = useState<Record<string, unknown>>({})
        return (
            <div className="w-[600px] p-4">
                <AttributeEditor value={value} onChange={setValue} />
            </div>
        )
    },
}
