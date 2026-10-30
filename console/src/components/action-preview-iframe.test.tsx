import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import { ActionPreviewIframe } from "./action-preview-iframe"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: translate }) }))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>

function renderPreview() {
    render(
        <QueryClientProvider client={queryClient}>
            <ActionPreviewIframe
                actionType="webhook"
                projectId="p1"
                mode="action-config"
                data={{}}
            />
        </QueryClientProvider>,
    )
}

describe("ActionPreviewIframe", () => {
    beforeAll(() => {
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the preview failed to load and loads it on retry", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce({
            error: "not found",
        } as unknown as ClientResult)

        renderPreview()

        expect(await screen.findByText("Couldn't load the preview.")).toBeTruthy()
        vi.mocked(oapiClient.GET).mockResolvedValue({
            data: "<p>preview</p>",
        } as unknown as ClientResult)
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByTitle("Action Preview")).toBeTruthy()
    })
})
