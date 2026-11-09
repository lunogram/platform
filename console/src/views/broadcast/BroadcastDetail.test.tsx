import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import { BroadcastDetailRoute } from "./BroadcastDetail"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))
vi.mock("i18next", () => ({ t: translate }))

const { toastError, toastSuccess } = vi.hoisted(() => ({
    toastError: vi.fn(),
    toastSuccess: vi.fn(),
}))
vi.mock("sonner", () => ({ toast: { error: toastError, success: toastSuccess } }))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), PATCH: vi.fn(), DELETE: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

// The preview renders campaign templates, which is not what these tests are about.
vi.mock("./BroadcastMessagePreview", () => ({ BroadcastMessagePreview: () => null }))

const project = { id: "p1", name: "Acme", locale: "en", has_provider: true } as Project
const path = "/projects/p1/broadcasts/b1"

const broadcast = {
    id: "b1",
    project_id: "p1",
    campaign_id: "c1",
    list_id: "l1",
    list_name: "Newsletter",
    list_type: "static",
    state: "pending",
    total: 2,
    sent: 0,
    created_at: "2026-10-01T10:00:00Z",
    updated_at: "2026-10-01T10:00:00Z",
    campaign: { id: "c1", name: "Weekly digest", channel: "email" },
}

const member = (id: string, name: string) => ({ id, full_name: name, email: `${id}@acme.test` })

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

const BROADCAST = "/api/admin/projects/{projectID}/broadcasts/{broadcastID}"
const LIST_USERS = "/api/admin/projects/{projectID}/lists/{listID}/users"

// Answers each GET by its path; a path without an answer fails the request.
function serveGets(answers: Record<string, (init: unknown) => Promise<ClientResult>>) {
    vi.mocked(oapiClient.GET).mockImplementation(
        ((url: string, init: unknown) =>
            answers[url]?.(init) ?? Promise.resolve(failed(`unexpected GET ${url}`))) as never,
    )
}

function renderDetail() {
    const router = createMemoryRouter(
        [
            {
                path: "/projects/:projectId/broadcasts/:broadcastId",
                element: <BroadcastDetailRoute />,
            },
        ],
        { initialEntries: [path] },
    )
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <RouterProvider router={router} />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("broadcast detail", () => {
    beforeAll(() => {
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        globalThis.ResizeObserver ??= class {
            observe() {}
            unobserve() {}
            disconnect() {}
        } as unknown as typeof ResizeObserver
        // Show a failed request right away instead of retrying it first.
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        serveGets({
            [BROADCAST]: async () => ok(broadcast),
            [LIST_USERS]: async () => ok({ results: [member("u1", "Ada Lovelace")], total: 1 }),
        })
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the broadcast failed to load and loads it on retry", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("broadcast not found"))

        renderDetail()

        expect(await screen.findByText("Couldn't load the broadcast.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect((await screen.findAllByText("Weekly digest")).length).toBeGreaterThan(0)
        expect(screen.queryByText("Couldn't load the broadcast.")).toBeNull()
    })

    it("says the recipients failed to load instead of loading forever", async () => {
        serveGets({
            [BROADCAST]: async () => ok(broadcast),
            [LIST_USERS]: async () => failed("list not found"),
        })

        renderDetail()

        expect(await screen.findByText("Couldn't load recipients.")).toBeTruthy()
        serveGets({
            [BROADCAST]: async () => ok(broadcast),
            [LIST_USERS]: async () => ok({ results: [member("u1", "Ada Lovelace")], total: 1 }),
        })
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("Ada Lovelace")).toBeTruthy()
    })

    it("reports a rejected cancel and does not claim it worked", async () => {
        vi.mocked(oapiClient.DELETE).mockResolvedValue(failed("broadcast already sending"))

        renderDetail()
        await userEvent.click(await screen.findByRole("button", { name: "Cancel" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("broadcast already sending"))
        expect(toastSuccess).not.toHaveBeenCalled()
    })

    it("confirms a cancel the server accepted and shows the new state", async () => {
        vi.mocked(oapiClient.DELETE).mockResolvedValue(ok(undefined))

        renderDetail()
        const cancel = await screen.findByRole("button", { name: "Cancel" })
        serveGets({
            [BROADCAST]: async () => ok({ ...broadcast, state: "cancelled" }),
            "/api/admin/projects/{projectID}/broadcasts/{broadcastID}/users": async () =>
                ok({ results: [], total: 0 }),
        })
        await userEvent.click(cancel)

        await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Broadcast cancelled"))
        await waitFor(() => expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull())
        expect(toastError).not.toHaveBeenCalled()
    })

    it("reports a rejected send", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("no provider configured"))

        renderDetail()
        await userEvent.click(await screen.findByRole("button", { name: "Send Now" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("no provider configured"))
    })

    it("reports a rejected schedule removal and does not claim it worked", async () => {
        serveGets({
            [BROADCAST]: async () =>
                ok({ ...broadcast, state: "scheduled", scheduled_at: "2099-01-01T09:00:00Z" }),
            [LIST_USERS]: async () => ok({ results: [], total: 0 }),
        })
        vi.mocked(oapiClient.PATCH).mockResolvedValue(failed("broadcast already sending"))

        renderDetail()
        await userEvent.click(await screen.findByRole("button", { name: /remove/i }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("broadcast already sending"))
        expect(toastSuccess).not.toHaveBeenCalled()
    })

    it("keeps the current recipients page when an older response arrives late", async () => {
        const page1 = Array.from({ length: 25 }, (_, i) => member(`a${i}`, `First ${i}`))
        let answerPage2: (() => void) | undefined
        serveGets({
            [BROADCAST]: async () => ok(broadcast),
            [LIST_USERS]: (init) => {
                const { offset } = (init as { params: { query: { offset: number } } }).params.query
                if (offset === 0) return Promise.resolve(ok({ results: page1, total: 26 }))
                // Page 2 answers slowly, after the user has already gone back.
                return new Promise((resolve) => {
                    answerPage2 = () =>
                        resolve(ok({ results: [member("b0", "Second page")], total: 26 }))
                })
            },
        })

        renderDetail()
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
