import { ChevronLeft, PenLine } from "lucide-react"
import { useTranslation } from "react-i18next"

import { oapiClient } from "@/oapi/client"
import type { components } from "@/oapi/management.generated"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { InlineEdit } from "@/components/ui/inline-edit"
import type { Journey } from "@/types"
import type { UUID } from "@/types/common"

interface JourneyEditorToolbarProps {
    projectId: UUID
    journey: Journey
    isArchived: boolean
    isMobile: boolean
    hasUnsavedChanges: boolean
    saving: boolean
    publishing: boolean
    onBack: () => void
    onJourneyChange: (journey: Journey) => void
    onSaveDraft: () => void
    onPublish: () => void
}

export function JourneyEditorToolbar({
    projectId,
    journey,
    isArchived,
    isMobile,
    hasUnsavedChanges,
    saving,
    publishing,
    onBack,
    onJourneyChange,
    onSaveDraft,
    onPublish,
}: JourneyEditorToolbarProps) {
    const { t } = useTranslation()

    return (
        <div className="flex items-center gap-2 sm:gap-3 px-3 sm:px-4 py-2.5 sm:py-2.5 border-b bg-background shrink-0">
            <Button
                variant="ghost"
                size="sm"
                onClick={onBack}
                className="gap-1 text-muted-foreground hover:text-foreground"
            >
                <ChevronLeft className="h-4 w-4" />
                <span className="hidden sm:inline">{t("journeys")}</span>
            </Button>

            <div className="h-4 w-px bg-border hidden sm:block" />

            <div className="flex-1 min-w-0">
                <InlineEdit
                    value={journey.name}
                    onSave={async (name) => {
                        const { data: updated } = await oapiClient.PATCH(
                            "/api/admin/projects/{projectID}/journeys/{journeyID}",
                            {
                                params: {
                                    path: { projectID: projectId, journeyID: journey.id },
                                },
                                // status/tags are accepted by the backend but not yet
                                // in the OpenAPI spec; cast to the documented schema.
                                body: {
                                    name,
                                    description: journey.description,
                                    status: journey.status,
                                    tags: journey.tags,
                                } as components["schemas"]["UpdateJourney"],
                            },
                        )
                        if (updated) onJourneyChange(updated as Journey)
                    }}
                    required
                    triggerClassName="gap-1.5 max-w-full"
                    pencilSize="h-3.5 w-3.5"
                >
                    <h1 className="text-sm sm:text-base font-semibold truncate">{journey.name}</h1>
                </InlineEdit>
            </div>

            <div className="flex items-center gap-1.5 sm:gap-2 shrink-0">
                {isArchived ? (
                    <Badge
                        variant="secondary"
                        className="bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-transparent"
                    >
                        {t("archived")}
                    </Badge>
                ) : (
                    <>
                        {hasUnsavedChanges ? (
                            <Badge
                                variant="secondary"
                                className="bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-transparent gap-1"
                            >
                                <PenLine className="h-3 w-3" />
                                {t("editing", "Editing")}
                            </Badge>
                        ) : journey.status === "published" ? (
                            <Badge
                                variant="secondary"
                                className="bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 border-transparent"
                            >
                                {t("published")}
                            </Badge>
                        ) : (
                            <Badge
                                variant="secondary"
                                className="bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400 border-transparent"
                            >
                                {t("draft")}
                            </Badge>
                        )}
                        {!isMobile &&
                            (hasUnsavedChanges ? (
                                <Button
                                    variant="outline"
                                    size="sm"
                                    onClick={onSaveDraft}
                                    isLoading={saving}
                                >
                                    <span className="hidden sm:inline">
                                        {t("journey_draft_save")}
                                    </span>
                                    <span className="sm:hidden">{t("save", "Save")}</span>
                                </Button>
                            ) : (
                                <Button size="sm" onClick={onPublish} isLoading={publishing}>
                                    {t("publish")}
                                </Button>
                            ))}
                    </>
                )}
            </div>
        </div>
    )
}
