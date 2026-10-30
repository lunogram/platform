import { useContext, useMemo } from "react"
import { useParams } from "react-router"
import { useQueryClient, useSuspenseQuery } from "@tanstack/react-query"
import { ListContext, ProjectContext } from "@/contexts"
import type { List, UseStateContext } from "@/types"
import type { UUID } from "@/types/common"
import ListDetail from "./ListDetail"
import { listQuery } from "./listQueries"

// While a list is generating, poll it quickly at first and slower once it is
// under way, as the page did before.
function generatingRefetchInterval(list: List | undefined) {
    if (list?.state !== "loading") return false
    const complete = list.progress?.complete ?? 0
    const total = list.progress?.total ?? 0
    const percent = total > 0 ? (complete / total) * 100 : 0
    return percent < 5 ? 1000 : 5000
}

// ListRoute provides the cached list to the list page. The route loader has
// already put the list in the cache, so this never suspends on first load.
export default function ListRoute() {
    const [project] = useContext(ProjectContext)
    const { entityId } = useParams()
    const queryClient = useQueryClient()
    const query = listQuery(project.id, entityId as UUID)
    const { data: list } = useSuspenseQuery({
        ...query,
        refetchInterval: (q) => generatingRefetchInterval(q.state.data),
    })
    const { queryKey } = query

    const value = useMemo<UseStateContext<List>>(
        () => [
            list,
            (next) =>
                queryClient.setQueryData<List>(queryKey, (prev) =>
                    prev && typeof next === "function" ? next(prev) : (next as List),
                ),
        ],
        // queryKey is rebuilt every render; the ids it is made of are stable.
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [list, queryClient, project.id, entityId],
    )

    return (
        <ListContext.Provider value={value}>
            {/* A different list starts with fresh page state (rule draft, search, paging). */}
            <ListDetail key={entityId} />
        </ListContext.Provider>
    )
}
