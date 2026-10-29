import { useContext, useMemo } from "react"
import { Outlet, useParams } from "react-router"
import { useQueryClient, useSuspenseQuery } from "@tanstack/react-query"
import { CampaignContext, ProjectContext } from "@/contexts"
import type { Campaign as CampaignType, UseStateContext } from "@/types"
import type { UUID } from "@/types/common"
import { campaignQuery } from "./queries"

// Campaign provides the cached campaign to every page below it. The route
// loader has already put it in the cache, so this never suspends on first load.
export default function Campaign() {
    const [project] = useContext(ProjectContext)
    const { entityId } = useParams()
    const queryClient = useQueryClient()
    const query = campaignQuery(project.id, entityId as UUID)
    const { data: campaign } = useSuspenseQuery(query)
    const { queryKey } = query

    const value = useMemo<UseStateContext<CampaignType>>(
        () => [
            campaign,
            (next) =>
                queryClient.setQueryData<CampaignType>(queryKey, (prev) =>
                    prev && typeof next === "function" ? next(prev) : (next as CampaignType),
                ),
        ],
        // queryKey is rebuilt every render; the ids it is made of are stable.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [campaign, queryClient, project.id, entityId],
    )

    return (
        <CampaignContext.Provider value={value}>
            <Outlet />
        </CampaignContext.Provider>
    )
}
