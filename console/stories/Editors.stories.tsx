import type { Meta, StoryObj } from "@storybook/react-vite"
import { useState } from "react"

import { AttributeEditor } from "@/components/ui/attribute-editor"
import { CodeEditor } from "@/components/ui/code-editor"
import { KeyValueEditor } from "@/components/ui/key-value-editor"
import { Label } from "@/components/ui/label"
import { TemplateInput } from "@/components/ui/template-input"
import type { VariableGroup } from "@/views/journey/JourneyVariableContext"

const meta: Meta = {
    title: "UI/Editors",
}

export default meta
type Story = StoryObj

const variables: VariableGroup[] = [
    {
        label: "User",
        variables: [
            { path: "user.email", label: "email", description: "string" },
            { path: "user.first_name", label: "first_name", description: "string" },
            { path: "user.plan", label: "plan", description: "string" },
        ],
    },
    {
        label: "Event",
        variables: [
            { path: "event.order.total", label: "order.total", description: "number" },
            { path: "event.order.currency", label: "order.currency", description: "string" },
        ],
    },
]

const payload = JSON.stringify(
    { order_id: "{{ event.order.id }}", total: 128.4, currency: "USD", items: 3 },
    null,
    2,
)

function CodeEditorDemo({ initial, readOnly }: { initial: string; readOnly?: boolean }) {
    const [value, setValue] = useState(initial)
    return (
        <CodeEditor
            value={value}
            onChange={setValue}
            language="json"
            readOnly={readOnly}
            minHeight={140}
            variables={readOnly ? undefined : variables}
        />
    )
}

export const CodeEditorStory: Story = {
    name: "Code editor",
    render: () => (
        <div className="flex max-w-2xl flex-col gap-6">
            <CodeEditorDemo initial={payload} />
            <CodeEditorDemo initial={'{\n  "total": 128.4,\n  "currency": USD\n}'} />
            <CodeEditorDemo initial={payload} readOnly />
        </div>
    ),
}

function TemplateInputDemo({
    initial,
    multiline,
    disabled,
    variant,
    placeholder,
}: {
    initial: string
    multiline?: boolean
    disabled?: boolean
    variant?: "default" | "compact"
    placeholder?: string
}) {
    const [value, setValue] = useState(initial)
    return (
        <TemplateInput
            value={value}
            onChange={setValue}
            variables={variables}
            multiline={multiline}
            disabled={disabled}
            variant={variant}
            placeholder={placeholder}
        />
    )
}

export const TemplateInputStory: Story = {
    name: "Template input",
    render: () => (
        <div className="flex max-w-md flex-col gap-4">
            <div className="grid gap-2">
                <Label>Subject</Label>
                <TemplateInputDemo initial="Your {{ user.plan }} receipt for {{ event.order.total }}" />
            </div>
            <div className="grid gap-2">
                <Label>Empty</Label>
                <TemplateInputDemo initial="" placeholder="Hi {{ user.first_name }}..." />
            </div>
            <div className="grid gap-2">
                <Label>Multiline</Label>
                <TemplateInputDemo
                    multiline
                    initial={"Hi {{ user.first_name }},\nThanks for your order."}
                />
            </div>
            <div className="grid gap-2">
                <Label>Compact</Label>
                <TemplateInputDemo variant="compact" initial="{{ user.email }}" />
            </div>
            <div className="grid gap-2">
                <Label>Disabled</Label>
                <TemplateInputDemo disabled initial="Order {{ event.order.total }}" />
            </div>
        </div>
    ),
}

function KeyValueEditorDemo({
    initial,
    withVariables,
}: {
    initial: Record<string, string>
    withVariables?: boolean
}) {
    const [value, setValue] = useState(initial)
    return (
        <KeyValueEditor
            value={value}
            onChange={setValue}
            keyPlaceholder="Header"
            addLabel="Add header"
            variables={withVariables ? variables : undefined}
        />
    )
}

export const KeyValue: Story = {
    name: "Key-value editor",
    render: () => (
        <div className="flex max-w-xl flex-col gap-6">
            <KeyValueEditorDemo initial={{}} />
            <KeyValueEditorDemo
                initial={{ Authorization: "Bearer sk_test_51", "X-Request-Source": "lunogram" }}
            />
            <KeyValueEditorDemo withVariables initial={{ "X-User": "{{ user.email }}" }} />
        </div>
    ),
}

function AttributeEditorDemo({
    initial,
    defaultTab,
}: {
    initial: Record<string, unknown>
    defaultTab?: "visual" | "code"
}) {
    const [value, setValue] = useState(initial)
    return <AttributeEditor value={value} onChange={setValue} defaultTab={defaultTab} />
}

const attributes = {
    first_name: "Jane",
    plan: "pro",
    seats: 12,
    beta: true,
    address: { city: "Amsterdam", country: "NL" },
    tags: ["vip", "newsletter"],
}

export const Attributes: Story = {
    render: () => (
        <div className="flex max-w-2xl flex-col gap-6">
            <AttributeEditorDemo initial={attributes} />
            <AttributeEditorDemo initial={attributes} defaultTab="code" />
            <AttributeEditorDemo initial={{}} />
        </div>
    ),
}
