import type { Meta, StoryObj } from "@storybook/react"
import { Popover, PopoverTrigger, PopoverContent } from "./popover"
import { Button } from "./button"

const meta: Meta<typeof Popover> = {
    component: Popover,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Popover>

export const Default: Story = {
    render: () => (
        <Popover>
            <PopoverTrigger asChild>
                <Button variant="outline">Open Popover</Button>
            </PopoverTrigger>
            <PopoverContent style={{ width: "280px" }}>
                <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                    <h4 style={{ fontWeight: 500, margin: 0 }}>Dimensions</h4>
                    <p style={{ fontSize: "14px", color: "hsl(var(--muted-foreground))", margin: 0 }}>
                        Set the dimensions for the layer.
                    </p>
                </div>
            </PopoverContent>
        </Popover>
    ),
}

export const WithForm: Story = {
    render: () => (
        <Popover>
            <PopoverTrigger asChild>
                <Button>Settings</Button>
            </PopoverTrigger>
            <PopoverContent style={{ width: "300px" }}>
                <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
                    <p style={{ fontWeight: 500, margin: 0 }}>Quick Settings</p>
                    <label style={{ display: "flex", flexDirection: "column", gap: "4px", fontSize: "14px" }}>
                        Display Name
                        <input
                            style={{ border: "1px solid hsl(var(--border))", borderRadius: "4px", padding: "4px 8px" }}
                            placeholder="John Doe"
                        />
                    </label>
                </div>
            </PopoverContent>
        </Popover>
    ),
}
