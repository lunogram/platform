import { useTranslation } from "react-i18next"
import { RefreshCw } from "lucide-react"

interface BroadcastProgressBarProps {
    streamedSent: number | null
    streamedTotal: number | null
}

export function BroadcastProgressBar({ streamedSent, streamedTotal }: BroadcastProgressBarProps) {
    const { t } = useTranslation()

    return (
        <div className="border-b bg-blue-50/50 dark:bg-blue-950/20 px-4 sm:px-6 py-3">
            <div className="flex items-center gap-3">
                <RefreshCw className="h-4 w-4 animate-spin text-blue-600 dark:text-blue-400" />
                <div className="flex-1">
                    <div className="flex items-center justify-between text-sm mb-1">
                        <span className="text-blue-700 dark:text-blue-300 font-medium">
                            {t("sending_broadcast", "Sending broadcast...")}
                        </span>
                        {streamedSent != null && (
                            <span className="text-blue-600/70 dark:text-blue-400/70 text-xs tabular-nums">
                                {streamedSent.toLocaleString()}
                                {streamedTotal != null && streamedTotal > 0
                                    ? ` / ${streamedTotal.toLocaleString()}`
                                    : ""}{" "}
                                {t("sent", "sent")}
                            </span>
                        )}
                    </div>
                    <div className="h-1.5 rounded-full bg-blue-200/60 dark:bg-blue-800/40 overflow-hidden">
                        {streamedSent != null && streamedTotal != null && streamedTotal > 0 ? (
                            <div
                                className="h-full rounded-full bg-blue-600 dark:bg-blue-400 transition-all duration-500"
                                style={{
                                    width: `${Math.min(100, (streamedSent / streamedTotal) * 100)}%`,
                                }}
                            />
                        ) : (
                            <div className="h-full rounded-full bg-blue-600 dark:bg-blue-400 animate-pulse w-full" />
                        )}
                    </div>
                </div>
            </div>
        </div>
    )
}
