import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import type { Project, Template } from "@/types"
import NewCampaign from "./NewCampaign"

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
    default: {
        campaigns: { update: vi.fn(), templates: { create: vi.fn() } },
        subscriptions: { search: vi.fn() },
    },
}))

vi.mock("@/oapi/client", () => {
    const client = { GET: vi.fn(), POST: vi.fn(), use: vi.fn() }
    return { default: client, oapiClient: client }
})

const project = { id: "p1", name: "Acme", locale: "en" } as Project
const formPath = "/projects/p1/campaigns/new/email"
const newsletter = { id: "s1", name: "Newsletter", channel: "email" }

type PostResult = Awaited<ReturnType<typeof oapiClient.POST>>
type SubscriptionsResult = Awaited<ReturnType<typeof api.subscriptions.search>>

function renderForm() {
    const router = createMemoryRouter(
        [
            { path: "/projects/:projectId/campaigns/new/:channel", element: <NewCampaign /> },
            { path: "/projects/:projectId/campaigns/:campaignId", element: <p>campaign page</p> },
            {
                path: "/projects/:projectId/campaigns/:campaignId/templates/:templateId",
                element: <p>template editor</p>,
            },
        ],
        { initialEntries: [formPath] },
    )
    render(
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <RouterProvider router={router} />
            </ProjectContext.Provider>
        </QueryClientProvider>,
    )
    return router
}

// Fills in the form and submits it once the subscription has been picked.
async function submitCampaign() {
    await userEvent.type(await screen.findByLabelText("campaign.setup.form.name.label"), "Welcome")
    const create = screen.getByRole<HTMLButtonElement>("button", { name: "Create" })
    await waitFor(() => expect(create.disabled).toBe(false))
    await userEvent.click(create)
}

describe("new campaign", () => {
    beforeAll(() => {
        // Radix components use pointer, scroll and resize APIs that jsdom lacks.
        Element.prototype.hasPointerCapture = () => false
        Element.prototype.releasePointerCapture = () => {}
        Element.prototype.scrollIntoView = () => {}
        globalThis.ResizeObserver ??= class {
            observe() {}
            unobserve() {}
            disconnect() {}
        }
        // Show a failed request right away instead of retrying it first.
        queryClient.setDefaultOptions({ queries: { retry: false } })
    })

    beforeEach(() => {
        vi.mocked(api.subscriptions.search).mockResolvedValue({
            results: [newsletter],
            total: 1,
        } as unknown as SubscriptionsResult)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("says subscriptions failed to load and loads them on retry", async () => {
        vi.mocked(api.subscriptions.search).mockRejectedValueOnce(new Error("network down"))

        renderForm()

        expect(await screen.findByText("Couldn't load subscriptions.")).toBeTruthy()
        await userEvent.click(screen.getByRole("button", { name: "Retry" }))
        // The first subscription of the channel is picked once they load.
        expect(await screen.findAllByText("Newsletter")).not.toHaveLength(0)
    })

    it("opens the campaign instead of offering to create it again when its setup fails", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue({ data: { id: "c9" } } as PostResult)
        vi.mocked(api.campaigns.templates.create).mockRejectedValue(new Error("network down"))

        const router = renderForm()
        await submitCampaign()

        await waitFor(() =>
            expect(router.state.location.pathname).toBe("/projects/p1/campaigns/c9"),
        )
        expect(oapiClient.POST).toHaveBeenCalledTimes(1)
        expect(toastError).toHaveBeenCalledWith(
            "The campaign was created, but its setup didn't finish",
            expect.anything(),
        )
    })

    it("opens the editor of a fully created campaign", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue({ data: { id: "c9" } } as PostResult)
        vi.mocked(api.campaigns.templates.create).mockResolvedValue({ id: "t1" } as Template)

        const router = renderForm()
        await submitCampaign()

        await waitFor(() =>
            expect(router.state.location.pathname).toBe("/projects/p1/campaigns/c9/templates/t1"),
        )
        expect(toastError).not.toHaveBeenCalled()
    })

    it("shows why the campaign couldn't be created and keeps the form", async () => {
        vi.mocked(oapiClient.POST).mockResolvedValue({
            error: { detail: "name already in use" },
        } as unknown as PostResult)

        const router = renderForm()
        await submitCampaign()

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("name already in use"))
        expect(router.state.location.pathname).toBe(formPath)
        expect(api.campaigns.templates.create).not.toHaveBeenCalled()
    })
})
