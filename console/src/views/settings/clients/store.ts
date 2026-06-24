// Data layer for the API & Clients interface, backed by the management
// auth-methods API. The list and detail routes share these helpers; the editor
// maps the view model with the adapters in ./model.

import { useCallback } from "react"
import api from "@/api"
import { useResolver } from "@/hooks"
import type { UUID } from "@/types/common"
import {
    authMethodToClient,
    clientToCreateParams,
    clientToUpdateParams,
    type Client,
} from "./model"

// useClients loads every auth method for the project as clients, with a reload
// callback for refreshing after a mutation.
export function useClients(projectId: UUID) {
    const [result, , reload, loading] = useResolver(
        useCallback(() => api.authMethods.search(projectId, { limit: 100 }), [projectId]),
    )
    const clients = (result?.results ?? []).map(authMethodToClient)
    return { clients, loading, reload }
}

// fetchClient loads a single client for the editor.
export async function fetchClient(projectId: UUID, id: UUID): Promise<Client> {
    return authMethodToClient(await api.authMethods.get(projectId, id))
}

// createClient creates a new auth method and returns it alongside the one-time
// secret, which is present only for api_key methods.
export async function createClient(
    projectId: UUID,
    draft: Client,
): Promise<{ client: Client; secret: string | null }> {
    const created = await api.authMethods.create(projectId, clientToCreateParams(draft))
    return { client: authMethodToClient(created), secret: created.secret ?? null }
}

// updateClient persists the mutable fields of an existing client (name,
// description, permissions, data scope).
export async function updateClient(projectId: UUID, draft: Client): Promise<void> {
    await api.authMethods.update(projectId, draft.id, clientToUpdateParams(draft))
}

export async function removeClient(projectId: UUID, id: UUID): Promise<void> {
    await api.authMethods.delete(projectId, id)
}
