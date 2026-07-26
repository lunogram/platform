import { describe, expect, it } from "vitest"
import {
    resolveMergeTags,
    templaticalPlainText,
    templaticalPreviewHtml,
} from "./templatical-preview"

const context = {
    user: { email: "admin@localhost", data: { admin: true } },
    unsubscribe_url: "https://lunogram.com/unsubscribe",
    now: "2026-07-26T00:00:00.000Z",
}

describe("resolveMergeTags", () => {
    it("resolves a nested user path", () => {
        expect(resolveMergeTags("Hello {{ user.email }}", context)).toBe("Hello admin@localhost")
        expect(resolveMergeTags("{{ user.data.admin }}", context)).toBe("true")
    })

    it("resolves tags outside the user scope", () => {
        // These are offered by the merge-tag picker but were previously
        // unresolvable, because only `user` was in the preview context.
        expect(resolveMergeTags("{{ unsubscribe_url }}", context)).toBe(
            "https://lunogram.com/unsubscribe",
        )
    })

    it("resolves filtered tags", () => {
        // The picker's "Current Year". Handlebars could not parse the `|`, and
        // because the pass covers the whole document its parse error took every
        // other tag down with it.
        expect(resolveMergeTags("{{ now | date: '%Y' }}", context)).toBe("2026")
    })

    it("renders an unknown path empty without affecting its neighbours", () => {
        expect(
            resolveMergeTags("a={{ user.email }} b={{ nope }} c={{ now | date: '%Y' }}", context),
        ).toBe("a=admin@localhost b= c=2026")
    })

    it("leaves rendered email scaffolding intact", () => {
        const html = [
            "<style>@media only screen and (min-width:480px) { .col { width:100% !important; } }</style>",
            "<!--[if mso | IE]><table><tr><td><![endif]-->",
            "<p>Hi {{ user.email }}</p>",
        ].join("\n")

        const out = resolveMergeTags(html, context)

        expect(out).toContain("@media only screen and (min-width:480px)")
        expect(out).toContain("<!--[if mso | IE]>")
        expect(out).toContain("Hi admin@localhost")
    })

    it("returns the input unchanged when there is nothing to render", () => {
        expect(resolveMergeTags("", context)).toBe("")
    })
})

describe("bundle readers", () => {
    it("read the html and plain text written by the backend", () => {
        const bundle = JSON.stringify({
            kind: "templatical",
            html: "<p>hi</p>",
            plainText: "hi",
        })

        expect(templaticalPreviewHtml(bundle)).toBe("<p>hi</p>")
        expect(templaticalPlainText(bundle)).toBe("hi")
    })

    it("preview as blank for a template that has not been saved yet", () => {
        expect(templaticalPreviewHtml(undefined)).toBe("")
        expect(templaticalPlainText(null)).toBe("")
        expect(templaticalPreviewHtml("not json")).toBe("")
    })
})
