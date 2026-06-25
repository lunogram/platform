import { useEffect, useState, useCallback } from "react"
import { useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { Loader2, Inbox } from "lucide-react"

import api from "../../api"
import type { ProjectInvite } from "../../types"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { toast } from "sonner"

function formatExpiry(expiresAt: string): string {
    return new Date(expiresAt).toLocaleDateString(undefined, {
        year: "numeric",
        month: "long",
        day: "numeric",
    })
}

type PageState =
    | { status: "loading" }
    | { status: "ready"; invites: ProjectInvite[] }
    | { status: "error" }

export default function MyInvites() {
    const { t } = useTranslation()
    const navigate = useNavigate()

    const [state, setState] = useState<PageState>({ status: "loading" })
    const [accepting, setAccepting] = useState<string | null>(null)

    const load = useCallback(async () => {
        setState({ status: "loading" })
        try {
            // A 401 here is handled by the axios interceptor, which redirects to
            // login and returns the user to this page afterwards.
            const result = await api.invites.mine()
            setState({ status: "ready", invites: result.results })
        } catch {
            setState({ status: "error" })
        }
    }, [])

    useEffect(() => {
        load()
    }, [load])

    const handleAccept = async (invite: ProjectInvite) => {
        setAccepting(invite.id)
        try {
            const project = await api.invites.accept(invite.id)
            toast.success(t("invite_accepted", "Invite accepted"))
            navigate(`/projects/${project.id}`)
        } catch {
            toast.error(t("invite_accept_failed", "Failed to accept the invite. Please try again."))
            setAccepting(null)
            // Refresh so a now-stale invite (expired/revoked) drops off the list.
            load()
        }
    }

    if (state.status === "loading") {
        return (
            <div className="min-h-screen flex items-center justify-center bg-muted/40">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
        )
    }

    if (state.status === "error") {
        return (
            <div className="min-h-screen flex items-center justify-center bg-muted/40 p-4">
                <Card className="w-full max-w-md">
                    <CardContent className="flex flex-col items-center gap-4 py-12 text-center">
                        <p className="text-sm text-muted-foreground">
                            {t("invites_load_failed", "Could not load your invites.")}
                        </p>
                        <Button variant="outline" onClick={load}>
                            {t("retry", "Retry")}
                        </Button>
                    </CardContent>
                </Card>
            </div>
        )
    }

    return (
        <div className="min-h-screen flex items-start justify-center bg-muted/40 p-4 pt-16">
            <Card className="w-full max-w-md">
                <CardHeader className="space-y-1 text-center">
                    <CardTitle className="text-2xl font-bold">
                        {t("my_invites_title", "Your invites")}
                    </CardTitle>
                    <CardDescription>
                        {t("my_invites_description", "Pending invites for your account.")}
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    {state.invites.length === 0 ? (
                        <div className="flex flex-col items-center gap-3 py-10 text-center">
                            <Inbox className="h-8 w-8 text-muted-foreground" />
                            <p className="text-sm text-muted-foreground">
                                {t("my_invites_empty", "You have no pending invites.")}
                            </p>
                            <Button variant="outline" onClick={() => navigate("/")}>
                                {t("go_home", "Go home")}
                            </Button>
                        </div>
                    ) : (
                        state.invites.map((invite) => (
                            <div key={invite.id} className="rounded-md border p-4 space-y-3">
                                <div className="flex items-center justify-between">
                                    <span className="font-medium truncate">
                                        {invite.project_name ?? t("project", "Project")}
                                    </span>
                                    <Badge variant="secondary">{invite.role}</Badge>
                                </div>
                                {invite.inviter_admin_email && (
                                    <p className="text-sm text-muted-foreground">
                                        {t("invited_by", "Invited by")} {invite.inviter_admin_email}
                                    </p>
                                )}
                                <p className="text-xs text-muted-foreground">
                                    {t("expires", "Expires")} {formatExpiry(invite.expires_at)}
                                </p>
                                <Button
                                    className="w-full"
                                    disabled={accepting !== null}
                                    onClick={() => handleAccept(invite)}
                                >
                                    {accepting === invite.id && (
                                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    )}
                                    {t("invite_accept", "Accept invite")}
                                </Button>
                            </div>
                        ))
                    )}
                </CardContent>
            </Card>
        </div>
    )
}
