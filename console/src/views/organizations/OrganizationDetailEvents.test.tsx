import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import OrganizationDetailEvents from "./OrganizationDetailEvents"
import { failed, ok, renderOrganizationTab, setUpDom } from "./testing"

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

const event = {
    id: "e1",
    name: "plan_upgraded",
    data: {},
    created_at: "2026-10-02T10:00:00Z",
}

async function submitEvent(name: string) {
    await userEvent.click(screen.getByRole("button", { name: "create" }))
    const dialog = await screen.findByRole("dialog")
    await userEvent.type(
        within(dialog).getByPlaceholderText("Type or select an event name..."),
        name,
    )
    await userEvent.click(within(dialog).getByRole("button", { name: "create" }))
}

describe("organization events", () => {
    beforeAll(setUpDom)

    beforeEach(() => {
        vi.mocked(oapiClient.GET).mockImplementation((async (path: string) =>
            path.endsWith("/schema")
                ? ok({ results: [] })
                : ok({ results: [event], total: 1 })) as unknown as typeof oapiClient.GET)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("reports a rejected event instead of confirming it", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("event name is reserved"))

        renderOrganizationTab(<OrganizationDetailEvents />)
        await screen.findByText("plan_upgraded")
        await submitEvent("signup")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("event name is reserved"))
        expect(toastSuccess).not.toHaveBeenCalled()
        // The dialog stays open with the input, so the user can correct it.
        expect(screen.getByRole("dialog")).toBeTruthy()
    })

    it("confirms a created event and closes the dialog", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(ok({}))

        renderOrganizationTab(<OrganizationDetailEvents />)
        await screen.findByText("plan_upgraded")
        await submitEvent("signup")

        await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Event created"))
        expect(toastError).not.toHaveBeenCalled()
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
    })

    it("says the events failed to load and loads them on retry", async () => {
        let failNext = true
        vi.mocked(oapiClient.GET).mockImplementation((async (path: string) => {
            if (path.endsWith("/schema")) return ok({ results: [] })
            if (failNext) {
                failNext = false
                return failed("database unavailable")
            }
            return ok({ results: [event], total: 1 })
        }) as unknown as typeof oapiClient.GET)

        renderOrganizationTab(<OrganizationDetailEvents />)

        expect(await screen.findByText("Couldn't load events.")).toBeTruthy()
        expect(screen.queryByText("no_events_yet")).toBeNull()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("plan_upgraded")).toBeTruthy()
    })

    it("shows the page that was asked for last when searches overlap", async () => {
        // The first search answers after the second; its results must not win.
        let answerFirst: (value: unknown) => void = () => {}
        vi.mocked(oapiClient.GET).mockImplementation((async (
            path: string,
            init: { params: { query?: { search?: string } } },
        ) => {
            if (path.endsWith("/schema")) return ok({ results: [] })
            const search = init.params.query?.search
            if (search === "pla") {
                await new Promise((resolve) => (answerFirst = resolve))
                return ok({ results: [{ ...event, id: "e2", name: "stale_result" }], total: 1 })
            }
            return ok({ results: [event], total: 1 })
        }) as unknown as typeof oapiClient.GET)

        renderOrganizationTab(<OrganizationDetailEvents />)
        await screen.findByText("plan_upgraded")
        const search = screen.getByPlaceholderText("Search events...")
        await userEvent.type(search, "pla")
        await waitFor(() =>
            expect(oapiClient.GET).toHaveBeenCalledWith(
                expect.stringContaining("/events"),
                expect.objectContaining({
                    params: expect.objectContaining({
                        query: expect.objectContaining({ search: "pla" }),
                    }),
                }),
            ),
        )
        await userEvent.type(search, "n")
        await waitFor(() =>
            expect(oapiClient.GET).toHaveBeenCalledWith(
                expect.stringContaining("/events"),
                expect.objectContaining({
                    params: expect.objectContaining({
                        query: expect.objectContaining({ search: "plan" }),
                    }),
                }),
            ),
        )
        answerFirst(undefined)

        await new Promise((resolve) => setTimeout(resolve, 50))
        expect(screen.queryByText("stale_result")).toBeNull()
        expect(screen.getByText("plan_upgraded")).toBeTruthy()
    })
})
