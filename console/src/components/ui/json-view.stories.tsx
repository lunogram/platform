import type { Meta, StoryObj } from "@storybook/react"
import { JsonView, JsonInline } from "./json-view"

const meta: Meta<typeof JsonView> = {
    title: "UI/JsonView",
    component: JsonView,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof JsonView>

const sampleData = {
    id: "usr_01j9x2k3m4n5p6q7",
    name: "Alice Smith",
    email: "alice@example.com",
    active: true,
    age: 29,
    roles: ["admin", "editor"],
    address: {
        street: "123 Main St",
        city: "Springfield",
        country: "US",
    },
    metadata: {
        created_at: "2024-01-15T10:30:00Z",
        website: "https://alice.example.com",
    },
}

export const Default: Story = {
    render: () => (
        <div className="w-[500px]">
            <JsonView data={sampleData} />
        </div>
    ),
}

export const WithTitle: Story = {
    render: () => (
        <div className="w-[500px]">
            <JsonView data={sampleData} title="User Record" />
        </div>
    ),
}

export const Collapsed: Story = {
    render: () => (
        <div className="w-[500px]">
            <JsonView data={sampleData} defaultExpanded={false} title="Collapsed by Default" />
        </div>
    ),
}

export const SimpleObject: Story = {
    render: () => (
        <div className="w-[400px]">
            <JsonView
                data={{ key: "value", count: 42, enabled: true, tags: ["a", "b", "c"] }}
                title="Simple Object"
            />
        </div>
    ),
}

export const Inline: StoryObj<typeof JsonInline> = {
    render: () => (
        <div className="flex flex-col gap-2 p-4">
            <p className="text-sm">
                Payload: <JsonInline data={{ event: "click", x: 10, y: 20 }} />
            </p>
            <p className="text-sm">
                Tags: <JsonInline data={["admin", "editor", "viewer"]} />
            </p>
            <p className="text-sm">
                Truncated:{" "}
                <JsonInline
                    data={{ very: "long", object: "with", many: "keys", that: "gets", truncated: true }}
                    maxLength={40}
                />
            </p>
        </div>
    ),
}
