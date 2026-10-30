import { useTranslation } from "react-i18next"
import type { OrganizationMember } from "../../oapi/client"

import { Button } from "@/components/ui/button"
import { TableCell, TableRow } from "@/components/ui/table"
import { AttributeEditor } from "@/components/ui/attribute-editor"

interface MemberExpandedRowProps {
    member: OrganizationMember
    memberData: Record<string, unknown>
    hasChanges: boolean
    isSaving: boolean
    onDataChange: (data: Record<string, unknown>) => void
    onSave: () => void
    onDiscard: () => void
}

export function MemberExpandedRow({
    member: _member,
    memberData,
    hasChanges,
    isSaving,
    onDataChange,
    onSave,
    onDiscard,
}: MemberExpandedRowProps) {
    const { t } = useTranslation()

    return (
        <TableRow className="bg-muted/50 hover:bg-muted/50">
            <TableCell colSpan={4} className="p-0">
                <div className="px-4 sm:px-6 py-4 flex flex-col gap-3">
                    {/* Member Attributes */}
                    <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                        {t("member_attributes")}
                    </p>
                    <AttributeEditor
                        value={memberData}
                        onChange={onDataChange}
                        emptyTitle={t("no_member_attributes")}
                        emptyDescription={t("add_member_attributes_description")}
                    />

                    {/* Save/Discard Actions */}
                    {hasChanges && (
                        <div className="flex items-center gap-2">
                            <Button
                                size="sm"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onSave()
                                }}
                                disabled={isSaving}
                            >
                                {isSaving ? t("saving") : t("save_changes")}
                            </Button>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={(e) => {
                                    e.stopPropagation()
                                    onDiscard()
                                }}
                                disabled={isSaving}
                            >
                                {t("discard")}
                            </Button>
                        </div>
                    )}
                </div>
            </TableCell>
        </TableRow>
    )
}
