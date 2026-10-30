import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { ProjectContext } from "@/contexts"
import { oapiClient } from "@/oapi/client"
import type { Project } from "@/types"
import ProjectOnboardingUsers from "./ProjectOnboardingUsers"

const translate = vi.hoisted(
    () => (key: string, fallback?: unknown) => (typeof fallback === "string" ? fallback : key),
)

vi.mock("react-i18next", () => ({
    useTranslation: () => ({ t: translate }),
}))

const toastError = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError, success: vi.fn() } }))

vi.mock("@/api", () => ({ default: { users: { addImport: vi.fn() } } }))
vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})
vi.mock("@/components/ui/user-import-dialog", () => ({ UserImportForm: () => null }))

const project = { id: "p1", name: "Acme", timezone: "UTC" } as Project
const base = "/projects/p1/onboarding"

function renderStep() {
    const router = createMemoryRouter(
        [
            {
                path: "/projects/:projectId/onboarding/users",
                element: <ProjectOnboardingUsers />,
            },
            {
                path: "/projects/:projectId/onboarding/getting-started",
                element: <p>getting started</p>,
            },
        ],
        { initialEntries: [`${base}/users`] },
    )
    render(
        <ProjectContext.Provider value={[project, () => {}]}>
            <RouterProvider router={router} />
        </ProjectContext.Provider>,
    )
    return router
}

describe("onboarding users step", () => {
    beforeEach(() => {
        vi.mocked(oapiClient.GET).mockResolvedValue({
            data: { id: "a1", email: "admin@example.com", first_name: "Ada" },
        } as never)
    })

    afterEach(() => {
        vi.clearAllMocks()
    })

    it("stays on the step when the admin user can't be created", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue({
            error: { detail: "subject quota reached" },
        } as never)

        const router = renderStep()
        await userEvent.click(screen.getByRole("button", { name: "skip" }))

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("subject quota reached"))
        expect(router.state.location.pathname).toBe(`${base}/users`)
    })

    it("moves on once the admin user is created", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue({ data: { id: "u1" } } as never)

        const router = renderStep()
        await userEvent.click(screen.getByRole("button", { name: "skip" }))

        await waitFor(() => expect(router.state.location.pathname).toBe(`${base}/getting-started`))
        expect(toastError).not.toHaveBeenCalled()
    })
})
