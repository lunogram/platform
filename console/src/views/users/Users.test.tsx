import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AxiosError } from "axios"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import type { Project } from "@/types"
import Users from "./Users"

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
    default: { users: { search: vi.fn(), create: vi.fn(), addImport: vi.fn() } },
}))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en" } as Project
const path = "/projects/:projectId/users"

const user = {
    id: "u1",
    identifier: [{ source: "default", external_id: "usr_1" }],
    email: "ada@example.com",
    data: {},
    created_at: "2026-10-01T10:00:00Z",
}

// An axios error the way the API client throws it.
function apiError(detail: string) {
    const err = new AxiosError("Request failed with status code 422")
    err.response = { data: { detail } } as AxiosError["response"]
    return err
}

function renderUsers() {
    const router = createMemoryRouter([{ path, element: <Users /> }], {
        initialEntries: ["/projects/p1/users"],
    })
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <RouterProvider router={router} />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("users list", () => {
    beforeAll(() => {
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
        vi.mocked(api.users.search).mockResolvedValue({ results: [user], total: 1 } as never)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the users failed to load and loads them on retry", async () => {
        vi.mocked(api.users.search).mockRejectedValueOnce(new Error("Network Error"))

        renderUsers()

        expect(await screen.findByText("Couldn't load users.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect((await screen.findAllByText("ada@example.com")).length).toBeGreaterThan(0)
    })

    it("reports a failed create and keeps the dialog open", async () => {
        vi.mocked(api.users.create).mockRejectedValue(apiError("identifier already in use"))

        renderUsers()
        await screen.findAllByText("ada@example.com")
        await userEvent.click(screen.getAllByRole("button", { name: "create_user" })[0])
        const dialog = await screen.findByRole("dialog")
        await userEvent.type(within(dialog).getByLabelText("Identifier"), "usr_1")
        await userEvent.click(within(dialog).getByRole("button", { name: "create" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("identifier already in use"))
        expect(screen.getByRole("dialog")).toBeTruthy()
    })
})
