import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { client } from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import ScheduledDetailTable from "./scheduled-detail-table"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: translate }) }))
vi.mock("i18next", () => ({ t: translate }))

const toast = vi.hoisted(() => ({ error: vi.fn(), success: vi.fn() }))
vi.mock("sonner", () => ({ toast }))

vi.mock("@/api", () => {
    const client = { get: vi.fn(), patch: vi.fn(), delete: vi.fn() }
    return { client, default: {} }
})

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), PUT: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en", has_provider: true } as Project

const item = {
    id: "i1",
    scheduled_id: "s1",
    scheduled_at: "2026-11-01T10:00:00Z",
    start_at: null,
    anchor_at: null,
    interval: null,
    paused_at: null,
    has_pending_events: false,
    data: null,
    created_at: "2026-10-01T10:00:00Z",
    updated_at: "2026-10-01T10:00:00Z",
}

type ClientResult = Awaited<ReturnType<typeof oapiClient.PUT>>
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

// mockGet answers the schemas and list requests; either can be made to fail.
function mockGet({ schemasFail = false, listFail = false } = {}) {
    vi.mocked(client.get).mockImplementation(async (url: string) => {
        if (url.endsWith("/schema")) {
            if (schemasFail) throw new Error("schemas unavailable")
            return { data: { results: [{ id: "s1", name: "weekly_digest" }] } }
        }
        if (listFail) throw new Error("list unavailable")
        return { data: { results: [item], total: 1, limit: 15, offset: 0 } }
    })
}

function renderTable() {
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <ScheduledDetailTable subjectId="u1" subjectType="users" />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("ScheduledDetailTable", () => {
    beforeAll(() => {
        // Radix dialogs and menus use pointer and scroll APIs that jsdom lacks.
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

    beforeEach(() => mockGet())

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("keeps the dialog and the input when the server rejects a new item", async () => {
        vi.mocked(oapiClient.PUT).mockResolvedValue(failed("interval is invalid"))

        renderTable()
        await screen.findByText("weekly_digest")
        await userEvent.click(screen.getByRole("button", { name: /Create/ }))
        const name = await screen.findByPlaceholderText("Type or select a schedule name...")
        await userEvent.type(name, "monthly_report")
        await userEvent.tab()
        await userEvent.type(screen.getByLabelText("Interval"), "every so often")
        await userEvent.click(screen.getByRole("button", { name: "create" }))

        await waitFor(() => expect(toast.error).toHaveBeenCalledWith("interval is invalid"))
        expect(toast.success).not.toHaveBeenCalled()
        expect(screen.getByRole("dialog")).toBeTruthy()
        expect((screen.getByLabelText("Interval") as HTMLInputElement).value).toBe("every so often")
    })

    it("closes the dialog after a successful create", async () => {
        vi.mocked(oapiClient.PUT).mockResolvedValue({ data: {} } as unknown as ClientResult)

        renderTable()
        await screen.findByText("weekly_digest")
        await userEvent.click(screen.getByRole("button", { name: /Create/ }))
        await userEvent.type(
            await screen.findByPlaceholderText("Type or select a schedule name..."),
            "monthly_report",
        )
        await userEvent.tab()
        await userEvent.click(screen.getByRole("button", { name: "create" }))

        await waitFor(() => expect(toast.success).toHaveBeenCalledWith("Scheduled item created"))
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
    })

    it("says the list failed to load and loads it on retry", async () => {
        mockGet({ listFail: true })

        renderTable()

        expect(await screen.findByText("Couldn't load scheduled items.")).toBeTruthy()
        mockGet()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("weekly_digest")).toBeTruthy()
    })

    it("says the schedule names failed to load instead of hiding it", async () => {
        mockGet({ schemasFail: true })

        renderTable()

        expect(await screen.findByText("Couldn't load schedule names.")).toBeTruthy()
    })
})
