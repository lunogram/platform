import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext, UserContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project, User } from "@/types"
import UserDetailEvents from "./UserDetailEvents"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))
vi.mock("i18next", () => ({ t: translate }))

const toastError = vi.hoisted(() => vi.fn())
const toastSuccess = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError, success: toastSuccess } }))

vi.mock("@/api", () => ({ default: { users: { events: vi.fn() } } }))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en" } as Project
const user = { id: "u1", identifier: [], data: {} } as unknown as User
const path = "/projects/p1/users/u1/events"

const event = { id: "e1", name: "signed_up", data: {}, created_at: "2026-10-01T10:00:00Z" }

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

function renderEvents() {
    const router = createMemoryRouter([{ path, element: <UserDetailEvents /> }], {
        initialEntries: [path],
    })
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <UserContext.Provider value={[user, () => {}]}>
                    <RouterProvider router={router} />
                </UserContext.Provider>
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("user events", () => {
    beforeAll(() => {
        // Radix popovers use pointer and scroll APIs that jsdom lacks.
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        globalThis.ResizeObserver ??= class {
            observe() {}
            unobserve() {}
            disconnect() {}
        }
        // Show a failed request right away instead of retrying it first.
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        vi.mocked(api.users.events).mockResolvedValue({ results: [event], total: 1 } as never)
        // The event name suggestions.
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [] }))
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the events failed to load and loads them on retry", async () => {
        vi.mocked(api.users.events).mockRejectedValueOnce(new Error("timeout"))

        renderEvents()

        expect(await screen.findByText("Couldn't load events.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("signed_up")).toBeTruthy()
    })

    it("reports a rejected event instead of saying it was created", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("event name is reserved"))

        renderEvents()
        await screen.findByText("signed_up")
        await userEvent.click(screen.getByRole("button", { name: "create" }))
        const dialog = await screen.findByRole("dialog")
        await userEvent.type(
            within(dialog).getByPlaceholderText("Type or select an event name..."),
            "purchase",
        )
        await userEvent.click(within(dialog).getByText("Data"))
        await userEvent.click(within(dialog).getByRole("button", { name: "create" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("event name is reserved"))
        expect(oapiClient.POST).toHaveBeenCalledTimes(1)
        expect(toastSuccess).not.toHaveBeenCalled()
        // The dialog stays open so the event can be fixed and sent again.
        expect(screen.getByRole("dialog")).toBeTruthy()
    })

    it("closes the dialog after the event was created", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(ok({}))

        renderEvents()
        await screen.findByText("signed_up")
        await userEvent.click(screen.getByRole("button", { name: "create" }))
        const dialog = await screen.findByRole("dialog")
        await userEvent.type(
            within(dialog).getByPlaceholderText("Type or select an event name..."),
            "purchase",
        )
        await userEvent.click(within(dialog).getByText("Data"))
        await userEvent.click(within(dialog).getByRole("button", { name: "create" }))

        await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Event created"))
        expect(toastError).not.toHaveBeenCalled()
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
    })
})
