import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import Broadcasts from "./Broadcasts"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))
vi.mock("i18next", () => ({ t: translate }))

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

vi.mock("@/api", () => ({
    default: { campaigns: { search: vi.fn() }, lists: { search: vi.fn() } },
}))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en", has_provider: true } as Project

const broadcast = (id: string, name: string) => ({
    id,
    campaign_id: "c1",
    list_id: "l1",
    list_name: "Newsletter",
    state: "completed",
    total: 1,
    sent: 1,
    created_at: "2026-10-01T10:00:00Z",
    campaign: { id: "c1", name, channel: "email" },
})

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

function renderList() {
    const router = createMemoryRouter(
        [{ path: "/projects/p1/broadcasts", element: <Broadcasts /> }],
        { initialEntries: ["/projects/p1/broadcasts"] },
    )
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <RouterProvider router={router} />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("broadcasts list", () => {
    beforeAll(() => {
        // Show a failed request right away instead of retrying it first.
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the list failed to load instead of showing no broadcasts", async () => {
        vi.mocked(oapiClient.GET)
            .mockResolvedValueOnce(failed("database unavailable"))
            .mockResolvedValue(ok({ results: [broadcast("b1", "Weekly digest")], total: 1 }))

        renderList()

        expect(await screen.findByText("Couldn't load broadcasts.")).toBeTruthy()
        expect(screen.queryByText("No broadcasts yet")).toBeNull()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("Weekly digest")).toBeTruthy()
    })

    it("keeps the current page when an older response arrives late", async () => {
        const page1 = Array.from({ length: 15 }, (_, i) => broadcast(`a${i}`, `First ${i}`))
        let answerPage2: (() => void) | undefined
        vi.mocked(oapiClient.GET).mockImplementation(((_url: string, init: unknown) => {
            const { offset } = (init as { params: { query: { offset: number } } }).params.query
            if (offset === 0) return Promise.resolve(ok({ results: page1, total: 16 }))
            // Page 2 answers slowly, after the user has already gone back.
            return new Promise((resolve) => {
                answerPage2 = () =>
                    resolve(ok({ results: [broadcast("b0", "Second page")], total: 16 }))
            })
        }) as never)

        renderList()
        await screen.findByText("First 0")
        await userEvent.click(screen.getByRole("button", { name: "next" }))
        await userEvent.click(screen.getByRole("button", { name: "previous" }))
        await waitFor(() => expect(answerPage2).toBeTruthy())

        answerPage2!()
        await new Promise((r) => setTimeout(r, 50))
        expect(screen.getByText("First 0")).toBeTruthy()
        expect(screen.queryByText("Second page")).toBeNull()
    })
})
