import type { PropsWithChildren, ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import {
    BookIcon,
    BuildingIcon,
    CampaignsIcon,
    IntegrationsIcon,
    JourneysIcon,
    ListsIcon,
    SettingsIcon,
    UsersIcon,
} from "@/components/icons"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import {
    Sidebar,
    SidebarContent,
    SidebarFooter,
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarHeader,
    SidebarInset,
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarProvider,
    SidebarRail,
} from "@/components/ui/sidebar"
import { WorkspaceSwitcher } from "@/components/workspace-switcher"
import { organizations, project, projects } from "./campaigns"

const links: { key: string; label: string; icon: ReactNode }[] = [
    { key: "campaigns", label: "campaign.plural", icon: <CampaignsIcon /> },
    { key: "journeys", label: "journeys", icon: <JourneysIcon /> },
    { key: "organizations", label: "organizations", icon: <BuildingIcon /> },
    { key: "users", label: "users", icon: <UsersIcon /> },
    { key: "lists", label: "lists", icon: <ListsIcon /> },
    { key: "integrations", label: "integrations", icon: <IntegrationsIcon /> },
    { key: "settings", label: "settings", icon: <SettingsIcon /> },
]

// ConsoleShell mirrors ProjectSidebar and AppSidebar (src/components/app-sidebar.tsx)
// with fixture data. The account row is drawn from primitives because the real
// UserDropdown asks the API which sign-in methods exist.
export function ConsoleShell({ active, children }: PropsWithChildren<{ active: string }>) {
    const { t } = useTranslation()
    return (
        <SidebarProvider>
            <Sidebar>
                <SidebarHeader>
                    <WorkspaceSwitcher
                        organizations={organizations}
                        projects={projects}
                        currentProject={project}
                    />
                </SidebarHeader>
                <SidebarContent>
                    <SidebarGroup>
                        <SidebarGroupLabel>Navigation</SidebarGroupLabel>
                        <SidebarGroupContent>
                            <SidebarMenu>
                                {links.map((link) => (
                                    <SidebarMenuItem key={link.key}>
                                        <SidebarMenuButton asChild isActive={link.key === active}>
                                            <Link to={link.key}>
                                                {link.icon}
                                                {t(link.label)}
                                            </Link>
                                        </SidebarMenuButton>
                                    </SidebarMenuItem>
                                ))}
                            </SidebarMenu>
                        </SidebarGroupContent>
                    </SidebarGroup>
                </SidebarContent>
                <SidebarFooter>
                    <a
                        href="#"
                        className="flex items-center gap-3 rounded-lg border bg-sidebar-accent p-3 text-sm transition-colors hover:bg-sidebar-accent/80"
                    >
                        <BookIcon />
                        <div className="flex flex-col">
                            <span className="font-medium">{t("sidebar.api_docs.title")}</span>
                            <span className="text-xs text-muted-foreground">
                                {t("sidebar.api_docs.description")}
                            </span>
                        </div>
                    </a>
                    <SidebarMenu>
                        <SidebarMenuItem>
                            <SidebarMenuButton size="lg">
                                <Avatar className="size-8">
                                    <AvatarFallback className="bg-primary text-primary-foreground text-xs font-medium">
                                        JC
                                    </AvatarFallback>
                                </Avatar>
                                <div className="grid flex-1 text-left text-sm leading-tight">
                                    <span className="truncate font-medium">Jane Cooper</span>
                                    <span className="truncate text-xs text-muted-foreground">
                                        jane@northwind.example
                                    </span>
                                </div>
                            </SidebarMenuButton>
                        </SidebarMenuItem>
                    </SidebarMenu>
                </SidebarFooter>
                <SidebarRail />
            </Sidebar>
            <SidebarInset>
                <div className="flex flex-1 flex-col gap-4">{children}</div>
            </SidebarInset>
        </SidebarProvider>
    )
}
