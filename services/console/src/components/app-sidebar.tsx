import * as React from "react";

import { ProjectSwitcher } from "@/components/project-switcher";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from "@/components/ui/sidebar";
import { Link, useLocation } from "react-router";
import type { SidebarLink } from "@/types/sidebar";
import { useContext } from "react";
import { AdminContext, ProjectContext } from "@/contexts";
import { useResolver } from "@/hooks";
import api from "@/api";
import { UserDropdown } from "./user-dropdown";

interface AppSidebarProps {
  links?: SidebarLink[];
}

export function AppSidebar({
  links,
  ...props
}: AppSidebarProps & React.ComponentProps<typeof Sidebar>) {
  const [project] = useContext(ProjectContext);
  const profile = useContext(AdminContext);
  const location = useLocation();

  const [allProjects] = useResolver(
    React.useCallback(async () => {
      try {
        return (await api.projects.all()).results;
      } catch (error) {
        console.error("Failed to fetch projects:", error);
        return [];
      }
    }, [])
  );

  return (
    <Sidebar {...props}>
      <SidebarHeader>
        {allProjects && allProjects.length > 0 && project && (
          <ProjectSwitcher projects={allProjects} currentProject={project} />
        )}
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Navigation</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {links?.map((item) => {
                const isActive = location.pathname.includes(String(item.to));
                return (
                  <SidebarMenuItem key={item.key}>
                    <SidebarMenuButton asChild isActive={isActive}>
                      <Link to={item.to}>
                        {item.icon}
                        {typeof item.children === "function"
                          ? item.children({
                              isActive: isActive,
                              isPending: false,
                              isTransitioning: false,
                            })
                          : item.children}
                      </Link>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <UserDropdown
          user={{
            name: profile?.first_name || "User",
            email: profile?.email || "user@example.com",
          }}
        />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
