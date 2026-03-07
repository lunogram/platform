import { useTranslation } from "react-i18next"
import { useMemo } from "react"

import { ActionPreviewIframe } from "@/components/action-preview-iframe"

import type { ActionFormValues } from "./action-form-types"

interface ActionPreviewPanelProps {
    selectedType?: string
    projectId: string
    /** Watched form values for live preview updates */
    config: ActionFormValues["config"]
    payload: ActionFormValues["payload"]
}

export function ActionPreviewPanel({
    selectedType,
    projectId,
    config,
    payload,
}: ActionPreviewPanelProps) {
    const { t } = useTranslation()

    const previewData = useMemo(
        () => ({
            config: { ...(config ?? {}), ...(payload ?? {}) },
            payload: payload ?? {},
        }),
        [config, payload],
    )

    return (
        <div className="flex flex-col w-full md:w-3/5 border-t md:border-t-0 md:border-l overflow-hidden">
            <div className="flex-1 overflow-y-auto p-4 sm:p-8">
                {selectedType ? (
                    <ActionPreviewIframe
                        actionType={selectedType}
                        projectId={projectId}
                        mode="action-config"
                        data={previewData}
                    />
                ) : (
                    <div className="flex items-center justify-center h-48 border rounded-lg text-muted-foreground text-sm">
                        {t("no_preview", "No preview available")}
                    </div>
                )}
            </div>
        </div>
    )
}
