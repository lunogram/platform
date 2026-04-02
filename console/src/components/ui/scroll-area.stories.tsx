import type { Meta, StoryObj } from "@storybook/react"
import { ScrollArea, ScrollBar } from "./scroll-area"

const meta: Meta<typeof ScrollArea> = {
    component: ScrollArea,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof ScrollArea>

const tags = Array.from({ length: 50 }, (_, i) => `Item ${i + 1}`)

export const Vertical: Story = {
    render: () => (
        <ScrollArea style={{ height: "200px", width: "300px", border: "1px solid hsl(var(--border))", borderRadius: "6px", padding: "8px" }}>
            {tags.map((tag) => (
                <div key={tag} style={{ padding: "4px 8px", fontSize: "14px" }}>
                    {tag}
                </div>
            ))}
        </ScrollArea>
    ),
}

export const Horizontal: Story = {
    render: () => (
        <ScrollArea style={{ width: "300px", border: "1px solid hsl(var(--border))", borderRadius: "6px", padding: "8px" }}>
            <div style={{ display: "flex", gap: "12px", width: "max-content" }}>
                {Array.from({ length: 20 }, (_, i) => (
                    <div
                        key={i}
                        style={{
                            minWidth: "80px",
                            height: "60px",
                            background: "hsl(var(--muted))",
                            borderRadius: "4px",
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                            fontSize: "14px",
                        }}
                    >
                        Card {i + 1}
                    </div>
                ))}
            </div>
            <ScrollBar orientation="horizontal" />
        </ScrollArea>
    ),
}
