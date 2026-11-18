import type { Meta, StoryObj } from "@storybook/react-vite"
import { FolderClosed, Megaphone, MoreHorizontal, Plus, Route, Settings, Users } from "lucide-react"

import {
    Sidebar,
    SidebarContent,
    SidebarFooter,
    SidebarGroup,
    SidebarGroupAction,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarHeader,
    SidebarInput,
    SidebarInset,
    SidebarMenu,
    SidebarMenuAction,
    SidebarMenuBadge,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarMenuSkeleton,
    SidebarMenuSub,
    SidebarMenuSubButton,
    SidebarMenuSubItem,
    SidebarProvider,
    SidebarRail,
    SidebarSeparator,
    SidebarTrigger,
} from "@/components/ui/sidebar"

const meta: Meta = {
    title: "UI/Sidebar",
    parameters: { layout: "fullscreen" },
}

export default meta
type Story = StoryObj

function DemoSidebar({ loading }: { loading?: boolean }) {
    return (
        <Sidebar collapsible="icon">
            <SidebarHeader>
                <SidebarInput placeholder="Search..." />
            </SidebarHeader>
            <SidebarContent>
                <SidebarGroup>
                    <SidebarGroupLabel>Navigation</SidebarGroupLabel>
                    <SidebarGroupContent>
                        <SidebarMenu>
                            <SidebarMenuItem>
                                <SidebarMenuButton isActive tooltip="Campaigns">
                                    <Megaphone />
                                    <span>Campaigns</span>
                                </SidebarMenuButton>
                                <SidebarMenuBadge>5</SidebarMenuBadge>
                            </SidebarMenuItem>
                            <SidebarMenuItem>
                                <SidebarMenuButton tooltip="Journeys">
                                    <Route />
                                    <span>Journeys</span>
                                </SidebarMenuButton>
                                <SidebarMenuAction showOnHover aria-label="More">
                                    <MoreHorizontal />
                                </SidebarMenuAction>
                            </SidebarMenuItem>
                            <SidebarMenuItem>
                                <SidebarMenuButton tooltip="Users">
                                    <Users />
                                    <span>Users</span>
                                </SidebarMenuButton>
                                <SidebarMenuSub>
                                    <SidebarMenuSubItem>
                                        <SidebarMenuSubButton href="#" isActive>
                                            <span>All users</span>
                                        </SidebarMenuSubButton>
                                    </SidebarMenuSubItem>
                                    <SidebarMenuSubItem>
                                        <SidebarMenuSubButton href="#">
                                            <span>Lists</span>
                                        </SidebarMenuSubButton>
                                    </SidebarMenuSubItem>
                                </SidebarMenuSub>
                            </SidebarMenuItem>
                        </SidebarMenu>
                    </SidebarGroupContent>
                </SidebarGroup>
                <SidebarSeparator />
                <SidebarGroup>
                    <SidebarGroupLabel>Projects</SidebarGroupLabel>
                    <SidebarGroupAction aria-label="Add project">
                        <Plus />
                    </SidebarGroupAction>
                    <SidebarGroupContent>
                        <SidebarMenu>
                            {loading
                                ? Array.from({ length: 3 }).map((_, i) => (
                                      <SidebarMenuItem key={i}>
                                          <SidebarMenuSkeleton showIcon />
                                      </SidebarMenuItem>
                                  ))
                                : ["Northwind Mobile", "Northwind Web"].map((name) => (
                                      <SidebarMenuItem key={name}>
                                          <SidebarMenuButton tooltip={name}>
                                              <FolderClosed />
                                              <span>{name}</span>
                                          </SidebarMenuButton>
                                      </SidebarMenuItem>
                                  ))}
                        </SidebarMenu>
                    </SidebarGroupContent>
                </SidebarGroup>
            </SidebarContent>
            <SidebarFooter>
                <SidebarMenu>
                    <SidebarMenuItem>
                        <SidebarMenuButton tooltip="Settings">
                            <Settings />
                            <span>Settings</span>
                        </SidebarMenuButton>
                    </SidebarMenuItem>
                </SidebarMenu>
            </SidebarFooter>
            <SidebarRail />
        </Sidebar>
    )
}

function Content() {
    return (
        <SidebarInset>
            <header className="flex h-14 items-center gap-2 border-b px-4">
                <SidebarTrigger />
                <span className="text-sm font-medium">Campaigns</span>
            </header>
            <div className="p-6 text-sm text-muted-foreground">Page content</div>
        </SidebarInset>
    )
}

export const Expanded: Story = {
    render: () => (
        <SidebarProvider>
            <DemoSidebar />
            <Content />
        </SidebarProvider>
    ),
}

export const Collapsed: Story = {
    render: () => (
        <SidebarProvider defaultOpen={false}>
            <DemoSidebar />
            <Content />
        </SidebarProvider>
    ),
}

export const Loading: Story = {
    render: () => (
        <SidebarProvider>
            <DemoSidebar loading />
            <Content />
        </SidebarProvider>
    ),
}
