import { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Save, Braces } from "lucide-react"
import { toast } from "sonner"
import { ProjectContext, OrganizationContext } from "../../contexts"
import oapiClient from "../../oapi/client"

import { Button } from "@/components/ui/button"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import OrganizationDetailIdentifiers from "./OrganizationDetailIdentifiers"

export default function OrganizationDetailAttrs() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [organization, setOrganization] = useContext(OrganizationContext)

    const initialData = (organization.data as Record<string, unknown>) ?? {}
    const [data, setData] = useState<Record<string, unknown>>(initialData)
    const [isDirty, setIsDirty] = useState(false)
    const [isSaving, setIsSaving] = useState(false)

    const handleChange = (newData: Record<string, unknown>) => {
        setData(newData)
        setIsDirty(JSON.stringify(newData) !== JSON.stringify(organization.data ?? {}))
    }

    const handleSave = async () => {
        setIsSaving(true)
        try {
            const { data: responseData, error } = await oapiClient.PATCH(
                "/api/admin/projects/{projectID}/subjects/organizations/{organizationID}",
                {
                    params: {
                        path: {
                            projectID: project.id,
                            organizationID: organization.id,
                        },
                    },
                    body: { data },
                },
            )
            if (error) {
                toast.error(t("save_error", "Failed to save attributes"))
                return
            }
            if (responseData) {
                setOrganization(responseData)
                setIsDirty(false)
                toast.success(t("save_success", "Attributes saved successfully"))
            }
        } catch {
            toast.error(t("save_error", "Failed to save attributes"))
        } finally {
            setIsSaving(false)
        }
    }

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
                value={initialData}
                onChange={handleChange}
                emptyTitle={t("no_attributes", "No attributes yet")}
                emptyDescription={t(
                    "no_organization_data_description",
                    "Add custom data to store additional information about this organization.",
                )}
            />
        </div>
    )
}
