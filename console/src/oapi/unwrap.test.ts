import { describe, expect, it } from "vitest"
import { problemDetail } from "@/utils"
import { ApiError, unwrap } from "./unwrap"

describe("unwrap", () => {
    it("returns the data of a successful request", () => {
        expect(unwrap({ data: { id: 1 } })).toEqual({ id: 1 })
    })

    it("returns undefined for a successful request without a body", () => {
        expect(unwrap({})).toBeUndefined()
    })

    it("throws an ApiError carrying the server's detail", () => {
        const problem = { title: "Bad Request", detail: "name is required" }
        let thrown: unknown
        try {
            unwrap({ error: problem })
        } catch (err) {
            thrown = err
        }

        expect(thrown).toBeInstanceOf(ApiError)
        expect((thrown as ApiError).problem).toBe(problem)
        expect(problemDetail(thrown)).toBe("name is required")
    })

    it("throws without a detail when the server gave none", () => {
        expect(() => unwrap({ error: "Internal Server Error" })).toThrow("Request failed")
        expect(problemDetail(new ApiError(null))).toBeNull()
    })
})
