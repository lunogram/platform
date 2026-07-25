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
    if (!bundle) return ""
    try {
        const parsed = JSON.parse(bundle) as { kind?: string; html?: string }
        return parsed.html ?? ""
    } catch {
        return ""
    }
}
