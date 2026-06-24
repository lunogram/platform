import { describe, it, expect } from "vitest"

import { validateRedirect } from "./validate-redirect"

// window.location.origin is "http://localhost:3000" under jsdom (configurable via vite.config).
const origin = window.location.origin

describe("validateRedirect", () => {
    it("falls back to / for empty/nullish input", () => {
        expect(validateRedirect(null)).toBe("/")
        expect(validateRedirect(undefined)).toBe("/")
        expect(validateRedirect("")).toBe("/")
    })

    it("allows same-origin relative paths", () => {
        expect(validateRedirect("/dashboard")).toBe("/dashboard")
        expect(validateRedirect("/dashboard?x=1#section")).toBe("/dashboard?x=1#section")
    })

    it("strips the origin from same-origin absolute URLs", () => {
        expect(validateRedirect(`${origin}/settings?tab=2`)).toBe("/settings?tab=2")
    })

    it("rejects cross-origin absolute URLs", () => {
        expect(validateRedirect("https://evil.com/phish")).toBe("/")
        expect(validateRedirect("http://evil.com")).toBe("/")
    })

    it("rejects protocol-relative URLs", () => {
        expect(validateRedirect("//evil.com")).toBe("/")
        expect(validateRedirect("//evil.com/path")).toBe("/")
    })

    it("rejects backslash-prefixed URLs", () => {
        expect(validateRedirect("\\\\evil.com")).toBe("/")
    })

    it("rejects URLs that normalize to a different origin", () => {
        // browsers/WHATWG normalize backslashes to forward slashes
        expect(validateRedirect("/\\evil.com")).toBe("/")
    })

    it("rejects non-http schemes (javascript:, data:)", () => {
        expect(validateRedirect("javascript:alert(1)")).toBe("/")
        expect(validateRedirect("data:text/html,<script>alert(1)</script>")).toBe("/")
    })
})
