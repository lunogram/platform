import { useContext, useMemo } from "react"
import { useParams } from "react-router"
import { useQueryClient, useSuspenseQuery } from "@tanstack/react-query"
import { OrganizationContext, ProjectContext } from "@/contexts"
import type { Organization as OrganizationType } from "@/oapi/client"
import type { UseStateContext } from "@/types"
import type { UUID } from "@/types/common"
import OrganizationDetail from "./OrganizationDetail"
import { organizationQuery } from "./queries"

// Organization provides the cached organization to the detail page and its
// tabs. The route loader has already put it in the cache, so this never
// suspends on first load.
export default function Organization() {
    const [project] = useContext(ProjectContext)
    const { organizationId } = useParams()
    const queryClient = useQueryClient()
    const query = organizationQuery(project.id, organizationId as UUID)
    const { data: organization } = useSuspenseQuery(query)
    const { queryKey } = query

    const value = useMemo<UseStateContext<OrganizationType>>(
        () => [
            organization,
            (next) =>
                queryClient.setQueryData<OrganizationType>(queryKey, (prev) =>
                    prev && typeof next === "function" ? next(prev) : (next as OrganizationType),
                ),
        ],
        // queryKey is rebuilt every render; the ids it is made of are stable.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [organization, queryClient, project.id, organizationId],
    )

    return (
        <OrganizationContext.Provider value={value}>
            {/* Keyed so that moving to another organization starts its tabs
                afresh instead of keeping the previous one's page, search and edits. */}
            <OrganizationDetail key={organization.id} />
        </OrganizationContext.Provider>
    )
}
