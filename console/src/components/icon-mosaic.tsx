import { useState } from "react"
import { cn } from "@/utils"

const DEFAULT_COLOR = "#000000"
const DEFAULT_ICON = "https://lunogram.com/logos/icon-black-512.png"

/** Resolve a brand colour, falling back to Lunogram black. */
function brandColor(color?: string): string {
    return color ?? DEFAULT_COLOR
}

const DEFAULT_COLS = 7
const DEFAULT_ROWS = 5

interface StaggeredMosaicProps {
    /** The provider currently being configured. */
    provider?: { id: string; name: string; icon?: string; color?: string }
    /** Number of columns (default 7). */
    cols?: number
    /** Number of rows (default 5). */
    rows?: number
    className?: string
}

/**
 * Decorative staggered grid with the selected provider's icon
 * prominently centered, surrounded by greyed-out cubes.
 *
 * Odd rows are offset by half a cell width to create the stagger.
 * The center cell holds the provider logo.
 */
export function StaggeredMosaic({
    provider,
    cols = DEFAULT_COLS,
    rows = DEFAULT_ROWS,
    className,
}: StaggeredMosaicProps) {
    const [imgFailed, setImgFailed] = useState(false)
    const color = brandColor(provider?.color)
    const icon = provider?.icon ?? DEFAULT_ICON
    const showImg = !imgFailed

    const centerRow = Math.floor(rows / 2)
    const centerCol = Math.floor(cols / 2)

    // Cell dimensions for calculating the offset needed to align center tile
    // with the vertical midpoint of the parent container.
    const cellSize = 80
    const gap = 10 // gap-2.5 = 10px
    const gridHeight = rows * cellSize + (rows - 1) * gap
    const centerTileMiddle = centerRow * (cellSize + gap) + cellSize / 2
    // Shift the grid up so the center tile's middle aligns with 50% of parent
    const offsetY = centerTileMiddle - gridHeight / 2

    // Radial mask should be centered on the center tile, not the grid midpoint
    const maskCenterY = (centerTileMiddle / gridHeight) * 100

    return (
        <div
            className={cn(
                "relative flex items-center justify-center select-none w-full",
                className,
            )}
        >
            {provider && (
                <div
                    className="pointer-events-none absolute inset-0"
                    style={{
                        background: `radial-gradient(circle at 50% 50%, ${color}10 0%, transparent 60%)`,
                    }}
                />
            )}

            <div
                className="relative flex flex-col gap-2.5"
                style={{
                    transform: `translateY(-${offsetY}px)`,
                    maskImage: `radial-gradient(ellipse 70% 70% at 50% ${maskCenterY}%, black 30%, transparent 100%)`,
                    WebkitMaskImage: `radial-gradient(ellipse 70% 70% at 50% ${maskCenterY}%, black 30%, transparent 100%)`,
                }}
            >
                {Array.from({ length: rows }, (_, row) => {
                    const isOffset = row % 2 === 1

                    return (
                        <div
                            key={row}
                            className="flex gap-2.5"
                            style={{
                                paddingLeft: isOffset ? 40 : 0,
                            }}
                        >
                            {Array.from({ length: cols }, (_, col) => {
                                const isCenter = row === centerRow && col === centerCol

                                if (isCenter && provider) {
                                    return (
                                        <div
                                            key={col}
                                            className="flex h-20 w-20 items-center justify-center rounded-2xl"
                                            style={{
                                                background: `${color}18`,
                                                border: `2px solid ${color}50`,
                                                boxShadow: `0 0 32px ${color}25, 0 4px 20px ${color}15`,
                                            }}
                                        >
                                            {showImg ? (
                                                <img
                                                    src={icon}
                                                    alt={provider?.name ?? ""}
                                                    className="h-10 w-10 object-contain"
                                                    draggable={false}
                                                    onError={() => setImgFailed(true)}
                                                />
                                            ) : (
                                                <span
                                                    className="font-mono text-lg font-bold leading-none"
                                                    style={{ color }}
                                                >
                                                    {provider.name.slice(0, 2).toUpperCase()}
                                                </span>
                                            )}
                                        </div>
                                    )
                                }

                                // Distance from center for opacity falloff
                                const dist = Math.sqrt(
                                    (row - centerRow) ** 2 + (col - centerCol) ** 2,
                                )
                                const maxDist = Math.sqrt(centerRow ** 2 + centerCol ** 2)
                                const opacity = Math.max(0.4, 1 - (dist / maxDist) * 0.7)

                                return (
                                    <div
                                        key={col}
                                        className="h-20 w-20 rounded-2xl border border-border/80 bg-background shadow-sm"
                                        style={{ opacity }}
                                    />
                                )
                            })}
                        </div>
                    )
                })}
            </div>
        </div>
    )
}
