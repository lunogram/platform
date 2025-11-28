import { ChevronDown } from "lucide-react";

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
} from "@/components/ui/sidebar";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "./dropdown-menu";
import { useCallback, useContext, useState, type ReactNode } from "react";
import { AdminContext, OrganizationContext, ProjectContext } from "@/contexts";
import api from "@/api";
import { useResolver } from "@/hooks";
import { checkProjectRole, getRecentProjects, logout, snakeToTitle } from "@/utils";
import { NavLink, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import type { Project, ProjectRole } from "@/types";
import { PreferencesContext } from "@/ui/PreferencesContext";
import { useClerk } from "@clerk/clerk-react";
import Modal from "@/ui/Modal";
import RadioInput from "@/ui/form/RadioInput";
import type { NavLinkProps } from "react-router";

interface AppSidebarProps {
  links?: Array<
    NavLinkProps & {
      key: string;
      icon: ReactNode;
      minRole?: ProjectRole;
      active?: (project: Project) => boolean;
    }
  >;
}

export function AppSidebar({ links }: AppSidebarProps) {
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();
  const { signOut } = useClerk();
  const profile = useContext(AdminContext);
  const [project] = useContext(ProjectContext);
  const [organization] = useContext(OrganizationContext);
  const [preferences, setPreferences] = useContext(PreferencesContext);
  const [isLanguageOpen, setIsLanguageOpen] = useState(false);
  
  const [recents] = useResolver(
    useCallback(async () => {
      const recentIds = getRecentProjects()
        .filter((p) => p.id !== project.id)
        .map((p) => p.id);
      const recents: Array<Project> = [];
      if (recentIds.length) {
        const projects = await api.projects
          .search({
            limit: recentIds.length,
            id: recentIds,
          })
          .then((r) => r.results ?? []);
        for (const id of recentIds) {
          const recent = projects.find((p) => p.id === id);
          if (recent) {
            recents.push(recent);
          }
        }
      }
      return [
        project,
        ...recents,
        {
          name: t("view_all"),
        } as Project,
      ];
    }, [project, t])
  );

  const handleProjectChange = async (selectedProject: Project) => {
    if (!selectedProject.id) {
      await navigate('/organization/projects');
    } else {
      await navigate(`/projects/${selectedProject.id}`);
    }
  };

  // Filter links based on role and active status
  const filteredLinks = links?.filter(
    ({ minRole, active }) =>
      (!minRole || checkProjectRole(minRole, project.role)) &&
      (!active || active(project))
  );

  return (
    <Sidebar>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton size="lg">
                  <div className="flex flex-col items-start flex-1 text-left">
                    <div className="text-xs text-muted-foreground">{t('project_selector')}</div>
                    <div className="text-sm font-medium">{project?.name}</div>
                  </div>
                  <ChevronDown className="ml-auto" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent className="w-[--radix-popper-anchor-width]">
                {recents?.map((proj) => (
                  <DropdownMenuItem
                    key={proj.id || 'view-all'}
                    onSelect={() => handleProjectChange(proj)}
                  >
                    <span>{proj.name}</span>
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>Navigation</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {filteredLinks?.map((link) => {
                // eslint-disable-next-line @typescript-eslint/no-unused-vars
                const { key, icon, minRole, active, children, to, ...rest } = link;
                return (
                  <SidebarMenuItem key={key}>
                    <SidebarMenuButton asChild>
                      <NavLink to={to} {...rest}>
                        {icon}
                        <span>{children as ReactNode}</span>
                      </NavLink>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      {profile && (
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <SidebarMenuButton size="lg">
                    <div className="flex items-center gap-2 flex-1">
                      <img
                        src={profile.image_url}
                        referrerPolicy="no-referrer"
                        className="h-8 w-8 rounded-full"
                        alt="Profile"
                      />
                      <div className="flex flex-col items-start flex-1 text-left">
                        <span className="text-sm font-medium">
                          {console.log('profile', profile)}
                          {profile.first_name
                            ? `${profile.first_name} ${profile.last_name ?? ''}`.trim()
                            : 'User'}
                        </span>
                        <span className="text-xs text-muted-foreground">
                          {snakeToTitle(project.role ?? organization.username)}
                        </span>
                      </div>
                    </div>
                    <ChevronDown className="ml-auto" />
                  </SidebarMenuButton>
                </DropdownMenuTrigger>
                <DropdownMenuContent className="w-[--radix-popper-anchor-width]">
                  <DropdownMenuItem onSelect={async () => await navigate('/organization')}>
                    {t('settings')}
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => setIsLanguageOpen(true)}>
                    {t('language')}
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onSelect={() =>
                      setPreferences({
                        ...preferences,
                        mode: preferences.mode === 'dark' ? 'light' : 'dark',
                      })
                    }
                  >
                    {preferences.mode === 'dark' ? t('light_mode') : t('dark_mode')}
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={async () => await logout(signOut)}>
                    {t('sign_out')}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      )}
      <Modal open={isLanguageOpen} onClose={() => setIsLanguageOpen(false)} title={t('language')}>
        <RadioInput
          label={t('language')}
          options={[
            { label: 'English', key: 'en' },
            { label: 'Español', key: 'es' },
            { label: '简体中文', key: 'zh' },
          ]}
          value={i18n.language}
          onChange={(value) => {
            setPreferences({ ...preferences, lang: value });
          }}
        />
      </Modal>
    </Sidebar>
  );
}
