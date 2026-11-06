import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import OrganizationDetailMembers from "./OrganizationDetailMembers"
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

vi.mock("@/api", () => ({ default: { users: { search: vi.fn() } } }))

const member = { id: "u1", email: "ann@example.com", organization_data: {} }
const candidate = { id: "u2", email: "bob@example.com" }

async function addMember() {
    await userEvent.click(await screen.findByRole("button", { name: "add_member" }))
    const dialog = await screen.findByRole("dialog")
    // The user has no name, so the email is shown as both name and email.
    const [name] = await within(dialog).findAllByText("bob@example.com")
    await userEvent.click(name)
}

describe("organization members", () => {
    beforeAll(setUpDom)

    beforeEach(() => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [member], total: 1 }))
        vi.mocked(api.users.search).mockResolvedValue({
            results: [candidate],
        } as unknown as Awaited<ReturnType<typeof api.users.search>>)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("keeps the user lookup open and reports a failed add", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("user is already a member"))

        renderOrganizationTab(<OrganizationDetailMembers />)
        await addMember()

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("user is already a member"))
        expect(screen.getByRole("dialog")).toBeTruthy()
    })

    it("closes the user lookup after a successful add", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(ok({}))

        renderOrganizationTab(<OrganizationDetailMembers />)
        await addMember()

        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
        expect(toastError).not.toHaveBeenCalled()
    })

    it("says the members failed to load instead of showing none", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("database unavailable"))

        renderOrganizationTab(<OrganizationDetailMembers />)

        expect(await screen.findByText("Couldn't load members.")).toBeTruthy()
        expect(screen.queryByText("no_members_yet")).toBeNull()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("ann@example.com")).toBeTruthy()
    })

    it("keeps the removal dialog open and reports a failed remove", async () => {
        vi.mocked(oapiClient.DELETE).mockResolvedValue(failed("user is the last owner"))

        renderOrganizationTab(<OrganizationDetailMembers />)
        const row = (await screen.findByText("ann@example.com")).closest("tr")!
        await userEvent.click(within(row).getByRole("button"))
        await userEvent.click(await screen.findByRole("menuitem", { name: "remove_member" }))
        const dialog = await screen.findByRole("dialog")
        await userEvent.click(within(dialog).getByRole("button", { name: "remove_member" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("user is the last owner"))
        expect(screen.getByRole("dialog")).toBeTruthy()
    })
})
