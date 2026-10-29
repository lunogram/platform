import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import Campaigns from "./Campaigns"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))
vi.mock("i18next", () => ({ t: translate }))

const toastError = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en", has_provider: true } as Project
const listPath = "/projects/p1/campaigns"

const campaign = {
    id: "c1",
    name: "Onboarding",
    channel: "email",
    delivery: { sent: 0, total: 0 },
    updated_at: "2026-10-01T10:00:00Z",
}

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

function renderList() {
    const router = createMemoryRouter(
        [
            { path: listPath, element: <Campaigns /> },
            { path: `${listPath}/:campaignId`, element: <p>campaign page</p> },
        ],
        { initialEntries: [listPath] },
    )
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <RouterProvider router={router} />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
    return router
}

// Opens the actions menu of the campaign's row and clicks one of its items.
async function clickRowAction(name: string, action: string) {
    const row = screen.getByText(name).closest("tr")!
    await userEvent.click(within(row).getByRole("button"))
    await userEvent.click(await screen.findByRole("menuitem", { name: action }))
}

describe("campaigns list", () => {
    beforeAll(() => {
        // Radix menus use pointer and scroll APIs that jsdom lacks.
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        // Show a failed request right away instead of retrying it first.
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [campaign], total: 1 }))
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the list failed to load and loads it on retry", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("database unavailable"))

        renderList()

        expect(await screen.findByText("Couldn't load campaigns.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("Onboarding")).toBeTruthy()
    })

    it("reports a failed archive and keeps the campaign listed", async () => {
        vi.mocked(oapiClient.DELETE).mockResolvedValue(failed("campaign is used by a journey"))

        renderList()
        await screen.findByText("Onboarding")
        await clickRowAction("Onboarding", "archive")

        await waitFor(() =>
            expect(toastError).toHaveBeenCalledWith("campaign is used by a journey"),
        )
        expect(screen.getByText("Onboarding")).toBeTruthy()
    })

    it("reports a failed duplicate and stays on the list", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("quota reached"))

        const router = renderList()
        await screen.findByText("Onboarding")
        await clickRowAction("Onboarding", "duplicate")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("quota reached"))
        expect(router.state.location.pathname).toBe(listPath)
    })

    it("opens the copy after a successful duplicate", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(ok({ ...campaign, id: "c2" }))

        const router = renderList()
        await screen.findByText("Onboarding")
        await clickRowAction("Onboarding", "duplicate")

        await waitFor(() => expect(router.state.location.pathname).toBe(`${listPath}/c2`))
        expect(toastError).not.toHaveBeenCalled()
    })
})
