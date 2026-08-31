/**
 * The console routes that work without a session.
 *
 * Anything that reacts to a 401 by sending the visitor to the login page has to
 * consult this first, or it bounces people off the very pages they were sent to
 * in order to obtain a session: an invitation lands on /register, and a reset
 * link lands on /reset-password held by somebody who by definition cannot sign
 * in yet.
 */
export const PUBLIC_PATHS = [
    "/login",
    "/register",
    "/forgot-password",
    "/reset-password",
    "/verify-email",
]

export function isPublicPage(pathname: string = window.location.pathname): boolean {
    return PUBLIC_PATHS.some((path) => pathname.startsWith(path))
}
