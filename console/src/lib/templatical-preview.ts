/**
 * Preview HTML for a visually authored (Templatical) email template.
 *
 * These templates are rendered by the backend when the template is saved, and
 * the resulting HTML is stored in the bundle. The console must not fall back to
 * compiling `code.source` for them: that field holds whatever JSX the template
 * carried before it was switched to the visual editor, kept only so the switch
 * stays reversible, and rendering it shows the wrong email entirely.
 *
 * Returns an empty string when the template has not been saved yet, which
 * previews as blank — the same as a code template that has not compiled.
 */
export function templaticalPreviewHtml(bundle: string | undefined | null): string {
    return readBundle(bundle).html ?? ""
}

/**
 * Plain-text alternative for a visually authored template.
 *
 * Derived by the backend from the rendered HTML, so like the HTML it reflects
 * the last save rather than unsaved edits.
 */
export function templaticalPlainText(bundle: string | undefined | null): string {
    return readBundle(bundle).plainText ?? ""
}

function readBundle(bundle: string | undefined | null): {
    kind?: string
    html?: string
    plainText?: string
} {
    if (!bundle) return {}
    try {
        return JSON.parse(bundle) as { kind?: string; html?: string; plainText?: string }
    } catch {
        return {}
    }
}
