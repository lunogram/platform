import { Panel } from "@xyflow/react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

interface JourneySelectionPanelProps {
    selectedCount: number
    onDuplicateSelected: () => void
}

export function JourneySelectionPanel({
    selectedCount,
    onDuplicateSelected,
}: JourneySelectionPanelProps) {
    const { t } = useTranslation()

    return (
        <Panel position="top-left">
            {selectedCount ? (
                <div className="flex items-center gap-2 bg-background/95 backdrop-blur-sm border rounded-lg shadow-lg px-3 py-1.5">
                    <Button onClick={onDuplicateSelected} size="sm" variant="ghost" className="h-7">
                        {t("duplicate_selected_steps", "Duplicate Selected Steps ({{count}})", {
                            count: selectedCount,
                        })}
                    </Button>
                </div>
            ) : (
                <div className="hidden sm:flex items-center bg-background/95 backdrop-blur-sm border rounded-lg shadow-lg px-3 py-1.5">
                    <span className="text-xs text-muted-foreground">
                        {t("shift_drag_multi_select", "Shift+Drag to Multi Select")}
                    </span>
                </div>
            )}
        </Panel>
    )
}
