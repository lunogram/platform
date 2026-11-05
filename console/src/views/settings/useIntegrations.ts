import { useContext, useMemo } from "react"
import { useTranslation } from "react-i18next"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import oapiClient from "@/oapi/client"
import type { Action, ActionMeta, Provider, ProviderMeta } from "@/oapi/client"
import { unwrap } from "@/oapi/unwrap"
import { ProjectContext } from "@/contexts"
import {
    actionKeys,
    actionsMetaQuery,
    actionsQuery,
    providerKeys,
    providersMetaQuery,
    providersQuery,
} from "./queries"

export type IntegrationRow =
    | {
          id: string
          kind: "provider"
          module: string
          name: string
          channels?: string[]
      }
    | {
          id: string
          kind: "action"
          module: string
          name: string
      }

export type IntegrationMeta = {
    kind: "provider" | "action"
    type: string
    name: string
    icon?: string
    color?: string
    locked?: boolean
}

/**
 * The project's providers and actions as one list, filtered by `search`.
 *
 * Every provider and action is loaded and searched here, so the list is never
 * cut off at a page size and a slow search can't overwrite a newer one.
 */
export function useIntegrations(search: string) {
    const [project] = useContext(ProjectContext)
    const providers = useQuery(providersQuery(project.id))
    const actions = useQuery(actionsQuery(project.id))
    const providerMetas = useQuery(providersMetaQuery(project.id))
    const actionMetas = useQuery(actionsMetaQuery(project.id))

    const metas = useMemo<IntegrationMeta[] | undefined>(() => {
        if (!providerMetas.data && !actionMetas.data) return undefined
        const providerOptions: IntegrationMeta[] = (providerMetas.data ?? []).map(
            (m: ProviderMeta) => ({
                kind: "provider",
                type: m.type,
                name: m.name,
                icon: m.icon,
                color: m.color,
                locked: m.locked,
            }),
        )

        const actionOptions: IntegrationMeta[] = (actionMetas.data ?? []).map((m: ActionMeta) => ({
            kind: "action",
            type: m.type,
            name: m.name,
            icon: m.icon,
            color: m.color,
            locked: false,
        }))

        return [...providerOptions, ...actionOptions]
    }, [providerMetas.data, actionMetas.data])

    const integrations = useMemo<IntegrationRow[]>(() => {
        if (!providers.data || !actions.data) return []

        const providerRows: IntegrationRow[] = providers.data.map((p: Provider) => ({
            id: p.id,
            kind: "provider",
            module: p.module,
            name: p.name,
            channels: p.channels,
        }))

        const actionRows: IntegrationRow[] = actions.data.map((a: Action) => ({
            id: a.id,
            kind: "action",
            module: a.type,
            name: a.name,
        }))

        const rows = [...providerRows, ...actionRows]
        if (!search) return rows

        const q = search.toLowerCase()
        return rows.filter((row) => {
            const byName = row.name.toLowerCase().includes(q)
            const byType = row.module.toLowerCase().includes(q)
            const byKind = row.kind.toLowerCase().includes(q)
            const byChannel =
                row.kind === "provider" && row.channels?.some((ch) => ch.toLowerCase().includes(q))
            return byName || byType || byKind || !!byChannel
        })
    }, [providers.data, actions.data, search])

    return {
        integrations,
        metas,
        isPending: providers.isPending || actions.isPending,
        isError: providers.isError || actions.isError,
        refetch: () => Promise.all([providers.refetch(), actions.refetch()]),
    }
}

/** Archives (deletes) a provider or an action. */
export function useArchiveIntegration() {
    const { t } = useTranslation()
    const [project] = useContext(ProjectContext)
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: async (integration: IntegrationRow) => {
            if (integration.kind === "action") {
                return unwrap(
                    await oapiClient.DELETE("/api/admin/projects/{projectID}/actions/{actionID}", {
                        params: { path: { projectID: project.id, actionID: integration.id } },
                    }),
                )
            }
            return unwrap(
                await oapiClient.DELETE(
                    "/api/admin/projects/{projectID}/providers/{type}/{providerID}",
                    {
                        params: {
                            path: {
                                projectID: project.id,
                                type: integration.module,
                                providerID: integration.id,
                            },
                        },
                    },
                ),
            )
        },
        onSuccess: (_, integration) =>
            queryClient.invalidateQueries({
                queryKey:
                    integration.kind === "action"
                        ? actionKeys.list(project.id)
                        : providerKeys.all(project.id),
            }),
        meta: {
            errorMessage: t("integration_archive_failed", "Couldn't archive the integration"),
        },
    })
}
