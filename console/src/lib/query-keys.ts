import type { SearchParams } from "@/types"
import type { UUID } from "@/types/common"

/**
 * Centralised TanStack Query key factory.
 *
 * Keeping every key in one place gives us a single source of truth for cache
 * invalidation: a mutation can invalidate `queryKeys.users.list(projectId)` to
 * refresh every paginated/filtered users query, or
 * `queryKeys.users.all(projectId)` to refresh lists *and* detail views at once.
 *
 * Convention: keys are hierarchical, from least to most specific —
 *   [entity] → [entity, projectId] → [entity, projectId, "list", params]
 * so a prefix match (how `invalidateQueries` works) cascades naturally.
 */
export const queryKeys = {
    users: {
        all: (projectId: UUID) => ["users", projectId] as const,
        list: (projectId: UUID, params?: Partial<SearchParams>) =>
            ["users", projectId, "list", params ?? {}] as const,
        detail: (projectId: UUID, userId: UUID) => ["users", projectId, "detail", userId] as const,
    },
} as const
