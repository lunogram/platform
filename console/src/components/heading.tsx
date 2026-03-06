import { cn } from "@/utils"

interface HeadingProps {
    title: React.ReactNode
    size?: "h2" | "h3" | "h4" | "h5"
    actions?: React.ReactNode
    children?: React.ReactNode
    className?: string
}

const sizeStyles = {
    h2: "py-5 pb-4",
    h3: "my-5 mb-2.5",
    h4: "my-2.5",
    h5: "my-1.5",
}

export default function Heading({
    title,
    actions,
    children,
    size = "h2",
    className,
}: HeadingProps) {
    const HeadingTitle = size as keyof JSX.IntrinsicElements
    return (
        <div
            className={cn(
                "flex items-center justify-between flex-wrap gap-2.5",
                sizeStyles[size],
                className,
            )}
        >
            <div className="flex flex-col">
                <HeadingTitle className="m-0 shrink-0 font-semibold tracking-tight">
                    {title}
                </HeadingTitle>
                {children && <div className="mt-1">{children}</div>}
            </div>
            {actions && <div className="flex justify-end self-start gap-2.5">{actions}</div>}
        </div>
    )
}
