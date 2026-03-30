import { getRandomColor } from "@/lib/colors"
import { getUserInitials } from "@/lib/name"
import type { RecipientRow } from "./broadcast-state"

/**
 * Build a lookup of grid positions sorted by Euclidean distance from the
 * center tile.  Positions at the same distance are ordered deterministically
 * (top-to-bottom, left-to-right) so the fill pattern stays symmetrical.
 */
function spiralPositions(rows: number, cols: number) {
    const centerRow = Math.floor(rows / 2)
    const centerCol = Math.floor(cols / 2)

    const positions: Array<{ row: number; col: number; dist: number }> = []
    for (let r = 0; r < rows; r++) {
        for (let c = 0; c < cols; c++) {
            positions.push({
                row: r,
                col: c,
                dist: Math.sqrt((r - centerRow) ** 2 + (c - centerCol) ** 2),
            })
        }
    }

    // Sort by distance, then row, then column for a stable, balanced order
    positions.sort((a, b) => a.dist - b.dist || a.row - b.row || a.col - b.col)
    return positions
}

/**
 * Ambient decorative grid for the broadcast detail header.
 * Fills tiles from the center outward with user initials when a `users`
 * array is provided.  Remaining tiles are rendered as empty decorative cells.
 */
export function BroadcastMosaic({
    color,
    users,
}: {
    color: string
    users?: RecipientRow[] | null
}) {
    const cols = 10
    const rows = 4
    const centerRow = Math.floor(rows / 2)
    const centerCol = Math.floor(cols / 2)
    const cellSize = 72
    const gap = 8

    const gridHeight = rows * cellSize + (rows - 1) * gap
    const centerTileMiddle = centerRow * (cellSize + gap) + cellSize / 2
    const offsetY = centerTileMiddle - gridHeight / 2
    const maskCenterY = (centerTileMiddle / gridHeight) * 100
    const maxDist = Math.sqrt(centerRow ** 2 + centerCol ** 2)

    // Map grid positions → user (if any), filled from center outward
    const positions = spiralPositions(rows, cols)
    const userAt = new Map<string, RecipientRow>()
    if (users?.length) {
        for (let i = 0; i < Math.min(users.length, positions.length); i++) {
            const { row, col } = positions[i]
            userAt.set(`${row},${col}`, users[i])
        }
    }

    return (
        <div className="relative flex items-center justify-center select-none w-full">
            <div
                className="pointer-events-none absolute inset-0"
                style={{
                    background: `radial-gradient(circle at 50% 50%, ${color}10 0%, transparent 60%)`,
                }}
            />
            <div
                className="relative flex flex-col gap-2"
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
                            className="flex gap-2"
                            style={{ paddingLeft: isOffset ? 36 : 0 }}
                        >
                            {Array.from({ length: cols }, (_, col) => {
                                const dist = Math.sqrt(
                                    (row - centerRow) ** 2 + (col - centerCol) ** 2,
                                )
                                const opacity = Math.max(0.35, 1 - (dist / maxDist) * 0.7)

                                const user = userAt.get(`${row},${col}`)
                                if (user) {
                                    const userColor = getRandomColor(
                                        user.full_name ?? user.email ?? user.id,
                                    )
                                    const initials = getUserInitials(user)
                                    const isCenter = row === centerRow && col === centerCol
                                    return (
                                        <div
                                            key={col}
                                            className="flex items-center justify-center rounded-2xl"
                                            style={{
                                                width: cellSize,
                                                height: cellSize,
                                                background: `${userColor}20`,
                                                border: `1.5px solid ${userColor}40`,
                                                ...(isCenter
                                                    ? {
                                                          boxShadow: `0 0 32px ${color}25, 0 4px 20px ${color}15`,
                                                      }
                                                    : {}),
                                            }}
                                        >
                                            <span
                                                className="font-semibold leading-none select-none"
                                                style={{
                                                    color: userColor,
                                                    fontSize: 20,
                                                }}
                                            >
                                                {initials}
                                            </span>
                                        </div>
                                    )
                                }
                                return (
                                    <div
                                        key={col}
                                        className="rounded-2xl border border-border/80 bg-background shadow-sm"
                                        style={{
                                            width: cellSize,
                                            height: cellSize,
                                            opacity,
                                        }}
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
