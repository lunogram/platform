import { QueryClient } from "@tanstack/react-query"
import { afterEach, describe, expect, it, vi } from "vitest"
import { oapiClient } from "@/oapi/client"
import { providerQuery, providersQuery } from "./queries"

vi.mock("@/api", () => ({ default: {} }))
vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

type RequestInit = {
    params?: { path?: { providerID?: string }; query?: { limit?: number; offset?: number } }
}

const providers = Array.from({ length: 150 }, (_, i) => ({
    id: `prov-${i + 1}`,
    name: `Provider ${i + 1}`,
    module: "resend",
}))

// Answers like the API: the list returns one page, the get returns one provider.
function serve(url: string, init?: RequestInit) {
    if (url.endsWith("/providers")) {
        const limit = init?.params?.query?.limit ?? 20
        const offset = init?.params?.query?.offset ?? 0
        return { data: { results: providers.slice(offset, offset + limit), total: 150 } }
    }
    const id = init?.params?.path?.providerID
    const found = providers.find((p) => p.id === id)
    return found ? { data: { ...found, data: { full: true } } } : { error: { detail: "not found" } }
}

describe("providers queries", () => {
    afterEach(() => {
        vi.clearAllMocks()
    })

    function client() {
        vi.mocked(oapiClient.GET).mockImplementation(((url: string, init?: RequestInit) =>
            Promise.resolve(serve(url, init))) as unknown as typeof oapiClient.GET)
        return new QueryClient({ defaultOptions: { queries: { retry: false } } })
    }

    it("pages through every provider", async () => {
        const queryClient = client()

        const all = await queryClient.fetchQuery(providersQuery("p1"))

        expect(all).toHaveLength(150)
    })

    it("finds a provider beyond the first hundred", async () => {
        const queryClient = client()

        const provider = await queryClient.fetchQuery(providerQuery(queryClient, "p1", "prov-120"))

        expect(provider?.name).toBe("Provider 120")
        expect(provider?.data).toEqual({ full: true })
    })

    it("answers null for a provider the project doesn't have", async () => {
        const queryClient = client()

        expect(await queryClient.fetchQuery(providerQuery(queryClient, "p1", "nope"))).toBeNull()
    })
})
