import * as React from "react"
import { NavLink } from "react-router"
import type { LucideIcon } from "lucide-react"

import { cn } from "@/utils"
import { Badge } from "@/components/ui/badge"

export interface NavTab {
    key: string
    label: string
    icon: LucideIcon
    badge?: number | null
}

export interface RouteNavTab extends NavTab {
    to: string
}

interface NavTabsBaseProps {
    className?: string
}

interface RouteNavTabsProps extends NavTabsBaseProps {
    tabs: RouteNavTab[]
    activeTab?: string
    value?: never
    onChange?: never
}

interface StateNavTabsProps extends NavTabsBaseProps {
    tabs: NavTab[]
    value: string
    onChange: (key: string) => void
    activeTab?: never
}

export type NavTabsProps = RouteNavTabsProps | StateNavTabsProps

function isRouteMode(props: NavTabsProps): props is RouteNavTabsProps {
    return "activeTab" in props && props.activeTab !== undefined
}

const tabClassName = (isActive: boolean) =>
    cn(
        "cursor-pointer flex items-center gap-2 px-3 sm:px-4 py-2.5 text-sm font-medium rounded-t-lg border-b-2 transition-colors whitespace-nowrap shrink-0",
        isActive
            ? "border-primary text-foreground bg-background"
            : "border-transparent text-muted-foreground hover:text-foreground hover:bg-muted/50",
    )

function TabBadge({ count }: { count: number }) {
    return (
        <Badge variant="secondary" className="h-5 min-w-5 px-1 text-[10px]">
            {count}
        </Badge>
    )
}

const NavTabs = React.forwardRef<HTMLElement, NavTabsProps>((props, ref) => {
    const { tabs, className } = props

    return (
        <nav
            ref={ref}
            className={cn("flex gap-1 -mb-px overflow-x-auto scrollbar-none", className)}
        >
            {tabs.map((tab) => {
                const Icon = tab.icon
                const hasBadge = tab.badge != null && tab.badge > 0

                if (isRouteMode(props)) {
                    const routeTab = tab as RouteNavTab
                    const isActive = props.activeTab === tab.key
                    return (
                        <NavLink
                            key={tab.key}
                            to={routeTab.to}
                            end={routeTab.to === ""}
                            className={tabClassName(isActive)}
                        >
                            <Icon className="h-4 w-4" />
                            {tab.label}
                            {hasBadge && <TabBadge count={tab.badge!} />}
                        </NavLink>
                    )
                }

                const isActive = props.value === tab.key
                return (
                    <button
                        type="button"
                        key={tab.key}
                        onClick={() => props.onChange(tab.key)}
                        className={tabClassName(isActive)}
                    >
                        <Icon className="h-4 w-4" />
                        {tab.label}
                        {hasBadge && <TabBadge count={tab.badge!} />}
                    </button>
                )
            })}
        </nav>
    )
})
NavTabs.displayName = "NavTabs"

export { NavTabs }
