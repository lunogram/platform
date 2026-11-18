import type { Meta, StoryObj } from "@storybook/react-vite"

import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from "@/components/ui/resizable"
import { ScrollArea, ScrollBar } from "@/components/ui/scroll-area"
import { Separator } from "@/components/ui/separator"

const meta: Meta = {
    title: "UI/Layout",
}

export default meta
type Story = StoryObj

export const SeparatorStory: Story = {
    name: "Separator",
    render: () => (
        <div className="max-w-sm">
            <div className="space-y-1">
                <h4 className="text-sm font-medium leading-none">Northwind Mobile</h4>
                <p className="text-sm text-muted-foreground">Customer messaging for the app.</p>
            </div>
            <Separator className="my-4" />
            <div className="flex h-5 items-center gap-4 text-sm">
                <span>Campaigns</span>
                <Separator orientation="vertical" />
                <span>Journeys</span>
                <Separator orientation="vertical" />
                <span>Users</span>
            </div>
        </div>
    ),
}

const events = Array.from({ length: 40 }, (_, i) => `order_completed #${1040 - i}`)

export const ScrollAreaStory: Story = {
    name: "Scroll area",
    render: () => (
        <div className="flex gap-6">
            <ScrollArea className="h-72 w-56 rounded-md border">
                <div className="p-4">
                    <h4 className="mb-4 text-sm font-medium leading-none">Events</h4>
                    {events.map((event) => (
                        <div key={event}>
                            <div className="text-sm">{event}</div>
                            <Separator className="my-2" />
                        </div>
                    ))}
                </div>
            </ScrollArea>
            <ScrollArea className="w-96 whitespace-nowrap rounded-md border">
                <div className="flex w-max gap-4 p-4">
                    {["Email", "SMS", "Push", "Inbox", "Webhook", "Slack", "Discord"].map(
                        (channel) => (
                            <div
                                key={channel}
                                className="flex h-20 w-28 items-center justify-center rounded-md bg-muted text-sm"
                            >
                                {channel}
                            </div>
                        ),
                    )}
                </div>
                <ScrollBar orientation="horizontal" />
            </ScrollArea>
        </div>
    ),
}

export const Resizable: Story = {
    render: () => (
        <div className="h-64 max-w-2xl rounded-lg border">
            <ResizablePanelGroup orientation="horizontal">
                <ResizablePanel defaultSize="40%" minSize="20%">
                    <div className="flex h-full items-center justify-center p-6 text-sm">
                        Editor
                    </div>
                </ResizablePanel>
                <ResizableHandle withHandle />
                <ResizablePanel defaultSize="60%" minSize="20%">
                    <div className="flex h-full items-center justify-center p-6 text-sm">
                        Preview
                    </div>
                </ResizablePanel>
            </ResizablePanelGroup>
        </div>
    ),
}
