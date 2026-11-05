import { queryOptions, useQuery } from "@tanstack/react-query"
import api from "@/api"
import type { Locale } from "@/types"
import type { UUID } from "@/types/common"

// The API caps list requests at 100 items, so this is every locale a
// project can realistically have. Use it everywhere locales are listed so
// pickers never disagree about which locales exist.
export const PROJECT_LOCALES_LIMIT = 100

export async function fetchProjectLocales(projectId: UUID): Promise<Locale[]> {
    const result = await api.locales.search(projectId, { limit: PROJECT_LOCALES_LIMIT })
    return result.results ?? []
}

export function projectLocalesQuery(projectId: UUID) {
    return queryOptions({
        queryKey: ["projects", projectId, "locales"],
        queryFn: () => fetchProjectLocales(projectId),
    })
}

/** @returns the project's locales, shared by every caller through the query cache. */
export function useProjectLocales(projectId: UUID): Locale[] {
    const { data } = useQuery(projectLocalesQuery(projectId))
    return data ?? []
}
