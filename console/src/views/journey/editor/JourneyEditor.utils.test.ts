// Unit tests for the entrance data_key defaulting: entrances need a data_key for
// their trigger ("input") event variables to be referenceable downstream as
// `journey.<data_key>.data.*`, so we assign one automatically and backfill any
// legacy entrances that lack one.

import { describe, expect, it } from "vitest"

import { defaultEntranceDataKey, stepsToNodes } from "./JourneyEditor.utils"
import type { JourneyStepMap } from "@/types"

describe("defaultEntranceDataKey", () => {
    it("uses 'entrance' when free", () => {
        expect(defaultEntranceDataKey([])).toBe("entrance")
    })

    it("falls back to a numbered suffix when taken", () => {
        expect(defaultEntranceDataKey(["entrance"])).toBe("entrance_2")
        expect(defaultEntranceDataKey(["entrance", "entrance_2"])).toBe("entrance_3")
    })

    it("ignores gaps and unrelated keys", () => {
        expect(defaultEntranceDataKey(["entrance", "purchase", "entrance_3"])).toBe("entrance_2")
    })
})

describe("stepsToNodes entrance data_key backfill", () => {
    it("assigns a default data_key to entrances that lack one", () => {
        const steps: JourneyStepMap = {
            a: { type: "entrance", name: "Entry", x: 0, y: 0 },
            b: { type: "gate", name: "Gate", x: 0, y: 100 },
        }
        const { nodes } = stepsToNodes(steps, {})
        const entrance = nodes.find((n) => n.id === "a")
        const gate = nodes.find((n) => n.id === "b")
        expect(entrance?.data.data_key).toBe("entrance")
        // Non-entrance steps are left untouched.
        expect(gate?.data.data_key).toBeUndefined()
    })

    it("preserves an existing data_key and de-duplicates against it", () => {
        const steps: JourneyStepMap = {
            a: { type: "entrance", name: "Entry 1", data_key: "entrance", x: 0, y: 0 },
            b: { type: "entrance", name: "Entry 2", x: 200, y: 0 },
        }
        const { nodes } = stepsToNodes(steps, {})
        expect(nodes.find((n) => n.id === "a")?.data.data_key).toBe("entrance")
        expect(nodes.find((n) => n.id === "b")?.data.data_key).toBe("entrance_2")
    })
})
