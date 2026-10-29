import { useCallback } from "react"
import api from "@/api"
import { useResolver } from "@/hooks"
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

export function useProjectLocales(projectId: UUID | undefined): Locale[] {
    const [locales] = useResolver(
        useCallback(
            async () => (projectId ? await fetchProjectLocales(projectId) : []),
            [projectId],
        ),
    )
    return locales ?? []
}
