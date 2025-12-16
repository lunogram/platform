import { AppSidebar } from "@/components/app-sidebar";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import type { PropsWithChildren, ReactNode } from "react";
import type { SidebarLink } from "@/types/sidebar";

interface SidebarProps {
  links?: SidebarLink[];
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
      <SidebarInset>
        <header className="flex h-16 shrink-0 md:hidden items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" />
        </header>
        <div className="flex flex-1 flex-col gap-4 p-4">{children}</div>
      </SidebarInset>
    </SidebarProvider>
  );
}
