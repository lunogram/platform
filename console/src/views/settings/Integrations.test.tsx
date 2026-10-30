import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import Integrations from "./Integrations"

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
    const client = { GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})
vi.mock("@/components/icon-mosaic", () => ({ StaggeredMosaic: () => null }))

const project = { id: "p1", name: "Acme", locale: "en" } as Project
const path = "/projects/p1/integrations"

const providers = Array.from({ length: 120 }, (_, i) => ({
    id: `prov-${i + 1}`,
    name: `Provider ${i + 1}`,
    module: "resend",
    channels: ["email"],
}))

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
type RequestInit = { params?: { query?: { limit?: number; offset?: number } } }

// Answers like the API: list endpoints return one page (default size 20).
function serve(url: string, init?: RequestInit) {
    const page = <T,>(items: T[]) => {
        const limit = init?.params?.query?.limit ?? 20
        const offset = init?.params?.query?.offset ?? 0
        return { results: items.slice(offset, offset + limit), total: items.length, limit, offset }
    }
    if (url.endsWith("/providers")) return { data: page(providers) }
    if (url.endsWith("/actions")) return { data: page([]) }
    return { data: [] }
}

function renderPage() {
    const router = createMemoryRouter([{ path, element: <Integrations /> }], {
        initialEntries: [path],
    })
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <RouterProvider router={router} />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("integrations list", () => {
    beforeAll(() => {
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        vi.mocked(oapiClient.GET).mockImplementation(((url: string, init?: RequestInit) =>
            Promise.resolve(serve(url, init))) as unknown as typeof oapiClient.GET)
        vi.spyOn(window, "confirm").mockReturnValue(true)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("lists every provider, not only the first page", async () => {
        renderPage()

        expect(await screen.findByText("Provider 1")).toBeTruthy()
        expect(screen.getByText("Provider 51")).toBeTruthy()
        expect(screen.getByText("Provider 120")).toBeTruthy()
    })

    it("says the list failed to load instead of showing an empty list", async () => {
        vi.mocked(oapiClient.GET).mockImplementation(((url: string, init?: RequestInit) =>
            Promise.resolve(
                url.endsWith("/providers")
                    ? ({ error: { detail: "boom" } } as unknown as ClientResult)
                    : serve(url, init),
            )) as unknown as typeof oapiClient.GET)

        renderPage()

        expect(await screen.findByText("Couldn't load integrations.")).toBeTruthy()
        expect(screen.queryByText("No integrations yet")).toBeNull()
    })

    it("reports a rejected archive and keeps the provider listed", async () => {
        vi.mocked(oapiClient.DELETE).mockResolvedValue({
            error: { detail: "provider is in use" },
        } as never)

        renderPage()
        const row = (await screen.findByText("Provider 1")).closest("tr")!
        await userEvent.click(within(row).getByRole("button", { name: "options" }))
        await userEvent.click(await screen.findByRole("menuitem", { name: "archive" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("provider is in use"))
        expect(screen.getByText("Provider 1")).toBeTruthy()
    })
})
