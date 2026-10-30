import { queryOptions, useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query"
import { t } from "i18next"
import { oapiClient, type Provider } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"

type SenderChannel = "email" | "sms"

export const senderIdentityKeys = {
    all: (projectId: string) => ["projects", projectId, "sender-identities"] as const,
    list: (projectId: string, filter: { channel: SenderChannel; providerId?: string }) =>
        [...senderIdentityKeys.all(projectId), "list", filter] as const,
    detail: (projectId: string, senderIdentityId: string) =>
        [...senderIdentityKeys.all(projectId), "detail", senderIdentityId] as const,
}

/** The project's sender identities for a channel, optionally of one provider. */
export function senderIdentitiesQuery(
    projectId: string,
    { channel, providerId }: { channel: SenderChannel; providerId?: string },
) {
    return queryOptions({
        queryKey: senderIdentityKeys.list(projectId, { channel, providerId }),
        queryFn: async () =>
            unwrap(
                await oapiClient.GET("/api/admin/projects/{projectID}/sender-identities", {
                    params: {
                        path: { projectID: projectId },
                        query: providerId ? { provider_id: providerId, channel } : { channel },
                    },
                }),
            ).results ?? [],
    })
}

/** One sender identity, shared by every row that shows it. */
export function senderIdentityQuery(projectId: string, senderIdentityId: string) {
    return queryOptions({
        queryKey: senderIdentityKeys.detail(projectId, senderIdentityId),
        queryFn: async () =>
            unwrap(
                await oapiClient.GET(
                    "/api/admin/projects/{projectID}/sender-identities/{senderIdentityID}",
                    {
                        params: {
                            path: { projectID: projectId, senderIdentityID: senderIdentityId },
                        },
                    },
                ),
            ),
    })
}

/**
 * The project's providers (integrations) that can send on a channel.
 *
 * The cache entry holds every provider of the project: the settings pages read
 * the same key, so it must never hold just the first page.
 */
export function channelProvidersQuery(projectId: string, channel: SenderChannel) {
    return queryOptions({
        queryKey: ["projects", projectId, "providers", "list"] as const,
        queryFn: async () => {
            const providers: Provider[] = []
            for (;;) {
                const page = unwrap(
                    await oapiClient.GET("/api/admin/projects/{projectID}/providers", {
                        params: {
                            path: { projectID: projectId },
                            // The API's maximum page size.
                            query: { limit: 100, offset: providers.length },
                        },
                    }),
                )
                const results = page.results ?? []
                providers.push(...results)
                if (results.length === 0 || providers.length >= page.total) return providers
            }
        },
        select: (providers) => providers.filter((p) => p.channels?.includes(channel)),
    })
}

/** Marks every cached sender identity list of the project as outdated. */
export function invalidateSenderIdentities(queryClient: QueryClient, projectId: string) {
    return queryClient.invalidateQueries({ queryKey: senderIdentityKeys.all(projectId) })
}

export function useDeleteSenderIdentity(projectId: string) {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (senderIdentityId: string) =>
            unwrap(
                await oapiClient.DELETE(
                    "/api/admin/projects/{projectID}/sender-identities/{senderIdentityID}",
                    {
                        params: {
                            path: { projectID: projectId, senderIdentityID: senderIdentityId },
                        },
                    },
                ),
            ),
        onSuccess: () => invalidateSenderIdentities(queryClient, projectId),
        meta: {
            errorMessage: t("sender_identity_delete_failed", "Couldn't remove the address."),
        },
    })
}
