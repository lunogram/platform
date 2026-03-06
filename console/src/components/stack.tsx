import { cn } from "@/utils"
import type { CSSProperties, PropsWithChildren } from "react"

type StackProps = PropsWithChildren<{
    className?: string
    style?: CSSProperties
    vertical?: boolean
}>

export default function Stack({ children, className, style, vertical }: StackProps) {
    return (
        <div
            className={cn(
                "flex gap-2.5 mb-2.5 flex-wrap",
                vertical && "flex-col flex-nowrap",
                className,
            )}
            style={style}
        >
            {children}
        </div>
    )
}
