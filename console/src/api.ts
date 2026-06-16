import Axios from "axios"
import { env } from "./config/env"
import { oapiClient } from "./oapi/client"
import type {
    Action,
    ActionCreateParams,
    ActionUpdateParams,
    Admin,
    AuthDriver,
    Campaign,
    CampaignCreateParams,
    CampaignUpdateParams,
    CampaignUser,
    EmailTemplate,
    Image,
    Journey,
    JourneyEntranceDetail,
    JourneyStepMap,
    JourneyUserStep,
    List,
    ListCreateParams,
    ListUpdateParams,
    Locale,
    Project,
    ProjectAdmin,
    ProjectAdminInviteParams,
    ProjectAdminParams,
    ProjectApiKey,
    ProjectApiKeyParams,
    Resource,
    RulePath,
    SearchParams,
    SearchResult,
    UserSchemaPath,
    Subscription,
    SubscriptionCreateParams,
    SubscriptionParams,
    SubjectOrganization,
    SubjectOrganizationCreateParams,
    SubjectOrganizationMember,
    SubjectOrganizationMemberParams,
    SubjectOrganizationUpdateParams,
    SubscriptionUpdateParams,
    Tag,
    Template,
    TemplateCreateParams,
    TemplateUpdateParams,
    User,
    UserEvent,
    UserSubscription,
    VariableSuggestions,
} from "./types"
import type { UUID } from "@/types/common"

function appendValue(params: URLSearchParams, name: string, value: unknown) {
    if (typeof value === "undefined" || value === null || typeof value === "function") return
    if (typeof value === "object") value = JSON.stringify(value)
    params.append(name, value + "")
}

export const client = Axios.create({
    ...env.api,
    paramsSerializer: (params) => {
        const s = new URLSearchParams()
        for (const [name, value] of Object.entries(params)) {
            if (Array.isArray(value)) {
                for (const item of value) {
                    appendValue(s, name, item)
                }
            } else {
                appendValue(s, name, value)
            }
        }
        return s.toString()
    },
})

client.interceptors.response.use(
    (response) => response,
    async (error) => {
        const isLoginPage = window.location.pathname.startsWith("/login")
        if (error.response.status === 401 && !isLoginPage) {
            api.auth.login()
        }
        throw error
    },
)

export interface NetworkError {
    response: {
        data: unknown
        status: number
    }
}

type OmitFields = "id" | "created_at" | "updated_at" | "deleted_at" | "stats" | "stats_at"

export interface EntityApi<T> {
    basePath: string
    search: (params: Partial<SearchParams>) => Promise<SearchResult<T>>
    create: (params: Omit<T, OmitFields>) => Promise<T>
    get: (id: UUID | string) => Promise<T>
    update: (id: UUID | string, params: Omit<T, OmitFields>) => Promise<T>
    delete: (id: UUID | string) => Promise<number>
}

function createEntityPath<T>(basePath: string): EntityApi<T> {
    return {
        basePath,
        search: async (params) =>
            await client.get<SearchResult<T>>(basePath, { params }).then((r) => r.data),
        create: async (params) => await client.post<T>(basePath, params).then((r) => r.data),
        get: async (id) => await client.get<T>(`${basePath}/${id}`).then((r) => r.data),
        update: async (id, params) =>
            await client.patch<T>(`${basePath}/${id}`, params).then((r) => r.data),
        delete: async (id) => await client.delete<number>(`${basePath}/${id}`).then((r) => r.data),
    }
}

export interface ProjectEntityPath<T, C = Omit<T, OmitFields>, U = Omit<T, OmitFields>> {
    prefix: string
    search: (projectId: UUID, params: SearchParams) => Promise<SearchResult<T>>
    create: (projectId: UUID, params: C) => Promise<T>
    get: (projectId: UUID, id: UUID) => Promise<T>
    update: (projectId: UUID, id: UUID, params: U) => Promise<T>
    delete: (projectId: UUID, id: UUID) => Promise<number>
}

const projectUrl = (projectId: UUID) => `/admin/projects/${projectId}`
export const apiUrl = (projectId: UUID, path: string) =>
    `${env.api.baseURL}/admin/projects/${projectId}/${path}`

function createProjectEntityPath<T, C = Omit<T, OmitFields>, U = Omit<T, OmitFields>>(
    prefix: string,
): ProjectEntityPath<T, C, U> {
    return {
        prefix,
        search: async (projectId, params) =>
            await client
                .get<SearchResult<T>>(`${projectUrl(projectId)}/${prefix}`, { params })
                .then((r) => r.data),
        create: async (projectId, params) =>
            await client.post<T>(`${projectUrl(projectId)}/${prefix}`, params).then((r) => r.data),
        get: async (projectId, entityId) =>
            await client
                .get<T>(`${projectUrl(projectId)}/${prefix}/${entityId}`)
                .then((r) => r.data),
        update: async (projectId, entityId, params) =>
            await client
                .patch<T>(`${projectUrl(projectId)}/${prefix}/${entityId}`, params)
                .then((r) => r.data),
        delete: async (projectId, entityId) =>
            await client
                .delete<number>(`${projectUrl(projectId)}/${prefix}/${entityId}`)
                .then((r) => r.data),
    }
}

const cache: {
    profile: null | Admin
} = {
    profile: null,
}

const api = {
    auth: {
        methods: async () => await client.get<AuthDriver[]>("/auth/methods").then((r) => r.data),
        basicAuth: async (email: string, password: string) => {
            await client.post("/auth/login/basic/callback", { email, password })
        },
        clerkAuth: async (token: string) => {
            await client.post(
                "/auth/login/clerk/callback",
                {},
                { headers: { Authorization: `Bearer ${token}` } },
            )
        },
        login() {
            window.location.href = `/login?r=${encodeURIComponent(window.location.href)}`
        },
    },

    profile: {
        get: async () => {
            if (!cache.profile) {
                cache.profile = await client.get<Admin>("/admin/profile").then((r) => r.data)
            }
            return cache.profile!
        },
    },

    admins: {
        ...createEntityPath<Admin>("/admin/tenant/admins"),
        whoami: async () => await client.get<Admin>("/admin/tenant/whoami").then((r) => r.data),
    },

    projects: {
        ...createEntityPath<Project>("/admin/projects"),
        all: async () =>
            await client.get<SearchResult<Project>>("/admin/projects").then((r) => r.data),
        pathSuggestions: async (projectId: UUID) => {
            let eventSuggestions = await client
                .get<{
                    results: VariableSuggestions["eventPaths"]
                }>(`${projectUrl(projectId)}/subjects/user/events/schema`)
                .then((r) => r.data.results ?? [])

            eventSuggestions = eventSuggestions.map((event) => {
                event.schema ??= []
                return event
            })

            const userSuggestions = await client
                .get<{
                    results: UserSchemaPath[]
                }>(`${projectUrl(projectId)}/subjects/users/schema`)
                .then((r) => r.data.results ?? [])

            // Fetch organization event schema (with fallback to empty array)
            let organizationEventSuggestions: VariableSuggestions["eventPaths"] = []
            try {
                const orgEvents = await client
                    .get<{
                        results: VariableSuggestions["eventPaths"]
                    }>(`${projectUrl(projectId)}/subjects/organization/events/schema`)
                    .then((r) => r.data.results ?? [])

                organizationEventSuggestions = orgEvents.map((event) => {
                    event.schema ??= []
                    return event
                })
            } catch (error) {
                // Organization events endpoint may not exist yet, fallback to empty
                console.debug("Failed to fetch organization event schemas:", error)
            }

            // Fetch organization user (member) schema (with fallback to empty array)
            let organizationUserSuggestions: VariableSuggestions["organizationUserPaths"] = []
            try {
                organizationUserSuggestions = await client
                    .get<{
                        results: VariableSuggestions["organizationUserPaths"]
                    }>(`${projectUrl(projectId)}/subjects/organizations/users/schema`)
                    .then((r) => r.data.results ?? [])
            } catch (error) {
                // Organization user schema endpoint may not exist yet, fallback to empty
                console.debug("Failed to fetch organization user schemas:", error)
            }

            // Fetch organization schema (with fallback to empty array)
            let organizationSuggestions: VariableSuggestions["organizationPaths"] = []
            try {
                organizationSuggestions = await client
                    .get<{
                        results: VariableSuggestions["organizationPaths"]
                    }>(`${projectUrl(projectId)}/subjects/organizations/schema`)
                    .then((r) => r.data.results ?? [])
            } catch (error) {
                // Organization schema endpoint may not exist yet, fallback to empty
                console.debug("Failed to fetch organization schemas:", error)
            }

            // Fetch scheduled schemas (with fallback to empty array)
            let scheduledSuggestions: VariableSuggestions["scheduledPaths"] = []
            try {
                const { data } = await oapiClient.GET(
                    "/api/admin/projects/{projectID}/subjects/user/scheduled/schema",
                    { params: { path: { projectID: projectId } } },
                )
                const scheduled = data?.results ?? []

                scheduledSuggestions = scheduled.map((s) => ({
                    ...s,
                    schema: s.schema ?? [],
                })) as VariableSuggestions["scheduledPaths"]
            } catch (error) {
                // Scheduled schema endpoint may not exist yet, fallback to empty
                console.debug("Failed to fetch scheduled schemas:", error)
            }

            return {
                eventPaths: eventSuggestions,
                userPaths: userSuggestions,
                scheduledPaths: scheduledSuggestions,
                organizationEventPaths: organizationEventSuggestions,
                organizationUserPaths: organizationUserSuggestions,
                organizationPaths: organizationSuggestions,
            }
        },
    },

    data: {
        userPaths: {
            search: async (projectId: UUID, params: SearchParams) =>
                await client
                    .get<
                        SearchResult<RulePath>
                    >(`${projectUrl(projectId)}/data/paths/users`, { params })
                    .then((r) => r.data),
            update: async (projectId: UUID, entityId: UUID, params: Partial<RulePath>) =>
                await client
                    .put<RulePath>(`${projectUrl(projectId)}/data/paths/users/${entityId}`, params)
                    .then((r) => r.data),
        },
        rebuild: async (projectId: UUID) =>
            await client.post(`${projectUrl(projectId)}/data/paths/sync`).then((r) => r.data),
    },

    apiKeys: createProjectEntityPath<
        ProjectApiKey,
        ProjectApiKeyParams,
        Omit<ProjectApiKeyParams, "scope">
    >("keys"),

    actions: createProjectEntityPath<Action, ActionCreateParams, ActionUpdateParams>("actions"),

    campaigns: {
        ...createProjectEntityPath<Campaign, CampaignCreateParams, CampaignUpdateParams>(
            "campaigns",
        ),
        users: async (projectId: UUID, campaignId: UUID, params: SearchParams) =>
            await client
                .get<
                    SearchResult<CampaignUser>
                >(`${projectUrl(projectId)}/campaigns/${campaignId}/users`, { params })
                .then((r) => r.data),
        duplicate: async (projectId: UUID, campaignId: UUID) =>
            await client
                .post<Campaign>(`${projectUrl(projectId)}/campaigns/${campaignId}/duplicate`)
                .then((r) => r.data),
        templates: {
            search: async (projectId: UUID, campaignId: UUID, params: SearchParams) =>
                await client
                    .get<
                        SearchResult<Template>
                    >(`${projectUrl(projectId)}/campaigns/${campaignId}/templates`, { params })
                    .then((r) => r.data),
            create: async (projectId: UUID, campaignId: UUID, params: TemplateCreateParams) =>
                await client
                    .post<Template>(
                        `${projectUrl(projectId)}/campaigns/${campaignId}/templates`,
                        params,
                    )
                    .then((r) => r.data),
            get: async (projectId: UUID, campaignId: UUID, templateId: UUID) =>
                await client
                    .get<Template>(
                        `${projectUrl(projectId)}/campaigns/${campaignId}/templates/${templateId}`,
                    )
                    .then((r) => r.data),
            update: async (
                projectId: UUID,
                campaignId: UUID,
                templateId: UUID,
                params: TemplateUpdateParams,
            ) =>
                await client
                    .patch<Template>(
                        `${projectUrl(projectId)}/campaigns/${campaignId}/templates/${templateId}`,
                        params,
                    )
                    .then((r) => r.data),
            delete: async (projectId: UUID, campaignId: UUID, templateId: UUID) =>
                await client
                    .delete<number>(
                        `${projectUrl(projectId)}/campaigns/${campaignId}/templates/${templateId}`,
                    )
                    .then((r) => r.data),
            sendTest: async (
                projectId: UUID,
                campaignId: UUID,
                templateId: UUID,
                params: { to: string; props?: Record<string, unknown> },
            ) =>
                await client.post(
                    `${projectUrl(projectId)}/campaigns/${campaignId}/templates/${templateId}/test`,
                    params,
                ),
        },
    },

    journeys: {
        ...createProjectEntityPath<Journey>("journeys"),
        duplicate: async (projectId: UUID, journeyId: UUID) =>
            await client
                .post<Campaign>(`${projectUrl(projectId)}/journeys/${journeyId}/duplicate`)
                .then((r) => r.data),
        version: async (projectId: UUID, journeyId: UUID) =>
            await client
                .post<Journey>(`${projectUrl(projectId)}/journeys/${journeyId}/version`)
                .then((r) => r.data),
        publish: async (projectId: UUID, journeyId: UUID) =>
            await client
                .post<Journey>(`${projectUrl(projectId)}/journeys/${journeyId}/publish`)
                .then((r) => r.data),
        steps: {
            get: async (projectId: UUID, journeyId: UUID) =>
                await client
                    .get<JourneyStepMap>(`/admin/projects/${projectId}/journeys/${journeyId}/steps`)
                    .then((r) => r.data),
            set: async (projectId: UUID, journeyId: UUID, stepData: JourneyStepMap) =>
                await client
                    .put<JourneyStepMap>(
                        `/admin/projects/${projectId}/journeys/${journeyId}/steps`,
                        stepData,
                    )
                    .then((r) => r.data),
            searchUsers: async (
                projectId: UUID,
                journeyId: UUID,
                stepId: UUID,
                params: SearchParams,
            ) =>
                await client
                    .get<
                        SearchResult<JourneyUserStep>
                    >(`/admin/projects/${projectId}/journeys/${journeyId}/steps/${stepId}/users`, { params })
                    .then((r) => r.data),
        },
        entrances: {
            search: async (projectId: UUID, journeyId: UUID, params: SearchParams) =>
                await client
                    .get<
                        SearchResult<JourneyUserStep>
                    >(`/admin/projects/${projectId}/journeys/${journeyId}/entrances`, { params })
                    .then((r) => r.data),
            log: async (projectId: UUID, entranceId: UUID) =>
                await client
                    .get<JourneyEntranceDetail>(
                        `${projectUrl(projectId)}/journeys/entrances/${entranceId}`,
                    )
                    .then((r) => r.data),
        },
        users: {
            getState: async (projectId: UUID, journeyId: UUID, userId: UUID, entranceId?: UUID) => {
                const response = await client.get<
                    Array<{
                        external_step_id: string
                        step_type: string
                        is_completed: boolean
                    }>
                >(`${projectUrl(projectId)}/journeys/${journeyId}/users/${userId}/state`, {
                    params: entranceId ? { entrance_id: entranceId } : undefined,
                })
                return response.data
            },
            skipDelay: async (projectId: UUID, journeyId: UUID, userId: UUID, stepId: UUID) =>
                await client
                    .post<JourneyEntranceDetail>(
                        `${projectUrl(projectId)}/journeys/${journeyId}/users/${userId}/steps/${stepId}/resume`,
                    )
                    .then((r) => r.data),
            removeFromJourney: async (
                projectId: UUID,
                journeyId: UUID,
                userId: UUID,
                stepId: UUID,
            ) =>
                await client
                    .delete<number>(
                        `${projectUrl(projectId)}/journeys/${journeyId}/users/${userId}/step/${stepId}`,
                    )
                    .then((r) => r.data),
        },
    },

    users: {
        ...createProjectEntityPath<User>("subjects/users"),
        lists: async (projectId: UUID, userId: UUID, params: SearchParams) =>
            await client
                .get<
                    SearchResult<List>
                >(`${projectUrl(projectId)}/subjects/users/${userId}/lists`, { params })
                .then((r) => r.data),
        events: async (projectId: UUID, userId: UUID, params: SearchParams) =>
            await client
                .get<
                    SearchResult<UserEvent>
                >(`${projectUrl(projectId)}/subjects/users/${userId}/events`, { params })
                .then((r) => r.data),
        subscriptions: async (projectId: UUID, userId: UUID, params: SearchParams) =>
            await client
                .get<
                    SearchResult<UserSubscription>
                >(`${projectUrl(projectId)}/subjects/users/${userId}/subscriptions`, { params })
                .then((r) => r.data),
        updateSubscriptions: async (
            projectId: UUID,
            userId: UUID,
            subscriptions: SubscriptionParams[],
        ) =>
            await client
                .patch(
                    `${projectUrl(projectId)}/subjects/users/${userId}/subscriptions`,
                    subscriptions,
                )
                .then((r) => r.data),
        addImport: async (projectId: UUID, file: File) => {
            const formData = new FormData()
            formData.append("file", file)
            await client.post(`${projectUrl(projectId)}/subjects/users/import`, formData)
        },
        journeys: {
            search: async (projectId: UUID, userId: UUID, params: SearchParams) =>
                await client
                    .get<
                        SearchResult<JourneyUserStep>
                    >(`${projectUrl(projectId)}/subjects/users/${userId}/journeys`, { params })
                    .then((r) => r.data),
        },
    },

    organizations: {
        ...createProjectEntityPath<
            SubjectOrganization,
            SubjectOrganizationCreateParams,
            SubjectOrganizationUpdateParams
        >("subjects/organizations"),
        members: {
            search: async (projectId: UUID, organizationId: UUID, params: SearchParams) =>
                await client
                    .get<
                        SearchResult<SubjectOrganizationMember>
                    >(`${projectUrl(projectId)}/subjects/organizations/${organizationId}/members`, { params })
                    .then((r) => r.data),
            add: async (
                projectId: UUID,
                organizationId: UUID,
                params: SubjectOrganizationMemberParams,
            ) =>
                await client
                    .post(
                        `${projectUrl(projectId)}/subjects/organizations/${organizationId}/members`,
                        params,
                    )
                    .then((r) => r.data),
            remove: async (projectId: UUID, organizationId: UUID, userId: UUID) =>
                await client
                    .delete(
                        `${projectUrl(projectId)}/subjects/organizations/${organizationId}/members/${userId}`,
                    )
                    .then((r) => r.data),
        },
    },

    lists: {
        ...createProjectEntityPath<List, ListCreateParams, ListUpdateParams>("lists"),
        users: async (projectId: UUID, listId: UUID, params: SearchParams) =>
            await client
                .get<
                    SearchResult<User>
                >(`${projectUrl(projectId)}/lists/${listId}/users`, { params })
                .then((r) => r.data),
        preview: async (projectId: UUID, listId: UUID, params: { limit?: number } = {}) =>
            await client
                .get<
                    SearchResult<User>
                >(`${projectUrl(projectId)}/lists/${listId}/users/preview`, { params })
                .then((r) => r.data),
        upload: async (projectId: UUID, listId: UUID, file: File) => {
            const formData = new FormData()
            formData.append("file", file)
            await client.post(`${projectUrl(projectId)}/lists/${listId}/users`, formData)
        },
        duplicate: async (projectId: UUID, listId: UUID) =>
            await client
                .post<List>(`${projectUrl(projectId)}/lists/${listId}/duplicate`)
                .then((r) => r.data),
        recount: async (projectId: UUID, listId: UUID) =>
            await client
                .post<List>(`${projectUrl(projectId)}/lists/${listId}/recount`)
                .then((r) => r.data),
    },

    projectAdmins: {
        search: async (projectId: UUID, params: SearchParams) =>
            await client
                .get<SearchResult<ProjectAdmin>>(`${projectUrl(projectId)}/admins`, { params })
                .then((r) => r.data),
        add: async (projectId: UUID, adminId: UUID, params: ProjectAdminParams) =>
            await client
                .put<ProjectAdmin>(`${projectUrl(projectId)}/admins/${adminId}`, params)
                .then((r) => r.data),
        invite: async (projectId: UUID, params: ProjectAdminInviteParams) =>
            await client
                .post<ProjectAdmin>(`${projectUrl(projectId)}/admins`, params)
                .then((r) => r.data),
        get: async (projectId: UUID, adminId: UUID) =>
            await client
                .get<ProjectAdmin>(`${projectUrl(projectId)}/admins/${adminId}`)
                .then((r) => r.data),
        remove: async (projectId: UUID, adminId: UUID) =>
            await client.delete(`${projectUrl(projectId)}/admins/${adminId}`).then((r) => r.data),
    },

    subscriptions: createProjectEntityPath<
        Subscription,
        SubscriptionCreateParams,
        SubscriptionUpdateParams
    >("subscriptions"),

    images: {
        ...createProjectEntityPath<Image>("documents"),
        create: async (projectId: UUID, image: File) => {
            const formData = new FormData()
            formData.append("files", image)
            return await client
                .post(`${projectUrl(projectId)}/documents`, formData)
                .then((r) => r.data)
        },
        get: async (projectId: UUID, id: UUID) =>
            await client
                .get<Blob>(`${projectUrl(projectId)}/documents/${id}`, {
                    responseType: "blob",
                })
                .then((r) => r.data),
    },

    resources: {
        all: async (projectId: UUID, type: string = "font") =>
            await client
                .get<Resource[]>(`${projectUrl(projectId)}/resources?type=${type}`)
                .then((r) => r.data),
        create: async (projectId: UUID, params: Partial<Resource>) =>
            await client
                .post<Resource>(`${projectUrl(projectId)}/resources`, params)
                .then((r) => r.data),
        delete: async (projectId: UUID, id: UUID) =>
            await client
                .delete<number>(`${projectUrl(projectId)}/resources/${id}`)
                .then((r) => r.data),
    },

    tags: {
        ...createProjectEntityPath<Tag>("tags"),
        used: async (projectId: UUID, entity: string) =>
            await client
                .get<Tag[]>(`${projectUrl(projectId)}/tags/used/${entity}`)
                .then((r) => r.data),
        assign: async (projectId: UUID, entity: string, entityId: UUID, tags: string[]) =>
            await client
                .put<string[]>(`${projectUrl(projectId)}/tags/assign`, { entity, entityId, tags })
                .then((r) => r.data),
        all: async (projectId: UUID) =>
            await client.get<Tag[]>(`${projectUrl(projectId)}/tags`).then((r) => r.data),
    },

    locales: {
        ...createProjectEntityPath<Locale>("locales"),
        getByKey: async (projectId: UUID, code: string) =>
            await client
                .get<Locale>(`${projectUrl(projectId)}/locales/${code}`)
                .then((r) => r.data),
    },

    emailTemplates: {
        search: async (projectId: UUID, params?: Partial<SearchParams>) =>
            await client
                .get<SearchResult<EmailTemplate>>(`${projectUrl(projectId)}/email/templates`, {
                    params,
                })
                .then((r) => r.data),
    },
}

export default api

declare global {
    interface Window {
        API: typeof api
    }
}

window.API = api
