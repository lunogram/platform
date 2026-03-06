import type { PropsWithChildren } from "react"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/utils"

import "./Tag.css"

export type TagVariant = "info" | "plain" | "success" | "error" | "warn"
export type TagSize = "tiny" | "regular" | "large"

export type TagProps = PropsWithChildren<{
    onClick?: () => void
    children?: React.ReactNode
    variant?: TagVariant
    size?: TagSize
}>

export default function Tag({ variant = "info", size = "regular", children, onClick }: TagProps) {
    return (
        <Badge className={cn("ui-tag", variant, size)}>
            {children}
            {onClick && <div className="tag-close bi-x" onClick={onClick} />}
        </Badge>
    )
}

export function TagGroup({ children }: PropsWithChildren) {
    return <div className="ui-tag-group">{children}</div>
}
