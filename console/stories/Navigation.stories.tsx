import type { Meta, StoryObj } from "@storybook/react-vite"
import { Activity, CalendarClock, ChevronsUpDown, Inbox, ListChecks, User } from "lucide-react"
import { useState } from "react"

import { Button } from "@/components/ui/button"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import type { NavTab, RouteNavTab } from "@/components/ui/nav-tabs"
import { NavTabs } from "@/components/ui/nav-tabs"
import {
    Pagination,
    PaginationContent,
    PaginationEllipsis,
    PaginationItem,
    PaginationLink,
    PaginationNext,
    PaginationPrevious,
} from "@/components/ui/pagination"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

const meta: Meta = {
    title: "UI/Navigation",
}

export default meta
type Story = StoryObj

export const TabsStory: Story = {
    name: "Tabs",
    render: () => (
        <Tabs defaultValue="visual" className="max-w-md">
            <TabsList>
                <TabsTrigger value="visual">Visual</TabsTrigger>
                <TabsTrigger value="code">Code</TabsTrigger>
                <TabsTrigger value="preview" disabled>
                    Preview
                </TabsTrigger>
            </TabsList>
            <TabsContent value="visual" className="text-sm text-muted-foreground">
                Edit attributes as a tree.
            </TabsContent>
            <TabsContent value="code" className="text-sm text-muted-foreground">
                Edit attributes as JSON.
            </TabsContent>
        </Tabs>
    ),
}

const userTabs: NavTab[] = [
    { key: "details", label: "Details", icon: User },
    { key: "events", label: "Events", icon: Activity, badge: 128 },
    { key: "journeys", label: "Journeys", icon: ListChecks, badge: 2 },
    { key: "inbox", label: "Inbox", icon: Inbox, badge: 0 },
    { key: "scheduled", label: "Scheduled", icon: CalendarClock },
]

function StateNavTabs() {
    const [value, setValue] = useState("events")
    return (
        <div className="border-b">
            <NavTabs tabs={userTabs} value={value} onChange={setValue} />
        </div>
    )
}

export const NavTabsState: Story = {
    name: "Nav tabs",
    render: () => <StateNavTabs />,
}

const routeTabs: RouteNavTab[] = userTabs.map((tab) => ({ ...tab, to: `/users/usr_01/${tab.key}` }))

export const NavTabsRoute: Story = {
    name: "Nav tabs / route links",
    render: () => (
        <div className="border-b">
            <NavTabs tabs={routeTabs} activeTab="details" />
        </div>
    ),
}

export const PaginationStory: Story = {
    name: "Pagination",
    render: () => (
        <Pagination>
            <PaginationContent>
                <PaginationItem>
                    <PaginationPrevious href="#" />
                </PaginationItem>
                <PaginationItem>
                    <PaginationLink href="#">1</PaginationLink>
                </PaginationItem>
                <PaginationItem>
                    <PaginationLink href="#" isActive>
                        2
                    </PaginationLink>
                </PaginationItem>
                <PaginationItem>
                    <PaginationLink href="#">3</PaginationLink>
                </PaginationItem>
                <PaginationItem>
                    <PaginationEllipsis />
                </PaginationItem>
                <PaginationItem>
                    <PaginationNext href="#" />
                </PaginationItem>
            </PaginationContent>
        </Pagination>
    ),
}

export const CollapsibleStory: Story = {
    name: "Collapsible",
    render: () => (
        <Collapsible defaultOpen className="max-w-sm space-y-2">
            <div className="flex items-center justify-between">
                <h4 className="text-sm font-semibold">Advanced settings</h4>
                <CollapsibleTrigger asChild>
                    <Button variant="ghost" size="sm" aria-label="Toggle">
                        <ChevronsUpDown className="h-4 w-4" />
                    </Button>
                </CollapsibleTrigger>
            </div>
            <CollapsibleContent className="space-y-2 text-sm">
                <div className="rounded-md border px-4 py-2">Rate limit: 100 per minute</div>
                <div className="rounded-md border px-4 py-2">Quiet hours: 22:00–08:00</div>
            </CollapsibleContent>
        </Collapsible>
    ),
}
