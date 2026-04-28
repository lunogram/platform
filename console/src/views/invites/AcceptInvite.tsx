import { useEffect, useState, useCallback } from "react"
import { useParams, useSearchParams, useNavigate } from "react-router"
import { useTranslation } from "react-i18next"
import { Loader2, Mail, ShieldAlert } from "lucide-react"

import api from "../../api"
import oapiClient from "../../oapi/client"
import { AUTH_DRIVERS } from "../../types"
import type { ProjectInvite, AuthDriver, Admin } from "../../types"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"

type PageState =
    | { status: "loading" }
    | { status: "error"; message: string }
    | { status: "unauthenticated"; invite: ProjectInvite; drivers: AuthDriver[] }
    | { status: "wrong-account"; invite: ProjectInvite }
    | { status: "ready"; invite: ProjectInvite; profile: Admin }
    | { status: "accepting" }
    | { status: "accepted" }

function formatExpiry(expiresAt: string): string {
    return new Date(expiresAt).toLocaleDateString(undefined, {
        year: "numeric",
        month: "long",
        day: "numeric",
    })
}

export default function AcceptInvite() {
    const { t } = useTranslation()
    const { token } = useParams() as { token: string }
    const [searchParams] = useSearchParams()
    const navigate = useNavigate()
    const autoAccept = searchParams.get("autoAccept") === "1"

    const [state, setState] = useState<PageState>({ status: "loading" })

    const doAccept = useCallback(
        async (invite: ProjectInvite) => {
            setState({ status: "accepting" })
            try {
                const { error } = await oapiClient.POST(
                    "/api/admin/projects/{projectID}/invites/accept/{token}",
                    {
                        params: {
                            path: {
                                projectID: invite.project_id!,
                                token,
                            },
                        },
                    },
                )
                if (error) throw error
                setState({ status: "accepted" })
                setTimeout(() => {
                    navigate(`/projects/${invite.project_id}`)
                }, 1200)
            } catch {
                setState({
                    status: "error",
                    message: t(
                        "invite_accept_failed",
                        "Failed to accept the invite. Please try again.",
                    ),
                })
            }
        },
        [token, navigate, t],
    )

    useEffect(() => {
        const init = async () => {
            // Fetch invite details (public endpoint, no auth needed)
            const { data: invite, error } = await oapiClient.GET("/api/invites/{token}", {
                params: { path: { token } },
            })

            if (error || !invite) {
                setState({
                    status: "error",
                    message: t("invite_not_found", "This invite link is invalid or has expired."),
                })
                return
            }

            if (invite.accepted_at) {
                setState({
                    status: "error",
                    message: t("invite_already_accepted", "This invite has already been accepted."),
                })
                return
            }

            if (invite.revoked_at) {
                setState({
                    status: "error",
                    message: t("invite_revoked", "This invite has been revoked."),
                })
                return
            }

            // Check auth status in parallel with driver fetch
            const [profileResult, driversResult] = await Promise.allSettled([
                api.profile.get(),
                api.auth.methods(),
            ])

            const profile = profileResult.status === "fulfilled" ? profileResult.value : null
            const drivers =
                driversResult.status === "fulfilled" ? driversResult.value : [AUTH_DRIVERS.BASIC]

            if (!profile) {
                // Not logged in
                setState({ status: "unauthenticated", invite: invite as ProjectInvite, drivers })
                return
            }

            // Logged in — check email match
            if (profile.email.toLowerCase() !== (invite.invitee_email ?? "").toLowerCase()) {
                setState({ status: "wrong-account", invite: invite as ProjectInvite })
                return
            }

            if (autoAccept) {
                // Came back from registration — auto-accept
                await doAccept(invite as ProjectInvite)
                return
            }

            setState({ status: "ready", invite: invite as ProjectInvite, profile })
        }

        init()
    }, [token, autoAccept, doAccept, t])

    const handleLogin = () => {
        navigate(`/login?r=${encodeURIComponent(`/invites/${token}`)}`)
    }

    const handleRegister = () => {
        navigate(`/register?r=${encodeURIComponent(`/invites/${token}?autoAccept=1`)}`)
    }

    if (state.status === "loading" || state.status === "accepting") {
        return (
            <div className="min-h-screen flex items-center justify-center bg-muted/40">
                <Card className="w-full max-w-sm">
                    <CardContent className="flex flex-col items-center justify-center py-12 gap-4">
                        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                        <p className="text-sm text-muted-foreground">
                            {state.status === "accepting"
                                ? t("invite_accepting", "Accepting invite...")
                                : t("loading")}
                        </p>
                    </CardContent>
                </Card>
            </div>
        )
    }

    if (state.status === "accepted") {
        return (
            <div className="min-h-screen flex items-center justify-center bg-muted/40">
                <Card className="w-full max-w-sm">
                    <CardContent className="flex flex-col items-center justify-center py-12 gap-4">
                        <p className="text-sm text-muted-foreground">
                            {t("invite_accepted_redirecting", "Invite accepted! Redirecting...")}
                        </p>
                    </CardContent>
                </Card>
            </div>
        )
    }

    if (state.status === "error") {
        return (
            <div className="min-h-screen flex items-center justify-center bg-muted/40 p-4">
                <Card className="w-full max-w-sm">
                    <CardContent className="pt-6">
                        <Alert variant="destructive">
                            <ShieldAlert className="h-4 w-4" />
                            <AlertTitle>{t("error")}</AlertTitle>
                            <AlertDescription>{state.message}</AlertDescription>
                        </Alert>
                    </CardContent>
                </Card>
            </div>
        )
    }

    if (state.status === "wrong-account") {
        return (
            <div className="min-h-screen flex items-center justify-center bg-muted/40 p-4">
                <Card className="w-full max-w-sm">
                    <CardHeader className="space-y-1 text-center">
                        <CardTitle className="text-2xl font-bold">
                            {t("invite_wrong_account_title", "Wrong account")}
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <Alert variant="destructive">
                            <ShieldAlert className="h-4 w-4" />
                            <AlertDescription>
                                {t(
                                    "invite_wrong_account",
                                    "This invite was not sent to your account. Please log out and sign in with the correct account.",
                                )}
                            </AlertDescription>
                        </Alert>
                    </CardContent>
                </Card>
            </div>
        )
    }

    if (state.status === "unauthenticated") {
        const { invite, drivers } = state
        const hasClerk = drivers.includes(AUTH_DRIVERS.CLERK)

        return (
            <div className="min-h-screen flex items-center justify-center bg-muted/40 p-4">
                <Card className="w-full max-w-sm">
                    <CardHeader className="space-y-1 text-center">
                        <CardTitle className="text-2xl font-bold">
                            {t("invite_title", "You've been invited")}
                        </CardTitle>
                        <CardDescription>
                            {t(
                                "invite_sign_in_prompt",
                                "Sign in or create an account to accept this invite.",
                            )}
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <InviteDetails invite={invite} />
                        <div className="flex flex-col gap-2 pt-2">
                            <Button className="w-full" onClick={handleLogin}>
                                <Mail className="mr-2 h-4 w-4" />
                                {t("login")}
                            </Button>
                            {hasClerk && (
                                <Button
                                    variant="outline"
                                    className="w-full"
                                    onClick={handleRegister}
                                >
                                    {t("invite_create_account", "Create account")}
                                </Button>
                            )}
                        </div>
                    </CardContent>
                </Card>
            </div>
        )
    }

    // status === "ready"
    const { invite } = state

    return (
        <div className="min-h-screen flex items-center justify-center bg-muted/40 p-4">
            <Card className="w-full max-w-sm">
                <CardHeader className="space-y-1 text-center">
                    <CardTitle className="text-2xl font-bold">
                        {t("invite_title", "You've been invited")}
                    </CardTitle>
                    <CardDescription>
                        {t("invite_review_prompt", "Review the details below and accept to join.")}
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                    <InviteDetails invite={invite} />
                    <Button className="w-full" onClick={() => doAccept(invite)}>
                        {t("invite_accept", "Accept invite")}
                    </Button>
                </CardContent>
            </Card>
        </div>
    )
}

function InviteDetails({ invite }: { invite: ProjectInvite }) {
    const { t } = useTranslation()

    return (
        <div className="rounded-md border p-4 space-y-2 text-sm">
            <div className="flex justify-between items-center">
                <span className="text-muted-foreground">{t("role")}</span>
                <Badge variant="secondary">{invite.role}</Badge>
            </div>
            {invite.inviter_admin_email && (
                <div className="flex justify-between items-center">
                    <span className="text-muted-foreground">{t("invited_by", "Invited by")}</span>
                    <span className="font-medium truncate max-w-45">
                        {invite.inviter_admin_email}
                    </span>
                </div>
            )}
            {invite.expires_at && (
                <div className="flex justify-between items-center">
                    <span className="text-muted-foreground">{t("expires", "Expires")}</span>
                    <span>{formatExpiry(invite.expires_at)}</span>
                </div>
            )}
        </div>
    )
}
