import { Eye, EyeOff, XCircle } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Panel } from "@xyflow/react"

import { Button } from "@/components/ui/button"

interface FollowingPanelProps {
    userId: string | null
    onStopFollowing: () => void
    onCancelExecution: (userId: string) => void
}

export function FollowingPanel({
    userId,
    onStopFollowing,
    onCancelExecution,
}: FollowingPanelProps) {
    const { t } = useTranslation()
    if (!userId) return null

    return (
        <Panel position="top-center">
            <div className="flex items-center gap-2 bg-background/95 backdrop-blur-sm border rounded-lg shadow-lg px-3 py-1.5">
                <Eye className="h-4 w-4 text-orange-500 animate-pulse" />
                <span className="text-sm font-medium">{t("following_user", "Following user")}</span>
                <div className="h-4 w-px bg-border" />
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={onStopFollowing}
                    className="h-7 gap-1.5 text-muted-foreground hover:text-foreground"
                >
                    <EyeOff className="h-3.5 w-3.5" />
                    {t("stop_following", "Stop")}
                </Button>
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => onCancelExecution(userId)}
                    className="h-7 gap-1.5 text-destructive hover:text-destructive hover:bg-destructive/10"
                >
                    <XCircle className="h-3.5 w-3.5" />
                    {t("cancel_execution", "Cancel")}
                </Button>
            </div>
        </Panel>
    )
}

interface ReplayPanelProps {
    visible: boolean
    onDismiss: () => void
}

export function ReplayPanel({ visible, onDismiss }: ReplayPanelProps) {
    const { t } = useTranslation()
    if (!visible) return null

    return (
        <Panel position="top-center">
            <div className="flex items-center gap-2 bg-background/95 backdrop-blur-sm border rounded-lg shadow-lg px-3 py-1.5">
                <Eye className="h-4 w-4 text-emerald-500" />
                <span className="text-sm font-medium">
                    {t("viewing_user_path", "Viewing user path")}
                </span>
                <div className="h-4 w-px bg-border" />
                <Button
                    variant="ghost"
                    size="sm"
                    onClick={onDismiss}
                    className="h-7 gap-1.5 text-muted-foreground hover:text-foreground"
                >
                    <EyeOff className="h-3.5 w-3.5" />
                    {t("dismiss", "Dismiss")}
                </Button>
            </div>
        </Panel>
    )
}
