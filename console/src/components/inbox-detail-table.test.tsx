import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import InboxDetailTable from "./inbox-detail-table"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: translate }) }))
vi.mock("i18next", () => ({ t: translate }))

const toast = vi.hoisted(() => ({ error: vi.fn(), success: vi.fn() }))
vi.mock("sonner", () => ({ toast }))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en", has_provider: true } as Project
const message = {
    id: "m1",
    channel: "inbox",
    content: { title: "Welcome aboard" },
    tags: [],
    priority: 3,
    scheduled_at: "2026-01-01T10:00:00Z",
    sent_at: "2026-01-01T10:00:00Z",
    read_at: null,
    archived_at: null,
    failed_at: null,
    created_at: "2026-01-01T09:00:00Z",
    updated_at: "2026-01-01T09:00:00Z",
}

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

function renderTable() {
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <InboxDetailTable subjectId="u1" subjectType="users" />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("InboxDetailTable", () => {
    beforeAll(() => {
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        globalThis.ResizeObserver ??= class {
            observe() {}
            unobserve() {}
            disconnect() {}
        } as unknown as typeof ResizeObserver
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the inbox failed to load and loads it on retry", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("database unavailable"))

        renderTable()

        expect(await screen.findByText("Couldn't load inbox messages.")).toBeTruthy()
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [message], total: 1 }))
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("Welcome aboard")).toBeTruthy()
    })

    it("reports a refused archive without a success message", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [message], total: 1 }))
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("message is locked"))

        renderTable()
        await screen.findByText("Welcome aboard")
        await userEvent.click(screen.getByRole("button", { name: "Open menu" }))
        await userEvent.click(await screen.findByRole("menuitem", { name: "Archive" }))

        await waitFor(() => expect(toast.error).toHaveBeenCalledWith("message is locked"))
        expect(toast.success).not.toHaveBeenCalled()
    })
})
