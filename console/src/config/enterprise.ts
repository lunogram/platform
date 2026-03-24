/**
 * Compile-time feature flag for enterprise features.
 *
 * Set via Vite's `define` option (see vite.config.ts).
 * When building the open-source edition, `__ENTERPRISE__` is `false`
 * and all enterprise-only code paths are tree-shaken from the bundle.
 *
 * Usage:
 *   if (__ENTERPRISE__) {
 *       // enterprise-only code
 *   }
 */

declare const __ENTERPRISE__: boolean

export const isEnterprise: boolean = typeof __ENTERPRISE__ !== "undefined" && __ENTERPRISE__
