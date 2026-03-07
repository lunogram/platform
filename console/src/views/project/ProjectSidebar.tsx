import { AppSidebar } from "@/components/app-sidebar"
import { SidebarInset, SidebarProvider, SidebarTrigger } from "@/components/ui/sidebar"
import type { PropsWithChildren, ReactNode } from "react"
import type { SidebarLink } from "@/types/sidebar"

interface SidebarProps {
    links?: SidebarLink[]
    prepend?: ReactNode
    append?: ReactNode
}

export default function ProjectSidebar({ children, links }: PropsWithChildren<SidebarProps>) {
    return (
        <SidebarProvider>
            <AppSidebar links={links} />
            <SidebarInset>
                <header className="flex h-16 shrink-0 md:hidden items-center justify-between border-b px-4 pt-[env(safe-area-inset-top)]">
                    <SidebarTrigger className="size-10 [&>svg]:size-6" />
                    <img src="/logo-dark.png" alt="Lunogram" className="h-7 dark:invert" />
                </header>
                <div className="flex flex-1 flex-col gap-4">{children}</div>
            </SidebarInset>
        </SidebarProvider>
    )
}
