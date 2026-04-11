/// <reference types="vite/client" />

/**
 * Compile-time feature flag for enterprise features.
 * Replaced by Vite's `define` option at build time.
 */
declare const __ENTERPRISE__: boolean

/**
 * Runtime configuration injected by the server via /config.js.
 * Values are environment-specific and served dynamically so the
 * Docker image does not need to be rebuilt per environment.
 */
interface WindowConfig {
    CLERK_PUBLISHABLE_KEY: string
}

declare interface Window {
    __CONFIG__?: WindowConfig
}
