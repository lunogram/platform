import type { NavLinkProps } from "react-router";
import type { PropsWithChildren, ReactNode } from "react";
import { SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar";
import { AppSidebar } from "@/components/ui/project-sidebar";
import type { Project, ProjectRole } from "../../types";

interface SidebarProps {
  links?: Array<
    NavLinkProps & {
      key: string;
      icon: ReactNode;
      minRole?: ProjectRole;
      active?: (project: Project) => boolean;
    }
  >;
  prepend?: ReactNode;
  append?: ReactNode;
}

export default function ProjectSidebar({
  children,
  links,
}: PropsWithChildren<SidebarProps>) {
  return (
    <SidebarProvider>
      <AppSidebar links={links} />
      <main>
        <SidebarTrigger />
        {children}
      </main>
    </SidebarProvider>
  );
}
