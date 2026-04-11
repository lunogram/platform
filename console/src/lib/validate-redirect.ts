export function validateRedirect(url: string | null | undefined): string {
    if (!url) return "/"

    const candidate = url
    if (!candidate || candidate.startsWith("//") || candidate.startsWith("\\\\")) {
        return "/"
    }

    try {
        const parsed = new URL(candidate, window.location.origin)

        if (parsed.origin !== window.location.origin) {
            return "/"
        }

        return `${parsed.pathname}${parsed.search}${parsed.hash}`
    } catch {
        return "/"
    }
}
