import type { ReactNode } from "react"
import { Inbox } from "lucide-react"

export interface InboxFrameProps {
    /** Message title */
    title?: string
    /** Timestamp text */
    time?: string
    /** Message body content */
    children?: ReactNode
    /** Priority level (0 = none, 1 = urgent, 2 = high, 3 = medium, 4 = low) */
    priority?: number
    /** Tags to display in the footer */
    tags?: string[]
    /** Override container classes */
    className?: string
    /** Translated labels for static UI text */
    labels?: {
        noTitle?: string
        priorityUrgent?: string
        priorityHigh?: string
        priorityMedium?: string
        priorityLow?: string
    }
}

const priorityStyles: Record<number, string> = {
    1: "bg-red-100 text-red-700",
    2: "bg-orange-100 text-orange-700",
    3: "bg-yellow-100 text-yellow-700",
    4: "bg-gray-100 text-gray-600",
}

const defaultPriorityLabels: Record<number, string> = {
    1: "Urgent",
    2: "High",
    3: "Medium",
    4: "Low",
}

export function InboxFrame({ title, time, children, priority, tags, className, labels }: InboxFrameProps) {
    const displayTime =
        time ?? new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    const noTitleLabel = labels?.noTitle ?? "No title"
    const priorityLabelMap: Record<number, string> = {
        1: labels?.priorityUrgent ?? defaultPriorityLabels[1],
        2: labels?.priorityHigh ?? defaultPriorityLabels[2],
        3: labels?.priorityMedium ?? defaultPriorityLabels[3],
        4: labels?.priorityLow ?? defaultPriorityLabels[4],
    }
    const prio = priority && priorityStyles[priority] ? { label: priorityLabelMap[priority], className: priorityStyles[priority] } : undefined
    const hasTags = tags && tags.length > 0
    const hasFooter = prio || hasTags

    return (
        <div className={className ?? "w-full h-full flex items-center justify-center p-6"}>
            <div className="bg-white rounded-2xl shadow-xl overflow-hidden border border-gray-200 w-full max-w-lg">
                {/* Header */}
                <div className="px-4 py-3 flex items-center gap-3">
                    <div className="flex-shrink-0 w-8 h-8 bg-violet-500 rounded-lg flex items-center justify-center">
                        <Inbox className="w-5 h-5 text-white" strokeWidth={1.5} />
                    </div>

                    <div className="flex-1 min-w-0">
                        <div className="flex items-center justify-between gap-2">
                            <span className="text-sm font-semibold text-gray-900 truncate">
                                {title || (
                                     <span className="text-gray-400 italic font-normal">
                                        {noTitleLabel}
                                    </span>
                                )}
                            </span>
                            <span className="text-xs text-gray-500 flex-shrink-0">
                                {displayTime}
                            </span>
                        </div>
                    </div>
                </div>

                {/* Body */}
                {children && (
                    <div className="px-4 pb-3">
                        <div className="text-sm text-gray-600 whitespace-pre-wrap">{children}</div>
                    </div>
                )}

                {/* Footer: priority + tags */}
                {hasFooter && (
                    <div className="px-4 pb-3 flex items-center gap-2 flex-wrap">
                        {prio && (
                            <span
                                className={`text-xs font-medium px-2 py-0.5 rounded-full ${prio.className}`}
                            >
                                {prio.label}
                            </span>
                        )}
                        {tags?.map((tag) => (
                            <span
                                key={tag}
                                className="text-xs px-2 py-0.5 rounded-full bg-gray-100 text-gray-600"
                            >
                                {tag}
                            </span>
                        ))}
                    </div>
                )}
            </div>
        </div>
    )
}
