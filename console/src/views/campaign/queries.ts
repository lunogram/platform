import {
    keepPreviousData,
    queryOptions,
    useMutation,
    useQueryClient,
    type QueryClient,
} from "@tanstack/react-query"
import { useContext } from "react"
import { t } from "i18next"
import api from "@/api"
import { CampaignContext, ProjectContext } from "@/contexts"
import { oapiClient } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import type { Campaign, Template } from "@/types"
import type { UUID } from "@/types/common"
import { warnUnknownTemplateData } from "@/validation/campaign/template/data"

type CampaignUpdateParams = Parameters<typeof api.campaigns.update>[2]
type TemplateCreateParams = Parameters<typeof api.campaigns.templates.create>[2]
type TemplateUpdateParams = Parameters<typeof api.campaigns.templates.update>[3]

export const campaignKeys = {
    all: (projectId: UUID) => ["projects", projectId, "campaigns"] as const,
    detail: (projectId: UUID, campaignId: UUID) =>
        [...campaignKeys.all(projectId), campaignId] as const,
}

/**
 * campaignQuery loads a campaign together with all of its templates.
 *
 * It is the only copy of template data in the campaign editor: pages read
 * templates from it, and every mutation below writes the server's response
 * back into it, so a save on one page is seen on every other page.
 */
export function campaignQuery(projectId: UUID, campaignId: UUID) {
    return queryOptions({
        queryKey: campaignKeys.detail(projectId, campaignId),
        queryFn: async () => {
            const campaign = await api.campaigns.get(projectId, campaignId)
            campaign.templates.forEach(warnUnknownTemplateData)
            return campaign
        },
    })
}

/** The project's subscriptions; one cached request shared by every campaign page. */
export function subscriptionsQuery(projectId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "subscriptions"],
        queryFn: async () =>
            (await api.subscriptions.search(projectId, { limit: 100 })).results ?? [],
    })
}

/** Marks every cached campaigns list of the project as outdated. */
export function invalidateCampaignLists(queryClient: QueryClient, projectId: UUID) {
    return queryClient.invalidateQueries({ queryKey: [...campaignKeys.all(projectId), "list"] })
}

export const CAMPAIGN_PAGE_SIZE = 25

interface CampaignListParams {
    offset: number
    search: string
    /** The archived view also lists deleted campaigns. */
    archived: boolean
}

/** One page of the campaigns list. */
export function campaignListQuery(
    projectId: UUID,
    { offset, search, archived }: CampaignListParams,
) {
    return queryOptions({
        queryKey: [...campaignKeys.all(projectId), "list", { offset, search, archived }] as const,
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/campaigns", {
                    params: {
                        path: { projectID: projectId },
                        query: {
                            limit: CAMPAIGN_PAGE_SIZE,
                            offset,
                            search: search || undefined,
                            ...(archived ? { include_deleted: true } : {}),
                        },
                    },
                }),
            ),
        // Keep the current page on screen while the next one loads.
        placeholderData: keepPreviousData,
    })
}

// useCampaignListMutation runs a campaigns-list action, then refreshes every
// cached list (and campaign) of the project so the change shows everywhere.
function useCampaignListMutation<T>(
    request: (projectId: UUID, campaignId: UUID) => Promise<T>,
    errorMessage: string,
) {
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (campaignId: UUID) => request(project.id, campaignId),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: campaignKeys.all(project.id) }),
        meta: { errorMessage },
    })
}

export function useDuplicateCampaign() {
    return useCampaignListMutation(
        async (projectId, campaignId) =>
            unwrap(
                await oapiClient.POST(
                    "/api/admin/projects/{projectID}/campaigns/{campaignID}/duplicate",
                    { params: { path: { projectID: projectId, campaignID: campaignId } } },
                ),
            ),
        t("campaign.duplicate_failed", "Couldn't duplicate the campaign"),
    )
}

export function useArchiveCampaign() {
    return useCampaignListMutation(
        async (projectId, campaignId) =>
            unwrap(
                await oapiClient.DELETE("/api/admin/projects/{projectID}/campaigns/{campaignID}", {
                    params: { path: { projectID: projectId, campaignID: campaignId } },
                }),
            ),
        t("campaign.archive_failed", "Couldn't archive the campaign"),
    )
}

export function useUnarchiveCampaign() {
    return useCampaignListMutation(
        async (projectId, campaignId) =>
            unwrap(
                await oapiClient.POST(
                    "/api/admin/projects/{projectID}/campaigns/{campaignID}/unarchive",
                    { params: { path: { projectID: projectId, campaignID: campaignId } } },
                ),
            ),
        t("campaign.unarchive_failed", "Couldn't unarchive the campaign"),
    )
}

// useCampaignCache addresses the cached copy of the campaign in context.
function useCampaignCache() {
    const [project] = useContext(ProjectContext)
    const [campaign] = useContext(CampaignContext)
    const queryClient = useQueryClient()
    const queryKey = campaignKeys.detail(project.id, campaign.id)

    const updateTemplates = (update: (templates: Template[]) => Template[]) =>
        queryClient.setQueryData<Campaign>(queryKey, (prev) =>
            prev ? { ...prev, templates: update(prev.templates) } : prev,
        )

    const setCampaign = (campaign: Campaign) => {
        campaign.templates.forEach(warnUnknownTemplateData)
        queryClient.setQueryData(queryKey, campaign)
        // The list shows the campaign's name, so it is outdated now too.
        void invalidateCampaignLists(queryClient, project.id)
    }

    const setTemplate = (template: Template) => {
        warnUnknownTemplateData(template)
        updateTemplates((templates) =>
            templates.some((t) => t.id === template.id)
                ? templates.map((t) => (t.id === template.id ? template : t))
                : [...templates, template],
        )
    }

    return { projectId: project.id, campaignId: campaign.id, setCampaign, setTemplate }
}

export function useUpdateCampaign() {
    const { projectId, campaignId, setCampaign } = useCampaignCache()
    return useMutation({
        mutationFn: (params: CampaignUpdateParams) =>
            api.campaigns.update(projectId, campaignId, params),
        onSuccess: setCampaign,
        meta: { errorMessage: t("campaign.details.save_failed", "Couldn't save the campaign") },
    })
}

export function useCreateTemplate() {
    const { projectId, campaignId, setTemplate } = useCampaignCache()
    return useMutation({
        mutationFn: (params: TemplateCreateParams) =>
            api.campaigns.templates.create(projectId, campaignId, params),
        onSuccess: setTemplate,
        meta: {
            errorMessage: t("campaign.template.create_failed", "Couldn't create the template"),
        },
    })
}

export function useUpdateTemplate() {
    const { projectId, campaignId, setTemplate } = useCampaignCache()
    return useMutation({
        mutationFn: ({ templateId, params }: { templateId: UUID; params: TemplateUpdateParams }) =>
            api.campaigns.templates.update(projectId, campaignId, templateId, params),
        onSuccess: setTemplate,
        meta: { errorMessage: t("campaign.template.save_failed", "Couldn't save the template") },
    })
}
