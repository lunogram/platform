import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import Organization from "./Organization"
import { organizationQuery } from "./queries"
import {
    failed,
    ok,
    organization,
    organizationPath,
    organizationsPath,
    renderRoutes,
    setUpDom,
} from "./testing"

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

// Mirrors the organization route in router.tsx.
function renderOrganization() {
    return renderRoutes(
        [
            { path: organizationsPath, element: <p>organizations list</p> },
            {
                path: "/projects/:projectId/organizations/:organizationId",
                loader: async ({ params }) => {
                    await queryClient.ensureQueryData(
                        organizationQuery(params.projectId!, params.organizationId!),
                    )
                    return null
                },
                element: <Organization />,
                children: [{ index: true, element: <p>details tab</p> }],
            },
        ],
        organizationPath,
    )
}

async function deleteOrganization() {
    // The actions menu button has no label; it is the page's only menu trigger.
    await screen.findByRole("heading", { level: 1 })
    await userEvent.click(document.querySelector<HTMLElement>('[aria-haspopup="menu"]')!)
    await userEvent.click(await screen.findByRole("menuitem", { name: "delete" }))
    const dialog = await screen.findByRole("dialog")
    await userEvent.click(within(dialog).getByRole("button", { name: "delete_organization" }))
}

async function rename(name: string) {
    await userEvent.click(await screen.findByRole("button", { name: "Edit value" }))
    const input = await screen.findByDisplayValue("Acme")
    await userEvent.clear(input)
    await userEvent.type(input, `${name}{Enter}`)
}

const heading = () => screen.getByRole("heading", { level: 1 }).textContent

describe("organization page", () => {
    beforeAll(setUpDom)

    beforeEach(() => {
        vi.mocked(oapiClient.GET).mockResolvedValue(ok(organization))
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("stays on the organization when the delete fails", async () => {
        vi.mocked(oapiClient.DELETE).mockResolvedValue(failed("organization has members"))

        const router = renderOrganization()
        await deleteOrganization()

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("organization has members"))
        expect(router.state.location.pathname).toBe(organizationPath)
        expect(screen.getByRole("dialog")).toBeTruthy()
    })

    it("opens the list after a successful delete", async () => {
        vi.mocked(oapiClient.DELETE).mockResolvedValue(ok(undefined))

        const router = renderOrganization()
        await deleteOrganization()

        await waitFor(() => expect(router.state.location.pathname).toBe(organizationsPath))
        expect(toastError).not.toHaveBeenCalled()
    })

    it("reports a failed rename and keeps showing the saved name", async () => {
        vi.mocked(oapiClient.PATCH).mockResolvedValue(failed("name is taken"))

        renderOrganization()
        await rename("Globex")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("name is taken"))
        expect(toastSuccess).not.toHaveBeenCalled()
        expect(heading()).toBe("Acme")
    })

    it("shows the new name after a rename", async () => {
        vi.mocked(oapiClient.PATCH).mockResolvedValue(ok({ ...organization, name: "Globex" }))

        renderOrganization()
        await rename("Globex")

        await waitFor(() => expect(heading()).toBe("Globex"))
        expect(toastSuccess).toHaveBeenCalledWith("Organization updated")
    })
})
