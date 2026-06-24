import { UserButton } from "@clerk/clerk-react"

import { SidebarMenu, SidebarMenuItem, useSidebar } from "@/components/ui/sidebar"
import type { UserDropdownProps } from "./types"

export function ClerkUserDropdown(_props: UserDropdownProps) {
    const { state } = useSidebar()
    const isCollapsed = state === "collapsed"

    return (
        <SidebarMenu>
            <SidebarMenuItem>
                <UserButton
                    showName={!isCollapsed}
                    appearance={{
                        elements: {
                            rootBox: "w-full",
                            userButtonTrigger: {
                                width: "100%",
                                padding: "0.5rem",
                                borderRadius: "0.375rem",
                                justifyContent: isCollapsed ? "center" : "flex-start",
                            },
                            userButtonBox: {
                                width: "100%",
                                gap: "0.5rem",
                                flexDirection: "row-reverse",
                            },
                            userButtonOuterIdentifier:
                                "text-sm font-medium text-sidebar-foreground truncate flex-1 text-left",
                            avatarBox: "size-8 shrink-0",
                            avatarImage: "rounded-lg",
                        },
                    }}
                />
            </SidebarMenuItem>
        </SidebarMenu>
    )
}
