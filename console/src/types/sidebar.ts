import type { ReactNode } from "react"
import type { NavLinkProps } from "react-router"
import type { Project, ProjectRole } from "@/types"

export interface SidebarLink extends NavLinkProps {
    key: string
    icon: ReactNode
    minRole?: ProjectRole
    active?: (project: Project) => boolean
}
