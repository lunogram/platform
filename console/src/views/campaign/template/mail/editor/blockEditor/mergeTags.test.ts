import { describe, expect, it } from "vitest"
import { toMergeTags } from "./mergeTags"

describe("toMergeTags", () => {
    it("maps a variable path to the liquid expression the send pipeline resolves", () => {
        const tags = toMergeTags([
            {
                label: "User",
                variables: [{ path: "user.first_name", label: "First name", types: ["string"] }],
            },
        ])

        expect(tags).toEqual([
            {
                label: "First name",
                value: "{{ user.first_name }}",
                group: "User",
                description: undefined,
            },
        ])
    })

    it("groups tags by their variable group and keeps descriptions", () => {
        const tags = toMergeTags([
            {
                label: "User",
                variables: [{ path: "user.email", label: "Email", description: "Primary" }],
            },
            {
                label: "Links",
                variables: [{ path: "unsubscribe_url", label: "Unsubscribe" }],
            },
        ])

        expect(tags.map((tag) => [tag.group, tag.value])).toEqual([
            ["User", "{{ user.email }}"],
            ["Links", "{{ unsubscribe_url }}"],
        ])
        expect(tags[0].description).toBe("Primary")
    })

    it("drops object and array variables, which have no scalar rendering", () => {
        const tags = toMergeTags([
            {
                label: "User",
                variables: [
                    { path: "user.data", label: "Data", types: ["object"] },
                    { path: "user.tags", label: "Tags", types: ["array"] },
                    { path: "user.id", label: "ID", types: ["string"] },
                ],
            },
        ])

        expect(tags.map((tag) => tag.value)).toEqual(["{{ user.id }}"])
    })

    it("keeps variables whose type is unknown or partly scalar", () => {
        const tags = toMergeTags([
            {
                label: "Event",
                variables: [
                    { path: "event.count", label: "Count" },
                    { path: "event.mixed", label: "Mixed", types: ["object", "string"] },
                ],
            },
        ])

        expect(tags).toHaveLength(2)
    })
})
