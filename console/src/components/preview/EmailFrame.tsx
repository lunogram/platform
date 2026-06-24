import type { ReactNode } from "react"

function ArchiveIcon() {
    return (
        <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5m6 4.125l2.25 2.25m0 0l2.25-2.25M12 13.875l-2.25-2.25M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
        </svg>
    )
}

function ReportIcon() {
    return (
        <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
        </svg>
    )
}

function TrashIcon() {
    return (
        <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
        </svg>
    )
}

function ChevronDownIcon() {
    return (
        <svg
            className="w-3 h-3"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            viewBox="0 0 24 24"
        >
            <path d="M19 9l-7 7-7-7" />
        </svg>
    )
}

function ReplyIcon() {
    return (
        <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            viewBox="0 0 24 24"
        >
            <path d="M9 15L3 9m0 0l6-6M3 9h12a6 6 0 010 12h-3" />
        </svg>
    )
}

function MoreVertIcon() {
    return (
        <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
            <path d="M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z" />
        </svg>
    )
}

export interface EmailFrameProps {
    /** Email subject line */
    subject?: string
    /** Display name of the sender */
    fromName?: string
    /** Reply-to address (shown below sender) */
    replyTo?: string
    /** Timestamp text shown in the header */
    time?: string
    /** The email body — either pre-rendered HTML string or a ReactNode (e.g. an iframe) */
    children: ReactNode
    /** Placeholder shown when there is no body content */
    emptyLabel?: string
    /** Override container classes (defaults to rounded card style) */
    className?: string
    /** Translated labels for static UI text */
    labels?: {
        noSubject?: string
        unknownSender?: string
        noContent?: string
    }
}

export function EmailFrame({
    subject,
    fromName,
    replyTo,
    time,
    children,
    emptyLabel,
    className,
    labels,
}: EmailFrameProps) {
    const noSubjectLabel = labels?.noSubject ?? "No subject"
    const unknownSenderLabel = labels?.unknownSender ?? "Unknown sender"
    const noContentLabel = emptyLabel ?? labels?.noContent ?? "No content available"
    const displayTime =
        time ??
        new Date().toLocaleTimeString("en", {
            hour: "numeric",
            minute: "2-digit",
        })

    return (
        <div
            className={
                className ??
                "bg-white border rounded-lg w-full overflow-hidden flex flex-col flex-1"
            }
        >
            <div className="px-6 py-4">
                <div className="flex items-start justify-between mb-4">
                    <h1 className="text-[22px] font-normal text-gray-900 flex-1 pr-4">
                        {subject || <span className="text-gray-400 italic">{noSubjectLabel}</span>}
                    </h1>
                    <div className="flex items-center gap-1 text-gray-600 flex-shrink-0" aria-hidden="true">
                        <span className="p-2 hover:bg-gray-100 rounded-full">
                            <ArchiveIcon />
                        </span>
                        <span className="p-2 hover:bg-gray-100 rounded-full">
                            <ReportIcon />
                        </span>
                        <span className="p-2 hover:bg-gray-100 rounded-full">
                            <TrashIcon />
                        </span>
                    </div>
                </div>

                <div className="flex items-start gap-3">
                    <div className="w-10 h-10 rounded-full bg-gradient-to-br from-blue-400 to-blue-600 flex items-center justify-center text-white font-medium flex-shrink-0">
                        {fromName ? fromName.charAt(0).toUpperCase() : "?"}
                    </div>

                    <div className="flex-1 min-w-0">
                        <div className="flex items-baseline gap-2 mb-1">
                            <span className="font-medium text-gray-900 text-sm">
                                {fromName || (
                                    <span className="text-gray-400 italic">{unknownSenderLabel}</span>
                                )}
                            </span>
                            <span className="text-xs text-gray-500">{displayTime}</span>
                        </div>
                        <div className="flex items-center gap-1 text-xs text-gray-600" aria-hidden="true">
                            <span>to me</span>
                            <span className="hover:bg-gray-100 px-1 rounded">
                                <ChevronDownIcon />
                            </span>
                        </div>
                        {replyTo && (
                            <div className="text-xs text-gray-500 mt-1">Reply-To: {replyTo}</div>
                        )}
                    </div>

                    <div className="flex items-center gap-1 flex-shrink-0" aria-hidden="true">
                        <span className="p-2 hover:bg-gray-100 rounded-full">
                            <ReplyIcon />
                        </span>
                        <span className="p-2 hover:bg-gray-100 rounded-full">
                            <MoreVertIcon />
                        </span>
                    </div>
                </div>
            </div>

            <div className="border-t border-gray-100 flex-1">
                {children || (
                    <div className="text-center py-12 text-gray-400 italic text-sm">
                        {noContentLabel}
                    </div>
                )}
            </div>
        </div>
    )
}
