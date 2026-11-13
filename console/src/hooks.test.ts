import { act, renderHook, waitFor } from "@testing-library/react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { useResolver } from "./hooks"

// deferred returns a promise together with the functions that settle it.
function deferred<T>() {
    let resolve!: (value: T) => void
    let reject!: (err: unknown) => void
    const promise = new Promise<T>((res, rej) => {
        resolve = res
        reject = rej
    })
    return { promise, resolve, reject }
}

describe("useResolver", () => {
    afterEach(() => {
        vi.restoreAllMocks()
    })

    it("ignores a response that arrives after a newer request", async () => {
        const slow = deferred<string>()
        const fast = deferred<string>()
        const { result, rerender } = renderHook(({ resolver }) => useResolver(resolver), {
            initialProps: { resolver: () => slow.promise },
        })

        // New params: a second request starts while the first is in flight.
        rerender({ resolver: () => fast.promise })
        await act(async () => fast.resolve("page 2"))
        await waitFor(() => expect(result.current[0]).toBe("page 2"))

        // The superseded request settles last and must not win.
        await act(async () => slow.resolve("page 1"))
        expect(result.current[0]).toBe("page 2")
        expect(result.current[3]).toBe(false)
    })

    it("ignores a failure of a superseded request", async () => {
        vi.spyOn(console, "error").mockImplementation(() => {})
        const slow = deferred<string>()
        const { result, rerender } = renderHook(({ resolver }) => useResolver(resolver), {
            initialProps: { resolver: () => slow.promise },
        })
        rerender({ resolver: () => Promise.resolve("fresh") })
        await waitFor(() => expect(result.current[0]).toBe("fresh"))

        await act(async () => slow.reject(new Error("timeout")))
        expect(result.current[4]).toBeUndefined()
    })

    it("exposes the error of a failed request and clears it on a successful reload", async () => {
        vi.spyOn(console, "error").mockImplementation(() => {})
        const failure = new Error("database unavailable")
        const resolver = vi.fn().mockRejectedValueOnce(failure).mockResolvedValue("loaded")
        const { result } = renderHook(() => useResolver(resolver))

        await waitFor(() => expect(result.current[4]).toBe(failure))
        expect(result.current[0]).toBeNull()
        expect(result.current[3]).toBe(false)

        await act(async () => result.current[2]())
        expect(result.current[0]).toBe("loaded")
        expect(result.current[4]).toBeUndefined()
    })

    it("keeps reload from rejecting when the request fails", async () => {
        vi.spyOn(console, "error").mockImplementation(() => {})
        const resolver = vi.fn().mockRejectedValue(new Error("nope"))
        const { result } = renderHook(() => useResolver(resolver))
        await waitFor(() => expect(result.current[3]).toBe(false))

        await expect(result.current[2]()).resolves.toBeUndefined()
    })
})
