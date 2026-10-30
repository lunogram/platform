import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import Locales from "./Locales"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))
vi.mock("i18next", () => ({ t: translate }))

const toastError = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }))

vi.mock("@/api", () => ({ default: { locales: { search: vi.fn() } } }))
vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})
// The real picker is a searchable popover; a plain button stands in for it.
vi.mock("@/components/locale/picker", () => ({
    LocalePicker: ({ onChange }: { onChange: (key: string) => void }) => (
        <button type="button" onClick={() => onChange("fr")}>
            pick french
        </button>
    ),
}))

const project = { id: "p1", name: "Acme", locale: "en" } as Project
const english = { id: "l1", key: "en", label: "English" }

type ClientResult = Awaited<ReturnType<typeof oapiClient.POST>>
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

function renderLocales() {
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <Locales />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("locales settings", () => {
    beforeAll(() => {
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        vi.mocked(api.locales.search).mockResolvedValue({
            results: [english],
            total: 1,
        } as never)
        // The page used to read the list through this client.
        vi.mocked(oapiClient.GET).mockResolvedValue({
            data: { results: [english], total: 1 },
        } as never)
        vi.spyOn(window, "confirm").mockReturnValue(true)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the list failed to load and loads it on retry", async () => {
        vi.mocked(api.locales.search).mockRejectedValueOnce(new Error("down"))

        renderLocales()

        expect(await screen.findByText("Couldn't load locales.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("English")).toBeTruthy()
    })

    it("reports a rejected create and keeps the dialog open", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("locale already exists"))

        renderLocales()
        await screen.findByText("English")
        await userEvent.click(screen.getByRole("button", { name: "Create Locale from header" }))
        const dialog = await screen.findByRole("dialog")
        await userEvent.click(within(dialog).getByText("pick french"))
        await userEvent.click(within(dialog).getByRole("button", { name: "create" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("locale already exists"))
        expect(screen.getByRole("dialog")).toBeTruthy()
    })

    it("reports a rejected delete and keeps the locale listed", async () => {
        vi.mocked(oapiClient.DELETE).mockResolvedValue(failed("locale is in use"))

        renderLocales()
        const row = (await screen.findByText("English")).closest("tr")!
        await userEvent.click(within(row).getByRole("button", { name: "action" }))
        await userEvent.click(await screen.findByRole("menuitem", { name: "delete" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("locale is in use"))
        expect(screen.getByText("English")).toBeTruthy()
    })
})
