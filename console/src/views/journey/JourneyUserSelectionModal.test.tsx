import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import { UserSelectionModal } from "./JourneyUserSelectionModal"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))
vi.mock("i18next", () => ({ t: translate }))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>

const user = (id: string, email: string) => ({ id, email, identifier: [] })

// The search a users request was made with.
const searchOf = (call: unknown[]) =>
    (call[1] as { params: { query: { search?: string } } }).params.query.search

function renderModal() {
    render(
        <QueryClientProvider client={queryClient}>
            <UserSelectionModal isOpen projectId="p1" onClose={() => {}} onSelect={() => {}} />
        </QueryClientProvider>,
    )
}

describe("journey user selection", () => {
    beforeAll(() => {
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("runs a search typed while the first page is still loading", async () => {
        let finishFirstPage: (result: ClientResult) => void = () => {}
        vi.mocked(oapiClient.GET).mockImplementation(((_: string, init: unknown) => {
            const search = searchOf([_, init])
            if (!search) {
                return new Promise<ClientResult>((resolve) => (finishFirstPage = resolve))
            }
            return Promise.resolve({
                data: { results: [user("u2", "ada@example.com")], total: 1 },
            } as unknown as ClientResult)
        }) as never)

        renderModal()
        await userEvent.type(screen.getByRole("textbox", { name: "Search users" }), "ada")

        expect(await screen.findByText("ada@example.com", {}, { timeout: 2000 })).toBeTruthy()
        expect(vi.mocked(oapiClient.GET).mock.calls.map(searchOf)).toContain("ada")

        // The first page arriving late must not replace the search results.
        finishFirstPage({
            data: { results: [user("u1", "bob@example.com")], total: 1 },
        } as unknown as ClientResult)
        await waitFor(() => expect(screen.queryByText("bob@example.com")).toBeNull())
        expect(screen.getByText("ada@example.com")).toBeTruthy()
    })

    it("says the users failed to load instead of showing none", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValue({
            error: { detail: "unavailable" },
        } as unknown as ClientResult)

        renderModal()

        expect(await screen.findByText("Couldn't load users.")).toBeTruthy()
        expect(screen.queryByText("No users yet")).toBeNull()
    })
})
