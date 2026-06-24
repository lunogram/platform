import { Bell } from "lucide-react"

export interface PushFrameProps {
    /** Notification title */
    title: string
    /** Notification body text */
    body: string
    /** Timestamp text (defaults to current time) */
    time?: string
}

export function PushFrame({ title, body, time }: PushFrameProps) {
    const displayTime =
        time ?? new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })

    return (
        <div className="w-full max-w-md">
            <div className="bg-white rounded-2xl shadow-xl overflow-hidden border border-gray-200">
                <div className="px-4 py-3 flex items-start gap-3">
                    <div className="flex-shrink-0 w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                        <Bell className="w-5 h-5 text-white" />
                    </div>

                    <div className="flex-1 flex gap-1 flex-col">
                        <div className="flex items-center justify-between">
                            <span className="text-sm font-semibold text-gray-900">{title}</span>
                            <span className="text-xs text-gray-500">{displayTime}</span>
                        </div>

                        <p className="text-sm text-gray-600 line-clamp-3">{body}</p>
                    </div>
                </div>
            </div>
        </div>
    )
}
