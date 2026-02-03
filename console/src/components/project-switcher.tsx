import { Check, ChevronsUpDown, Plus } from "lucide-react";
import { useNavigate } from "react-router";
import type { Project } from "@/types";
import { getRandomColor, getRandomIcon } from "@/lib/projects";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";

export function ProjectSwitcher({
  projects,
  currentProject,
}: {
  projects: Project[];
  currentProject: Project;
}) {
  const navigate = useNavigate();
  const projectColor = currentProject?.name ? getRandomColor(currentProject.name) : "#6366f1";
  const projectIcon = currentProject?.name ? getRandomIcon(currentProject.name) : "folder";

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger className="w-full">
            <SidebarMenuButton
              size="lg"
              className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground cursor-pointer"
            >
              <div 
                className="flex aspect-square size-8 items-center justify-center rounded-lg text-white"
                style={{ backgroundColor: projectColor }}
              >
                <i className={`fa-solid fa-${projectIcon} text-sm`} />
              </div>
              <div className="flex flex-col gap-0.5 leading-none">
                <span className="font-semibold">Projects</span>
                <span className="">
                  {currentProject?.name || "Select Project"}
                </span>
              </div>
              <ChevronsUpDown className="ml-auto" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-[--radix-dropdown-menu-trigger-width] min-w-56"
            align="start"
            side="bottom"
            sideOffset={4}
          >
            {projects.map((project) => (
              <DropdownMenuItem
                key={project.id}
                onSelect={() => navigate(`/projects/${project.id}`)}
                className="cursor-pointer"
              >
                {project.name}
                {project.id === currentProject.id && (
                  <Check className="ml-auto" />
                )}
              </DropdownMenuItem>
            ))}
            {projects.length > 0 && <DropdownMenuSeparator />}
            <DropdownMenuItem
              onSelect={() => navigate("/onboarding/project")}
              className="gap-2 cursor-pointer"
            >
              <Plus className="size-4" />
              Create New Project
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
