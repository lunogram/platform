import { useCallback, useContext } from "react"
import { JourneyContext, ProjectContext } from "../../contexts"
import { SearchTable, useSearchTableQueryState } from "@/components/search-table"
import oapiClient from "@/oapi/client"

export default function JourneyUserEntrances() {
    const [project] = useContext(ProjectContext)
    const [journey] = useContext(JourneyContext)

    const projectId = project.id
    const journeyId = journey.id

    const state = useSearchTableQueryState(
        useCallback(
            async (params) => {
                const res = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/journeys/{journeyID}/entrances",
                    {
                        params: {
                            path: {
                                projectID: projectId,
                                journeyID: journeyId,
                            },
                            query: {
                                limit: params.limit,
                                offset: params.cursor ? +params.cursor : 0,
                            },
                        },
                    },
                )
                if (!res.data) return null
                const { results, limit, offset, total } = res.data
                return {
                    results,
                    limit,
                    nextCursor:
                        typeof total === "number" &&
                        typeof limit === "number" &&
                        typeof offset === "number" &&
                        offset + results.length < total
                            ? String(offset + results.length)
                            : "",
                }
            },
            [projectId, journeyId],
        ),
    )

    return (
        <SearchTable
            {...state}
            columns={[
                {
                    key: "user.external_id",
                },
            ]}
        />
    )
}
