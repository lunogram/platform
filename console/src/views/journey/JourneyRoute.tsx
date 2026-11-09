import { useContext, useMemo } from "react"
import { Outlet, useParams } from "react-router"
import { useQueryClient, useSuspenseQuery } from "@tanstack/react-query"
import { JourneyContext, ProjectContext } from "@/contexts"
import type { Journey, UseStateContext } from "@/types"
import type { UUID } from "@/types/common"
import { journeyQuery } from "./queries"

// JourneyRoute provides the cached journey to the pages below it. The route
// loader has already put it in the cache, so this never suspends on first load,
// and a save that writes the cache is seen here instead of a stale copy.
export default function JourneyRoute() {
    const [project] = useContext(ProjectContext)
    const { entityId } = useParams()
    const queryClient = useQueryClient()
    const query = journeyQuery(project.id, entityId as UUID)
    const { data: journey } = useSuspenseQuery(query)
    const { queryKey } = query

    const value = useMemo<UseStateContext<Journey>>(
        () => [
            journey,
            (next) =>
                queryClient.setQueryData<Journey>(queryKey, (prev) =>
                    prev && typeof next === "function" ? next(prev) : (next as Journey),
                ),
        ],
        // queryKey is rebuilt every render; the ids it is made of are stable.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [journey, queryClient, project.id, entityId],
    )

    return (
        <JourneyContext.Provider value={value}>
            <Outlet />
        </JourneyContext.Provider>
    )
}
