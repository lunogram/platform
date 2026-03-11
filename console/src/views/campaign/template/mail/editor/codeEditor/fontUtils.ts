import * as ReactEmailComponents from "@react-email/components"

// System / web-safe fonts that don't need @font-face declarations.
const SYSTEM_FONTS = new Set([
    "arial",
    "helvetica",
    "helvetica neue",
    "times new roman",
    "times",
    "georgia",
    "courier new",
    "courier",
    "verdana",
    "tahoma",
    "trebuchet ms",
    "lucida sans",
    "lucida sans unicode",
    "lucida grande",
    "lucida console",
    "palatino linotype",
    "book antiqua",
    "palatino",
    "impact",
    "comic sans ms",
    "system-ui",
    "ui-sans-serif",
    "ui-serif",
    "ui-monospace",
    "ui-rounded",
    "-apple-system",
    "blinkmacsystemfont",
    "segoe ui",
    "roboto",
    "noto sans",
    "ubuntu",
    "cantarell",
    "fira sans",
    "droid sans",
    "sans-serif",
    "serif",
    "monospace",
    "cursive",
    "fantasy",
    "apple color emoji",
    "segoe ui emoji",
    "segoe ui symbol",
    "noto color emoji",
])

/**
 * Extract custom (non-system) font family names from a Tailwind config's
 * `theme.fontFamily` and `theme.extend.fontFamily` fields.
 *
 * Each fontFamily entry can be a string (single font) or an array of strings
 * (font stack). We extract the _first_ font in each stack since that's the
 * primary font that needs to be loaded via @font-face.
 */
export function extractCustomFonts(config: Record<string, unknown>): string[] {
    const fonts: string[] = []
    const theme = config?.theme as Record<string, unknown> | undefined
    if (!theme) return fonts

    // Collect from both theme.fontFamily and theme.extend.fontFamily
    const fontFamilySources: Record<string, unknown>[] = []
    if (theme.fontFamily && typeof theme.fontFamily === "object") {
        fontFamilySources.push(theme.fontFamily as Record<string, unknown>)
    }
    const extend = theme.extend as Record<string, unknown> | undefined
    if (extend?.fontFamily && typeof extend.fontFamily === "object") {
        fontFamilySources.push(extend.fontFamily as Record<string, unknown>)
    }

    for (const fontFamily of fontFamilySources) {
        for (const value of Object.values(fontFamily)) {
            let primaryFont: string | undefined
            if (typeof value === "string") {
                primaryFont = value.split(",")[0].trim()
            } else if (Array.isArray(value) && value.length > 0) {
                primaryFont = String(value[0]).trim()
            }
            if (primaryFont) {
                // Remove quotes
                primaryFont = primaryFont.replace(/^['"]|['"]$/g, "")
                if (primaryFont && !SYSTEM_FONTS.has(primaryFont.toLowerCase())) {
                    fonts.push(primaryFont)
                }
            }
        }
    }

    return [...new Set(fonts)]
}

/**
 * Extract custom font names from `font-*` CSS class usage in template source code.
 *
 * When users write `className="font-koala"` or `className="font-inter"`, this
 * extracts the font name part (e.g. "koala", "inter"). Names that match
 * Tailwind's built-in font utilities (sans, serif, mono) are excluded.
 *
 * For multi-word fonts, converts kebab-case to Title Case:
 *   "font-open-sans" → "Open Sans"
 *   "font-playfair-display" → "Playfair Display"
 */
export function extractFontClassNames(sourceCode: string): string[] {
    const builtinFonts = new Set(["sans", "serif", "mono"])
    const fontClasses = new Set<string>()

    // Match font-{name} in className strings (both single and double quotes, template literals)
    const classNameRegex = /className\s*=\s*(?:"[^"]*"|'[^']*'|{`[^`]*`})/g
    let classMatch: RegExpExecArray | null
    while ((classMatch = classNameRegex.exec(sourceCode)) !== null) {
        const classValue = classMatch[0]
        // Extract individual font-* classes
        const fontRegex = /\bfont-([\w-]+)/g
        let fontMatch: RegExpExecArray | null
        while ((fontMatch = fontRegex.exec(classValue)) !== null) {
            const name = fontMatch[1]
            // Skip built-in Tailwind font utilities and weight classes
            if (
                builtinFonts.has(name) ||
                // Font weight classes: font-thin, font-light, font-normal, etc.
                [
                    "thin",
                    "extralight",
                    "light",
                    "normal",
                    "medium",
                    "semibold",
                    "bold",
                    "extrabold",
                    "black",
                ].includes(name)
            ) {
                continue
            }
            fontClasses.add(name)
        }
    }

    // Convert kebab-case class names to Title Case font names
    return [...fontClasses].map((name) =>
        name
            .split("-")
            .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
            .join(" "),
    )
}

/**
 * Build Google Fonts @font-face CSS for a list of font names.
 *
 * Generates a `<style>` block with @font-face declarations that load
 * fonts from the Google Fonts CSS2 API. Each font is loaded in woff2
 * format with weight range 100-900 for maximum flexibility.
 */
export function buildGoogleFontsCss(fontNames: string[]): string {
    if (fontNames.length === 0) return ""

    // Build Google Fonts CSS2 API URL
    const familyParams = fontNames
        .map((name) => `family=${encodeURIComponent(name)}:ital,wght@0,100..900;1,100..900`)
        .join("&")
    const googleFontsUrl = `https://fonts.googleapis.com/css2?${familyParams}&display=swap`

    // Generate @import rule (more reliable in email clients than <link>)
    return `<style>@import url('${googleFontsUrl}');</style>`
}

/**
 * Inject font CSS into rendered HTML.
 *
 * Inserts @font-face declarations into the <head> of the rendered email HTML.
 * If no <head> tag is found, prepends to the HTML.
 */
export function injectFontCss(html: string, fontCss: string): string {
    if (!fontCss) return html

    // Insert just before </head>
    const headCloseIdx = html.indexOf("</head>")
    if (headCloseIdx !== -1) {
        return html.slice(0, headCloseIdx) + fontCss + html.slice(headCloseIdx)
    }

    // No </head> found — insert after <head> or <head ...>
    const headOpenMatch = html.match(/<head[^>]*>/i)
    if (headOpenMatch) {
        const insertIdx = headOpenMatch.index! + headOpenMatch[0].length
        return html.slice(0, insertIdx) + fontCss + html.slice(insertIdx)
    }

    // Last resort: prepend
    return fontCss + html
}

/**
 * Build a default Tailwind config object that includes any custom font
 * definitions detected from user source code.
 *
 * This merges user-provided theme settings (fontFamily, colors, etc.)
 * with the `pixelBasedPreset` from `@react-email/components`.
 *
 * @param userConfig - An optional Tailwind config extracted from user code
 * @param sourceCode - The original source code (used for class-based font detection fallback)
 * @returns An object with the merged config and the list of detected font names
 */
export function buildFontAwareTailwindConfig(
    userConfig: Record<string, unknown> | null,
    sourceCode: string,
): { config: Record<string, unknown>; detectedFonts: string[] } {
    let detectedFonts: string[] = []

    const defaultTailwindConfig: Record<string, unknown> = {
        presets: [ReactEmailComponents.pixelBasedPreset],
    }

    // Merge user's theme.fontFamily / theme.extend.fontFamily
    // into the injected config so that classes like `font-koala`
    // actually resolve to the right font-family CSS value.
    if (userConfig) {
        detectedFonts = extractCustomFonts(userConfig)
        const userTheme = userConfig.theme as Record<string, unknown> | undefined
        if (userTheme) {
            const mergedTheme: Record<string, unknown> = { extend: {} }
            const mergedExtend = mergedTheme.extend as Record<string, unknown>

            // Copy fontFamily from theme.extend.fontFamily
            const userExtend = userTheme.extend as Record<string, unknown> | undefined
            if (userExtend?.fontFamily) {
                mergedExtend.fontFamily = userExtend.fontFamily
            }
            // Copy fontFamily from theme.fontFamily (top-level override)
            if (userTheme.fontFamily) {
                mergedTheme.fontFamily = userTheme.fontFamily
            }
            // Copy colors from theme.extend.colors
            if (userExtend?.colors) {
                mergedExtend.colors = userExtend.colors
            }

            defaultTailwindConfig.theme = mergedTheme
        }
    }

    // If no fontFamily was found in the config object, scan the
    // user's source code for font-* class names and auto-generate
    // fontFamily entries so Tailwind can resolve them.
    if (detectedFonts.length === 0) {
        const classBasedFonts = extractFontClassNames(sourceCode)
        if (classBasedFonts.length > 0) {
            detectedFonts = classBasedFonts
            // Build fontFamily mapping: "font-koala" → { koala: ["Koala", "sans-serif"] }
            const fontFamilyMap: Record<string, string[]> = {}
            for (const fontName of classBasedFonts) {
                const key = fontName.toLowerCase().replace(/\s+/g, "-")
                fontFamilyMap[key] = [fontName, "sans-serif"]
            }
            if (!defaultTailwindConfig.theme) {
                defaultTailwindConfig.theme = { extend: {} }
            }
            const theme = defaultTailwindConfig.theme as Record<string, unknown>
            if (!theme.extend) theme.extend = {}
            const extend = theme.extend as Record<string, unknown>
            extend.fontFamily = {
                ...(extend.fontFamily as Record<string, unknown> | undefined),
                ...fontFamilyMap,
            }
        }
    }

    return { config: defaultTailwindConfig, detectedFonts }
}
