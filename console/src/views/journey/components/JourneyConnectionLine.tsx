import { memo } from "react"
import type { ConnectionLineComponentProps } from "@xyflow/react"
import { Position, getBezierPath } from "@xyflow/react"

const SOURCE_LEAD_LENGTH = 28

const getLeadPoint = (position: Position, x: number, y: number, distance: number) => {
    switch (position) {
        case Position.Top:
            return { x, y: y - distance }
        case Position.Right:
            return { x: x + distance, y }
        case Position.Left:
            return { x: x - distance, y }
        case Position.Bottom:
        default:
            return { x, y: y + distance }
    }
}

export const JourneyConnectionLine = memo(
    ({
        fromX,
        fromY,
        toX,
        toY,
        fromPosition,
        toPosition,
        connectionStatus,
    }: ConnectionLineComponentProps) => {
        const dragDistance = Math.hypot(toX - fromX, toY - fromY)
        const leadDistance = Math.min(SOURCE_LEAD_LENGTH, dragDistance * 0.35)
        const leadPoint = getLeadPoint(fromPosition, fromX, fromY, leadDistance)

        const [curvePath] = getBezierPath({
            sourceX: leadPoint.x,
            sourceY: leadPoint.y,
            sourcePosition: fromPosition,
            targetX: toX,
            targetY: toY,
            targetPosition: toPosition,
        })
        const curveCommandIndex = curvePath.indexOf("C")
        const path =
            curveCommandIndex === -1
                ? curvePath
                : `M${fromX},${fromY} L${leadPoint.x},${leadPoint.y} ${curvePath.slice(curveCommandIndex)}`

        const isInvalid = connectionStatus === "invalid"

        return (
            <g className={isInvalid ? "journey-connection-invalid" : undefined}>
                <path className="journey-connection-line-shadow" d={path} fill="none" />
                <path className="journey-connection-line" d={path} fill="none" />
            </g>
        )
    },
)
