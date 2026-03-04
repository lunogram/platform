/**
 * Standardized locale utilities based on IETF BCP 47 language tags.
 *
 * All locales use the canonical form produced by Intl:
 *   - Language-only:       "en", "fr", "zh"
 *   - Language + region:   "en-US", "pt-BR", "zh-CN"
 *
 * This module provides:
 *   1. A curated list of commonly-used locales (covers 95%+ of real-world usage)
 *   2. Display-name resolution via Intl.DisplayNames
 *   3. Validation helpers
 */

export interface LocaleEntry {
    /** BCP 47 tag, e.g. "en-US" */
    key: string
    /** Human-readable label, e.g. "English (United States)" */
    label: string
}

// ---------------------------------------------------------------------------
// Curated locale list – covers the most widely-used language + region combos.
// Keys follow BCP 47 (language[-Script][-Region]).
// ---------------------------------------------------------------------------

const LOCALE_KEYS = [
    // Major languages – base codes
    'en', 'es', 'fr', 'de', 'it', 'pt', 'nl', 'ru', 'ja', 'ko',
    'zh', 'ar', 'hi', 'bn', 'pa', 'tr', 'vi', 'th', 'pl', 'uk',
    'ro', 'cs', 'el', 'hu', 'sv', 'da', 'fi', 'nb', 'sk', 'bg',
    'hr', 'sr', 'sl', 'lt', 'lv', 'et', 'ms', 'id', 'tl', 'sw',
    'he', 'fa', 'ur', 'ta', 'te', 'ml', 'kn', 'mr', 'gu', 'ca',
    'eu', 'gl',

    // Regional variants
    'en-US', 'en-GB', 'en-AU', 'en-CA', 'en-IN', 'en-NZ', 'en-ZA', 'en-IE',
    'es-ES', 'es-MX', 'es-AR', 'es-CO', 'es-CL', 'es-PE', 'es-VE',
    'fr-FR', 'fr-CA', 'fr-BE', 'fr-CH',
    'de-DE', 'de-AT', 'de-CH',
    'it-IT', 'it-CH',
    'pt-BR', 'pt-PT',
    'nl-NL', 'nl-BE',
    'zh-CN', 'zh-TW', 'zh-HK',
    'ar-SA', 'ar-EG', 'ar-AE', 'ar-MA',
    'ru-RU',
    'ja-JP',
    'ko-KR',
    'hi-IN',
    'bn-BD', 'bn-IN',
    'sv-SE',
    'da-DK',
    'fi-FI',
    'nb-NO',
    'pl-PL',
    'tr-TR',
    'vi-VN',
    'th-TH',
    'ms-MY',
    'id-ID',
    'ro-RO',
    'cs-CZ',
    'el-GR',
    'hu-HU',
    'uk-UA',
    'sk-SK',
    'bg-BG',
    'hr-HR',
    'sr-RS',
    'sl-SI',
    'lt-LT',
    'lv-LV',
    'et-EE',
    'he-IL',
    'fa-IR',
    'ur-PK',
    'ta-IN', 'ta-LK',
    'te-IN',
    'ml-IN',
    'kn-IN',
    'mr-IN',
    'gu-IN',
    'pa-IN',
    'sw-KE', 'sw-TZ',
    'tl-PH',
    'ca-ES',
    'eu-ES',
    'gl-ES',
] as const

// ---------------------------------------------------------------------------
// Display name resolution
// ---------------------------------------------------------------------------

/** Resolve a BCP 47 key into a human-readable label using Intl.DisplayNames. */
export function resolveLocaleName(key: string): string {
    try {
        const dn = new Intl.DisplayNames(['en'], { type: 'language' })
        return dn.of(key) ?? key
    } catch {
        return key
    }
}

// Build the full list once at module load
export const LOCALES: LocaleEntry[] = LOCALE_KEYS.map(key => ({
    key,
    label: resolveLocaleName(key),
}))

// Fast lookup set for validation
const LOCALE_KEY_SET = new Set<string>(LOCALE_KEYS)

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

/**
 * BCP 47 regex (simplified but sufficient for practical use):
 *   language        2–3 letters
 *   script          optional, 4 letters (e.g. Latn, Hans)
 *   region          optional, 2 letters or 3 digits
 *
 * Examples that match: en, en-US, zh-Hans, zh-Hans-CN, sr-Latn-RS
 */
const BCP47_RE = /^[a-z]{2,3}(-[A-Z][a-z]{3})?(-([A-Z]{2}|\d{3}))?$/

/** Returns true when the key is a structurally valid BCP 47 tag. */
export function isValidLocaleKey(key: string): boolean {
    return BCP47_RE.test(key)
}

/** Returns true when the key is in our curated list. */
export function isKnownLocale(key: string): boolean {
    return LOCALE_KEY_SET.has(key)
}

// ---------------------------------------------------------------------------
// Search / filter
// ---------------------------------------------------------------------------

/**
 * Filter the curated locale list by a search query (case-insensitive).
 * Matches against both key and label.
 */
export function searchLocales(query: string, list: LocaleEntry[] = LOCALES): LocaleEntry[] {
    if (!query) return list
    const q = query.toLowerCase()
    return list.filter(
        l => l.key.toLowerCase().includes(q) || l.label.toLowerCase().includes(q),
    )
}
