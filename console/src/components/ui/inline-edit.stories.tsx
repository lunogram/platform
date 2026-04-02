import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { InlineEdit } from "./inline-edit"

const meta: Meta<typeof InlineEdit> = {
    title: "UI/InlineEdit",
    component: InlineEdit,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof InlineEdit>

export const Default: Story = {
    render: () => {
        const [value, setValue] = useState("Alice Smith")
        return (
            <div className="p-4">
                <p className="text-sm text-muted-foreground mb-2">Click the name to edit:</p>
                <InlineEdit
                    value={value}
                    onSave={async (v) => {
                        await new Promise((r) => setTimeout(r, 500))
                        setValue(v)
                    }}
                    placeholder="Enter a name"
                />
            </div>
        )
    },
}

export const Empty: Story = {
    render: () => {
        const [value, setValue] = useState("")
        return (
            <div className="p-4">
                <p className="text-sm text-muted-foreground mb-2">Empty — click to set a value:</p>
                <InlineEdit
                    value={value}
                    onSave={async (v) => {
                        await new Promise((r) => setTimeout(r, 300))
                        setValue(v)
                    }}
                    placeholder="Click to add a description"
                />
            </div>
        )
    },
}

export const AlignCenter: Story = {
    render: () => {
        const [value, setValue] = useState("Centered popover")
        return (
            <div className="p-4 flex justify-center">
                <InlineEdit
                    value={value}
                    onSave={async (v) => {
                        await new Promise((r) => setTimeout(r, 300))
                        setValue(v)
                    }}
                    align="center"
                />
            </div>
        )
    },
}

export const AlignEnd: Story = {
    render: () => {
        const [value, setValue] = useState("End-aligned popover")
        return (
            <div className="p-4 flex justify-end">
                <InlineEdit
                    value={value}
                    onSave={async (v) => {
                        await new Promise((r) => setTimeout(r, 300))
                        setValue(v)
                    }}
                    align="end"
                />
            </div>
        )
    },
}
