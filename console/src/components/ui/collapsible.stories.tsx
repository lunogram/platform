import { useState } from "react"
import type { Meta, StoryObj } from "@storybook/react"
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "./collapsible"
import { Button } from "./button"

const meta: Meta<typeof Collapsible> = {
    component: Collapsible,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof Collapsible>

export const Default: Story = {
    render: () => {
        const [open, setOpen] = useState(false)
        return (
            <Collapsible open={open} onOpenChange={setOpen} style={{ width: "320px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                    <span style={{ fontWeight: 500 }}>Advanced options</span>
                    <CollapsibleTrigger asChild>
                        <Button variant="ghost" size="sm">
                            {open ? "Hide" : "Show"}
                        </Button>
                    </CollapsibleTrigger>
                </div>
                <CollapsibleContent>
                    <div style={{ paddingTop: "8px", display: "flex", flexDirection: "column", gap: "4px" }}>
                        <p style={{ fontSize: "14px" }}>Option 1: Enable feature flags</p>
                        <p style={{ fontSize: "14px" }}>Option 2: Debug mode</p>
                        <p style={{ fontSize: "14px" }}>Option 3: Verbose logging</p>
                    </div>
                </CollapsibleContent>
            </Collapsible>
        )
    },
}

export const DefaultOpen: Story = {
    render: () => {
        const [open, setOpen] = useState(true)
        return (
            <Collapsible open={open} onOpenChange={setOpen} style={{ width: "320px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                    <span style={{ fontWeight: 500 }}>Details</span>
                    <CollapsibleTrigger asChild>
                        <Button variant="ghost" size="sm">
                            {open ? "Collapse" : "Expand"}
                        </Button>
                    </CollapsibleTrigger>
                </div>
                <CollapsibleContent>
                    <p style={{ fontSize: "14px", paddingTop: "8px" }}>
                        This section starts expanded by default.
                    </p>
                </CollapsibleContent>
            </Collapsible>
        )
    },
}
