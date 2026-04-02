import type { Meta, StoryObj } from "@storybook/react"
import { useState } from "react"
import { ColorPicker } from "./color-picker"

const meta: Meta<typeof ColorPicker> = {
    title: "UI/ColorPicker",
    component: ColorPicker,
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj<typeof ColorPicker>

export const Default: Story = {
    render: () => {
        const [color, setColor] = useState("#3b82f6")
        return (
            <div className="p-4">
                <ColorPicker value={color} onChange={setColor} />
                <p className="text-sm text-muted-foreground mt-2">Selected: {color}</p>
            </div>
        )
    },
}

export const WithClear: Story = {
    render: () => {
        const [color, setColor] = useState("#22c55e")
        return (
            <div className="p-4">
                <ColorPicker
                    value={color}
                    onChange={setColor}
                    onClear={() => setColor("#000000")}
                />
                <p className="text-sm text-muted-foreground mt-2">Selected: {color}</p>
            </div>
        )
    },
}

export const Inline: Story = {
    render: () => {
        const [color, setColor] = useState("#a855f7")
        return (
            <div className="p-4 w-48">
                <ColorPicker
                    value={color}
                    onChange={setColor}
                    onClear={() => setColor("#000000")}
                    inline
                />
                <p className="text-sm text-muted-foreground mt-2">Selected: {color}</p>
            </div>
        )
    },
}
