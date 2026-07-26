import { describe, expect, it } from "vitest"
import {
    exportFileName,
    parseTemplaticalDocument,
    serializeTemplaticalDocument,
} from "./documentJson"

const doc = { blocks: [{ type: "title", id: "a" }], settings: { backgroundColor: "#fff" } }

describe("parseTemplaticalDocument", () => {
    it("accepts a document and returns it", () => {
        const result = parseTemplaticalDocument(JSON.stringify(doc))

        expect(result.ok).toBe(true)
        expect(result.ok && result.doc).toEqual(doc)
    })

    it("accepts an empty document", () => {
        // createDefaultTemplateContent() produces zero blocks, so an empty
        // template is legitimate and must survive a round trip.
        const result = parseTemplaticalDocument(JSON.stringify({ blocks: [], settings: {} }))

        expect(result.ok).toBe(true)
    })

    it("keeps block shapes it does not recognise", () => {
        // A document written by a newer editor may carry blocks this console
        // knows nothing about. Rejecting them would make exports one-way.
        const future = { blocks: [{ type: "custom:qrcode", payload: { v: 1 } }], settings: {} }
        const result = parseTemplaticalDocument(JSON.stringify(future))

        expect(result.ok).toBe(true)
        expect(result.ok && result.doc).toEqual(future)
    })

    it("rejects empty input", () => {
        expect(parseTemplaticalDocument("   ")).toEqual({ ok: false, error: "Nothing to import." })
    })

    it("rejects malformed JSON with the parser's reason", () => {
        const result = parseTemplaticalDocument("{ nope")

        expect(result.ok).toBe(false)
        expect(result.ok === false && result.error).toMatch(/^Not valid JSON: /)
    })

    it("rejects JSON that is not a template document", () => {
        for (const input of [
            '"a string"',
            "[]",
            "null",
            JSON.stringify({ blocks: [] }),
            JSON.stringify({ settings: {} }),
            JSON.stringify({ blocks: {}, settings: {} }),
            JSON.stringify({ blocks: [], settings: [] }),
        ]) {
            const result = parseTemplaticalDocument(input)
            expect(result.ok, `expected ${input} to be rejected`).toBe(false)
        }
    })
})

describe("serializeTemplaticalDocument", () => {
    it("round-trips through the parser", () => {
        const result = parseTemplaticalDocument(serializeTemplaticalDocument(doc))

        expect(result.ok && result.doc).toEqual(doc)
    })
})

describe("exportFileName", () => {
    it("slugifies the campaign and locale", () => {
        expect(exportFileName("Welcome Email", "de-DE")).toBe("welcome-email-de-de.json")
    })

    it("collapses punctuation without leaving stray separators", () => {
        expect(exportFileName("Spring / Summer '26!", "en")).toBe("spring-summer-26-en.json")
    })

    it("falls back when there is nothing usable to name it after", () => {
        expect(exportFileName("", "")).toBe("email-template.json")
        expect(exportFileName("!!!", "")).toBe("email-template.json")
    })
})
