/**
 * Clean up React streaming HTML output.
 *
 * React's renderToReadableStream produces HTML with Suspense boundary markers:
 * - <!--$?--> and <!--/$--> mark Suspense boundaries
 * - <!--$--> marks completed Suspense content
 * - <template id="B:X"> contains fallback content (to be replaced by S:X)
 * - <template id="P:X"> contains postponed placeholder (to be replaced by S:X+1)
 * - <div hidden id="S:X"> contains resolved content
 * - <script>$RC/$RX/$RS</script> handle client-side replacement
 *
 * The streaming format nests placeholders: B:0 -> S:0 (which may contain P:1 -> S:1, etc.)
 * We need to process iteratively until all placeholders are resolved.
 */
export function cleanStreamingHtml(html: string): string {
    // First, do string-based replacements for the streaming artifacts
    // This handles the nested template structure that DOMParser struggles with

    // Remove the outer DOCTYPE from react-email, we'll add our own later
    let cleanedHtml = html.replace(/<!DOCTYPE[^>]*>/gi, "")

    // Remove Suspense boundary comments
    cleanedHtml = cleanedHtml.replace(/<!--\$\?-->/g, "")
    cleanedHtml = cleanedHtml.replace(/<!--\/\$-->/g, "")
    cleanedHtml = cleanedHtml.replace(/<!--\$-->/g, "")

    // Extract all S:X hidden divs and their content
    const resolvedContent = new Map<string, string>()
    // More robust extraction: find all S:X divs
    const allHiddenDivs = cleanedHtml.matchAll(
        /<div hidden id="S:(\d+)">([\s\S]*?)(?=<div hidden id="S:|\s*$)/g,
    )
    for (const match of allHiddenDivs) {
        const slotId = match[1]
        let content = match[2]
        // Remove the closing </div> that belongs to this S:X div
        // The content ends just before the next S:X div or end of string
        content = content.replace(/<\/div>\s*$/, "")
        resolvedContent.set(slotId, content)
    }

    // Remove all the hidden S:X divs from the HTML
    cleanedHtml = cleanedHtml.replace(/<div hidden id="S:\d+">[\s\S]*$/g, "")

    // Now iteratively replace templates with their resolved content
    // B:X templates are replaced by S:X content
    // P:X templates are replaced by S:(X) content (same ID)
    let previousHtml = ""
    let iterations = 0
    const maxIterations = 10 // Safety limit

    while (previousHtml !== cleanedHtml && iterations < maxIterations) {
        previousHtml = cleanedHtml
        iterations++

        // Replace B:X templates with S:X content
        cleanedHtml = cleanedHtml.replace(/<template id="B:(\d+)"><\/template>/g, (_, slotId) => {
            return resolvedContent.get(slotId) || ""
        })

        // Replace P:X templates with S:X content (P uses same numbering as S)
        cleanedHtml = cleanedHtml.replace(/<template id="P:(\d+)"><\/template>/g, (_, slotId) => {
            return resolvedContent.get(slotId) || ""
        })
    }

    // Remove all $RC/$RX/$RS/$RR scripts (client-side replacement scripts)
    cleanedHtml = cleanedHtml.replace(/<script>\$R[CXSR][\s\S]*?<\/script>/g, "")
    cleanedHtml = cleanedHtml.replace(/<script>[\s\S]*?\$R[CXSR][\s\S]*?<\/script>/g, "")

    // Clean up any remaining script tags that look like React streaming scripts
    cleanedHtml = cleanedHtml.replace(/<script>[^<]*function\s*\$[A-Z]+[^<]*<\/script>/g, "")

    // Add back proper DOCTYPE for email
    const doctype =
        '<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">'
    cleanedHtml = doctype + cleanedHtml

    return cleanedHtml
}
