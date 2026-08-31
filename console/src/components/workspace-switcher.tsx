import { useState } from "react"
import { Check, ChevronsUpDown, Loader2, Plus } from "lucide-react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import api from "@/api"
import type { AdminOrganization, Project } from "@/types"
import { getRandomColor, getRandomIcon } from "@/lib/colors"
import { navigateFromOverlay } from "@/lib/ui-utils"

import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    useSidebar,
} from "@/components/ui/sidebar"

interface WorkspaceSwitcherProps {
    organizations: AdminOrganization[]
    projects: Project[]
    currentProject: Project
}

// WorkspaceSwitcher is the sidebar's single header control: where you are, and
// the one place to go somewhere else.
//
// Organization and project used to be two stacked controls, each with its own
// tile, its own two-line label and its own chevron — six pieces of chrome above
// the first navigation item, for a pair of values that are read together and
// change rarely. They are one line here for the same reason a breadcrumb is one
// line: the organization is the context the project sits in, not a peer of it.
export function WorkspaceSwitcher({
    organizations,
    projects,
    currentProject,
}: WorkspaceSwitcherProps) {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const { setOpenMobile } = useSidebar()
    const [switching, setSwitching] = useState(false)

    // is_active is set by the API from the RESOLVED active organization, so
    // exactly one entry is normally flagged; the fallback only guards a stale
    // read. An admin always belongs to at least one, but the switcher is only
    // worth showing once there is somewhere else to go.
    const currentOrganization = organizations.find((o) => o.is_active) ?? organizations[0]
    const canSwitchOrganization = organizations.length > 1

    const projectName = currentProject?.name ?? ""
    const projectColor = projectName ? getRandomColor(projectName) : "#6366f1"
    const projectIcon = projectName ? getRandomIcon(projectName) : "folder"

    const selectOrganization = async (organization: AdminOrganization) => {
        if (organization.is_active || switching) return
        setSwitching(true)
        try {
            await api.adminOrganizations.setActive(organization.id)
            // The active organization scopes almost everything server-side, so a
            // full reload is the simplest way to land in a clean, consistent state.
            window.location.href = "/"
        } catch {
            toast.error(t("org_switch_failed", "Failed to switch organization."))
            setSwitching(false)
        }
    }

    const goTo = (path: string) => {
        navigateFromOverlay(navigate, () => setOpenMobile(false), path)
    }

    return (
        <SidebarMenu>
            <SidebarMenuItem>
                <DropdownMenu>
                    <DropdownMenuTrigger asChild className="w-full">
                        <SidebarMenuButton
                            size="lg"
                            className="cursor-pointer data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
                        >
                            <div
                                className="flex aspect-square size-8 shrink-0 items-center justify-center rounded-lg text-white [[data-mobile=true]_&]:size-10 [[data-mobile=true]_&]:rounded-xl"
                                style={{ backgroundColor: projectColor }}
                            >
                                {switching ? (
                                    <Loader2 className="size-4 animate-spin" />
                                ) : (
                                    <i
                                        className={`fa-solid fa-${projectIcon} text-sm [[data-mobile=true]_&]:text-base`}
                                    />
                                )}
                            </div>
                            <div className="flex min-w-0 flex-col text-left leading-tight">
                                {currentOrganization && (
                                    <span className="truncate text-xs text-muted-foreground">
                                        {currentOrganization.name}
                                    </span>
                                )}
                                <span className="truncate font-medium">
                                    {projectName || t("select_project", "Select project")}
                                </span>
                            </div>
                            <ChevronsUpDown className="ml-auto shrink-0 text-muted-foreground" />
                        </SidebarMenuButton>
                    </DropdownMenuTrigger>

                    <DropdownMenuContent
                        className="w-[--radix-dropdown-menu-trigger-width] min-w-60"
                        align="start"
                        side="bottom"
                        sideOffset={4}
                    >
                        <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">
                            {t("projects", "Projects")}
                        </DropdownMenuLabel>
                        {projects.map((project) => (
                            <DropdownMenuItem
                                key={project.id}
                                onSelect={() => goTo(`/projects/${project.id}`)}
                                className="cursor-pointer gap-2"
                            >
                                <span
                                    className="flex size-5 shrink-0 items-center justify-center rounded text-[10px] text-white"
                                    style={{ backgroundColor: getRandomColor(project.name) }}
                                >
                                    <i className={`fa-solid fa-${getRandomIcon(project.name)}`} />
                                </span>
                                <span className="truncate">{project.name}</span>
                                {project.id === currentProject?.id && (
                                    <Check className="ml-auto size-4 shrink-0" />
                                )}
                            </DropdownMenuItem>
                        ))}
                        <DropdownMenuItem
                            onSelect={() => goTo("/onboarding/project")}
                            className="cursor-pointer gap-2 text-muted-foreground"
                        >
                            <Plus className="size-4" />
                            {t("create_new_project", "Create new project")}
                        </DropdownMenuItem>

                        {canSwitchOrganization && (
                            <>
                                <DropdownMenuSeparator />
                                <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">
                                    {t("organization", "Organization")}
                                </DropdownMenuLabel>
                                {organizations.map((organization) => (
                                    <DropdownMenuItem
                                        key={organization.id}
                                        onSelect={() => selectOrganization(organization)}
                                        className="cursor-pointer"
                                    >
                                        <span className="truncate">{organization.name}</span>
                                        {organization.is_active && (
                                            <Check className="ml-auto size-4 shrink-0" />
                                        )}
                                    </DropdownMenuItem>
                                ))}
                            </>
                        )}
                    </DropdownMenuContent>
                </DropdownMenu>
            </SidebarMenuItem>
        </SidebarMenu>
    )
}
