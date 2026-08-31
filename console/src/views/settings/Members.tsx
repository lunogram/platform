import { useContext, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { UserPlus } from "lucide-react"
import { toast } from "sonner"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { checkProjectRole, problemDetail } from "@/utils"
import type { ProjectInvite } from "@/types"
import InviteDialog from "./members/InviteDialog"
import InviteList, { type InviteListHandle } from "./members/InviteList"
import InviteSentDialog from "./members/InviteSentDialog"
import MemberList from "./members/MemberList"

import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

// Members is the project's people screen: who has access today, and who has
// been invited but has not accepted yet.
export default function Members() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)

    const [tab, setTab] = useState("members")
    const [isInviting, setIsInviting] = useState(false)
    const [isSaving, setIsSaving] = useState(false)
    const [createdInvite, setCreatedInvite] = useState<ProjectInvite | null>(null)
    const inviteListRef = useRef<InviteListHandle>(null)

    const canManage = checkProjectRole("admin", project.role)

    // A fresh invite belongs on the invites tab. When that tab is already open
    // its list is mounted and has to be told to refetch; otherwise switching to
    // it mounts the list, which loads the invite on its own.
    const showNewInvite = async () => {
        if (tab === "invites") {
            await inviteListRef.current?.reload()
            return
        }
        setTab("invites")
    }

    return (
        <div className="flex flex-col gap-6">
            <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
                <div className="flex flex-col gap-1">
                    <h2 className="text-2xl font-semibold tracking-tight">{t("members")}</h2>
                    <p className="text-sm text-muted-foreground">
                        {t(
                            "members_page_description",
                            "Manage who can access this project and what they may do.",
                        )}
                    </p>
                </div>
                <Button
                    onClick={() => setIsInviting(true)}
                    disabled={!canManage}
                    aria-label={t("invite_member", "Invite member")}
                >
                    <UserPlus className="mr-2 h-4 w-4" />
                    {t("invite_member", "Invite member")}
                </Button>
            </div>

            {!canManage && (
                <p className="rounded-lg bg-muted p-3 text-sm text-muted-foreground">
                    {t("invite_permission_denied")}
                </p>
            )}

            <Tabs value={tab} onValueChange={setTab} className="flex flex-col gap-4">
                <TabsList className="w-fit">
                    <TabsTrigger value="members">{t("members")}</TabsTrigger>
                    <TabsTrigger value="invites">{t("invites")}</TabsTrigger>
                </TabsList>
                <TabsContent value="members" className="mt-0">
                    <MemberList canManage={canManage} />
                </TabsContent>
                <TabsContent value="invites" className="mt-0">
                    <InviteList ref={inviteListRef} canManage={canManage} />
                </TabsContent>
            </Tabs>

            <InviteSentDialog
                invite={createdInvite}
                projectName={project.name}
                onClose={() => setCreatedInvite(null)}
            />

            <InviteDialog
                isOpen={isInviting}
                onClose={() => setIsInviting(false)}
                onSave={async (data) => {
                    setIsSaving(true)
                    try {
                        const invite = await api.invites.create(project.id, data)
                        await showNewInvite()
                        setIsInviting(false)
                        setCreatedInvite(invite)
                    } catch (err) {
                        toast.error(
                            problemDetail(err) ??
                                t("invite_create_failed", "Couldn't create the invite"),
                        )
                    } finally {
                        setIsSaving(false)
                    }
                }}
                isSaving={isSaving}
                userRole={project.role}
            />
        </div>
    )
}
