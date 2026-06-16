export function validateRedirect(url: string | null | undefined): string {
    if (!url) return "/"

    if (url.startsWith("//") || url.startsWith("\\\\")) {
        return "/"
    }

    try {
        const parsed = new URL(url, window.location.origin)

        if (parsed.origin !== window.location.origin) {
            return "/"
        }

        return `${parsed.pathname}${parsed.search}${parsed.hash}`
    } catch {
        return "/"
    }
}
