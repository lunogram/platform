import { QueryClientProvider } from "@tanstack/react-query"
import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryRouter, RouterProvider } from "react-router"
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { oapiClient } from "@/oapi/client"
import { ApiError } from "@/oapi/unwrap"
import type { Campaign, Project, Template } from "@/types"
import CampaignView from "./Campaign"
import CampaignDetails from "./CampaignDetails"
import { campaignKeys, campaignQuery } from "./queries"
import TemplateContent from "./template/Content"
import TemplateReview from "./template/Review"
import { TemplateRoute } from "./template/TemplateProvider"

// These tests render the real campaign pages and routes; only the network is mocked.

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
        campaigns: {
            get: vi.fn(),
            update: vi.fn(),
            search: vi.fn(async () => ({ results: [], total: 0 })),
            templates: { create: vi.fn(), update: vi.fn() },
        },
        locales: { search: vi.fn() },
        lists: { search: vi.fn(async () => ({ results: [], total: 0 })) },
        subscriptions: { search: vi.fn(async () => ({ results: [], total: 0 })) },
        users: { search: vi.fn(async () => ({ results: [], total: 0 })) },
    },
}))

vi.mock("@/oapi/client", () => {
    const client = {
        GET: vi.fn(async () => ({ data: { results: [], total: 0 } })),
        POST: vi.fn(async () => ({ data: {} })),
        use: vi.fn(),
    }
    return { default: client, oapiClient: client }
})

vi.mock("@/lib/path-suggestions", () => ({
    fetchPathSuggestions: vi.fn(async () => ({ userPaths: [] })),
}))

const project = { id: "p1", name: "Acme", locale: "en" } as Project

const english = {
    id: "t-en",
    campaign_id: "c1",
    type: "inbox",
    locale: "en",
    sender_identity_id: null,
    data: { title: "Hello there", body: "Glad you joined" },
} as unknown as Template

const dutch = {
    id: "t-nl",
    campaign_id: "c1",
    type: "inbox",
    locale: "nl",
    sender_identity_id: null,
    data: { title: "Hallo daar", body: "Fijn dat je er bent" },
} as unknown as Template

const campaign = {
    id: "c1",
    name: "Onboarding",
    channel: "inbox",
    transactional: true,
    variables: [],
    templates: [english, dutch],
} as unknown as Campaign

const detailsPath = "/projects/p1/campaigns/c1"
const contentPath = `${detailsPath}/templates/t-en`

function renderAt(path: string) {
    const router = createMemoryRouter(
        [
            {
                path: "/projects/:projectId/campaigns/:entityId",
                loader: async ({ params }) => {
                    await queryClient.ensureQueryData(
                        campaignQuery(params.projectId as string, params.entityId as string),
                    )
                    return null
                },
                element: <CampaignView />,
                children: [
                    { index: true, element: <CampaignDetails /> },
                    {
                        path: "templates/:templateId",
                        element: <TemplateRoute />,
                        children: [
                            { index: true, element: <TemplateContent /> },
                            { path: "review", element: <TemplateReview /> },
                        ],
                    },
                ],
            },
        ],
        { initialEntries: [path] },
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

const locales = [
    { key: "en", label: "English" },
    { key: "nl", label: "Dutch" },
]

// The locale picker in the editor's footer, labelled with the locale on screen.
const footerLocalePicker = () =>
    screen
        .getAllByRole("combobox")
        .find((el) => locales.some((locale) => el.textContent?.includes(locale.label)))

async function chooseFooterLocale(label: string) {
    await waitFor(() => expect(footerLocalePicker()).toBeTruthy())
    await userEvent.click(footerLocalePicker()!)
    await userEvent.click(await screen.findByRole("option", { name: new RegExp(label) }))
}

const cachedCampaign = () => queryClient.getQueryData<Campaign>(campaignKeys.detail("p1", "c1"))

describe("campaign editor", () => {
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
    })

    beforeEach(() => {
        vi.mocked(api.campaigns.get).mockImplementation(async () => structuredClone(campaign))
        vi.mocked(api.locales.search).mockResolvedValue({
            results: locales,
            total: 2,
        } as unknown as Awaited<ReturnType<typeof api.locales.search>>)
        // The footer's locale picker searches locales itself.
        vi.mocked(oapiClient.GET).mockImplementation((async (path: string) => ({
            data: path.includes("/locales")
                ? { results: locales, total: 2 }
                : { results: [], total: 0 },
        })) as unknown as typeof oapiClient.GET)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("shows a saved template on the details page without reloading", async () => {
        const saved = { ...english, data: { title: "Hello again", body: "Glad you joined" } }
        vi.mocked(api.campaigns.templates.update).mockResolvedValue(saved as Template)

        const router = renderAt(contentPath)
        await screen.findAllByText("Hello there")

        await userEvent.click(screen.getByRole("button", { name: "next" }))
        await waitFor(() => expect(router.state.location.pathname).toBe(`${contentPath}/review`))

        expect(api.campaigns.templates.update).toHaveBeenCalledWith("p1", "c1", "t-en", {
            data: { title: "Hello there", body: "Glad you joined" },
            sender_identity_id: null,
        })

        await router.navigate(detailsPath)

        // The details page reads the campaign the save wrote into, not a stale copy.
        expect(await screen.findAllByText("Hello again")).not.toHaveLength(0)
        expect(screen.queryByText("Hello there")).toBeNull()
        expect(api.campaigns.get).toHaveBeenCalledTimes(1)
    })

    it("switches title and body together when changing locale on the details page", async () => {
        renderAt(detailsPath)
        await screen.findAllByText("Hello there")

        // The picker shows the locale's label once the project's locales have loaded.
        const localePicker = (await screen.findByText("English")).closest("button")
        expect(localePicker).not.toBeNull()
        await userEvent.click(localePicker!)
        await userEvent.click(await screen.findByRole("option", { name: "Dutch" }))

        expect(await screen.findAllByText("Hallo daar")).not.toHaveLength(0)
        expect(screen.getAllByText("Fijn dat je er bent")).not.toHaveLength(0)
        expect(screen.queryByText("Hello there")).toBeNull()
        expect(screen.queryByText("Glad you joined")).toBeNull()
    })

    it("shows why a save failed and stays on the step", async () => {
        vi.mocked(api.campaigns.templates.update).mockRejectedValue(
            new ApiError({ detail: "title is too long" }),
        )

        const router = renderAt(contentPath)
        await screen.findAllByText("Hello there")

        const next = screen.getByRole("button", { name: "next" })
        await userEvent.click(next)

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("title is too long"))
        // Shown once: the mutation reports it, and the step doesn't repeat it.
        expect(toastError).toHaveBeenCalledTimes(1)
        expect(router.state.location.pathname).toBe(contentPath)
        expect(cachedCampaign()?.templates).toEqual(campaign.templates)
        // The button is usable again, so the user can fix the content and retry.
        await waitFor(() =>
            expect(screen.getByRole<HTMLButtonElement>("button", { name: "next" }).disabled).toBe(
                false,
            ),
        )
    })
    it("keeps the footer's locale picker on the current locale when switching is blocked", async () => {
        vi.mocked(api.campaigns.templates.update).mockRejectedValue(
            new ApiError({ detail: "title is too long" }),
        )

        const router = renderAt(contentPath)
        await screen.findAllByText("Hello there")
        await chooseFooterLocale("Dutch")

        await waitFor(() => expect(toastError).toHaveBeenCalledWith("title is too long"))
        expect(router.state.location.pathname).toBe(contentPath)
        expect(footerLocalePicker()?.textContent).toContain("English")
    })

    it("switches the template and the footer's locale picker together", async () => {
        vi.mocked(api.campaigns.templates.update).mockResolvedValue(english)

        const router = renderAt(contentPath)
        await screen.findAllByText("Hello there")
        await chooseFooterLocale("Dutch")

        await waitFor(() =>
            expect(router.state.location.pathname).toBe(`${detailsPath}/templates/t-nl`),
        )
        await waitFor(() => expect(footerLocalePicker()?.textContent).toContain("Dutch"))
    })
})
