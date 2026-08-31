/**
 * Copies the console URL where an invitee finds the invites addressed to them.
 * An invite carries no token — it is bound to the invitee's verified email —
 * so the link is the same for everybody and only useful once they sign in.
 */
export async function copyInviteLink() {
    await navigator.clipboard.writeText(`${window.location.origin}/invites`)
}
