import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { CodeEditor } from "./code-editor"

const meta: Meta<typeof CodeEditor> = {
    title: "UI/CodeEditor",
    component: CodeEditor,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof CodeEditor>

const sampleJson = JSON.stringify(
    {
        name: "Alice",
        age: 30,
        roles: ["admin", "editor"],
        settings: {
            theme: "dark",
            notifications: true,
        },
    },
    null,
    2,
)

export const JsonMode: Story = {
    render: () => {
        const [value, setValue] = useState(sampleJson)
        return (
            <div className="w-[600px] p-4">
                <CodeEditor value={value} onChange={setValue} language="json" />
            </div>
        )
    },
}

export const AutoDetect: Story = {
    render: () => {
        const [value, setValue] = useState(sampleJson)
        return (
            <div className="w-[600px] p-4">
                <CodeEditor value={value} onChange={setValue} language="auto" />
            </div>
        )
    },
}

export const PlainText: Story = {
    render: () => {
        const [value, setValue] = useState("Hello World\nThis is plain text content.")
        return (
            <div className="w-[600px] p-4">
                <CodeEditor value={value} onChange={setValue} language="none" />
            </div>
        )
    },
}

export const ReadOnly: Story = {
    render: () => (
        <div className="w-[600px] p-4">
            <CodeEditor value={sampleJson} onChange={() => {}} language="json" readOnly />
        </div>
    ),
}

export const WithPlaceholder: Story = {
    render: () => {
        const [value, setValue] = useState("")
        return (
            <div className="w-[600px] p-4">
                <CodeEditor
                    value={value}
                    onChange={setValue}
                    language="json"
                    placeholder='{ "key": "value" }'
                    minHeight={150}
                />
            </div>
        )
    },
}

export const CustomHeight: Story = {
    render: () => {
        const [value, setValue] = useState(sampleJson)
        return (
            <div className="w-[600px] p-4">
                <CodeEditor
                    value={value}
                    onChange={setValue}
                    language="json"
                    minHeight={200}
                    maxHeight={300}
                />
            </div>
        )
    },
}
