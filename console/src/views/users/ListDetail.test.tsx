import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AxiosError } from "axios"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import type { Project, Rule } from "@/types"
import type { UUID } from "@/types/common"
import ListRoute from "./ListRoute"
import { listQuery } from "./listQueries"

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
        lists: { get: vi.fn(), update: vi.fn(), users: vi.fn(), delete: vi.fn() },
    },
}))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

// Not under test, and it loads data of its own.
vi.mock("@/views/broadcast/CreateBroadcastDialog", () => ({ CreateBroadcastDialog: () => null }))

// The real rule builder is covered by its own tests; here a rule edit is one click.
vi.mock("./rules/RuleBuilder", () => ({
    default: ({ rule, setRule }: { rule: Rule; setRule: (rule: Rule) => void }) => (
        <button type="button" onClick={() => setRule({ ...rule, value: "edited" })}>
            edit rule
        </button>
    ),
}))

const project = { id: "p1", name: "Acme", locale: "en" } as Project

const list = {
    id: "l1",
    name: "VIPs",
    type: "dynamic",
    state: "ready",
    rule: { uuid: "r1", type: "wrapper", group: "parent", path: "", operator: "and" },
    draft_rule: null,
    users_count: 1,
    is_visible: true,
    created_at: "2026-10-01T10:00:00Z",
    updated_at: "2026-10-01T10:00:00Z",
}

const member = { id: "u1", email: "ada@example.com", identifier: [] }

function apiError(detail: string) {
    const err = new AxiosError("Request failed with status code 422")
    err.response = { data: { detail } } as AxiosError["response"]
    return err
}

function renderList() {
    const router = createMemoryRouter(
        [
            {
                path: "/projects/:projectId/lists/:entityId",
                loader: async ({ params }) => {
                    await queryClient.ensureQueryData(
                        listQuery(params.projectId as UUID, params.entityId as UUID),
                    )
                    return null
                },
                element: <ListRoute />,
            },
            { path: "/projects/:projectId/lists", element: <p>lists page</p> },
        ],
        { initialEntries: ["/projects/p1/lists/l1"] },
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

describe("list detail", () => {
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
        vi.mocked(api.lists.get).mockResolvedValue(list as never)
        vi.mocked(api.lists.users).mockResolvedValue({ results: [member], total: 1 } as never)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the users failed to load instead of showing an empty list", async () => {
        vi.mocked(api.lists.users).mockRejectedValueOnce(new Error("Network Error"))

        renderList()

        expect(await screen.findByText("Couldn't load the users of this list.")).toBeTruthy()
        expect(screen.queryByText("No users in this list yet")).toBeNull()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect((await screen.findAllByText("ada@example.com")).length).toBeGreaterThan(0)
    })

    it("keeps the rule edits marked unsaved when saving them fails", async () => {
        vi.mocked(api.lists.update).mockRejectedValue(apiError("rule is invalid"))

        renderList()
        await userEvent.click(await screen.findByRole("button", { name: "edit rule" }))
        expect(screen.getByText("Unsaved changes")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "rules_save" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("rule is invalid"))
        expect(screen.getByText("Unsaved changes")).toBeTruthy()
    })

    it("clears the unsaved mark once the rule is saved", async () => {
        vi.mocked(api.lists.update).mockImplementation((async (
            _projectId: string,
            _listId: string,
            params: { rule: Rule },
        ) => ({ ...list, draft_rule: params.rule })) as never)

        renderList()
        await userEvent.click(await screen.findByRole("button", { name: "edit rule" }))
        await userEvent.click(screen.getByRole("button", { name: "rules_save" }))

        await waitFor(() => expect(screen.queryByText("Unsaved changes")).toBeNull())
        expect(toastError).not.toHaveBeenCalled()
    })
})
