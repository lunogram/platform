import type { ReactNode } from "react"
import { QueryClientProvider } from "@tanstack/react-query"
import { act, renderHook } from "@testing-library/react"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import api from "@/api"
import { CampaignContext, ProjectContext } from "@/contexts"
import { queryClient } from "@/lib/query"
import { ApiError } from "@/oapi/unwrap"
import type { Campaign, Project, Template } from "@/types"
import { campaignKeys, useCreateTemplate, useUpdateCampaign, useUpdateTemplate } from "./queries"

vi.mock("@/api", () => ({
    default: {
        campaigns: {
            update: vi.fn(),
            templates: { create: vi.fn(), update: vi.fn() },
        },
    },
}))

const toastError = vi.hoisted(() => vi.fn())
vi.mock("sonner", () => ({ toast: { error: toastError } }))

const project = { id: "project-1" } as Project
const english = {
    id: "template-en",
    type: "email",
    locale: "en",
    data: { subject: "Hi" },
} as unknown as Template
const dutch = {
    id: "template-nl",
    type: "email",
    locale: "nl",
    data: { subject: "Hoi" },
} as unknown as Template
const campaign = { id: "campaign-1", name: "Welcome", templates: [english, dutch] } as Campaign
const key = campaignKeys.detail(project.id, campaign.id)

function wrapper({ children }: { children: ReactNode }) {
    return (
        <QueryClientProvider client={queryClient}>
            <ProjectContext.Provider value={[project, () => {}]}>
                <CampaignContext.Provider value={[campaign, () => {}]}>
                    {children}
                </CampaignContext.Provider>
            </ProjectContext.Provider>
        </QueryClientProvider>
    )
}

const cached = () => queryClient.getQueryData<Campaign>(key)

describe("campaign mutations", () => {
    beforeEach(() => {
        queryClient.setQueryData(key, campaign)
    })

    afterEach(() => {
        queryClient.clear()
        vi.clearAllMocks()
    })

    it("writes a saved template into the cached campaign", async () => {
        const saved = { ...english, data: { subject: "Hello" } } as Template
        vi.mocked(api.campaigns.templates.update).mockResolvedValue(saved)

        const { result } = renderHook(() => useUpdateTemplate(), { wrapper })
        await act(() =>
            result.current.mutateAsync({
                templateId: english.id,
                params: { data: saved.data, sender_identity_id: null },
            }),
        )

        // Any page reading the campaign, such as the details page, now sees the save.
        expect(cached()?.templates).toEqual([saved, dutch])
    })

    it("adds a created template to the cached campaign", async () => {
        const german = {
            id: "template-de",
            type: "email",
            locale: "de",
            data: {},
        } as unknown as Template
        vi.mocked(api.campaigns.templates.create).mockResolvedValue(german)

        const { result } = renderHook(() => useCreateTemplate(), { wrapper })
        await act(() => result.current.mutateAsync({ locale: "de", data: german.data }))

        expect(cached()?.templates).toEqual([english, dutch, german])
    })

    it("replaces the cached campaign with the updated one", async () => {
        const renamed = { ...campaign, name: "Welcome back" }
        vi.mocked(api.campaigns.update).mockResolvedValue(renamed)

        const { result } = renderHook(() => useUpdateCampaign(), { wrapper })
        await act(() => result.current.mutateAsync({ name: "Welcome back" }))

        expect(cached()).toEqual(renamed)
    })

    it("shows a failed save and leaves the cache unchanged", async () => {
        vi.mocked(api.campaigns.templates.update).mockRejectedValue(
            new ApiError({ detail: "subject is required" }),
        )

        const { result } = renderHook(() => useUpdateTemplate(), { wrapper })
        await act(async () => {
            await expect(
                result.current.mutateAsync({
                    templateId: english.id,
                    params: { data: english.data, sender_identity_id: null },
                }),
            ).rejects.toThrow("subject is required")
        })

        expect(toastError).toHaveBeenCalledWith("subject is required")
        expect(cached()).toEqual(campaign)
    })
})
