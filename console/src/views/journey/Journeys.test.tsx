import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Journey, Project, SearchResult } from "@/types"
import Journeys from "./Journeys"

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
    default: {
        journeys: { search: vi.fn(), create: vi.fn(), duplicate: vi.fn(), delete: vi.fn() },
    },
}))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en" } as Project
const listPath = "/projects/p1/journeys"

const journey = {
    id: "j1",
    name: "Welcome flow",
    status: "draft",
    created_at: "2026-10-01T10:00:00Z",
    updated_at: "2026-10-01T10:00:00Z",
} as Journey

const page = (results: Journey[]) =>
    ({ results, total: results.length, nextCursor: "", limit: 15 }) as SearchResult<Journey>

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

function renderList() {
    const router = createMemoryRouter(
        [
            { path: listPath, element: <Journeys /> },
            { path: `${listPath}/:journeyId`, element: <p>journey editor</p> },
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

// Opens the actions menu of the journey's row and clicks one of its items.
async function clickRowAction(name: string, action: string) {
    const row = screen.getByText(name).closest("tr")!
    const buttons = within(row).getAllByRole("button")
    await userEvent.click(buttons[buttons.length - 1])
    await userEvent.click(await screen.findByRole("menuitem", { name: action }))
}

describe("journeys list", () => {
    beforeAll(() => {
        // Radix menus and dialogs use pointer and scroll APIs that jsdom lacks.
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        vi.mocked(api.journeys.search).mockResolvedValue(page([journey]))
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the list failed to load and loads it on retry", async () => {
        vi.mocked(api.journeys.search).mockRejectedValueOnce(new Error("database unavailable"))

        renderList()

        expect(await screen.findByText("Couldn't load journeys.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("Welcome flow")).toBeTruthy()
    })

    it("says the archived list failed to load instead of loading forever", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValue(failed("database unavailable"))

        renderList()
        await screen.findByText("Welcome flow")
        await userEvent.click(screen.getByRole("button", { name: "Show archived" }))

        expect(await screen.findByText("Couldn't load journeys.")).toBeTruthy()
    })

    it("reports a failed archive and keeps the journey listed", async () => {
        vi.mocked(api.journeys.delete).mockRejectedValue(new Error("in use"))

        renderList()
        await screen.findByText("Welcome flow")
        await clickRowAction("Welcome flow", "archive")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("Couldn't archive the journey"))
        expect(screen.getByText("Welcome flow")).toBeTruthy()
    })

    it("reports a failed unarchive and stays on the archived list", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [journey], total: 1 }))
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("journey was deleted"))

        renderList()
        await screen.findByText("Welcome flow")
        await userEvent.click(screen.getByRole("button", { name: "Show archived" }))
        await waitFor(() => expect(oapiClient.GET).toHaveBeenCalled())
        await clickRowAction("Welcome flow", "Unarchive")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("journey was deleted"))
        expect(
            screen.getByRole("button", { name: "Show archived" }).getAttribute("aria-pressed"),
        ).toBe("true")
    })

    it("opens the copy after a successful duplicate", async () => {
        vi.mocked(api.journeys.duplicate).mockResolvedValue({ id: "j2" } as never)

        const router = renderList()
        await screen.findByText("Welcome flow")
        await clickRowAction("Welcome flow", "duplicate")

        await waitFor(() => expect(router.state.location.pathname).toBe(`${listPath}/j2`))
        expect(toastError).not.toHaveBeenCalled()
    })

    it("reports a failed create and keeps the dialog open", async () => {
        vi.mocked(api.journeys.create).mockRejectedValue(new Error("quota reached"))

        const router = renderList()
        await userEvent.click(screen.getByRole("button", { name: "Create Journey from header" }))
        const dialog = await screen.findByRole("dialog")
        await userEvent.type(within(dialog).getByLabelText(/name/), "Onboarding")
        await userEvent.click(within(dialog).getByRole("button", { name: "save" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("Couldn't create the journey"))
        expect(screen.getByRole("dialog")).toBeTruthy()
        expect(router.state.location.pathname).toBe(listPath)
    })
})
