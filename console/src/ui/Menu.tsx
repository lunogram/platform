import type { PropsWithChildren } from "react"
import { Button } from "@/components/ui/button"
import "./Menu.css"
import { ThreeDotsIcon } from "../components/icons"

import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"

interface MenuProps {
    thing?: string
    variant?: "default" | "destructive" | "outline" | "secondary" | "ghost" | "link"
    size?: "default" | "min" | "sm" | "lg" | "icon"
    placement?:
        | "top"
        | "top-start"
        | "top-end"
        | "right"
        | "right-start"
        | "right-end"
        | "bottom"
        | "bottom-start"
        | "bottom-end"
        | "left"
        | "left-start"
        | "left-end"
    button?: React.ReactNode
}

interface MenuItemProps {
    onClick?: () => void
}

export function MenuItem({ children, onClick }: PropsWithChildren<MenuItemProps>) {
    return (
        <DropdownMenuItem
            className="ui-menu-item"
            onSelect={(event) => {
                onClick?.()
                event.preventDefault()
            }}
        >
            {children}
        </DropdownMenuItem>
    )
}

export default function Menu({
    children,
    variant,
    size,
    placement,
    button,
}: PropsWithChildren<MenuProps>) {
    const defaultButton = button ?? (
        <Button variant={variant ?? "outline"} size={size ?? "min"}>
            <ThreeDotsIcon />
        </Button>
    )

    // Convert Popper placement to Radix side/align
    const getRadixPlacement = (placement?: string) => {
        if (!placement) return { side: "bottom" as const, align: "end" as const }

        const parts = placement.split("-")
        const side = parts[0] as "top" | "right" | "bottom" | "left"
        const align = parts[1] === "start" ? "start" : parts[1] === "end" ? "end" : "center"

        return { side, align: align as "start" | "end" | "center" }
    }

    const { side, align } = getRadixPlacement(placement)

    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>{defaultButton}</DropdownMenuTrigger>
            <DropdownMenuContent
                className="ui-menu"
                side={side}
                align={align}
                onClick={(e) => e.stopPropagation()}
            >
                {children}
            </DropdownMenuContent>
        </DropdownMenu>
    )
}
