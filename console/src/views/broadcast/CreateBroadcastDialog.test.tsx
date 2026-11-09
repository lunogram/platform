import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { useState } from "react"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { List, Project } from "@/types"
import type { UUID } from "@/types/common"
import { CreateBroadcastDialog } from "./CreateBroadcastDialog"

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
    default: { campaigns: { search: vi.fn() }, lists: { search: vi.fn() } },
}))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en", has_provider: true } as Project
const campaign = { id: "c1", name: "Weekly digest", channel: "email" }
const list = { id: "l1", name: "Newsletter", state: "ready", users_count: 3 } as List

type ClientResult = Awaited<ReturnType<typeof oapiClient.POST>>
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

const onCreated = vi.fn()

// Hosts the dialog the way pages do: closed until the user opens it.
function Host() {
    const [open, setOpen] = useState(false)
    return (
        <>
            <button onClick={() => setOpen(true)}>open dialog</button>
            <CreateBroadcastDialog
                open={open}
                onOpenChange={setOpen}
                campaignId={"c1" as UUID}
                listId={"l1" as UUID}
                list={list}
                onCreated={onCreated}
            />
        </>
    )
}

function renderHost() {
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <Host />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
}

describe("create broadcast dialog", () => {
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
        vi.mocked(api.campaigns.search).mockResolvedValue({ results: [campaign] } as never)
        vi.mocked(api.lists.search).mockResolvedValue({ results: [list] } as never)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("loads campaigns and lists only once it is opened", async () => {
        renderHost()
        expect(api.campaigns.search).not.toHaveBeenCalled()
        expect(api.lists.search).not.toHaveBeenCalled()

        await userEvent.click(screen.getByRole("button", { name: "open dialog" }))

        await waitFor(() => expect(api.campaigns.search).toHaveBeenCalled())
        expect(api.lists.search).toHaveBeenCalled()
    })

    it("says the campaigns failed to load instead of offering none", async () => {
        vi.mocked(api.campaigns.search)
            .mockRejectedValueOnce(new Error("network down"))
            .mockResolvedValue({ results: [campaign] } as never)

        renderHost()
        await userEvent.click(screen.getByRole("button", { name: "open dialog" }))

        expect(await screen.findByText("Couldn't load campaigns.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        await waitFor(() => expect(screen.queryByText("Couldn't load campaigns.")).toBeNull())
    })

    it("reports a rejected create and stays open", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("list is empty"))

        renderHost()
        await userEvent.click(screen.getByRole("button", { name: "open dialog" }))
        // Wait for the confirmation, which needs both pickers loaded.
        await screen.findByText(/This will send/)
        await userEvent.click(screen.getByRole("button", { name: "Send Broadcast" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("list is empty"))
        expect(onCreated).not.toHaveBeenCalled()
    })
})
