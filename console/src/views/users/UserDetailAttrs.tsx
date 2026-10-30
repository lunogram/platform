import { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Save, Braces } from "lucide-react"
import { toast } from "sonner"
import { UserContext } from "../../contexts"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import { Button } from "@/components/ui/button"
import UserDetailIdentifiers from "./UserDetailIdentifiers"
import { UserDevicesSection } from "./UserDevicesSection"
import { useUpdateUser } from "./queries"

const NO_DATA: Record<string, unknown> = {}

export default function UserDetailAttrs() {
    const { t } = useTranslation()
    const [user] = useContext(UserContext)
    const updateUser = useUpdateUser(t("save_error", "Failed to save attributes"))
    const isSaving = updateUser.isPending

    // The user's saved attributes, until they are edited. The draft is local,
    // so a refetched user (e.g. after an identifier save) shows up as long as
    // nothing is being edited, and Save sends exactly what is on screen.
    const savedData = (user.data as Record<string, unknown> | undefined) ?? NO_DATA
    const [draft, setDraft] = useState<Record<string, unknown> | null>(null)
    const isDirty = draft !== null && JSON.stringify(draft) !== JSON.stringify(savedData)

    const handleSave = () => {
        if (!draft) return
        const sent = draft
        updateUser.mutate(
            { data: sent },
            {
                onSuccess: () => {
                    // Edits made while saving stay as the new draft.
                    setDraft((current) => (current === sent ? null : current))
                    toast.success(t("save_success", "Attributes saved successfully"))
                },
            },
        )
    }

    return (
        <div className="space-y-8">
            {/* Identifiers Section */}
            <UserDetailIdentifiers />

            <UserDevicesSection />

            {/* Custom Attributes Section */}
            <div className="space-y-6">
                {/* Section Header */}
                <div className="flex items-center justify-between">
                    <div>
                        <h2 className="text-base font-medium">
                            <Braces className="inline h-4 w-4 mr-1.5 -mt-0.5" />
                            {t("custom_attributes", "Custom attributes")}
                        </h2>
                        <p className="text-sm text-muted-foreground mt-0.5">
                            {t(
                                "user_data_description",
                                "Store custom metadata and attributes for this user",
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
                    value={draft ?? savedData}
                    onChange={setDraft}
                    emptyTitle={t("no_attributes", "No attributes yet")}
                    emptyDescription={t(
                        "no_user_data_description",
                        "Add custom data to store additional information about this user.",
                    )}
                />
            </div>
        </div>
    )
}
