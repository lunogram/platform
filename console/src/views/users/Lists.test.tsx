import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AxiosError } from "axios"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import Lists from "./Lists"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))
vi.mock("i18next", () => ({ t: translate }))

const toastError = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }))

vi.mock("@/api", () => ({
    default: { lists: { search: vi.fn(), delete: vi.fn(), duplicate: vi.fn() } },
}))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en" } as Project
const listPath = "/projects/p1/lists"

const list = {
    id: "l1",
    name: "VIPs",
    type: "dynamic",
    state: "ready",
    users_count: 3,
    created_at: "2026-10-01T10:00:00Z",
    updated_at: "2026-10-01T10:00:00Z",
}

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

function apiError(detail: string) {
    const err = new AxiosError("Request failed with status code 409")
    err.response = { data: { detail } } as AxiosError["response"]
    return err
}

function renderLists() {
    const router = createMemoryRouter(
        [
            { path: "/projects/:projectId/lists", element: <Lists /> },
            { path: "/projects/:projectId/lists/:listId", element: <p>list page</p> },
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

async function clickRowAction(name: string, action: string) {
    const row = screen.getByText(name).closest("tr")!
    await userEvent.click(within(row).getByRole("button"))
    await userEvent.click(await screen.findByRole("menuitem", { name: action }))
}

describe("lists overview", () => {
    beforeAll(() => {
        // Radix menus use pointer and scroll APIs that jsdom lacks.
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        // Show a failed request right away instead of retrying it first.
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        vi.mocked(api.lists.search).mockResolvedValue({ results: [list], total: 1 } as never)
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [list], total: 1 }))
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the lists failed to load and loads them on retry", async () => {
        vi.mocked(api.lists.search).mockRejectedValueOnce(new Error("Network Error"))
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("database unavailable"))

        renderLists()

        expect(await screen.findByText("Couldn't load lists.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("VIPs")).toBeTruthy()
    })

    it("reports a failed archive and keeps the list", async () => {
        vi.mocked(api.lists.delete).mockRejectedValue(apiError("list is used by a journey"))

        renderLists()
        await screen.findByText("VIPs")
        await clickRowAction("VIPs", "archive")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("list is used by a journey"))
        expect(screen.getByText("VIPs")).toBeTruthy()
    })

    it("reports a failed duplicate and stays on the overview", async () => {
        vi.mocked(api.lists.duplicate).mockRejectedValue(apiError("quota reached"))

        const router = renderLists()
        await screen.findByText("VIPs")
        await clickRowAction("VIPs", "duplicate")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("quota reached"))
        expect(router.state.location.pathname).toBe(listPath)
    })
})
