import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { DateTimeEdit } from "./datetime-edit"

const meta: Meta<typeof DateTimeEdit> = {
    title: "UI/DateTimeEdit",
    component: DateTimeEdit,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof DateTimeEdit>

export const Default: Story = {
    render: () => {
        const [value, setValue] = useState("2024-01-15T10:30:00.000Z")
        return (
            <div className="p-4">
                <p className="text-sm text-muted-foreground mb-2">Click the date to edit:</p>
                <DateTimeEdit
                    value={value}
                    onSave={async (v) => {
                        await new Promise((r) => setTimeout(r, 500))
                        setValue(v)
                    }}
                />
                <p className="text-xs text-muted-foreground mt-2">ISO value: {value}</p>
            </div>
        )
    },
}

export const AlignCenter: Story = {
    render: () => {
        const [value, setValue] = useState("2024-01-15T10:30:00.000Z")
        return (
            <div className="p-4 flex justify-center">
                <DateTimeEdit
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
        const [value, setValue] = useState("2024-06-30T23:59:00.000Z")
        return (
            <div className="p-4 flex justify-end">
                <DateTimeEdit
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
