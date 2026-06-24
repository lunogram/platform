import { useState } from "react"
import { Check, ChevronsUpDown, Building2, Loader2 } from "lucide-react"
import { toast } from "sonner"
import { useTranslation } from "react-i18next"

import type { AdminOrganization } from "@/types"
import api from "@/api"

import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from "@/components/ui/sidebar"

export function OrganizationSwitcher({ organizations }: { organizations: AdminOrganization[] }) {
    const { t } = useTranslation()
    const [switching, setSwitching] = useState(false)

    // Nothing to switch between — hide the control entirely.
    if (organizations.length < 2) {
        return null
    }

    // is_active is set by the API from the RESOLVED active organization, so
    // exactly one entry is normally flagged. The organizations[0] fallback only
    // guards the unexpected case where none is (e.g. a stale read); the
    // length < 2 check above guarantees organizations[0] exists here.
    const current = organizations.find((o) => o.is_active) ?? organizations[0]

    const handleSelect = async (organization: AdminOrganization) => {
        if (organization.is_active || switching) {
            return
        }
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

    return (
        <SidebarMenu>
            <SidebarMenuItem>
                <DropdownMenu>
                    <DropdownMenuTrigger asChild className="w-full">
                        <SidebarMenuButton
                            size="lg"
                            className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground cursor-pointer"
                        >
                            <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                                {switching ? (
                                    <Loader2 className="size-4 animate-spin" />
                                ) : (
                                    <Building2 className="size-4" />
                                )}
                            </div>
                            <div className="flex flex-col gap-0.5 leading-none text-left">
                                <span className="text-xs text-muted-foreground">
                                    {t("organization", "Organization")}
                                </span>
                                <span className="font-semibold truncate">{current.name}</span>
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
                        {organizations.map((organization) => (
                            <DropdownMenuItem
                                key={organization.id}
                                onSelect={() => handleSelect(organization)}
                                className="cursor-pointer"
                            >
                                {organization.name}
                                {organization.is_active && <Check className="ml-auto" />}
                            </DropdownMenuItem>
                        ))}
                    </DropdownMenuContent>
                </DropdownMenu>
            </SidebarMenuItem>
        </SidebarMenu>
    )
}
