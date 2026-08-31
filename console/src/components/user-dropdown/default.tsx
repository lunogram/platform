import { useCallback, useEffect, useState } from "react"
import { KeyRound, LogOut } from "lucide-react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
    SidebarMenu,
    SidebarMenuButton,
    SidebarMenuItem,
    useSidebar,
} from "@/components/ui/sidebar"
import { logout, cn } from "@/utils"
import api from "@/api"
import { AUTH_DRIVERS } from "@/types"
import { ChangePasswordDialog } from "./change-password-dialog"
import type { UserDropdownProps } from "./types"

function getInitials(name: string): string {
    const parts = name.trim().split(/\s+/)
    if (parts.length >= 2) {
        return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
    }
    return name.slice(0, 2).toUpperCase()
}

export function DefaultUserDropdown({ user }: UserDropdownProps) {
    const { state } = useSidebar()
    const isCollapsed = state === "collapsed"

    const initials = getInitials(user.name)
    const [changingPassword, setChangingPassword] = useState(false)
    const [hasPasswordDriver, setHasPasswordDriver] = useState(false)

    // Offered only where it can work. On a deployment that authenticates through
    // an upstream there is no password here to change, and the endpoint would
    // answer as much.
    useEffect(() => {
        api.auth
            .cachedMethods()
            .then((methods) => setHasPasswordDriver(methods.includes(AUTH_DRIVERS.BASIC)))
            .catch(() => setHasPasswordDriver(false))
    }, [])

    const handleLogout = useCallback(async () => {
        await logout(undefined)
    }, [])

    return (
        <SidebarMenu>
            <SidebarMenuItem>
                <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                        <SidebarMenuButton
                            size="lg"
                            className={cn(
                                "w-full cursor-pointer data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground",
                                isCollapsed && "justify-center",
                            )}
                        >
                            <Avatar className="size-8 [[data-mobile=true]_&]:size-10">
                                <AvatarFallback className="bg-primary text-primary-foreground text-xs font-medium [[data-mobile=true]_&]:text-sm">
                                    {initials}
                                </AvatarFallback>
                            </Avatar>

                            {!isCollapsed && (
                                <>
                                    <div className="grid flex-1 text-left text-sm leading-tight">
                                        <span className="truncate font-medium">{user.name}</span>
                                        {user.email && user.name !== user.email && (
                                            <span className="truncate text-xs text-muted-foreground">
                                                {user.email}
                                            </span>
                                        )}
                                    </div>
                                </>
                            )}
                        </SidebarMenuButton>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent
                        className="w-(--radix-dropdown-menu-trigger-width) min-w-56 rounded-lg"
                        side="top"
                        align="start"
                        sideOffset={4}
                    >
                        {hasPasswordDriver && (
                            <>
                                <DropdownMenuItem
                                    onSelect={() => setChangingPassword(true)}
                                    className="cursor-pointer"
                                >
                                    <KeyRound className="mr-2 size-4" />
                                    Change password
                                </DropdownMenuItem>
                                <DropdownMenuSeparator />
                            </>
                        )}
                        <DropdownMenuItem onSelect={handleLogout} className="cursor-pointer">
                            <LogOut className="mr-2 size-4" />
                            Sign out
                        </DropdownMenuItem>
                    </DropdownMenuContent>
                </DropdownMenu>
            </SidebarMenuItem>

            <ChangePasswordDialog open={changingPassword} onOpenChange={setChangingPassword} />
        </SidebarMenu>
    )
}
