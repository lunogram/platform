import type { PropsWithChildren, ReactNode } from "react"
import Heading from "@/components/heading"
import { cn } from "@/utils"

type PageHeaderProps = PropsWithChildren<{
    title: ReactNode
    actions?: ReactNode
    desc?: ReactNode
    banner?: ReactNode
    fullscreen?: boolean
}>

export default function PageContent({
    actions,
    children,
    desc,
    title,
    banner,
    fullscreen = false,
}: PageHeaderProps) {
    return (
        <div className={cn("page-content", fullscreen && "fullscreen")}>
            {banner && <div className="page-banner">{banner}</div>}
            <Heading title={title} actions={actions}>
                {desc}
            </Heading>
            {children}
        </div>
    )
}
