import type { ReactNode } from "react"
import { UserRound } from "lucide-react"

export interface PhoneFrameProps {
    /** Sender name shown in the header */
    sender?: string
    /** The chat bubble content */
    message: ReactNode
    /** Context line (e.g. "Text Message") */
    contextLabel?: string
    /** Date line (e.g. "Today") */
    contextDate?: string
}

export function PhoneFrame({
    sender,
    message,
    contextLabel = "Text Message",
    contextDate = "Today",
}: PhoneFrameProps) {
    return (
        <div className="w-[390px] h-[533px] bg-zinc-900 rounded-t-[70px] p-3 pb-0 shadow-2xl">
            <div className="w-full h-full bg-white rounded-t-[58px] overflow-hidden flex flex-col">
                {/* Notch */}
                <div className="h-12 bg-white flex items-start justify-center px-8 pt-3">
                    <div className="w-32 h-8 bg-zinc-900 rounded-full" />
                </div>

                {/* Contact header */}
                <div className="bg-white border-b border-gray-200 px-4 py-3 flex items-center justify-center">
                    <div className="flex flex-col items-center">
                        <div className="w-12 h-12 bg-gray-300 rounded-full flex items-center justify-center mb-1">
                            <UserRound className="w-7 h-7 text-gray-500" strokeWidth={1.5} />
                        </div>
                        {sender && <span className="text-sm font-medium">{sender}</span>}
                    </div>
                </div>

                {/* Chat body */}
                <div className="flex-1 bg-white px-4 py-6 overflow-y-auto">
                    <div className="flex flex-col items-center mb-6">
                        <span className="text-gray-500 text-xs">{contextLabel}</span>
                        <span className="text-gray-400 text-xs">{contextDate}</span>
                    </div>

                    <div className="flex justify-start mb-6">
                        <div className="max-w-[75%]">
                            <div className="bg-gray-200 rounded-3xl rounded-bl-sm px-4 py-3">
                                {message}
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    )
}
