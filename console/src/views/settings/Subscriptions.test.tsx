import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { AxiosError, AxiosHeaders } from "axios"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import type { Project } from "@/types"
import Subscriptions from "./Subscriptions"

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
    default: { subscriptions: { search: vi.fn(), create: vi.fn(), update: vi.fn() } },
}))

const project = { id: "p1", name: "Acme", locale: "en" } as Project
const newsletter = { id: "s1", name: "Newsletter", channel: "email", is_public: true }

function rejected(detail: string) {
    const headers = new AxiosHeaders()
    return new AxiosError("Request failed", "ERR_BAD_REQUEST", undefined, undefined, {
        status: 409,
        statusText: "Conflict",
        headers,
        config: { headers },
        data: { detail },
    })
}

function renderPage() {
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <Subscriptions />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("subscriptions settings", () => {
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

    beforeEach(() => {
        vi.mocked(api.subscriptions.search).mockResolvedValue({
            results: [newsletter],
            total: 1,
        } as never)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the list failed to load and loads it on retry", async () => {
        vi.mocked(api.subscriptions.search).mockRejectedValueOnce(new Error("down"))

        renderPage()

        expect(await screen.findByText("Couldn't load subscriptions.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("Newsletter")).toBeTruthy()
    })

    it("reports a rejected create and keeps the dialog open", async () => {
        vi.mocked(api.subscriptions.create).mockRejectedValue(rejected("name already taken"))

        renderPage()
        await screen.findByText("Newsletter")
        await userEvent.click(screen.getByRole("button", { name: "create_subscription" }))
        const dialog = await screen.findByRole("dialog")
        await userEvent.type(within(dialog).getByLabelText(/name/), "Promotions")
        await userEvent.click(within(dialog).getByRole("button", { name: "create_subscription" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("name already taken"))
        expect(screen.getByRole("dialog")).toBeTruthy()
    })
})
