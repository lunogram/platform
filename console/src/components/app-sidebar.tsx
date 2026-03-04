import * as React from "react";
import { useTranslation } from "react-i18next";

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
import type { Admin } from "@/types";
import { BookIcon } from "./icons";

interface AppSidebarProps {
  links?: SidebarLink[];
}

export function AppSidebar({
  links,
  ...props
}: AppSidebarProps & React.ComponentProps<typeof Sidebar>) {
  const { t } = useTranslation();
  const [project] = useContext(ProjectContext);
  const profile = useContext(AdminContext);
  const location = useLocation();

  const getUserDisplayName = (profile: Admin | null) => {
    if (profile?.first_name && profile?.last_name) {
      return `${profile.first_name} ${profile.last_name}`;
    }
    return profile?.first_name || profile?.email || "User";
  };

  const [allProjects] = useResolver(
    React.useCallback(async () => {
      try {
        return (await api.projects.all()).results;
      } catch (error) {
        console.error("Failed to fetch projects:", error);
        return [];
      }
    }, []),
  );

  return (
    <Sidebar {...props}>
      <SidebarHeader>
        {allProjects && project && (
          <ProjectSwitcher projects={allProjects} currentProject={project} />
        )}
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Navigation</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {links
                ?.filter((item) => !item.active || item.active(project))
                .map((item) => {
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
        <a
          href="/api/"
          target="_blank"
          rel="noopener noreferrer"
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
        <UserDropdown
          user={{
            name: getUserDisplayName(profile),
            email: profile?.email || "",
          }}
        />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
