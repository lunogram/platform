import type { Meta, StoryObj } from "@storybook/react"
import { Home, Settings, Users, Bell, LogOut } from "lucide-react"
import {
    SidebarProvider,
    Sidebar,
    SidebarContent,
    SidebarGroup,
    SidebarGroupLabel,
    SidebarGroupContent,
    SidebarMenu,
    SidebarMenuItem,
    SidebarMenuButton,
    SidebarHeader,
    SidebarFooter,
    SidebarTrigger,
    SidebarInset,
} from "./sidebar"

const meta: Meta = {
    title: "UI/Sidebar",
    tags: ["autodocs"],
}

export default meta
type Story = StoryObj

export const Default: Story = {
    render: () => (
        <SidebarProvider>
            <div className="flex h-[500px] w-full overflow-hidden rounded-lg border">
                <Sidebar>
                    <SidebarHeader className="px-4 py-3">
                        <span className="font-semibold text-sm">My App</span>
                    </SidebarHeader>
                    <SidebarContent>
                        <SidebarGroup>
                            <SidebarGroupLabel>Navigation</SidebarGroupLabel>
                            <SidebarGroupContent>
                                <SidebarMenu>
                                    <SidebarMenuItem>
                                        <SidebarMenuButton>
                                            <Home className="h-4 w-4" />
                                            <span>Home</span>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                    <SidebarMenuItem>
                                        <SidebarMenuButton>
                                            <Users className="h-4 w-4" />
                                            <span>Users</span>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                    <SidebarMenuItem>
                                        <SidebarMenuButton>
                                            <Bell className="h-4 w-4" />
                                            <span>Notifications</span>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                </SidebarMenu>
                            </SidebarGroupContent>
                        </SidebarGroup>
                        <SidebarGroup>
                            <SidebarGroupLabel>System</SidebarGroupLabel>
                            <SidebarGroupContent>
                                <SidebarMenu>
                                    <SidebarMenuItem>
                                        <SidebarMenuButton>
                                            <Settings className="h-4 w-4" />
                                            <span>Settings</span>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                </SidebarMenu>
                            </SidebarGroupContent>
                        </SidebarGroup>
                    </SidebarContent>
                    <SidebarFooter className="px-4 py-3">
                        <SidebarMenu>
                            <SidebarMenuItem>
                                <SidebarMenuButton>
                                    <LogOut className="h-4 w-4" />
                                    <span>Sign out</span>
                                </SidebarMenuButton>
                            </SidebarMenuItem>
                        </SidebarMenu>
                    </SidebarFooter>
                </Sidebar>
                <SidebarInset className="flex flex-col flex-1">
                    <header className="flex items-center gap-2 border-b px-4 py-3">
                        <SidebarTrigger />
                        <h1 className="text-sm font-semibold">Dashboard</h1>
                    </header>
                    <main className="flex-1 p-6">
                        <p className="text-muted-foreground text-sm">
                            Main content area. Toggle the sidebar with the trigger button.
                        </p>
                    </main>
                </SidebarInset>
            </div>
        </SidebarProvider>
    ),
}

export const CollapsedByDefault: Story = {
    render: () => (
        <SidebarProvider defaultOpen={false}>
            <div className="flex h-[500px] w-full overflow-hidden rounded-lg border">
                <Sidebar>
                    <SidebarHeader className="px-4 py-3">
                        <span className="font-semibold text-sm">My App</span>
                    </SidebarHeader>
                    <SidebarContent>
                        <SidebarGroup>
                            <SidebarGroupLabel>Navigation</SidebarGroupLabel>
                            <SidebarGroupContent>
                                <SidebarMenu>
                                    <SidebarMenuItem>
                                        <SidebarMenuButton>
                                            <Home className="h-4 w-4" />
                                            <span>Home</span>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                    <SidebarMenuItem>
                                        <SidebarMenuButton>
                                            <Users className="h-4 w-4" />
                                            <span>Users</span>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                    <SidebarMenuItem>
                                        <SidebarMenuButton>
                                            <Settings className="h-4 w-4" />
                                            <span>Settings</span>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                </SidebarMenu>
                            </SidebarGroupContent>
                        </SidebarGroup>
                    </SidebarContent>
                </Sidebar>
                <SidebarInset className="flex flex-col flex-1">
                    <header className="flex items-center gap-2 border-b px-4 py-3">
                        <SidebarTrigger />
                        <h1 className="text-sm font-semibold">Dashboard</h1>
                    </header>
                    <main className="flex-1 p-6">
                        <p className="text-muted-foreground text-sm">
                            Sidebar is collapsed by default. Click the trigger to expand it.
                        </p>
                    </main>
                </SidebarInset>
            </div>
        </SidebarProvider>
    ),
}
