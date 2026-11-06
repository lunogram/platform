import { useContext, useMemo } from "react"
import { useParams } from "react-router"
import { useQueryClient, useSuspenseQuery } from "@tanstack/react-query"
import { ProjectContext, UserContext } from "@/contexts"
import type { User, UseStateContext } from "@/types"
import type { UUID } from "@/types/common"
import UserDetail from "./UserDetail"
import { userQuery } from "./queries"

// UserRoute provides the cached user to the user page and its tabs, so a save
// on one tab is seen by the others. The route loader has already put the user
// in the cache, so this never suspends on first load.
export default function UserRoute() {
    const [project] = useContext(ProjectContext)
    const { entityId } = useParams()
    const queryClient = useQueryClient()
    const query = userQuery(project.id, entityId as UUID)
    const { data: user } = useSuspenseQuery(query)
    const { queryKey } = query

    const value = useMemo<UseStateContext<User>>(
        () => [
            user,
            (next) =>
                queryClient.setQueryData<User>(queryKey, (prev) =>
                    prev && typeof next === "function" ? next(prev) : (next as User),
                ),
        ],
        // queryKey is rebuilt every render; the ids it is made of are stable.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [user, queryClient, project.id, entityId],
    )

    return (
        <UserContext.Provider value={value}>
            {/* A different user starts with fresh page state (drafts, search, paging). */}
            <UserDetail key={entityId} />
        </UserContext.Provider>
    )
}
