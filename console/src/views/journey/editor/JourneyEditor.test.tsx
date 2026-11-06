import { QueryClientProvider } from "@tanstack/react-query"
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import type { Journey, JourneyStepMap, Project } from "@/types"
import JourneyRoute from "../JourneyRoute"
import { journeyQuery } from "../queries"
import JourneyEditor from "./JourneyEditor"

// These tests render the real journey route and editor; only the network is mocked.

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
    Trans: ({ children }: { children: unknown }) => children,
}))
vi.mock("i18next", () => ({ t: translate }))

const toastError = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }))

vi.mock("@/api", () => ({
    default: {
        journeys: {
            get: vi.fn(),
            update: vi.fn(),
            publish: vi.fn(),
            steps: { get: vi.fn(), set: vi.fn() },
            users: { getState: vi.fn(async () => []) },
        },
    },
}))

vi.mock("@/oapi/client", () => {
    const client = {
        GET: vi.fn(async () => ({ data: { results: [], total: 0 } })),
        POST: vi.fn(async () => ({ data: {} })),
        use: vi.fn(),
    }
    return { default: client, oapiClient: client }
})

vi.mock("@/lib/path-suggestions", () => ({
    fetchPathSuggestions: vi.fn(async () => ({ userPaths: [], eventPaths: [] })),
}))

const project = { id: "p1", name: "Acme", locale: "en" } as Project

const journey = (id: string, name: string) =>
    ({ id, name, status: "draft", stats: {} }) as unknown as Journey

const journeys: Record<string, Journey> = {
    a: journey("a", "Welcome flow"),
    b: journey("b", "Win-back flow"),
}

// Unconnected balancer steps, keyed by their name.
const stepsOf = (...names: string[]) =>
    Object.fromEntries(
        names.map((name) => [name, { type: "balancer", name, data: {}, x: 0, y: 0 }]),
    ) as unknown as JourneyStepMap

const steps: Record<string, JourneyStepMap> = {
    a: stepsOf("Split A"),
    b: stepsOf("Split B", "Holdout B"),
}

function renderAt(journeyId: string) {
    const router = createMemoryRouter(
        [
            {
                path: "/projects/:projectId/journeys/:entityId",
                loader: async ({ params }) => {
                    await queryClient.ensureQueryData(
                        journeyQuery(params.projectId as string, params.entityId as string),
                    )
                    return null
                },
                element: <JourneyRoute />,
                children: [{ index: true, element: <JourneyEditor /> }],
            },
        ],
        { initialEntries: [`/projects/p1/journeys/${journeyId}`] },
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

// Selects the step on the canvas and deletes it, which leaves unsaved changes.
async function deleteStep(name: string) {
    const node = (await screen.findByText(name)).closest(".react-flow__node") as HTMLElement
    // A plain click: d3-drag can't handle the mousedown user-event sends in jsdom.
    fireEvent.click(node)
    fireEvent.keyDown(document.body, { key: "Delete", code: "Delete" })
    await screen.findByText("journey_draft_save")
}

describe("journey editor", () => {
    beforeAll(() => {
        // React Flow and Radix use browser APIs that jsdom lacks.
        globalThis.ResizeObserver = class {
            observe() {}
            unobserve() {}
            disconnect() {}
        }
        window.matchMedia = ((query: string) => ({
            matches: false,
            media: query,
            addEventListener: () => {},
            removeEventListener: () => {},
        })) as unknown as typeof window.matchMedia
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        vi.mocked(api.journeys.get).mockImplementation(async (_, id) => journeys[id])
        vi.mocked(api.journeys.steps.get).mockImplementation(async (_, id) => steps[id])
        vi.mocked(api.journeys.steps.set).mockImplementation(async (_, __, map) => map)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("shows the other journey's steps after following a link to it", async () => {
        const router = renderAt("a")
        expect(await screen.findByText("Split A")).toBeTruthy()

        await act(() => router.navigate("/projects/p1/journeys/b"))

        expect(await screen.findByText("Win-back flow")).toBeTruthy()
        expect(await screen.findByText("Split B")).toBeTruthy()
        expect(screen.queryByText("Split A")).toBeNull()
        expect(api.journeys.steps.get).toHaveBeenCalledWith("p1", "b")
    })

    it("saves the other journey's own steps after following a link to it", async () => {
        const router = renderAt("a")
        await screen.findByText("Split A")
        await act(() => router.navigate("/projects/p1/journeys/b"))
        await screen.findByText("Split B")

        await deleteStep("Holdout B")
        await userEvent.click(screen.getByText("journey_draft_save"))

        await waitFor(() => expect(api.journeys.steps.set).toHaveBeenCalledTimes(1))
        // B's canvas, minus the deleted step, is saved to B; nothing of A's.
        const [, journeyId, saved] = vi.mocked(api.journeys.steps.set).mock.calls[0]
        expect(journeyId).toBe("b")
        expect(Object.keys(saved)).toEqual(["Split B"])
    })

    it("reports a failed save and keeps the unsaved changes", async () => {
        vi.mocked(api.journeys.steps.set).mockRejectedValue(new Error("network down"))

        renderAt("a")
        await deleteStep("Split A")
        await userEvent.click(screen.getByText("journey_draft_save"))

        await waitFor(() => expect(toastError).toHaveBeenCalled())
        expect(screen.getByText("journey_draft_save")).toBeTruthy()
    })

    it("reports a failed rename and keeps the saved name", async () => {
        vi.mocked(api.journeys.update).mockRejectedValue(new Error("network down"))

        renderAt("a")
        await screen.findByText("Split A")
        await userEvent.click(screen.getByRole("button", { name: "Edit value" }))
        const input = await screen.findByDisplayValue("Welcome flow")
        await userEvent.clear(input)
        await userEvent.type(input, "Renamed{Enter}")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("Couldn't save the journey"))
        expect(screen.getByRole("heading", { name: "Welcome flow" })).toBeTruthy()
    })

    it("says the steps failed to load and loads them on retry", async () => {
        vi.mocked(api.journeys.steps.get).mockRejectedValueOnce(new Error("timeout"))

        renderAt("a")

        expect(await screen.findByText("Failed to load journey steps.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("Split A")).toBeTruthy()
    })
})
