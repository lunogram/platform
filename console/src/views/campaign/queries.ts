import { queryOptions, useMutation, useQueryClient } from "@tanstack/react-query"
import { useContext } from "react"
import { t } from "i18next"
import api from "@/api"
import { CampaignContext, ProjectContext } from "@/contexts"
import type { Campaign, Template } from "@/types"
import type { UUID } from "@/types/common"

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
        queryFn: () => api.campaigns.get(projectId, campaignId),
    })
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

    return {
        projectId: project.id,
        campaignId: campaign.id,
        setCampaign: (campaign: Campaign) => queryClient.setQueryData(queryKey, campaign),
        updateTemplates,
    }
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
    const { projectId, campaignId, updateTemplates } = useCampaignCache()
    return useMutation({
        mutationFn: (params: TemplateCreateParams) =>
            api.campaigns.templates.create(projectId, campaignId, params),
        onSuccess: (template) => updateTemplates((templates) => [...templates, template]),
        meta: {
            errorMessage: t("campaign.template.create_failed", "Couldn't create the template"),
        },
    })
}

export function useUpdateTemplate() {
    const { projectId, campaignId, updateTemplates } = useCampaignCache()
    return useMutation({
        mutationFn: ({ templateId, params }: { templateId: UUID; params: TemplateUpdateParams }) =>
            api.campaigns.templates.update(projectId, campaignId, templateId, params),
        onSuccess: (updated) =>
            updateTemplates((templates) =>
                templates.map((template) => (template.id === updated.id ? updated : template)),
            ),
        meta: { errorMessage: t("campaign.template.save_failed", "Couldn't save the template") },
    })
}
