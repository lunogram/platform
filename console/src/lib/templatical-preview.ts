import { Liquid } from "liquidjs"

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

/**
 * Liquid, matching the merge-tag syntax the block editor is configured with and
 * the engine the send pipeline uses server-side. Non-strict so an unknown path
 * renders empty instead of throwing, the same as a real send.
 */
const liquid = new Liquid({ strictVariables: false, strictFilters: false })

/**
 * Substitute merge tags in a rendered email against a preview context.
 *
 * The backend renders a visually authored template once, at save time, leaving
 * merge tags as literal `{{ … }}` for the Liquid pass that runs per recipient.
 * Previewing therefore means running that same pass in the console.
 *
 * It must be Liquid rather than Handlebars: the merge-tag picker offers
 * filtered tags such as `{{ now | date: '%Y' }}`, and Handlebars cannot parse
 * the `|`. Because the substitution covers the whole document, a parse error
 * takes every other tag down with it, not just the offending one.
 *
 * Falls back to the unsubstituted HTML if rendering fails, so a preview never
 * goes blank.
 */
export function resolveMergeTags(html: string, context: Record<string, unknown>): string {
    if (!html) return html
    try {
        return liquid.parseAndRenderSync(html, context)
    } catch (error) {
        console.warn("Merge tag preview failed:", error)
        return html
    }
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
