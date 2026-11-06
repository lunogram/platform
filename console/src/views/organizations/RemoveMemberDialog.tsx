import { useTranslation } from "react-i18next"
import type { OrganizationMember } from "../../oapi/client"
import { getUserDisplayName, getUserInitials, getPrimaryExternalId } from "@/lib/name"
import { useRemoveMember } from "./queries"

import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

interface RemoveMemberDialogProps {
    /** The member to confirm the removal of; the dialog is closed without one. */
    member: OrganizationMember | null
    onClose: () => void
    onRemoved: () => void
}

export function RemoveMemberDialog({
    member: memberToRemove,
    onClose,
    onRemoved,
}: RemoveMemberDialogProps) {
    const { t } = useTranslation()
    const removeMutation = useRemoveMember()
    const isRemoving = removeMutation.isPending

    // On failure the mutation cache shows the error and the dialog stays open.
    const removeMember = () => {
        if (!memberToRemove) return
        removeMutation.mutate(memberToRemove.id, { onSuccess: onRemoved })
    }

    return (
        <Dialog open={memberToRemove !== null} onOpenChange={(open) => !open && onClose()}>
            <DialogContent>
                <DialogHeader>
                    <DialogTitle>{t("remove_member")}</DialogTitle>
                    <DialogDescription>
                        {t(
                            "remove_member_warning",
                            "Are you sure you want to remove this member from the organization?",
                        )}
                    </DialogDescription>
                </DialogHeader>
                {memberToRemove && (
                    <div className="py-4">
                        <div className="flex items-center gap-3 p-3 rounded-lg bg-muted">
                            <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary/10 text-primary text-sm font-medium">
                                {getUserInitials(memberToRemove)}
                            </div>
                            <div>
                                <p className="font-medium">{getUserDisplayName(memberToRemove)}</p>
                                <p className="text-sm text-muted-foreground">
                                    {memberToRemove.email ||
                                        getPrimaryExternalId(
                                            memberToRemove as unknown as Record<string, unknown>,
                                        )}
                                </p>
                            </div>
                        </div>
                    </div>
                )}
                <DialogFooter>
                    <Button variant="outline" onClick={onClose} disabled={isRemoving}>
                        {t("cancel")}
                    </Button>
                    <Button variant="destructive" onClick={removeMember} disabled={isRemoving}>
                        {isRemoving ? t("removing") : t("remove_member")}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
