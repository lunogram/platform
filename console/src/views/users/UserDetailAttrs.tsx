import { useContext, useState } from "react"
import { useTranslation } from "react-i18next"
import { Save } from "lucide-react"
import { toast } from "react-hot-toast/headless"
import { ProjectContext, UserContext } from "../../contexts"
import api from "../../api"

import { Button } from "@/components/ui/button"
import { AttributeEditor } from "@/components/ui/attribute-editor"
import type { User } from "../../types"

export default function UserDetailAttrs() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const [user, setUser] = useContext(UserContext)

    const initialData = (user.data as Record<string, unknown>) ?? {}
    const [data, setData] = useState<Record<string, unknown>>(initialData)
    const [isDirty, setIsDirty] = useState(false)
    const [isSaving, setIsSaving] = useState(false)

    const handleChange = (newData: Record<string, unknown>) => {
        setData(newData)
        setIsDirty(JSON.stringify(newData) !== JSON.stringify(user.data ?? {}))
    }

    const handleSave = async () => {
        setIsSaving(true)
        try {
            const updatedUser = await api.users.update(project.id, user.id, { data } as User)
            if (updatedUser) {
                setUser(updatedUser)
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
        <div className="space-y-6">
            {/* Section Header */}
            <div className="flex items-center justify-between">
                <div>
                    <h2 className="text-base font-medium">
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
                value={initialData}
                onChange={handleChange}
                emptyTitle={t("no_attributes", "No attributes yet")}
                emptyDescription={t(
                    "no_user_data_description",
                    "Add custom data to store additional information about this user.",
                )}
            />
        </div>
    )
}
