/**
 * Reading and writing the visual editor's document as portable JSON.
 *
 * Templatical stores a template as `{ blocks, settings }`, which is the whole
 * point of the format — it moves between templates, campaigns and instances
 * unchanged. These helpers are the console's import/export ends of that.
 *
 * Validation is structural rather than exhaustive: it mirrors the check the
 * renderer makes (`isTemplaticalDocument` in `renderer/templatical.ts`) so the
 * console rejects the same documents the backend would fail to compile. It
 * deliberately does not validate individual blocks — the editor tolerates block
 * shapes this console does not know about, and rejecting them here would break
 * documents written by a newer version.
 */

/** Shape every Templatical document has, whatever its blocks contain. */
interface TemplaticalDocumentShape {
    blocks: unknown[]
    settings: Record<string, unknown>
}

export type ParseResult = { ok: true; doc: Record<string, unknown> } | { ok: false; error: string }

/**
 * Parse pasted or uploaded JSON into a document safe to hand to the editor.
 *
 * Returns a message rather than throwing, because every failure here is user
 * input that needs explaining rather than a bug.
 */
export function parseTemplaticalDocument(text: string): ParseResult {
    const trimmed = text.trim()
    if (!trimmed) return { ok: false, error: "Nothing to import." }

    let parsed: unknown
    try {
        parsed = JSON.parse(trimmed)
    } catch (error) {
        return {
            ok: false,
            error: `Not valid JSON: ${error instanceof Error ? error.message : String(error)}`,
        }
    }

    if (!isDocumentShape(parsed)) {
        return {
            ok: false,
            error: 'Not a template document — expected an object with a "blocks" array and a "settings" object.',
        }
    }

    return { ok: true, doc: parsed as unknown as Record<string, unknown> }
}

/** Serialize a document for the clipboard or a downloaded file. */
export function serializeTemplaticalDocument(doc: unknown): string {
    return JSON.stringify(doc, null, 2)
}

/**
 * Build a filename for an exported document, e.g. `welcome-email-de-DE.json`.
 *
 * Falls back to a generic name when the campaign has no usable name, so the
 * download never lands as `.json` with an empty stem.
 */
export function exportFileName(campaignName: string, locale: string): string {
    // Slugify the joined string rather than each part: slugifying first would
    // leave a separator behind on a part ending in punctuation, and joining
    // would then double it up.
    const stem = [campaignName, locale]
        .join(" ")
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-+|-+$/g, "")

    return `${stem || "email-template"}.json`
}

function isDocumentShape(value: unknown): value is TemplaticalDocumentShape {
    if (typeof value !== "object" || value === null) return false
    const candidate = value as Partial<TemplaticalDocumentShape>
    return (
        Array.isArray(candidate.blocks) &&
        typeof candidate.settings === "object" &&
        candidate.settings !== null &&
        !Array.isArray(candidate.settings)
    )
}
