import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import Organizations from "./Organizations"
import { failed, ok, organization, organizationsPath, renderRoutes, setUpDom } from "./testing"

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
    const client = { GET: vi.fn(), POST: vi.fn(), PATCH: vi.fn(), DELETE: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

function renderList() {
    return renderRoutes(
        [{ path: "/projects/:projectId/organizations", element: <Organizations /> }],
        organizationsPath,
    )
}

async function createOrganization(identifier: string) {
    await userEvent.click(screen.getByRole("button", { name: "Create Organization from header" }))
    const dialog = await screen.findByRole("dialog")
    await userEvent.type(within(dialog).getByPlaceholderText("e.g., org-123"), identifier)
    await userEvent.click(within(dialog).getByRole("button", { name: "create" }))
}

describe("organizations list", () => {
    beforeAll(setUpDom)

    beforeEach(() => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok({ results: [organization], total: 1 }))
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says the list failed to load and loads it on retry", async () => {
        vi.mocked(oapiClient.GET).mockResolvedValueOnce(failed("database unavailable"))

        renderList()

        expect(await screen.findByText("Couldn't load organizations.")).toBeTruthy()
        expect(screen.queryByText("no_organizations_yet")).toBeNull()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        expect(await screen.findByText("Acme")).toBeTruthy()
    })

    it("keeps the dialog open and reports a failed create", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(failed("identifier already exists"))

        renderList()
        await screen.findByText("Acme")
        await createOrganization("org-1")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("identifier already exists"))
        expect(screen.getByRole("dialog")).toBeTruthy()
        expect(screen.getByDisplayValue("org-1")).toBeTruthy()
    })

    it("closes the dialog and reloads the list after a create", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue(ok(organization))

        renderList()
        await screen.findByText("Acme")
        await createOrganization("org-1")

        await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull())
        expect(oapiClient.GET).toHaveBeenCalledTimes(2)
        expect(toastError).not.toHaveBeenCalled()
    })
})
