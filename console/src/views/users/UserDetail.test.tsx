import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AxiosError } from "axios"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import type { Project } from "@/types"
import type { UUID } from "@/types/common"
import UserRoute from "./UserRoute"
import { userQuery } from "./queries"

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

vi.mock("@/api", () => ({
    default: { users: { get: vi.fn(), update: vi.fn(), delete: vi.fn() } },
}))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en" } as Project

const user = {
    id: "u1",
    identifier: [{ source: "default", external_id: "usr_1" }],
    full_name: "Ada Lovelace",
    email: "ada@example.com",
    data: {},
    created_at: "2026-10-01T10:00:00Z",
}

function apiError(detail: string) {
    const err = new AxiosError("Request failed with status code 422")
    err.response = { data: { detail } } as AxiosError["response"]
    return err
}

function renderUser() {
    const router = createMemoryRouter(
        [
            {
                path: "/projects/:projectId/users/:entityId",
                loader: async ({ params }) => {
                    await queryClient.ensureQueryData(
                        userQuery(params.projectId as UUID, params.entityId as UUID),
                    )
                    return null
                },
                element: <UserRoute />,
                children: [{ index: true, element: <p>details tab</p> }],
            },
            { path: "/projects/:projectId/users", element: <p>users page</p> },
        ],
        { initialEntries: ["/projects/p1/users/u1"] },
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

async function saveEmail(email: string) {
    await userEvent.click((await screen.findAllByRole("button", { name: "Edit value" }))[0])
    const input = await screen.findByDisplayValue("ada@example.com")
    await userEvent.clear(input)
    await userEvent.type(input, email)
    await userEvent.click(screen.getByRole("button", { name: "save" }))
}

describe("user page", () => {
    beforeAll(() => {
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        globalThis.ResizeObserver ??= class {
            observe() {}
            unobserve() {}
            disconnect() {}
        }
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        vi.mocked(api.users.get).mockResolvedValue(user as never)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("shows a saved email everywhere the user is read from", async () => {
        vi.mocked(api.users.update).mockResolvedValue({
            ...user,
            email: "ada@lovelace.dev",
        } as never)

        renderUser()
        await saveEmail("ada@lovelace.dev")

        expect(await screen.findByText("ada@lovelace.dev")).toBeTruthy()
        expect(toastSuccess).toHaveBeenCalledWith("Email updated")
        // The cached user, which every tab reads, is the saved one.
        expect(queryClient.getQueryData(userQuery("p1" as UUID, "u1" as UUID).queryKey)).toEqual(
            expect.objectContaining({ email: "ada@lovelace.dev" }),
        )
    })

    it("reports a failed email save and keeps the old email", async () => {
        vi.mocked(api.users.update).mockRejectedValue(apiError("email is taken"))

        renderUser()
        await saveEmail("taken@example.com")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("email is taken"))
        expect(toastSuccess).not.toHaveBeenCalled()
        expect(screen.getByText("ada@example.com")).toBeTruthy()
    })

    it("reports a failed delete and keeps the dialog open", async () => {
        vi.mocked(api.users.delete).mockRejectedValue(apiError("user has running journeys"))

        const router = renderUser()
        await screen.findByText("Ada Lovelace", { selector: "h1" })
        const menuButtons = screen.getAllByRole("button", { name: "" })
        await userEvent.click(menuButtons[menuButtons.length - 1])
        await userEvent.click(await screen.findByRole("menuitem", { name: "delete" }))
        await userEvent.click(await screen.findByRole("button", { name: "delete_user" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("user has running journeys"))
        expect(screen.getByRole("dialog")).toBeTruthy()
        expect(router.state.location.pathname).toBe("/projects/p1/users/u1")
    })
})
