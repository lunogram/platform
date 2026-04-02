import type { Meta, StoryObj } from "@storybook/react"
import { ResizablePanelGroup, ResizablePanel, ResizableHandle } from "./resizable"

const meta: Meta<typeof ResizablePanelGroup> = {
    component: ResizablePanelGroup,
    tags: ["autodocs"],
}
export default meta

type Story = StoryObj<typeof ResizablePanelGroup>

export const Horizontal: Story = {
    render: () => (
        <ResizablePanelGroup
            direction="horizontal"
            style={{ height: "200px", border: "1px solid hsl(var(--border))", borderRadius: "6px" }}
        >
            <ResizablePanel defaultSize={50}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%", padding: "16px" }}>
                    <span style={{ fontSize: "14px" }}>Left Panel</span>
                </div>
            </ResizablePanel>
            <ResizableHandle withHandle />
            <ResizablePanel defaultSize={50}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%", padding: "16px" }}>
                    <span style={{ fontSize: "14px" }}>Right Panel</span>
                </div>
            </ResizablePanel>
        </ResizablePanelGroup>
    ),
}

export const Vertical: Story = {
    render: () => (
        <ResizablePanelGroup
            direction="vertical"
            style={{ height: "300px", border: "1px solid hsl(var(--border))", borderRadius: "6px" }}
        >
            <ResizablePanel defaultSize={40}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%", padding: "16px" }}>
                    <span style={{ fontSize: "14px" }}>Top Panel</span>
                </div>
            </ResizablePanel>
            <ResizableHandle withHandle />
            <ResizablePanel defaultSize={60}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%", padding: "16px" }}>
                    <span style={{ fontSize: "14px" }}>Bottom Panel</span>
                </div>
            </ResizablePanel>
        </ResizablePanelGroup>
    ),
}

export const ThreeColumns: Story = {
    render: () => (
        <ResizablePanelGroup
            direction="horizontal"
            style={{ height: "200px", border: "1px solid hsl(var(--border))", borderRadius: "6px" }}
        >
            <ResizablePanel defaultSize={25}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%" }}>
                    <span style={{ fontSize: "14px" }}>Sidebar</span>
                </div>
            </ResizablePanel>
            <ResizableHandle />
            <ResizablePanel defaultSize={50}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%" }}>
                    <span style={{ fontSize: "14px" }}>Content</span>
                </div>
            </ResizablePanel>
            <ResizableHandle />
            <ResizablePanel defaultSize={25}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%" }}>
                    <span style={{ fontSize: "14px" }}>Inspector</span>
                </div>
            </ResizablePanel>
        </ResizablePanelGroup>
    ),
}
