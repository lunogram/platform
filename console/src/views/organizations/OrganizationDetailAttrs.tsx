import { useContext, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Save, Braces } from "lucide-react"
import { toast } from "sonner"
import { OrganizationContext } from "../../contexts"
import { useUpdateOrganization } from "./queries"

import { Button } from "@/components/ui/button"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import OrganizationDetailIdentifiers from "./OrganizationDetailIdentifiers"

export default function OrganizationDetailAttrs() {
    const { t } = useTranslation()
    const [organization] = useContext(OrganizationContext)
    const updateOrganization = useUpdateOrganization(t("save_error", "Failed to save attributes"))

    const savedData = useMemo(
        () => (organization.data as Record<string, unknown>) ?? {},
        [organization.data],
    )
    // Only unsaved edits are kept here; without any, the editor shows the
    // organization as cached, so it follows a refetch or a save elsewhere.
    const [edited, setEdited] = useState<Record<string, unknown> | null>(null)
    const data = edited ?? savedData
    const isDirty = edited !== null && JSON.stringify(edited) !== JSON.stringify(savedData)
    const isSaving = updateOrganization.isPending

    const handleSave = () =>
        updateOrganization.mutate(
            { data },
            {
                onSuccess: () => {
                    setEdited(null)
                    toast.success(t("save_success", "Attributes saved successfully"))
                },
            },
        )

    return (
        <div className="space-y-8">
            {/* Identifiers Section */}
            <OrganizationDetailIdentifiers />

            {/* Section Header */}
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                <div>
                    <h2 className="text-base font-medium">
                        <Braces className="inline h-4 w-4 mr-1.5 -mt-0.5" />
                        {t("custom_attributes", "Custom attributes")}
                    </h2>
                    <p className="text-sm text-muted-foreground mt-0.5">
                        {t(
                            "organization_data_description",
                            "Store custom metadata and attributes for this organization",
                        )}
                    </p>
                </div>
                {isDirty && (
                    <div className="flex items-center gap-3">
                        <span className="text-sm text-amber-600 dark:text-amber-500">
                            {t("unsaved_changes", "Unsaved changes")}
                        </span>
                        <Button onClick={handleSave} disabled={isSaving} size="sm">
                            <Save className="h-4 w-4 mr-2" />
                            {isSaving ? t("saving") : t("save")}
                        </Button>
                    </div>
                )}
            </div>

            {/* Attribute Editor */}
            <AttributeEditor
                value={data}
                onChange={setEdited}
                emptyTitle={t("no_attributes", "No attributes yet")}
                emptyDescription={t(
                    "no_organization_data_description",
                    "Add custom data to store additional information about this organization.",
                )}
            />
        </div>
    )
}
