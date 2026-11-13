import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type { ReactNode } from "react"
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import { SenderIdentityCombobox } from "../sender-identity-combobox"
import { SenderIdentityList } from "../sender-identity-list"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)
vi.mock("react-i18next", () => ({ useTranslation: () => ({ t: translate }) }))
vi.mock("i18next", () => ({ t: translate }))

const toastError = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const senderId = "6f1c2a7e-3b4d-4e5f-8a9b-0c1d2e3f4a5b"
const identity = {
    id: senderId,
    channel: "email",
    provider_id: "prov1",
    traits: { address: "hello@acme.test", name: "Acme" },
}

type ClientResult = Awaited<ReturnType<typeof oapiClient.GET>>
const ok = (data: unknown) => ({ data }) as unknown as ClientResult
const failed = (detail: string) => ({ error: { detail } }) as unknown as ClientResult

function wrap(node: ReactNode) {
    return render(<QueryClientProvider client={queryClient}>{node}</QueryClientProvider>)
}

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

afterEach(() => {
    queryClient.clear()
    vi.clearAllMocks()
})

describe("SenderIdentityCombobox", () => {
    it("says the selected sender failed to load instead of looking unset", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValue(failed("database unavailable"))

        wrap(
            <SenderIdentityCombobox
                projectId="p1"
                channel="email"
                value={senderId}
                onChange={() => {}}
            />,
        )

        expect(await screen.findByText("Couldn't load the selected sender")).toBeTruthy()
        expect(screen.queryByText("Select sender address...")).toBeNull()
    })

    it("offers a retry in the popover when the addresses failed to load", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("database unavailable"))

        wrap(<SenderIdentityCombobox projectId="p1" channel="email" onChange={() => {}} />)
        await userEvent.click(screen.getByRole("combobox"))

        expect(await screen.findByText("Couldn't load sender addresses.")).toBeTruthy()
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [identity] }))
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("hello@acme.test")).toBeTruthy()
    })

    it("shows the selected sender once it loads", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [identity] }))

        wrap(
            <SenderIdentityCombobox
                projectId="p1"
                channel="email"
                value={senderId}
                onChange={() => {}}
            />,
        )

        expect(await screen.findByText("hello@acme.test")).toBeTruthy()
    })
})

describe("SenderIdentityList", () => {
    it("reports a failed removal and keeps the address", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [identity] }))
        vi.mocked(oapiClient.DELETE).mockResolvedValue(failed("address is in use"))

        wrap(<SenderIdentityList projectId="p1" providerId="prov1" channel="email" />)
        await screen.findByText("hello@acme.test")
        const row = screen.getByText("hello@acme.test").closest("tr")!
        await userEvent.click(row.querySelector("button")!)
        await userEvent.click(screen.getByRole("button", { name: "Remove" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("address is in use"))
        // The confirmation stays so the user can retry or cancel.
        expect(screen.getByRole("button", { name: "Remove" })).toBeTruthy()
    })

    it("says the addresses failed to load instead of an endless skeleton", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("database unavailable"))

        wrap(<SenderIdentityList projectId="p1" providerId="prov1" channel="email" />)

        expect(await screen.findByText("Couldn't load sender addresses.")).toBeTruthy()
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [identity] }))
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("hello@acme.test")).toBeTruthy()
    })
})
