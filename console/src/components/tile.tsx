import clsx from "clsx"
import type { PropsWithChildren, ReactNode } from "react"

type TileProps = {
    onClick?: () => void
    selected?: boolean
    iconUrl?: string
    title: ReactNode
    size?: "large" | "regular"
    children: ReactNode
} & Omit<JSX.IntrinsicElements["div"], "title">

export default function Tile({
    onClick,
    selected = false,
    children,
    className,
    iconUrl,
    title,
    size = "regular",
    ...rest
}: TileProps) {
    return (
        <div
            {...rest}
            className={clsx(
                "flex items-center gap-2.5 rounded-md border transition-shadow duration-100",
                size === "large" ? "px-5 py-[30px]" : "p-5",
                selected ? "border-primary" : "border-border",
                onClick !== undefined && "cursor-pointer hover:shadow-md",
                className,
            )}
            onClick={onClick}
            tabIndex={0}
        >
            {iconUrl && (
                <img
                    src={iconUrl}
                    className="max-w-[40px] w-full shrink-0 rounded-md"
                    aria-hidden
                />
            )}
            <div className="grow">
                <h5 className={clsx("m-0 mb-1.5", size === "large" && "text-3xl mb-2.5")}>
                    {title}
                </h5>
                <p className="text-[13px] m-0 text-muted-foreground">{children}</p>
            </div>
        </div>
    )
}

interface TileGridProps extends PropsWithChildren {
    numColumns?: number
}

export function TileGrid({ children, numColumns = 3 }: TileGridProps) {
    return (
        <div
            className="grid gap-2.5 my-4 max-sm:!grid-cols-1"
            style={{ gridTemplateColumns: `repeat(${numColumns}, 1fr)` }}
        >
            {children}
        </div>
    )
}
