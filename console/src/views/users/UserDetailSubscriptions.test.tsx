import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { ProjectContext, UserContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project, User } from "@/types"
import UserDetailSubscriptions from "./UserDetailSubscriptions"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))
vi.mock("i18next", () => ({ t: translate }))

const toastError = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), PATCH: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en" } as Project
const user = { id: "u1", identifier: [], data: {} } as unknown as User

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

const subscriptions = Array.from({ length: 150 }, (_, i) => ({
    subscription_id: `s${i}`,
    name: `Newsletter ${i}`,
    channel: "email",
    state: "subscribed",
}))

// Serves the user's subscriptions a page (of at most `limit`) at a time, as the API does.
function serveSubscriptions(all = subscriptions) {
    vi.mocked(oapiClient.GET).mockImplementation((async (
        _path: string,
        init: { params: { query: { limit: number; offset?: number } } },
    ) => {
        const { limit, offset = 0 } = init.params.query
        return ok({ results: all.slice(offset, offset + limit), total: all.length, limit, offset })
    }) as never)
}

function renderSubscriptions() {
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <UserContext.Provider value={[user, () => {}]}>
                    <UserDetailSubscriptions />
                </UserContext.Provider>
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

async function confirmUnsubscribeAll() {
    await userEvent.click(screen.getByRole("button", { name: "unsubscribe_all" }))
    const dialog = await screen.findByRole("dialog")
    await userEvent.click(within(dialog).getByRole("button", { name: "confirm" }))
}

describe("user subscriptions", () => {
    beforeAll(() => {
        // Show a failed request right away instead of retrying it first.
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        serveSubscriptions()
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("unsubscribes from every subscription, not only the ones on the first page", async () => {
        vi.mocked(oapiClient.PATCH).mockResolvedValue(ok({}))

        renderSubscriptions()
        await screen.findByText("Newsletter 0")
        await confirmUnsubscribeAll()

        await waitFor(() => expect(oapiClient.PATCH).toHaveBeenCalledTimes(1))
        const [, { body }] = vi.mocked(oapiClient.PATCH).mock.calls[0] as unknown as [
            string,
            { body: unknown[] },
        ]
        expect(body).toHaveLength(150)
        expect(body).toContainEqual({ subscription_id: "s149", state: "unsubscribed" })
        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
    })

    it("reports a failed unsubscribe-all and keeps the dialog open", async () => {
        vi.mocked(oapiClient.PATCH).mockResolvedValue(failed("user is locked"))

        renderSubscriptions()
        await screen.findByText("Newsletter 0")
        await confirmUnsubscribeAll()

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("user is locked"))
        expect(screen.getByRole("dialog")).toBeTruthy()
    })

    it("reports a failed toggle", async () => {
        vi.mocked(oapiClient.PATCH).mockResolvedValue(failed("subscription is archived"))

        renderSubscriptions()
        await screen.findByText("Newsletter 0")
        await userEvent.click(screen.getAllByRole("switch")[0])
        const dialog = await screen.findByRole("dialog")
        await userEvent.click(within(dialog).getByRole("button", { name: "confirm" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("subscription is archived"))
    })

    it("says the subscriptions failed to load and loads them on retry", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("database unavailable"))

        renderSubscriptions()

        expect(await screen.findByText("Couldn't load subscriptions.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("Newsletter 0")).toBeTruthy()
    })
})
