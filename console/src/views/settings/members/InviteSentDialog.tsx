import { useEffect, useState } from "react"
import { Trans, useTranslation } from "react-i18next"
import { Check, Clock, Copy, MailCheck, Shield } from "lucide-react"
import { snakeToTitle } from "@/utils"
import type { ProjectInvite } from "@/types"
import { copyInviteLink } from "./inviteLink"

import { Button } from "@/components/ui/button"
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog"

interface InviteSentDialogProps {
    invite: ProjectInvite | null
    projectName: string
    onClose: () => void
}

// InviteSentDialog confirms what an invite actually did: who was mailed, what
// they will be able to do, and how long they have to accept.
export default function InviteSentDialog({ invite, projectName, onClose }: InviteSentDialogProps) {
    const { t } = useTranslation()
    const [copied, setCopied] = useState(false)

    // The label is a transient acknowledgement, not state worth keeping: a
    // dialog that reopens showing "Copied" would be lying about this invite.
    useEffect(() => {
        if (!copied) return
        const timeout = setTimeout(() => setCopied(false), 1500)
        return () => clearTimeout(timeout)
    }, [copied])

    useEffect(() => {
        if (!invite) setCopied(false)
    }, [invite])

    const copy = async () => {
        await copyInviteLink()
        setCopied(true)
    }

    const expiresAt = invite
        ? new Date(invite.expires_at).toLocaleString(undefined, {
              month: "short",
              day: "numeric",
              hour: "2-digit",
              minute: "2-digit",
          })
        : ""

    return (
        <Dialog
            open={!!invite}
            onOpenChange={(open) => {
                if (!open) onClose()
            }}
        >
            <DialogContent className="sm:max-w-md">
                <DialogHeader className="items-center gap-1 text-center sm:text-center">
                    <span className="mb-1 flex h-12 w-12 items-center justify-center rounded-full bg-green-soft text-green-hard">
                        <MailCheck className="h-6 w-6" strokeWidth={2} />
                    </span>
                    <DialogTitle>{t("invite_sent", "Invitation sent")}</DialogTitle>
                    <DialogDescription>
                        <Trans
                            i18nKey="invite_sent_description"
                            defaults="We emailed <strong>{{email}}</strong> a link to join {{project}}."
                            values={{ email: invite?.invitee_email ?? "", project: projectName }}
                            components={{
                                strong: <span className="font-medium text-foreground" />,
                            }}
                        />
                    </DialogDescription>
                </DialogHeader>

                <dl className="grid gap-3 rounded-lg border bg-surface-soft px-4 py-3 text-sm">
                    <div className="flex items-center justify-between gap-4">
                        <dt className="flex items-center gap-2 text-muted-foreground">
                            <Shield className="h-4 w-4 shrink-0" />
                            {t("role")}
                        </dt>
                        <dd className="font-medium">{snakeToTitle(invite?.role ?? "")}</dd>
                    </div>
                    <div className="flex items-center justify-between gap-4">
                        <dt className="flex items-center gap-2 text-muted-foreground">
                            <Clock className="h-4 w-4 shrink-0" />
                            {t("expires")}
                        </dt>
                        <dd className="font-medium">{expiresAt}</dd>
                    </div>
                </dl>

                <p className="text-xs leading-relaxed text-muted-foreground">
                    {t(
                        "invite_sent_note",
                        "They accept by signing in with this address. If they do not have an account yet, they will be asked to create one first.",
                    )}
                </p>

                <DialogFooter className="gap-2 sm:justify-between">
                    <Button type="button" variant="outline" onClick={copy}>
                        {copied ? (
                            <>
                                <Check className="mr-2 h-4 w-4" />
                                {t("copied", "Copied")}
                            </>
                        ) : (
                            <>
                                <Copy className="mr-2 h-4 w-4" />
                                {t("copy_link")}
                            </>
                        )}
                    </Button>
                    <Button onClick={onClose}>{t("done", "Done")}</Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    )
}
