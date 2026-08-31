import { useEffect, useState, useCallback } from "react"
import { Link, useNavigate, useSearchParams } from "react-router"
import { useTranslation } from "react-i18next"
import { ArrowRight, Inbox, Loader2, MailOpen } from "lucide-react"

import api from "../../api"
import type { NetworkError } from "../../api"
import type { ProjectInvite } from "../../types"
import { snakeToTitle } from "@/utils"
import { getRandomColor, getRandomIcon } from "@/lib/colors"

import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { toast } from "sonner"

function formatExpiry(expiresAt: string): string {
    return new Date(expiresAt).toLocaleDateString(undefined, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
    })
}

// ProjectAvatar is the mark the sidebar's project switcher shows, so a project
// is recognisable here before its name is read — and recognisably the same one
// once the invitation is accepted. Both the colour and the glyph are derived
// from the name, so nothing has to be stored or passed through the invite.
function ProjectAvatar({ name }: { name: string }) {
    return (
        <div
            className="flex aspect-square size-10 shrink-0 items-center justify-center rounded-xl text-white"
            style={{ backgroundColor: getRandomColor(name) }}
        >
            <i className={`fa-solid fa-${getRandomIcon(name)} text-base`} />
        </div>
    )
}

type PageState =
    | { status: "loading" }
    | { status: "ready"; invites: ProjectInvite[] }
    | { status: "anonymous" }
    | { status: "error" }

// InviteShell is the frame all four states sit in, so the page does not resize
// or re-centre as it moves between them.
function InviteShell({ children }: { children: React.ReactNode }) {
    return (
        <div className="min-h-screen bg-surface-soft px-5 py-16 sm:py-24">
            <div className="mx-auto w-full max-w-lg">{children}</div>
        </div>
    )
}

export default function MyInvites() {
    const { t } = useTranslation()
    const navigate = useNavigate()
    const [searchParams] = useSearchParams()

    const [state, setState] = useState<PageState>({ status: "loading" })
    const [accepting, setAccepting] = useState<string | null>(null)

    // The address the invitation was sent to. It only prefills the sign-up form
    // — the invite is granted on the address the account proves, never on what
    // the link happens to carry.
    const invitedEmail = searchParams.get("email") ?? ""

    const load = useCallback(async () => {
        setState({ status: "loading" })
        try {
            const result = await api.invites.mine()
            setState({ status: "ready", invites: result.results })
        } catch (err) {
            // Somebody following an invitation may have no account yet, so a 401
            // is a fork rather than a failure: it is answered with both ways to
            // get one rather than by assuming they already have.
            const status = (err as NetworkError)?.response?.status
            setState({ status: status === 401 ? "anonymous" : "error" })
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
            <InviteShell>
                <div className="flex justify-center py-16">
                    <Loader2 className="h-6 w-6 animate-spin text-ink-soft" />
                </div>
            </InviteShell>
        )
    }

    if (state.status === "anonymous") {
        const returnTo = encodeURIComponent("/invites")
        const signUpHref = invitedEmail
            ? `/register?email=${encodeURIComponent(invitedEmail)}&r=${returnTo}`
            : `/register?r=${returnTo}`

        return (
            <InviteShell>
                <Card className="p-6">
                    <div className="flex flex-col items-center gap-4 text-center">
                        <span className="flex h-11 w-11 items-center justify-center rounded-xl bg-surface-muted text-ink-soft">
                            <MailOpen className="h-5 w-5" strokeWidth={1.5} />
                        </span>
                        <div className="space-y-1.5">
                            <h1 className="text-2xl font-semibold tracking-tight">
                                {t("invites_anonymous_title", "You have been invited")}
                            </h1>
                            <p className="text-sm text-ink-soft">
                                {invitedEmail
                                    ? t("invites_anonymous_description_email", {
                                          defaultValue:
                                              "Sign in as {{email}} to see your invitation, or create an account with that address.",
                                          email: invitedEmail,
                                      })
                                    : t(
                                          "invites_anonymous_description",
                                          "Sign in with the address your invitation was sent to, or create an account with it.",
                                      )}
                            </p>
                        </div>
                        <div className="flex w-full flex-col gap-2 pt-1">
                            <Button asChild className="w-full">
                                <Link to={`/login?r=${returnTo}`}>
                                    {t("invites_anonymous_sign_in", "Sign in")}
                                </Link>
                            </Button>
                            <Button asChild variant="outline" className="w-full">
                                <Link to={signUpHref}>
                                    {t("invites_anonymous_register", "Create an account")}
                                </Link>
                            </Button>
                        </div>
                    </div>
                </Card>
            </InviteShell>
        )
    }

    if (state.status === "error") {
        return (
            <InviteShell>
                <Card className="p-6">
                    <div className="flex flex-col items-center gap-4 py-6 text-center">
                        <p className="text-sm text-ink-soft">
                            {t("invites_load_failed", "Could not load your invites.")}
                        </p>
                        <Button variant="outline" onClick={load}>
                            {t("retry", "Retry")}
                        </Button>
                    </div>
                </Card>
            </InviteShell>
        )
    }

    return (
        <InviteShell>
            <div className="space-y-6">
                <div className="space-y-1.5">
                    <h1 className="text-3xl font-semibold tracking-tight">
                        {t("my_invites_title", "Your invites")}
                    </h1>
                    <p className="text-[15px] text-ink-soft">
                        {state.invites.length === 0
                            ? t("my_invites_description", "Pending invites for your account.")
                            : t("my_invites_count", {
                                  defaultValue_one: "{{count}} project is waiting for you.",
                                  defaultValue_other: "{{count}} projects are waiting for you.",
                                  count: state.invites.length,
                              })}
                    </p>
                </div>

                {state.invites.length === 0 ? (
                    <Card className="p-6">
                        <div className="flex flex-col items-center gap-3 py-10 text-center">
                            <Inbox className="h-7 w-7 text-ink-soft" strokeWidth={1.5} />
                            <p className="text-sm text-ink-soft">
                                {t("my_invites_empty", "You have no pending invites.")}
                            </p>
                            <Button
                                variant="ghost"
                                size="sm"
                                className="mt-1"
                                onClick={() => navigate("/")}
                            >
                                {t("go_home", "Go home")}
                                <ArrowRight className="ml-1.5 h-3.5 w-3.5" />
                            </Button>
                        </div>
                    </Card>
                ) : (
                    <div className="divide-y divide-border overflow-hidden rounded-xl border bg-card">
                        {state.invites.map((invite) => {
                            const projectName = invite.project_name ?? t("project", "Project")
                            const isAccepting = accepting === invite.id
                            return (
                                <div
                                    key={invite.id}
                                    className="flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:gap-5"
                                >
                                    <ProjectAvatar name={projectName} />

                                    <div className="min-w-0 flex-1 space-y-1">
                                        <div className="flex items-center gap-2">
                                            <span className="truncate font-medium">
                                                {projectName}
                                            </span>
                                            <Badge variant="secondary" className="shrink-0">
                                                {snakeToTitle(invite.role)}
                                            </Badge>
                                        </div>
                                        <p className="truncate text-sm text-ink-soft">
                                            {invite.inviter_admin_email
                                                ? t("invited_by_on", {
                                                      defaultValue: "Invited by {{email}}",
                                                      email: invite.inviter_admin_email,
                                                  })
                                                : t("invited_to_project", "You have been invited")}
                                        </p>
                                        <p className="text-xs text-ink-soft">
                                            {t("expires", "Expires")}{" "}
                                            {formatExpiry(invite.expires_at)}
                                        </p>
                                    </div>

                                    <Button
                                        className="shrink-0 sm:w-auto"
                                        disabled={accepting !== null}
                                        onClick={() => handleAccept(invite)}
                                    >
                                        {isAccepting && (
                                            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                        )}
                                        {t("invite_accept", "Accept invite")}
                                    </Button>
                                </div>
                            )
                        })}
                    </div>
                )}
            </div>
        </InviteShell>
    )
}
