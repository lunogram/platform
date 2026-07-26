import { describe, expect, it } from "vitest"
import { Liquid } from "liquidjs"
import { toDisplayConditions } from "./displayConditions"

const liquid = new Liquid({ strictVariables: false, strictFilters: false })

describe("toDisplayConditions", () => {
    it("offers a true/not-true pair for each boolean variable", () => {
        const conditions = toDisplayConditions([
            {
                label: "User",
                variables: [{ path: "user.data.isPro", label: "data.isPro", types: ["boolean"] }],
            },
        ])

        expect(conditions).toEqual([
            {
                label: "data.isPro is true",
                before: "{% if user.data.isPro %}",
                after: "{% endif %}",
                group: "User",
                description: "Only shown when user.data.isPro is true",
            },
            {
                label: "data.isPro is not true",
                before: "{% unless user.data.isPro %}",
                after: "{% endunless %}",
                group: "User",
                description: "Only shown when user.data.isPro is false or not set",
            },
        ])
    })

    it("skips variables that are not booleans", () => {
        const conditions = toDisplayConditions([
            {
                label: "User",
                variables: [
                    { path: "user.email", label: "Email", types: ["string"] },
                    { path: "user.data", label: "Data", types: ["object"] },
                    { path: "user.created_at", label: "Created", types: ["date"] },
                    { path: "user.untyped", label: "Untyped" },
                ],
            },
        ])

        expect(conditions).toEqual([])
    })

    it("skips a path the backend has seen hold more than one type", () => {
        const conditions = toDisplayConditions([
            {
                label: "User",
                variables: [
                    { path: "user.data.flaky", label: "flaky", types: ["boolean", "string"] },
                ],
            },
        ])

        expect(conditions).toEqual([])
    })

    it("groups conditions by their variable group", () => {
        const conditions = toDisplayConditions([
            {
                label: "User",
                variables: [{ path: "user.data.admin", label: "data.admin", types: ["boolean"] }],
            },
            {
                label: "Campaign",
                variables: [{ path: "campaign.beta", label: "beta", types: ["boolean"] }],
            },
        ])

        expect([...new Set(conditions.map((c) => c.group))]).toEqual(["User", "Campaign"])
        expect(conditions).toHaveLength(4)
    })

    // The guards are only useful if the engine the send pipeline runs accepts
    // them, so exercise them as real templates rather than asserting on strings.
    describe("the emitted Liquid", () => {
        const [isTrue, isNotTrue] = toDisplayConditions([
            {
                label: "User",
                variables: [{ path: "user.data.isPro", label: "data.isPro", types: ["boolean"] }],
            },
        ])

        const render = (condition: { before: string; after: string }, context: object) =>
            liquid.parseAndRenderSync(`${condition.before}BLOCK${condition.after}`, context)

        it("shows the block to a matching recipient and hides it from the rest", () => {
            expect(render(isTrue, { user: { data: { isPro: true } } })).toBe("BLOCK")
            expect(render(isTrue, { user: { data: { isPro: false } } })).toBe("")
            expect(render(isTrue, { user: { data: {} } })).toBe("")
        })

        it("treats an absent attribute the same as an explicit false", () => {
            expect(render(isNotTrue, { user: { data: { isPro: false } } })).toBe("BLOCK")
            expect(render(isNotTrue, { user: { data: {} } })).toBe("BLOCK")
            expect(render(isNotTrue, { user: { data: { isPro: true } } })).toBe("")
        })

        it("is exhaustive — every recipient matches exactly one of the pair", () => {
            for (const context of [
                { user: { data: { isPro: true } } },
                { user: { data: { isPro: false } } },
                { user: { data: {} } },
            ]) {
                const shown = [isTrue, isNotTrue].filter((c) => render(c, context) === "BLOCK")
                expect(shown).toHaveLength(1)
            }
        })
    })
})
