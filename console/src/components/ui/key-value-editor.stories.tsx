import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { KeyValueEditor } from "./key-value-editor"

const meta: Meta<typeof KeyValueEditor> = {
    title: "UI/KeyValueEditor",
    component: KeyValueEditor,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof KeyValueEditor>

export const Default: Story = {
    render: () => {
        const [value, setValue] = useState<Record<string, string>>({
            Content_Type: "application/json",
            Authorization: "Bearer token",
        })
        return (
            <div className="w-[500px] p-4">
                <KeyValueEditor value={value} onChange={setValue} />
                <pre className="mt-4 p-3 rounded bg-muted text-xs font-mono overflow-auto">
                    {JSON.stringify(value, null, 2)}
                </pre>
            </div>
        )
    },
}

export const Empty: Story = {
    render: () => {
        const [value, setValue] = useState<Record<string, string>>({})
        return (
            <div className="w-[500px] p-4">
                <KeyValueEditor
                    value={value}
                    onChange={setValue}
                    keyPlaceholder="Header name"
                    valuePlaceholder="Header value"
                />
            </div>
        )
    },
}

export const CustomLabels: Story = {
    render: () => {
        const [value, setValue] = useState<Record<string, string>>({
            NODE_ENV: "production",
            PORT: "3000",
            DATABASE_URL: "postgres://localhost/mydb",
        })
        return (
            <div className="w-[500px] p-4">
                <KeyValueEditor
                    value={value}
                    onChange={setValue}
                    keyPlaceholder="Variable name"
                    valuePlaceholder="Variable value"
                    addLabel="Add Variable"
                />
            </div>
        )
    },
}
