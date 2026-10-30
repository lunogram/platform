import { QueryClientProvider } from "@tanstack/react-query"
import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import { MediaManager } from "./MediaManager"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: translate }) }))
vi.mock("i18next", () => ({ t: translate }))

const toastError = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }))

vi.mock("@/api", () => ({ default: { images: { create: vi.fn() } } }))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), DELETE: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en", has_provider: true } as Project
const image = {
    id: "img1",
    name: "logo",
    filename: "logo.png",
    url: "https://cdn.test/logo.png",
    content_type: "image/png",
    size_bytes: 10,
}

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

function renderManager() {
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <MediaManager open onOpenChange={() => {}} onSelect={() => {}} />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("MediaManager", () => {
    beforeAll(() => {
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

    it("says the images failed to load instead of showing none", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("storage unavailable"))

        renderManager()

        expect(await screen.findByText("Couldn't load images.")).toBeTruthy()
        expect(screen.queryByText("No images yet")).toBeNull()
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [image] }))
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByAltText("logo")).toBeTruthy()
    })

    it("reports a failed upload", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [] }))
        vi.mocked(api.images.create).mockRejectedValue(new Error("too large"))

        renderManager()
        await screen.findByText("No images yet")
        const input = document.querySelector("input[type=file]") as HTMLInputElement
        fireEvent.change(input, {
            target: { files: [new File(["x"], "big.png", { type: "image/png" })] },
        })

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("Couldn't upload the image."))
    })

    it("reports a failed delete and keeps the image", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [image] }))
        vi.mocked(oapiClient.DELETE).mockResolvedValue(failed("image is used by a template"))

        renderManager()
        const img = await screen.findByAltText("logo")
        const deleteButton = img.parentElement!.querySelector("button")!
        await userEvent.click(deleteButton)

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("image is used by a template"))
        expect(screen.getByAltText("logo")).toBeTruthy()
    })
})
