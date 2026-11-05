import { afterEach, describe, expect, it, vi } from "vitest"
import type { Template } from "@/types"
import {
    defaultTemplateData,
    knownTemplateData,
    unknownTemplateDataKeys,
    warnUnknownTemplateData,
} from "./data"

describe("template data", () => {
    afterEach(() => {
        vi.restoreAllMocks()
    })

    it("lists keys an older console wrote that the schema no longer knows", () => {
        const data = { subject: "Hi", replyTo: "a@b.c", editor: "visual", editorMode: "code" }
        expect(unknownTemplateDataKeys("email", data)).toEqual(["replyTo", "editor"])
    })

    it("keeps only known keys when building a save payload", () => {
        const data = { title: "Hi", body: "There", custom: { a: 1 }, url: "https://x" }
        expect(knownTemplateData("push", data)).toEqual({ title: "Hi", body: "There" })
    })

    it("keeps the values of known keys untouched", () => {
        const code = { source: "<Html />", bundle: "compiled" }
        const data = { subject: "Hi", code, plaintext: { custom: "Hi there" } }
        expect(knownTemplateData("email", data)).toEqual(data)
    })

    it("starts new templates with only keys the backend reads", () => {
        expect(defaultTemplateData("push")).toEqual({ title: "", body: "" })
        expect(Object.keys(defaultTemplateData("email"))).toEqual(["from", "subject"])
    })

    it("warns about a loaded template with unknown keys", () => {
        const warn = vi.spyOn(console, "warn").mockImplementation(() => {})
        const template = {
            id: "t1",
            type: "sms",
            data: { body: "Hi", message: "old" },
        } as unknown as Template

        warnUnknownTemplateData(template)

        expect(warn).toHaveBeenCalledWith(expect.stringContaining("message"))
    })

    it("handles a template whose channel the console doesn't know", () => {
        const warn = vi.spyOn(console, "warn").mockImplementation(() => {})
        const template = { id: "t1", type: "fax", data: { page: 1 } } as unknown as Template

        expect(() => warnUnknownTemplateData(template)).not.toThrow()
        expect(warn).toHaveBeenCalledWith(expect.stringContaining("fax"))
        expect(knownTemplateData(template.type, template.data)).toEqual({ page: 1 })
    })

    it("stays quiet for a template with only known keys", () => {
        const warn = vi.spyOn(console, "warn").mockImplementation(() => {})
        const template = { id: "t1", type: "sms", data: { body: "Hi" } } as unknown as Template

        warnUnknownTemplateData(template)

        expect(warn).not.toHaveBeenCalled()
    })
})
